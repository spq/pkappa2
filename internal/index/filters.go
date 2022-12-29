package index

import (
	"bufio"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"unsafe"

	"github.com/spq/pkappa2/internal/tools/bitmask"
)

type (
	Filter struct {
		executablePath   string
		name             string
		streams          chan *Stream
		cmd              *exec.Cmd
		cmdRunning       bool
		attachedTags     []string
		cachePath        string
		availableStreams bitmask.LongBitmask
	}
)

const (
	InvalidStreamID = ^uint64(0)
)

func (fltr *Filter) startFilterIfNeeded() {
	if fltr.IsRunning() {
		return
	}

	if len(fltr.attachedTags) == 0 {
		return
	}

	go fltr.startFilter()
}

// JSON Protocol
type (
	FilterStreamMetadata struct {
		ClientHost string
		ClientPort uint16
		ServerHost string
		ServerPort uint16
		Protocol   string
	}
	FilterStreamChunk struct {
		Direction string
		Content   string
	}
)

func New(executablePath string, name string, cachePath string) (*Filter, error) {
	filter := Filter{
		executablePath: executablePath,
		name:           name,
		streams:        make(chan *Stream, 100),
		cmd:            exec.Command(executablePath),
		cmdRunning:     false,
		cachePath:      cachePath,
	}

	if _, err := os.Stat(filter.CachePath()); err == nil {
		reader, err := filter.Reader()
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		for _, streamID := range reader.Streams() {
			filter.availableStreams.Set(uint(streamID))
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	return &filter, nil
}

func (filter *Filter) startFilter() {
	stdin, err := filter.cmd.StdinPipe()
	if err != nil {
		log.Printf("Filter (%s): Failed to create stdin pipe: %q", filter.name, err)
		return
	}
	stdout, err := filter.cmd.StdoutPipe()
	if err != nil {
		log.Printf("Filter (%s): Failed to create stdout pipe: %q", filter.name, err)
		return
	}
	stderr, err := filter.cmd.StderrPipe()
	if err != nil {
		log.Printf("Filter (%s): Failed to create stderr pipe: %q", filter.name, err)
		return
	}

	// Dump stderr directly
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Printf("Filter (%s) stderr: %s", filter.name, scanner.Text())
		}
	}()

	err = filter.cmd.Start()
	if err != nil {
		log.Printf("Filter (%s): Failed to start process: %q", filter.name, err)
		return
	}
	filter.cmdRunning = true
	defer filter.cmd.Process.Kill()
	defer filter.cmd.Wait()
	defer func() { filter.cmdRunning = false }()

	directionsToString := map[Direction]string{
		DirectionClientToServer: "client-to-server",
		DirectionServerToClient: "server-to-client",
	}
	directionsToInt := map[string]Direction{
		"client-to-server": DirectionClientToServer,
		"server-to-client": DirectionServerToClient,
	}

	invalidState := false
	stdinJson := json.NewEncoder(stdin)
	stdoutScanner := bufio.NewScanner(stdout)
outer:
	for stream := range filter.streams {
		if invalidState {
			continue
		}
		log.Printf("Filter (%s): Running for stream %d", filter.name, stream.StreamID)
		metadata := FilterStreamMetadata{
			ClientHost: stream.ClientHostIP(),
			ClientPort: stream.ClientPort,
			ServerHost: stream.ServerHostIP(),
			ServerPort: stream.ServerPort,
			Protocol:   stream.Protocol(),
		}

		// TODO: Start a timeout here, so that we don't wait forever for the filter to respond
		packets, err := stream.Data() // FIXME: Lock index reader
		if err != nil {
			log.Printf("Filter (%s): Failed to get packets: %q", filter.name, err)
			invalidState = true
			break
		}
		if err = stdinJson.Encode(metadata); err != nil {
			log.Printf("Filter (%s): Failed to send stream metadata: %q", filter.name, err)
			invalidState = true
			break
		}

		for _, packet := range packets {
			jsonPacket := FilterStreamChunk{
				Direction: directionsToString[packet.Direction],
				Content:   base64.StdEncoding.EncodeToString(packet.Content),
			}
			if err = stdinJson.Encode(jsonPacket); err != nil {
				// FIXME: Should we notify the filter about this somehow?
				log.Printf("Filter (%s): Failed to send packet: %q", filter.name, err)
				invalidState = true
				break outer
			}
		}

		if _, err := stdin.Write([]byte("\n")); err != nil {
			log.Printf("Filter (%s): Failed to send newline: %q", filter.name, err)
			invalidState = true
			break
		}

		var filteredPackets []Data
		var filteredMetadata FilterStreamMetadata
		for stdoutScanner.Scan() {
			filterLine := stdoutScanner.Text()
			if filterLine == "" {
				break
			}

			var filteredPacket FilterStreamChunk
			if err := json.Unmarshal([]byte(filterLine), &filteredPacket); err != nil {
				log.Printf("Filter (%s): Failed to read filtered packet: %q", filter.name, err)
				invalidState = true
				break outer
			}
			decodedData, err := base64.StdEncoding.DecodeString(filteredPacket.Content)
			if err != nil {
				log.Printf("Filter (%s): Failed to decode filtered packet data: %q", filter.name, err)
				invalidState = true
				break outer
			}

			direction, ok := directionsToInt[filteredPacket.Direction]
			if !ok {
				log.Printf("Filter (%s): Invalid direction: %q", filter.name, filteredPacket.Direction)
				invalidState = true
				break outer
			}
			filteredPackets = append(filteredPackets, Data{Content: decodedData, Direction: direction})
		}

		if !stdoutScanner.Scan() {
			log.Printf("Filter (%s): Failed to read filtered stream metadata: %q", filter.name, err)
			invalidState = true
			break
		}
		filterLine := stdoutScanner.Text()
		if err := json.Unmarshal([]byte(filterLine), &filteredMetadata); err != nil {
			log.Printf("Filter (%s): Failed to read filtered stream metadata: %q", filter.name, err)
			invalidState = true
			break
		}

		// log.Printf("Filter (%s): Filtered stream: %q", filter.name, filteredMetadata)
		// for _, filteredPacket := range filteredPackets {
		// 	log.Printf("Filter (%s): Filtered packet: %q", filter.name, filteredPacket)
		// }

		// TODO: Add processed results to the stream for use in queries
		// Persist processed results to disk
		filter.appendStream(stream, filteredPackets)
	}
}

