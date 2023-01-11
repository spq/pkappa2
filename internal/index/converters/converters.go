package converters

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/spq/pkappa2/internal/index"
)

const (
	MAX_PROCESS_COUNT = 8
)

type (
	Converter struct {
		executablePath string
		name           string
		// Keep track of when a process was claimed by a stream.
		// If the epoch changed since the process was claimed, the process is no longer valid.
		reset_epoch int
		// Used by Reset to stop new processes from starting while resetting is in process.
		rwmutex sync.RWMutex
		// Synchronizes access to the `started_processes` and `available_processes` members
		mutex sync.Mutex
		// Used to signal waiting Data() calls that a process is available.
		// reserveProcess() waits on this channel when all processes are in use.
		signal chan struct{}
		// All processes started for this converter.
		started_processes map[*Process]bool
		// Processes that are currently idle.
		available_processes []*Process
		// Processes that died unexpectedly.
		failed_processes []*Process
	}
	ProcessStats struct {
		Running  bool
		ExitCode int
		Stderr   []string
	}
	// JSON Protocol
	converterStreamMetadata struct {
		ClientHost string
		ClientPort uint16
		ServerHost string
		ServerPort uint16
		Protocol   string
	}
	converterStreamChunk struct {
		Direction string
		Content   string
	}
)

var (
	directionsToString = map[index.Direction]string{
		index.DirectionClientToServer: "client-to-server",
		index.DirectionServerToClient: "server-to-client",
	}
	directionsToInt = map[string]index.Direction{
		"client-to-server": index.DirectionClientToServer,
		"server-to-client": index.DirectionServerToClient,
	}
)

func New(converterName, executablePath string) *Converter {
	converter := Converter{
		executablePath:    executablePath,
		name:              converterName,
		signal:            make(chan struct{}),
		started_processes: map[*Process]bool{},
	}

	return &converter
}

func (converter *Converter) Name() string {
	return converter.name
}

func (converter *Converter) ProcessStats() []ProcessStats {
	converter.mutex.Lock()
	defer converter.mutex.Unlock()

	// TODO: Keep stderr and exitcode of processes that have exited.
	output := []ProcessStats{}
	for process := range converter.started_processes {
		output = append(output, ProcessStats{
			Running:  true,
			ExitCode: 0,
			Stderr:   process.Stderr(),
		})
	}
	for _, process := range converter.failed_processes {
		output = append(output, ProcessStats{
			Running:  false,
			ExitCode: process.ExitCode(),
			Stderr:   process.Stderr(),
		})
	}
	return output
}

// Stop the converter process.
func (converter *Converter) Reset() {
	converter.rwmutex.Lock()
	defer converter.rwmutex.Unlock()

	// Signal in-use processes to stop after they finish their current job.
	converter.reset_epoch++

	// Kill all currently idle processes.
	for _, process := range converter.available_processes {
		close(process.input)
		delete(converter.started_processes, process)
		// Tell any waiting Data call to start a new process.
		select {
		case converter.signal <- struct{}{}:
		default:
		}
	}
	converter.available_processes = nil
}

func (converter *Converter) reserveProcess() (*Process, int) {
	// See if we want to stop the process and we're in a Reset call. Reset would grab a write lock.
	converter.rwmutex.RLock()
	defer converter.rwmutex.RUnlock()

	converter.mutex.Lock()
	defer converter.mutex.Unlock()

	// TODO: If Reset is called before Data is called, the process will start again, which we might not want.
	for {
		if len(converter.available_processes) > 0 {
			process := converter.available_processes[len(converter.available_processes)-1]
			converter.available_processes = converter.available_processes[:len(converter.available_processes)-1]
			return process, converter.reset_epoch
		}

		if len(converter.started_processes) < MAX_PROCESS_COUNT {
			process := NewProcess(converter.name, converter.executablePath)
			converter.started_processes[process] = true
			return process, converter.reset_epoch
		}

		// Wait for signal from process that it's done.
		converter.mutex.Unlock()
		converter.rwmutex.RUnlock()
		<-converter.signal
		converter.rwmutex.RLock()
		converter.mutex.Lock()
	}
}

