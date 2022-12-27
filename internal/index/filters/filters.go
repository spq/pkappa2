package filters

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"

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

	directions := map[index.Direction]string{
		index.DirectionClientToServer: "client-to-server",
		index.DirectionServerToClient: "server-to-client",
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
				Direction: directions[packet.Direction],
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

		var filteredPackets []FilterStreamData
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
			filteredPackets = append(filteredPackets, filteredPacket)
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