func (filter *Filter) EnqueueStream(stream *Stream) {
	filter.streams <- stream
}

func (filter *Filter) AttachTag(tag string) {
	// TODO: Check if tag is already attached
	filter.attachedTags = append(filter.attachedTags, tag)
	filter.startFilterIfNeeded()
}

func (filter *Filter) DetachTag(tag string) error {
	for i, t := range filter.attachedTags {
		if t == tag {
			filter.attachedTags = append(filter.attachedTags[:i], filter.attachedTags[i+1:]...)
			break
		}
	}

	if len(filter.attachedTags) == 0 {
		if err := filter.KillProcess(); err != nil {
			return err
		}
		if err := filter.purgeCache(); err != nil {
			return err
		}
	}
	return nil
}

func (filter *Filter) IsRunning() bool {
	return filter.cmdRunning
}

func (filter *Filter) Name() string {
	return filter.name
}

func CachePath(cachePath string, filterName string) string {
	filename := fmt.Sprintf("filterindex-%s.fidx", filterName)
	return filepath.Join(cachePath, filename)
}
func (filter *Filter) CachePath() string {
	return CachePath(filter.cachePath, filter.name)
}
func (filter *Filter) purgeCache() error {
	return PurgeFilterCache(filter.cachePath, filter.name)
}
func PurgeFilterCache(indexDir string, filterName string) error {
	return os.Remove(CachePath(indexDir, filterName))
}

