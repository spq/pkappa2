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
	Converter struct {
		executablePath   string
		name             string
		streams          chan *Stream
		cmd              *exec.Cmd
		cmdRunning       bool
		attachedTags     []string
		cachePath        string
		availableStreams bitmask.LongBitmask
	}
	// JSON Protocol
	ConverterStreamMetadata struct {
		ClientHost string
		ClientPort uint16
		ServerHost string
		ServerPort uint16
		Protocol   string
	}
	ConverterStreamChunk struct {
		Direction string
		Content   string
	}
)

const (
	InvalidStreamID = ^uint64(0)
)

func NewConverter(executablePath string, name string, cachePath string) (*Converter, error) {
	converter := Converter{
		executablePath: executablePath,
		name:           name,
		streams:        make(chan *Stream, 100),
		cmd:            exec.Command(executablePath),
		cmdRunning:     false,
		cachePath:      cachePath,
	}

	if _, err := os.Stat(converter.CachePath()); err == nil {
		reader, err := converter.Reader()
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		for _, streamID := range reader.Streams() {
			converter.availableStreams.Set(uint(streamID))
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	return &converter, nil
}

func (converter *Converter) startProcessIfNeeded() {
	if converter.IsRunning() {
		return
	}

	if len(converter.attachedTags) == 0 {
		return
	}

	go converter.startConverter()
}

func (converter *Converter) startConverter() {
	stdin, err := converter.cmd.StdinPipe()
	if err != nil {
		log.Printf("Converter (%s): Failed to create stdin pipe: %q", converter.name, err)
		return
	}
	stdout, err := converter.cmd.StdoutPipe()
	if err != nil {
		log.Printf("Converter (%s): Failed to create stdout pipe: %q", converter.name, err)
		return
	}
	stderr, err := converter.cmd.StderrPipe()
	if err != nil {
		log.Printf("Converter (%s): Failed to create stderr pipe: %q", converter.name, err)
		return
	}

	// Dump stderr directly
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Printf("Converter (%s) stderr: %s", converter.name, scanner.Text())
		}
	}()

	err = converter.cmd.Start()
	if err != nil {
		log.Printf("Converter (%s): Failed to start process: %q", converter.name, err)
		return
	}
	converter.cmdRunning = true
	defer converter.cmd.Process.Kill()
	defer converter.cmd.Wait()
	defer func() { converter.cmdRunning = false }()

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
	for stream := range converter.streams {
		if invalidState {
			continue
		}
		log.Printf("Converter (%s): Running for stream %d", converter.name, stream.StreamID)
		metadata := ConverterStreamMetadata{
			ClientHost: stream.ClientHostIP(),
			ClientPort: stream.ClientPort,
			ServerHost: stream.ServerHostIP(),
			ServerPort: stream.ServerPort,
			Protocol:   stream.Protocol(),
		}

		// TODO: Start a timeout here, so that we don't wait forever for the converter to respond
		packets, err := stream.Data() // FIXME: Lock index reader
		if err != nil {
			log.Printf("Converter (%s): Failed to get packets: %q", converter.name, err)
			invalidState = true
			break
		}
		if err = stdinJson.Encode(metadata); err != nil {
			log.Printf("Converter (%s): Failed to send stream metadata: %q", converter.name, err)
			invalidState = true
			break
		}

		for _, packet := range packets {
			jsonPacket := ConverterStreamChunk{
				Direction: directionsToString[packet.Direction],
				Content:   base64.StdEncoding.EncodeToString(packet.Content),
			}
			if err = stdinJson.Encode(jsonPacket); err != nil {
				// FIXME: Should we notify the converter about this somehow?
				log.Printf("Converter (%s): Failed to send packet: %q", converter.name, err)
				invalidState = true
				break outer
			}
		}

		if _, err := stdin.Write([]byte("\n")); err != nil {
			log.Printf("Converter (%s): Failed to send newline: %q", converter.name, err)
			invalidState = true
			break
		}

		var convertedPackets []Data
		var convertedMetadata ConverterStreamMetadata
		for stdoutScanner.Scan() {
			converterLine := stdoutScanner.Text()
			if converterLine == "" {
				break
			}

			var convertedPacket ConverterStreamChunk
			if err := json.Unmarshal([]byte(converterLine), &convertedPacket); err != nil {
				log.Printf("Converter (%s): Failed to read converted packet: %q", converter.name, err)
				invalidState = true
				break outer
			}
			decodedData, err := base64.StdEncoding.DecodeString(convertedPacket.Content)
			if err != nil {
				log.Printf("Converter (%s): Failed to decode converted packet data: %q", converter.name, err)
				invalidState = true
				break outer
			}

			direction, ok := directionsToInt[convertedPacket.Direction]
			if !ok {
				log.Printf("Converter (%s): Invalid direction: %q", converter.name, convertedPacket.Direction)
				invalidState = true
				break outer
			}
			convertedPackets = append(convertedPackets, Data{Content: decodedData, Direction: direction})
		}

		if !stdoutScanner.Scan() {
			log.Printf("Converter (%s): Failed to read converted stream metadata: %q", converter.name, err)
			invalidState = true
			break
		}
		converterLine := stdoutScanner.Text()
		if err := json.Unmarshal([]byte(converterLine), &convertedMetadata); err != nil {
			log.Printf("Converter (%s): Failed to read converted stream metadata: %q", converter.name, err)
			invalidState = true
			break
		}

		// log.Printf("Converter (%s): Converted stream: %q", converter.name, convertedMetadata)
		// for _, convertedPacket := range convertedPackets {
		// 	log.Printf("Converter (%s): Converted packet: %q", converter.name, convertedPacket)
		// }

		// TODO: Add processed results to the stream for use in queries
		// Persist processed results to disk
		converter.appendStream(stream, convertedPackets)
	}
}

