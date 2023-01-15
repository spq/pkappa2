package converters

import (
	"bufio"
	"container/ring"
	"log"
	"os/exec"
	"sync"
)

type (
	Process struct {
		converterName  string
		executablePath string
		cmd            *exec.Cmd
		input          chan []byte
		output         chan []byte
		stderrRing     *ring.Ring
		stderrLock     sync.RWMutex
		exitCode       int
	}
)

const (
	// Number of lines to keep in the stderr buffer.
	STDERR_BUFFER_SIZE = 512
)

// To stop the process, close the input channel.
// The output channel will be closed when the process exits.
func NewProcess(converterName string, executablePath string) *Process {
	process := Process{
		converterName:  converterName,
		executablePath: executablePath,
		cmd:            nil,
		input:          make(chan []byte),
		output:         make(chan []byte),
		stderrRing:     ring.New(STDERR_BUFFER_SIZE),
		stderrLock:     sync.RWMutex{},
	}

	go process.run()
	return &process
}

func ReadLine(reader *bufio.Reader) ([]byte, error) {
	result := []byte{}
	for {
		line, isPrefix, err := reader.ReadLine()
		if err != nil {
			return nil, err
		}
		result = append(result, line...)
		if !isPrefix {
			return result, nil
		}
	}
}

func (process *Process) Stderr() []string {
	process.stderrLock.RLock()
	defer process.stderrLock.RUnlock()

	output := []string{}
	process.stderrRing.Do(func(value any) {
		if value != nil {
			output = append(output, value.(string))
		}
	})
	return output
}

func (process *Process) ExitCode() int {
	return process.exitCode
}

// Run until input channel is closed
func (process *Process) run() {
	process.cmd = exec.Command(process.executablePath)
	stdout, err := process.cmd.StdoutPipe()
	if err != nil {
		log.Printf("Converter (%s): Failed to create stdout pipe: %q", process.converterName, err)
		close(process.output)
		return
	}

	// Pipe stdout to output channel
	go func() {
		reader := bufio.NewReaderSize(stdout, 65536)
		for {
			line, err := ReadLine(reader)
			if err != nil {
				break
			}
			process.output <- line
		}
		close(process.output)
	}()

	stderr, err := process.cmd.StderrPipe()
	if err != nil {
		log.Printf("Converter (%s): Failed to create stderr pipe: %q", process.converterName, err)
		stdout.Close()
		return
	}

	// Dump stderr directly
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			log.Printf("Converter (%s) stderr: %s", process.converterName, line)

			process.stderrLock.Lock()
			process.stderrRing.Value = line
			process.stderrRing = process.stderrRing.Next()
			process.stderrLock.Unlock()
		}
	}()

	stdin, err := process.cmd.StdinPipe()
	if err != nil {
		log.Printf("Converter (%s): Failed to create stdin pipe: %q", process.converterName, err)
		stdout.Close()
		stderr.Close()
		return
	}

	err = process.cmd.Start()
	if err != nil {
		log.Printf("Converter (%s): Failed to start process: %q", process.converterName, err)
		stdout.Close()
		stderr.Close()
		stdin.Close()
		return
	}

	for line := range process.input {
		if _, err := stdin.Write(line); err != nil {
			log.Printf("Converter (%s): Failed to write to stdin: %q", process.converterName, err)
			// wait for process to exit and close std pipes.
			if err := process.cmd.Wait(); err != nil {
				log.Printf("Converter (%s): Failed to wait for process: %q", process.converterName, err)
				process.exitCode = -1
			} else {
				process.exitCode = process.cmd.ProcessState.ExitCode()
			}

			// drain input channel to unblock caller
			for range process.input {
			}
			return
		}
	}

	// FIXME: Wait doesn't work if we kill the process first. Are the resources still cleared?
	//        Maybe just silently ignore errors when waiting and just check if ProcessState is nil?
	process.cmd.Process.Kill()
	if err := process.cmd.Wait(); err != nil {
		log.Printf("Converter (%s): Failed to wait for process: %q", process.converterName, err)
		process.exitCode = -1
		return
	}
	process.exitCode = process.cmd.ProcessState.ExitCode()
}