func (filter *Filter) appendStream(stream *Stream, filteredPackets []Data) error {
	writer, err := filter.Writer()
	if err != nil {
		return err
	}
	defer writer.Close()
	if filter.HasStream(stream.StreamID) {
		writer.InvalidateStream([]*Stream{stream})
	}
	if err := writer.AppendStream(stream, filteredPackets); err != nil {
		return err
	}
	filter.availableStreams.Set(uint(stream.StreamID))
	return nil
}

func (filter *Filter) KillProcess() error {
	if filter.cmd.Process != nil {
		if err := filter.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("error: could not kill filter %s: %q", filter.name, err)
		}
		filter.cmd.Process.Wait()
		filter.cmdRunning = false
	}
	close(filter.streams)
	return nil
}

func (filter *Filter) Reset() error {
	if err := filter.KillProcess(); err != nil {
		return err
	}

	// FIXME: Delay restart to avoid "text file busy" error
	filter.streams = make(chan *Stream, 100)
	filter.cmd = exec.Command(filter.executablePath)
	// filter.availableStreams = bitmask.LongBitmask()
	// filter.purgeCache()

	// Start the process
	filter.startFilterIfNeeded()
	return nil
}

func (filter *Filter) Data(streamID uint64) ([]Data, uint64, uint64, error) {
	reader, err := filter.Reader()
	if err != nil {
		return nil, 0, 0, err
	}
	defer reader.Close()
	return reader.ReadStream(streamID)
}

func (filter *Filter) DataForSearch(streamID uint64) ([2][]byte, [][2]int, uint64, uint64, error) {
	reader, err := filter.Reader()
	if err != nil {
		return [2][]byte{}, [][2]int{}, 0, 0, err
	}
	defer reader.Close()
	return reader.ReadStreamForSearch(streamID)
}

func (filter *Filter) HasStream(streamID uint64) bool {
	return filter.availableStreams.IsSet(uint(streamID))
}

// File format
type (
	filterStreamSection struct {
		StreamID uint64
		DataSize uint64
	}

	FilterWriter struct {
		filename string
		file     *os.File
		buffer   *bufio.Writer
	}

	FilterReader struct {
		filename           string
		file               *os.File
		containedStreamIds map[uint64]uint64
	}
)

