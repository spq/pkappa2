package manager

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"log"
	"os/exec"

	"github.com/spq/pkappa2/internal/index"
)

type (
	filter struct {
		path         string
		name         string
		streams      chan *index.Stream
		cmd          *exec.Cmd
		attachedTags []string
	}
)

func (fltr *filter) startFilterIfNeeded() {
	if fltr.cmd.Process != nil {
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

func (fltr *filter) startFilter() {
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
