package cmdline

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/yzp0n/ncdn/controller/protocol"
)

type stdIO struct {
	m         sync.Mutex
	stdinPipe io.WriteCloser
	id        uint32
	Cmd       *exec.Cmd
}

type OutputCommand struct {
	Msg  *protocol.ControlMessage
	Data []byte
}

type commandSender struct {
	id         uint32
	outType    protocol.OutputType
	outputChan chan OutputCommand
}

func (s *commandSender) Write(d []byte) (int, error) {
	if len(d) == 0 {
		return 0, nil // No data to write
	}
	msg := protocol.CommandLineOutput(s.id, s.outType, uint64(len(d)))
	s.outputChan <- OutputCommand{
		Msg:  msg,
		Data: d,
	}
	return len(d), nil
}

func (s *stdIO) Control(ctx context.Context, cmd protocol.CmdInstructionType) error {
	s.m.Lock()
	defer s.m.Unlock()
	if s.Cmd == nil {
		return nil // No command to control
	}
	switch cmd {
	case protocol.CmdInstructionType_CloseStdin:
		if s.stdinPipe != nil {
			if err := s.stdinPipe.Close(); err != nil {
				return err
			}
			s.stdinPipe = nil // Close the stdin pipe
		}
	case protocol.CmdInstructionType_Kill:
		if s.Cmd != nil {
			if err := s.Cmd.Process.Kill(); err != nil {
				return err
			}
			s.Cmd = nil // Reset Cmd after killing
		}
	default:
		return errors.New("unsupported command instruction type")
	}
	return nil
}

func (s *stdIO) Input(ctx context.Context, input []byte, output chan OutputCommand) error {
	s.m.Lock()
	defer s.m.Unlock()
	if s.Cmd == nil {
		// this is the first input, so we need to create the command
		args := strings.Split(string(input), " ")
		s.Cmd = exec.CommandContext(ctx, args[0], args[1:]...)
		var err error
		stdin, err := s.Cmd.StdinPipe()
		if err != nil {
			return err
		}
		s.stdinPipe = stdin
		s.Cmd.Stdout = &commandSender{
			id:         s.id,
			outType:    protocol.OutputType_Stdout,
			outputChan: output,
		}
		s.Cmd.Stderr = &commandSender{
			id:         s.id,
			outType:    protocol.OutputType_Stderr,
			outputChan: output,
		}
		if err := s.Cmd.Start(); err != nil {
			return err
		}
		go func() {
			if err := s.Cmd.Wait(); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					// Command exited with an error
					exitCode := exitErr.ExitCode()
					exitMsg := protocol.CommandLineExit(s.id, uint32(exitCode))
					output <- OutputCommand{Msg: exitMsg}
				} else {
					// Some other error occurred
					exitMsg := protocol.CommandLineExit(s.id, 1) // Exit code 1 for error
					errMsg := protocol.LogMessage(protocol.LogLevel_Error, "Command execution failed: "+err.Error())
					output <- OutputCommand{Msg: exitMsg}
					output <- OutputCommand{Msg: errMsg}
				}
			} else {
				output <- OutputCommand{
					Msg: protocol.CommandLineExit(s.id, 0), // Exit code 0
				}
			}
			s.m.Lock()
			s.Cmd = nil // Reset Cmd on completion
			s.m.Unlock()
		}()
	}
	if s.stdinPipe == nil {
		return nil // No stdin pipe available
	}
	_, err := s.stdinPipe.Write(input)
	return err
}

type Manager interface {
	Input(ctx context.Context, id uint32, input []byte) error
	Control(ctx context.Context, id uint32, cmd protocol.CmdInstructionType) error
	Output() <-chan OutputCommand
}

func NewManager() Manager {
	return &manager{
		ioMap:  make(map[uint32]*stdIO),
		output: make(chan OutputCommand, 100), // Buffered channel to avoid blocking
	}
}

type manager struct {
	m      sync.Mutex
	ioMap  map[uint32]*stdIO
	output chan OutputCommand
}

func (m *manager) Input(ctx context.Context, id uint32, input []byte) error {
	m.m.Lock()
	stdio, ok := m.ioMap[id]
	if !ok {
		// new stdIO
		stdio = &stdIO{id: id}
		m.ioMap[id] = stdio
	}
	m.m.Unlock()
	return stdio.Input(ctx, input, m.output)
}

func (m *manager) Control(ctx context.Context, id uint32, cmd protocol.CmdInstructionType) error {
	m.m.Lock()
	stdio, ok := m.ioMap[id]
	if !ok {
		m.m.Unlock()
		return errors.New("command not found")
	}
	m.m.Unlock()
	return stdio.Control(ctx, cmd)
}

func (m *manager) Output() <-chan OutputCommand {
	m.m.Lock()
	defer m.m.Unlock()
	if m.output == nil {
		m.output = make(chan OutputCommand, 100) // Buffered channel to avoid blocking
	}
	return m.output
}