func (converter *Converter) EnqueueStream(stream *Stream) {
	converter.streams <- stream
}

func (converter *Converter) AttachTag(tag string) {
	// TODO: Check if tag is already attached
	converter.attachedTags = append(converter.attachedTags, tag)
	converter.startProcessIfNeeded()
}

func (converter *Converter) DetachTag(tag string) error {
	for i, t := range converter.attachedTags {
		if t == tag {
			converter.attachedTags = append(converter.attachedTags[:i], converter.attachedTags[i+1:]...)
			break
		}
	}

	if len(converter.attachedTags) == 0 {
		if err := converter.KillProcess(); err != nil {
			return err
		}
		if err := converter.purgeCache(); err != nil {
			return err
		}
	}
	return nil
}

func (converter *Converter) IsRunning() bool {
	return converter.cmdRunning
}

func (converter *Converter) Name() string {
	return converter.name
}

func CachePath(cachePath string, converterName string) string {
	filename := fmt.Sprintf("converterindex-%s.cidx", converterName)
	return filepath.Join(cachePath, filename)
}
func (converter *Converter) CachePath() string {
	return CachePath(converter.cachePath, converter.name)
}
func (converter *Converter) purgeCache() error {
	return PurgeConverterCache(converter.cachePath, converter.name)
}
func PurgeConverterCache(indexDir string, converterName string) error {
	return os.Remove(CachePath(indexDir, converterName))
}

func (converter *Converter) appendStream(stream *Stream, convertedPackets []Data) error {
	writer, err := converter.Writer()
	if err != nil {
		return err
	}
	defer writer.Close()
	if converter.HasStream(stream.StreamID) {
		writer.InvalidateStream([]*Stream{stream})
	}
	if err := writer.AppendStream(stream, convertedPackets); err != nil {
		return err
	}
	converter.availableStreams.Set(uint(stream.StreamID))
	return nil
}

func (converter *Converter) KillProcess() error {
	if converter.cmd.Process != nil {
		if err := converter.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("error: could not kill converter %s: %q", converter.name, err)
		}
		converter.cmd.Process.Wait()
		converter.cmdRunning = false
	}
	close(converter.streams)
	return nil
}

func (converter *Converter) Reset() error {
	if err := converter.KillProcess(); err != nil {
		return err
	}

	// FIXME: Delay restart to avoid "text file busy" error
	converter.streams = make(chan *Stream, 100)
	converter.cmd = exec.Command(converter.executablePath)
	// converter.availableStreams = bitmask.LongBitmask()
	// converter.purgeCache()

	converter.startProcessIfNeeded()
	return nil
}

