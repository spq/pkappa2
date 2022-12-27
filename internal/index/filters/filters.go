package filters

import (
	"bufio"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime/debug"
	"unsafe"

	"github.com/spq/pkappa2/internal/index"
)

type (
	Filter struct {
		path         string
		name         string
		streams      chan *index.Stream
		cmd          *exec.Cmd
		attachedTags []string
	}
	filteredData struct {
		Data      []byte
		Direction index.Direction
	}
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
	FilterStreamData struct {
		Direction string
		Data      string
	}
)

func NewFilter(path string, name string) *Filter {
	return &Filter{
		path:    path,
		name:    name,
		streams: make(chan *index.Stream, 100),
		cmd:     exec.Command(path),
	}
}

func (fltr *Filter) startFilter() {
	stdin, err := fltr.cmd.StdinPipe()
	if err != nil {
		log.Printf("Filter (%s): Failed to create stdin pipe: %q", fltr.name, err)
		return
	}
	stdout, err := fltr.cmd.StdoutPipe()
	if err != nil {
		log.Printf("Filter (%s): Failed to create stdout pipe: %q", fltr.name, err)
		return
	}
	stderr, err := fltr.cmd.StderrPipe()
	if err != nil {
		log.Printf("Filter (%s): Failed to create stderr pipe: %q", fltr.name, err)
		return
	}

	// Dump stderr directly
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Printf("Filter (%s): %s", fltr.name, scanner.Text())
		}
	}()

	err = fltr.cmd.Start()
	if err != nil {
		log.Printf("Filter (%s): Failed to start process: %q", fltr.name, err)
		return
	}
	defer fltr.cmd.Process.Kill()
	defer fltr.cmd.Wait()

	directionsToString := map[index.Direction]string{
		index.DirectionClientToServer: "client-to-server",
		index.DirectionServerToClient: "server-to-client",
	}
	directionsToInt := map[string]index.Direction{
		"client-to-server": index.DirectionClientToServer,
		"server-to-client": index.DirectionServerToClient,
	}

	invalidState := false
	stdinJson := json.NewEncoder(stdin)
	stdoutScanner := bufio.NewScanner(stdout)
outer:
	for stream := range fltr.streams {
		if invalidState {
			continue
		}
		log.Printf("running filter %s for stream %d", fltr.Name(), stream.StreamID)
		metadata := FilterStreamMetadata{
			ClientHost: stream.ClientHostIP(),
			ClientPort: stream.ClientPort,
			ServerHost: stream.ServerHostIP(),
			ServerPort: stream.ServerPort,
			Protocol:   stream.Protocol(),
		}

		// TODO: Start a timeout here, so that we don't wait forever for the filter to respond
		packets, err := stream.Data()
		if err != nil {
			log.Printf("Filter (%s): Failed to get packets: %q", fltr.name, err)
			invalidState = true
			break
		}
		if err = stdinJson.Encode(metadata); err != nil {
			log.Printf("Filter (%s): Failed to send stream metadata: %q", fltr.name, err)
			invalidState = true
			break
		}

		for _, packet := range packets {
			jsonPacket := FilterStreamData{
				Direction: directionsToString[packet.Direction],
				Data:      base64.StdEncoding.EncodeToString(packet.Content),
			}
			if err = stdinJson.Encode(jsonPacket); err != nil {
				// FIXME: Should we notify the filter about this somehow?
				log.Printf("Filter (%s): Failed to send packet: %q", fltr.name, err)
				invalidState = true
				break outer
			}
		}

		if _, err := stdin.Write([]byte("\n")); err != nil {
			log.Printf("Filter (%s): Failed to send newline: %q", fltr.name, err)
			invalidState = true
			break
		}

		var filteredPackets []filteredData
		var filteredMetadata FilterStreamMetadata
		for stdoutScanner.Scan() {
			filterLine := stdoutScanner.Text()
			if filterLine == "" {
				break
			}

			var filteredPacket FilterStreamData
			if err := json.Unmarshal([]byte(filterLine), &filteredPacket); err != nil {
				log.Printf("Filter (%s): Failed to read filtered packet: %q", fltr.name, err)
				invalidState = true
				break outer
			}
			decodedData, err := base64.StdEncoding.DecodeString(filteredPacket.Data)
			if err != nil {
				log.Printf("Filter (%s): Failed to decode filtered packet data: %q", fltr.name, err)
				invalidState = true
				break outer
			}

			direction, ok := directionsToInt[filteredPacket.Direction]
			if !ok {
				log.Printf("Filter (%s): Invalid direction: %q", fltr.name, filteredPacket.Direction)
				invalidState = true
				break outer
			}
			filteredPackets = append(filteredPackets, filteredData{Data: decodedData, Direction: direction})
		}

		if !stdoutScanner.Scan() {
			log.Printf("Filter (%s): Failed to read filtered stream metadata: %q", fltr.name, err)
			invalidState = true
			break
		}
		filterLine := stdoutScanner.Text()
		if err := json.Unmarshal([]byte(filterLine), &filteredMetadata); err != nil {
			log.Printf("Filter (%s): Failed to read filtered stream metadata: %q", fltr.name, err)
			invalidState = true
			break
		}

		log.Printf("Filter (%s): Filtered stream: %q", fltr.name, filteredMetadata)
		for _, filteredPacket := range filteredPackets {
			log.Printf("Filter (%s): Filtered packet: %q", fltr.name, filteredPacket)
		}

		// TODO: Add processed results to the stream for use in queries
		// TODO: Persist processed results to disk
	}
}

