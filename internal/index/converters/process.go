package converters

import (
	"bufio"
	"log"
	"os/exec"
)

type (
	Process struct {
		converterName  string
		executablePath string
		cmd            *exec.Cmd
		input          chan []byte
		output         chan string
		status         chan error
	}
)

func NewProcess(converterName string, executablePath string) *Process {
	process := Process{
		converterName:  converterName,
		executablePath: executablePath,
		cmd:            nil,
		input:          make(chan []byte),
		output:         make(chan string),
		status:         make(chan error),
	}

	process.Start()
	return &process
}

func (process *Process) Start() {
	if process.IsRunning() {
		go process.runProcess()
	}
}

func (process *Process) Abort() error {
	if process.cmd == nil {
		return nil
	}
	process.cmd.Process.Kill()
	close(process.input)
	err := <-process.status
	process.cmd = nil
	return err
}

func (process *Process) IsRunning() bool {
	return process.cmd != nil
}

func (process *Process) runProcess() {
	defer func() {
		if r := recover(); r != nil {
			process.status <- r.(error)
		}
	}()
	if process.IsRunning() {
		return
	}
	process.cmd = exec.Command(process.executablePath)
	stdout, err := process.cmd.StdoutPipe()
	if err != nil {
		log.Printf("Filter (%s): Failed to create stdout pipe: %q", process.converterName, err)
		close(process.output)
		return
	}

	// Pipe stdout to output channel
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			process.output <- scanner.Text()
		}
		close(process.output)
	}()

	stderr, err := process.cmd.StderrPipe()
	if err != nil {
		log.Printf("Filter (%s): Failed to create stderr pipe: %q", process.converterName, err)
		stdout.Close()
		return
	}

	// Dump stderr directly
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Printf("Filter (%s) stderr: %s", process.converterName, scanner.Text())
		}
	}()

	stdin, err := process.cmd.StdinPipe()
	if err != nil {
		log.Printf("Filter (%s): Failed to create stdin pipe: %q", process.converterName, err)
		stdout.Close()
		stderr.Close()
		return
	}

	err = process.cmd.Start()
	if err != nil {
		log.Printf("Filter (%s): Failed to start process: %q", process.converterName, err)
		stdout.Close()
		stderr.Close()
		stdin.Close()
		return
	}
	defer process.cmd.Process.Kill()
	defer process.cmd.Wait()

	for line := range process.input {
		if _, err := stdin.Write(line); err != nil {
			log.Printf("Filter (%s): Failed to write to stdin: %q", process.converterName, err)
			return
		}
	}
}