func (converter *Converter) releaseProcess(process *Process, reset_epoch int) {
	converter.rwmutex.RLock()
	defer converter.rwmutex.RUnlock()

	converter.mutex.Lock()
	defer converter.mutex.Unlock()

	// Signal that a process is available.
	select {
	case converter.signal <- struct{}{}:
	default:
	}

	if reset_epoch != converter.reset_epoch {
		// The converter was reset while this process was running.
		close(process.input)
		// Drain the output until the process exits.
		for range process.output {
		}
		delete(converter.started_processes, process)
		if process.ExitCode() != 0 {
			converter.failed_processes = append(converter.failed_processes, process)
		}
		return
	}

	converter.available_processes = append(converter.available_processes, process)
}

func (converter *Converter) Data(stream *index.Stream) (data []index.Data, clientBytes, serverBytes uint64, err error) {
	// TODO: Start a timeout here, so that we don't wait forever for the converter to respond

	// Grab stream data before getting any locks, since this can take a while.
	packets, err := stream.Data()
	if err != nil {
		return nil, 0, 0, fmt.Errorf("converter (%s): Failed to get packets: %w", converter.name, err)
	}

	metadata := converterStreamMetadata{
		ClientHost: stream.ClientHostIP(),
		ClientPort: stream.ClientPort,
		ServerHost: stream.ServerHostIP(),
		ServerPort: stream.ServerPort,
		Protocol:   stream.Protocol(),
	}

	metadataEncoded, err := json.Marshal(metadata)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("converter (%s): Failed to encode metadata: %w", converter.name, err)
	}

	process, reset_epoch := converter.reserveProcess()

	log.Printf("Converter (%s): Running for stream %d", converter.name, stream.ID())

	// Initiate converter protocol
	process.input <- metadataEncoded

	for _, packet := range packets {
		jsonPacket := converterStreamChunk{
			Direction: directionsToString[packet.Direction],
			Content:   base64.StdEncoding.EncodeToString(packet.Content),
		}
		// FIXME: Should we notify the converter about this somehow?
		jsonPacketEncoded, err := json.Marshal(jsonPacket)
		if err != nil {
			converter.releaseProcess(process, -1)
			return nil, 0, 0, fmt.Errorf("converter (%s): Failed to encode packet: %w", converter.name, err)
		}
		process.input <- jsonPacketEncoded
	}

	process.input <- []byte("\n")

	// TODO: The converter process could send a response line after every received json line,
	//       and the stdout buffer could fill up causing a deadlock.
	//       select?
	for line := range process.output {
		if len(line) == 0 {
			break
		}
		var convertedPacket converterStreamChunk
		if err := json.Unmarshal(line, &convertedPacket); err != nil {
			converter.releaseProcess(process, -1)
			return nil, 0, 0, fmt.Errorf("converter (%s): Failed to read converted packet: %w", converter.name, err)
		}
		decodedData, err := base64.StdEncoding.DecodeString(convertedPacket.Content)
		if err != nil {
			converter.releaseProcess(process, -1)
			return nil, 0, 0, fmt.Errorf("converter (%s): Failed to decode converted packet data: %w", converter.name, err)
		}

		direction, ok := directionsToInt[convertedPacket.Direction]
		if !ok {
			converter.releaseProcess(process, -1)
			return nil, 0, 0, fmt.Errorf("converter (%s): Invalid direction: %q", converter.name, convertedPacket.Direction)
		}
		data = append(data, index.Data{Content: decodedData, Direction: direction})
		if direction == index.DirectionClientToServer {
			clientBytes += uint64(len(decodedData))
		} else {
			serverBytes += uint64(len(decodedData))
		}
	}
	var convertedMetadata converterStreamMetadata
	line, ok := <-process.output
	if !ok {
		converter.releaseProcess(process, -1)
		return nil, 0, 0, fmt.Errorf("converter (%s): Converter process exited unexpectedly", converter.name)
	}
	if err := json.Unmarshal(line, &convertedMetadata); err != nil {
		converter.releaseProcess(process, -1)
		return nil, 0, 0, fmt.Errorf("converter (%s): Failed to read converted metadata: %w", converter.name, err)
	}

	// log.Printf("Converter (%s): Converted stream: %q", converter.name, convertedMetadata)
	// for _, convertedPacket := range data {
	// 	log.Printf("Converter (%s): Converted packet: %q", converter.name, convertedPacket)
	// }

	converter.releaseProcess(process, reset_epoch)
	return
}