func (filter *Filter) EnqueueStream(stream *index.Stream) {
	filter.streams <- stream
}

func (filter *Filter) AttachTag(tag string) {
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
	}
	return nil
}

func (filter *Filter) IsRunning() bool {
	return filter.cmd.Process != nil && filter.cmd.ProcessState == nil
}

func (filter *Filter) Name() string {
	return filter.name
}

func (filter *Filter) KillProcess() error {
	if filter.cmd.Process != nil {
		if err := filter.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("error: could not kill filter %s: %q", filter.name, err)
		}
		filter.cmd.Process.Wait()
	}
	close(filter.streams)
	return nil
}

func (filter *Filter) RestartProcess() error {
	if err := filter.KillProcess(); err != nil {
		return err
	}

	// FIXME: Delay restart to avoid "text file busy" error
	filter.streams = make(chan *index.Stream, 100)
	filter.cmd = exec.Command(filter.path)

	// Start the process
	filter.startFilterIfNeeded()
	return nil
}

// File format
type (
	filterStreamSection struct {
		StreamID uint64
		DataSize uint64
	}

	Writer struct {
		filename string
		file     *os.File
		buffer   *bufio.Writer
	}

	Reader struct {
		filename           string
		file               *os.File
		containedStreamIds map[uint64]uint64
	}
)

func NewWriter(filename string) (*Writer, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	return &Writer{
		filename: filename,
		file:     file,
		buffer:   bufio.NewWriter(file),
	}, nil
}

func (writer *Writer) write(what interface{}) error {
	err := binary.Write(writer.buffer, binary.LittleEndian, what)
	if err != nil {
		debug.PrintStack()
	}
	return err
}

func (writer *Writer) WriteStream(stream *index.Stream, filteredPackets []filteredData) error {
	// [u64 stream id] [u64 data size] [u8 varint chunk sizes] [client data] [server data]
	dataSize := uint64(0)
	for _, filteredPacket := range filteredPackets {
		dataSize += uint64(len(filteredPacket.Data))
	}

	segmentation := []byte(nil)
	buf := [10]byte{}
	for pIndex, wantDir := 0, index.DirectionClientToServer; pIndex < len(filteredPackets); {
		// TODO: Merge packets with the same direction. Do we even want to allow filters to change the direction?
		filteredPacket := filteredPackets[pIndex]
		sz := len(filteredPacket.Data)
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
	}
	// Append a length of 0 to indicate the end of the chunk sizes
	segmentation = append(segmentation, 0)

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

	for _, direction := range []index.Direction{index.DirectionClientToServer, index.DirectionServerToClient} {
		for _, filteredPacket := range filteredPackets {
			if filteredPacket.Direction != direction {
				continue
			}
			if _, err := writer.buffer.Write(filteredPacket.Data); err != nil {
				return err
			}
		}
	}

	return nil
}

func NewReader(filename string) (*Reader, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	reader := &Reader{
		filename:           filename,
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
		reader.containedStreamIds[streamSection.StreamID] = pos
		if _, err := buffer.Discard(int(streamSection.DataSize)); err != nil {
			return nil, err
		}
		pos += streamSection.DataSize + uint64(unsafe.Sizeof(streamSection))
	}
	return reader, nil
}

func (reader *Reader) ReadStream(streamID uint64) ([]filteredData, error) {
	// [u64 stream id] [u64 data size] [u8 varint chunk sizes] [client data] [server data]
	pos, ok := reader.containedStreamIds[streamID]
	if !ok {
		return nil, nil
	}
	if _, err := reader.file.Seek(int64(pos), io.SeekStart); err != nil {
		return nil, err
	}

	buffer := bufio.NewReader(reader.file)
	data := []filteredData{}

	type sizeAndDirection struct {
		Size      uint64
		Direction index.Direction
	}
	// Read chunk sizes
	dataSizes := []sizeAndDirection{}
	prevWasZero := false
	direction := index.DirectionClientToServer
	clientBytes := uint64(0)
	serverBytes := uint64(0)
	for {
		sz := uint64(0)
		for {
			b, err := buffer.ReadByte()
			if err != nil {
				return nil, err
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
		if direction == index.DirectionClientToServer {
			clientBytes += sz
		} else {
			serverBytes += sz
		}
		direction = direction.Reverse()
	}

	// Read data
	clientData := make([]byte, clientBytes)
	if _, err := io.ReadFull(buffer, clientData); err != nil {
		return nil, err
	}
	serverData := make([]byte, serverBytes)
	if _, err := io.ReadFull(buffer, serverData); err != nil {
		return nil, err
	}

	// Split data into chunks
	for _, ds := range dataSizes {
		if ds.Size == 0 {
			continue
		}
		var bytes []byte
		if ds.Direction == index.DirectionClientToServer {
			bytes = clientData[:ds.Size]
			clientData = clientData[ds.Size:]
		} else {
			bytes = serverData[:ds.Size]
			serverData = serverData[ds.Size:]
		}
		data = append(data, filteredData{
			Direction: ds.Direction,
			Data:      bytes,
		})
	}
	return data, nil
}