func (converter *Converter) Data(streamID uint64) ([]Data, uint64, uint64, error) {
	reader, err := converter.Reader()
	if err != nil {
		return nil, 0, 0, err
	}
	defer reader.Close()
	return reader.ReadStream(streamID)
}

func (converter *Converter) DataForSearch(streamID uint64) ([2][]byte, [][2]int, uint64, uint64, error) {
	reader, err := converter.Reader()
	if err != nil {
		return [2][]byte{}, [][2]int{}, 0, 0, err
	}
	defer reader.Close()
	return reader.ReadStreamForSearch(streamID)
}

func (converter *Converter) HasStream(streamID uint64) bool {
	return converter.availableStreams.IsSet(uint(streamID))
}

// File format
type (
	converterStreamSection struct {
		StreamID uint64
		DataSize uint64
	}

	ConverterWriter struct {
		filename string
		file     *os.File
		buffer   *bufio.Writer
	}

	ConverterReader struct {
		filename           string
		file               *os.File
		containedStreamIds map[uint64]uint64
	}
)

func (converter *Converter) Writer() (*ConverterWriter, error) {
	file, err := os.OpenFile(converter.CachePath(), os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	return &ConverterWriter{
		filename: converter.CachePath(),
		file:     file,
		buffer:   bufio.NewWriter(file),
	}, nil
}

func (writer *ConverterWriter) write(what interface{}) error {
	err := binary.Write(writer.buffer, binary.LittleEndian, what)
	if err != nil {
		debug.PrintStack()
	}
	return err
}

func (w *ConverterWriter) Close() error {
	if err := w.buffer.Flush(); err != nil {
		return err
	}
	return w.file.Close()
}

func (writer *ConverterWriter) AppendStream(stream *Stream, convertedPackets []Data) error {
	if _, err := writer.file.Seek(0, io.SeekEnd); err != nil {
		return err
	}

	// [u64 stream id] [u64 data size] [u8 varint chunk sizes] [client data] [server data]
	dataSize := uint64(0)
	for _, convertedPacket := range convertedPackets {
		dataSize += uint64(len(convertedPacket.Content))
	}

	segmentation := []byte(nil)
	buf := [10]byte{}
	for pIndex, wantDir := 0, DirectionClientToServer; pIndex < len(convertedPackets); {
		// TODO: Merge packets with the same direction. Do we even want to allow converters to change the direction?
		convertedPacket := convertedPackets[pIndex]
		sz := len(convertedPacket.Content)
		dir := convertedPacket.Direction
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
	streamSection := converterStreamSection{
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
		for _, convertedPacket := range convertedPackets {
			if convertedPacket.Direction != direction {
				continue
			}
			if _, err := writer.buffer.Write(convertedPacket.Content); err != nil {
				return err
			}
		}
	}

	return nil
}

func (writer *ConverterWriter) InvalidateStream(stream []*Stream) error {
	if err := writer.buffer.Flush(); err != nil {
		return err
	}
	if _, err := writer.file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	// Find stream in file and replace streamid with InvalidStreamID
	for {
		streamSection := converterStreamSection{}
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

func (converter *Converter) Reader() (*ConverterReader, error) {
	file, err := os.Open(converter.CachePath())
	if err != nil {
		return nil, err
	}
	reader := &ConverterReader{
		filename:           converter.CachePath(),
		file:               file,
		containedStreamIds: make(map[uint64]uint64),
	}

	buffer := bufio.NewReader(file)
	// Read all stream ids
	pos := uint64(0)
	for {
		streamSection := converterStreamSection{}
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

func (r *ConverterReader) Close() error {
	return r.file.Close()
}

func (r *ConverterReader) HasStream(streamID uint64) bool {
	_, ok := r.containedStreamIds[streamID]
	return ok
}

func (r *ConverterReader) Streams() []uint64 {
	keys := make([]uint64, len(r.containedStreamIds))
	i := 0
	for k := range r.containedStreamIds {
		keys[i] = k
	}
	return keys
}

func (reader *ConverterReader) ReadStreamForSearch(streamID uint64) ([2][]byte, [][2]int, uint64, uint64, error) {
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

func (reader *ConverterReader) ReadStream(streamID uint64) ([]Data, uint64, uint64, error) {
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
