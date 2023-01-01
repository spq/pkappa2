package converters

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"

	"github.com/spq/pkappa2/internal/index"
)

type (
	Converter struct {
		executablePath string
		name           string
		jobs           chan func()
		running        bool
		process        *Process
	}
	Result struct {
		data        []index.Data
		clientBytes uint64
		serverBytes uint64
		err         error
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

func New(converterName string, executablePath string) *Converter {
	converter := Converter{
		executablePath: executablePath,
		name:           converterName,
		jobs:           make(chan func()),
		running:        true,
		process:        NewProcess(converterName, executablePath),
	}

	go func() {
		for f := range converter.jobs {
			f()
		}
	}()

	return &converter
}

func (converter *Converter) Reset() {
	converter.running = false
	converter.process.Abort()
}

func (converter *Converter) Data(stream *index.Stream) (data []index.Data, clientBytes, serverBytes uint64, err error) {
	ch := make(chan Result)
	converter.jobs <- func() {
		if !converter.running {
			ch <- Result{
				data:        nil,
				clientBytes: 0,
				serverBytes: 0,
				err:         fmt.Errorf("converter (%s): Cannot convert since the process is not running", converter.name),
			}
			return
		}
		ch <- converter.tryData(stream)
	}
	result := <-ch
	close(ch)
	return result.data, result.clientBytes, result.serverBytes, result.err
}

func (converter *Converter) tryData(stream *index.Stream) (result Result) {
	defer func() {
		if r := recover(); r != nil {
			result.err = r.(error)
		}
	}()
	directionsToString := map[index.Direction]string{
		index.DirectionClientToServer: "client-to-server",
		index.DirectionServerToClient: "server-to-client",
	}
	directionsToInt := map[string]index.Direction{
		"client-to-server": index.DirectionClientToServer,
		"server-to-client": index.DirectionServerToClient,
	}

	result = Result{
		data:        nil,
		clientBytes: 0,
		serverBytes: 0,
		err:         nil,
	}

	// TODO: Start a timeout here, so that we don't wait forever for the converter to respond
	log.Printf("Converter (%s): Running for stream %d", converter.name, stream.StreamID)
	packets, err := stream.Data() // FIXME: Lock index reader
	if err != nil {
		result.err = fmt.Errorf("converter (%s): Failed to get packets: %w", converter.name, err)
		return result
	}
	metadata := ConverterStreamMetadata{
		ClientHost: stream.ClientHostIP(),
		ClientPort: stream.ClientPort,
		ServerHost: stream.ServerHostIP(),
		ServerPort: stream.ServerPort,
		Protocol:   stream.Protocol(),
	}

	metadataEncoded, err := json.Marshal(metadata)
	if err != nil {
		result.err = fmt.Errorf("converter (%s): Failed to encode metadata: %w", converter.name, err)
		return result
	}
	converter.process.input <- metadataEncoded

	for _, packet := range packets {
		jsonPacket := ConverterStreamChunk{
			Direction: directionsToString[packet.Direction],
			Content:   base64.StdEncoding.EncodeToString(packet.Content),
		}
		// FIXME: Should we notify the converter about this somehow?
		jsonPacketEncoded, err := json.Marshal(jsonPacket)
		if err != nil {
			result.err = fmt.Errorf("converter (%s): Failed to encode packet: %w", converter.name, err)
			return result
		}
		converter.process.input <- jsonPacketEncoded
	}

	converter.process.input <- []byte("\n")

	var convertedPackets []index.Data
	clientBytes, serverBytes := uint64(0), uint64(0)
	for line := range converter.process.output {
		if line == "" {
			break
		}
		var convertedPacket ConverterStreamChunk
		if err := json.Unmarshal([]byte(line), &convertedPacket); err != nil {
			result.err = fmt.Errorf("converter (%s): Failed to read converted packet: %w", converter.name, err)
			return result
		}
		decodedData, err := base64.StdEncoding.DecodeString(convertedPacket.Content)
		if err != nil {
			result.err = fmt.Errorf("converter (%s): Failed to decode converted packet data: %w", converter.name, err)
			return result
		}

		direction, ok := directionsToInt[convertedPacket.Direction]
		if !ok {
			result.err = fmt.Errorf("converter (%s): Invalid direction: %q", converter.name, convertedPacket.Direction)
			return result
		}
		convertedPackets = append(convertedPackets, index.Data{Content: decodedData, Direction: direction})
		if direction == index.DirectionClientToServer {
			clientBytes += uint64(len(decodedData))
		} else {
			serverBytes += uint64(len(decodedData))
		}
	}
	var convertedMetadata ConverterStreamMetadata
	line := <-converter.process.output
	if err := json.Unmarshal([]byte(line), &convertedMetadata); err != nil {
		result.err = fmt.Errorf("converter (%s): Failed to read converted metadata: %w", converter.name, err)
		return result
	}

	result.data = convertedPackets
	result.clientBytes = clientBytes
	result.serverBytes = serverBytes

	// log.Printf("Converter (%s): Converted stream: %q", converter.name, convertedMetadata)
	// for _, convertedPacket := range convertedPackets {
	// 	log.Printf("Converter (%s): Converted packet: %q", converter.name, convertedPacket)
	// }

	return result
}