func (filter *Filter) Writer() (*FilterWriter, error) {
	file, err := os.OpenFile(filter.CachePath(), os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	return &FilterWriter{
		filename: filter.CachePath(),
		file:     file,
		buffer:   bufio.NewWriter(file),
	}, nil
}

func (writer *FilterWriter) write(what interface{}) error {
	err := binary.Write(writer.buffer, binary.LittleEndian, what)
	if err != nil {
		debug.PrintStack()
	}
	return err
}

func (w *FilterWriter) Close() error {
	if err := w.buffer.Flush(); err != nil {
		return err
	}
	return w.file.Close()
}

func (writer *FilterWriter) AppendStream(stream *Stream, filteredPackets []Data) error {
	if _, err := writer.file.Seek(0, io.SeekEnd); err != nil {
		return err
	}

	// [u64 stream id] [u64 data size] [u8 varint chunk sizes] [client data] [server data]
	dataSize := uint64(0)
	for _, filteredPacket := range filteredPackets {
		dataSize += uint64(len(filteredPacket.Content))
	}

	segmentation := []byte(nil)
	buf := [10]byte{}
	for pIndex, wantDir := 0, DirectionClientToServer; pIndex < len(filteredPackets); {
		// TODO: Merge packets with the same direction. Do we even want to allow filters to change the direction?
		filteredPacket := filteredPackets[pIndex]
		sz := len(filteredPacket.Content)
		dir := filteredPacket.Direction
		// Write a length of 0 if the server sent the first packet.
		if dir != wantDir {
			segmentation = append(segmentation, 0)
			wantDir = wantDir.Reverse()
		}
		pos := len(buf)
		flag := byte(0)
		for {
			pos--
			buf[pos] = byte(sz&0x7f) | flag
			flag = 0x80
			sz >>= 7
			if sz == 0 {
				break
			}
		}
		segmentation = append(segmentation, buf[pos:]...)
		wantDir = wantDir.Reverse()
		pIndex++
	}
	// Append two lengths of 0 to indicate the end of the chunk sizes
	segmentation = append(segmentation, 0, 0)

	// Write stream section
	streamSection := filterStreamSection{
		StreamID: stream.ID(),
		DataSize: dataSize + uint64(len(segmentation)),
	}
	if err := writer.write(streamSection); err != nil {
		return err
	}
	if err := writer.write(segmentation); err != nil {
		return err
	}

	for _, direction := range []Direction{DirectionClientToServer, DirectionServerToClient} {
		for _, filteredPacket := range filteredPackets {
			if filteredPacket.Direction != direction {
				continue
			}
			if _, err := writer.buffer.Write(filteredPacket.Content); err != nil {
				return err
			}
		}
	}

	return nil
}

func (writer *FilterWriter) InvalidateStream(stream []*Stream) error {
	if err := writer.buffer.Flush(); err != nil {
		return err
	}
	if _, err := writer.file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	// Find stream in file and replace streamid with InvalidStreamID
	for {
		streamSection := filterStreamSection{}
		if err := binary.Read(writer.file, binary.LittleEndian, &streamSection); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		for i, s := range stream {
			if s.StreamID == streamSection.StreamID {
				streamSection.StreamID = InvalidStreamID
				if _, err := writer.file.Seek(-int64(unsafe.Sizeof(streamSection)), io.SeekCurrent); err != nil {
					return err
				}
				if err := binary.Write(writer.file, binary.LittleEndian, streamSection); err != nil {
					return err
				}
				stream = append(stream[:i], stream[i+1:]...)
				break
			}
		}
		if _, err := writer.file.Seek(int64(streamSection.DataSize), io.SeekCurrent); err != nil {
			return err
		}
	}
	return nil
}

func (filter *Filter) Reader() (*FilterReader, error) {
	file, err := os.Open(filter.CachePath())
	if err != nil {
		return nil, err
	}
	reader := &FilterReader{
		filename:           filter.CachePath(),
		file:               file,
		containedStreamIds: make(map[uint64]uint64),
	}

	buffer := bufio.NewReader(file)
	// Read all stream ids
	pos := uint64(0)
	for {
		streamSection := filterStreamSection{}
		if err := binary.Read(buffer, binary.LittleEndian, &streamSection); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		pos += uint64(unsafe.Sizeof(streamSection))
		if streamSection.StreamID != InvalidStreamID {
			reader.containedStreamIds[streamSection.StreamID] = pos
		}
		if _, err := buffer.Discard(int(streamSection.DataSize)); err != nil {
			return nil, err
		}
		pos += streamSection.DataSize
	}
	return reader, nil
}

func (r *FilterReader) Close() error {
	return r.file.Close()
}

func (r *FilterReader) HasStream(streamID uint64) bool {
	_, ok := r.containedStreamIds[streamID]
	return ok
}

func (r *FilterReader) Streams() []uint64 {
	keys := make([]uint64, len(r.containedStreamIds))
	i := 0
	for k := range r.containedStreamIds {
		keys[i] = k
	}
	return keys
}

func (reader *FilterReader) ReadStreamForSearch(streamID uint64) ([2][]byte, [][2]int, uint64, uint64, error) {
	// [u64 stream id] [u64 data size] [u8 varint chunk sizes] [client data] [server data]
	pos, ok := reader.containedStreamIds[streamID]
	if !ok {
		return [2][]byte{}, [][2]int{}, 0, 0, fmt.Errorf("stream %d not found in %s", streamID, reader.filename)
	}
	if _, err := reader.file.Seek(int64(pos), io.SeekStart); err != nil {
		return [2][]byte{}, [][2]int{}, 0, 0, err
	}

	buffer := bufio.NewReader(reader.file)

	// Read chunk sizes
	dataSizes := [][2]int{{}}
	prevWasZero := false
	direction := DirectionClientToServer
	clientBytes := uint64(0)
	serverBytes := uint64(0)
	for {
		last := dataSizes[len(dataSizes)-1]
		sz := uint64(0)
		for {
			b, err := buffer.ReadByte()
			if err != nil {
				return [2][]byte{}, [][2]int{}, 0, 0, err
			}
			sz <<= 7
			sz |= uint64(b & 0x7f)
			if b < 0x80 {
				break
			}
		}
		if sz == 0 {
			if prevWasZero {
				break
			} else {
				prevWasZero = true
				direction = direction.Reverse()
				continue
			}
		}
		new := [2]int{
			last[0],
			last[1],
		}
		new[direction] += int(sz)
		dataSizes = append(dataSizes, new)
		prevWasZero = false
		if direction == DirectionClientToServer {
			clientBytes += sz
		} else {
			serverBytes += sz
		}
		direction = direction.Reverse()
	}

	// Read data
	clientData := make([]byte, clientBytes)
	if _, err := io.ReadFull(buffer, clientData); err != nil {
		return [2][]byte{}, [][2]int{}, 0, 0, err
	}
	serverData := make([]byte, serverBytes)
	if _, err := io.ReadFull(buffer, serverData); err != nil {
		return [2][]byte{}, [][2]int{}, 0, 0, err
	}
	return [2][]byte{clientData, serverData}, dataSizes, clientBytes, serverBytes, nil
}

func (reader *FilterReader) ReadStream(streamID uint64) ([]Data, uint64, uint64, error) {
	// [u64 stream id] [u64 data size] [u8 varint chunk sizes] [client data] [server data]
	pos, ok := reader.containedStreamIds[streamID]
	if !ok {
		return nil, 0, 0, fmt.Errorf("stream %d not found in %s", streamID, reader.filename)
	}
	if _, err := reader.file.Seek(int64(pos), io.SeekStart); err != nil {
		return nil, 0, 0, err
	}

	buffer := bufio.NewReader(reader.file)
	data := []Data{}

	type sizeAndDirection struct {
		Size      uint64
		Direction Direction
	}
	// Read chunk sizes
	dataSizes := []sizeAndDirection{}
	prevWasZero := false
	direction := DirectionClientToServer
	clientBytes := uint64(0)
	serverBytes := uint64(0)
	for {
		sz := uint64(0)
		for {
			b, err := buffer.ReadByte()
			if err != nil {
				return nil, 0, 0, err
			}
			sz <<= 7
			sz |= uint64(b & 0x7f)
			if b < 0x80 {
				break
			}
		}
		if sz == 0 && prevWasZero {
			break
		}
		dataSizes = append(dataSizes, sizeAndDirection{Direction: direction, Size: sz})
		prevWasZero = sz == 0
		if direction == DirectionClientToServer {
			clientBytes += sz
		} else {
			serverBytes += sz
		}
		direction = direction.Reverse()
	}

	// Read data
	clientData := make([]byte, clientBytes)
	if _, err := io.ReadFull(buffer, clientData); err != nil {
		return nil, 0, 0, err
	}
	serverData := make([]byte, serverBytes)
	if _, err := io.ReadFull(buffer, serverData); err != nil {
		return nil, 0, 0, err
	}

	// Split data into chunks
	for _, ds := range dataSizes {
		if ds.Size == 0 {
			continue
		}
		var bytes []byte
		if ds.Direction == DirectionClientToServer {
			bytes = clientData[:ds.Size]
			clientData = clientData[ds.Size:]
		} else {
			bytes = serverData[:ds.Size]
			serverData = serverData[ds.Size:]
		}
		data = append(data, Data{
			Direction: ds.Direction,
			Content:   bytes,
		})
	}
	return data, clientBytes, serverBytes, nil
}
