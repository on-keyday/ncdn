package remoteshell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"github.com/creack/pty"
	"github.com/yzp0n/ncdn/controller/protocol"
)

type stdIO struct {
	m         sync.Mutex
	stdinPipe io.WriteCloser
	id        uint32
	Cmd       *exec.Cmd
	usePty    bool
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
	cloned := make([]byte, len(d))
	copy(cloned, d)
	chunk := cloned[:]
	for len(chunk) > 65535 {
		oneChunk := chunk[:65535]
		msg := protocol.CommandLineOutput(s.id, s.outType, protocol.MakeChunkInfo(false, uint32(len(oneChunk))))
		s.outputChan <- OutputCommand{
			Msg:  msg,
			Data: oneChunk,
		}
		chunk = chunk[65535:]
	}
	if len(chunk) > 0 {
		msg := protocol.CommandLineOutput(s.id, s.outType, protocol.MakeChunkInfo(false, uint32(len(chunk))))
		s.outputChan <- OutputCommand{
			Msg:  msg,
			Data: chunk,
		}
	}
	return len(d), nil
}

func (s *stdIO) Control(ctx context.Context, cmd protocol.CmdInstructionType, arg byte, output chan OutputCommand) error {
	s.m.Lock()
	defer s.m.Unlock()
	switch cmd {
	case protocol.CmdInstructionType_CloseStdin:
		if s.stdinPipe != nil {
			if err := s.stdinPipe.Close(); err != nil {
				return err
			}
			s.stdinPipe = nil // Close the stdin pipe
		}
	case protocol.CmdInstructionType_Kill:
		if s.Cmd != nil && s.Cmd.Process != nil {
			if arg == 0 {
				if err := s.Cmd.Process.Kill(); err != nil {
					return err
				}
			} else {
				if err := s.Cmd.Process.Signal(syscall.Signal(arg - 1)); err != nil {
					return fmt.Errorf("failed to send signal %d to command %d: %w", arg, s.id, err)
				}
			}
			output <- OutputCommand{
				Msg: protocol.LogMessage(protocol.LogLevel_Info,
					fmt.Sprintf("Trying to kill command %d (PID: %d)", s.id, s.Cmd.Process.Pid)),
			}
			slog.Info("Tried to kill", "id", s.id, "pid", s.Cmd.Process.Pid)
		} else {
			output <- OutputCommand{
				Msg: protocol.LogMessage(protocol.LogLevel_Warn,
					fmt.Sprintf("No command running with ID %d to kill", s.id)),
			}
		}
	case protocol.CmdInstructionType_UsePty:
		s.usePty = true
	default:
		return errors.New("unsupported command instruction type")
	}
	return nil
}

func (s *stdIO) Input(mgr *manager, ctx context.Context, input []byte, output chan OutputCommand) error {
	s.m.Lock()
	defer s.m.Unlock()
	if s.Cmd == nil {
		// this is the first input, so we need to create the command
		args := strings.Split(string(input), " ")
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		isPty := s.usePty
		var ptyFile *os.File
		if s.usePty {
			startWithPty, err := pty.StartWithSize(cmd, nil)
			if err != nil {
				return err
			}
			s.stdinPipe = startWithPty
			ptyFile = startWithPty
		} else {
			stdin, err := cmd.StdinPipe()
			if err != nil {
				return err
			}
			s.stdinPipe = stdin
			cmd.Stdout = &commandSender{
				id:         s.id,
				outType:    protocol.OutputType_Stdout,
				outputChan: output,
			}
			cmd.Stderr = &commandSender{
				id:         s.id,
				outType:    protocol.OutputType_Stderr,
				outputChan: output,
			}
			if err := cmd.Start(); err != nil {
				return err
			}
		}
		s.Cmd = cmd
		go func() {
			if isPty {
				_, err := io.Copy(&commandSender{
					id:         s.id,
					outType:    protocol.OutputType_Stdout,
					outputChan: output,
				}, ptyFile)
				if err != nil {
					output <- OutputCommand{
						Msg: protocol.LogMessage(protocol.LogLevel_Error, "Failed to read from pty: "+err.Error()),
					}
				}
			}
			if err := cmd.Wait(); err != nil {
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
			mgr.m.Lock()
			if inst, ok := mgr.ioMap[s.id]; ok && inst == s {
				delete(mgr.ioMap, s.id) // Remove from manager's map
			}
			mgr.m.Unlock()
		}()
		output <- OutputCommand{
			Msg: protocol.LogMessage(protocol.LogLevel_Info, "Command started: "+strings.Join(args, " ")),
		}
		return nil
	}
	if s.stdinPipe == nil {
		return nil // No stdin pipe available
	}
	_, err := s.stdinPipe.Write(input)
	return err
}

func (s *stdIO) ResizeWindow(x, y int) error {
	s.m.Lock()
	defer s.m.Unlock()
	if s.Cmd == nil {
		return errors.New("command not running")
	}
	return nil
}

type Manager interface {
	Input(ctx context.Context, id uint32, input []byte) error
	Control(ctx context.Context, id uint32, cmd protocol.CmdInstructionType, arg byte) error
	Output() <-chan OutputCommand
	ResizeWindow(id uint32, x, y int) error
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
		m.output <- OutputCommand{
			Msg: protocol.LogMessage(protocol.LogLevel_Info, fmt.Sprintf("Starting new command with ID %d", id)),
		}
	}
	m.m.Unlock()
	return stdio.Input(m, ctx, input, m.output)
}

func (m *manager) ResizeWindow(id uint32, x, y int) error {
	m.m.Lock()
	defer m.m.Unlock()
	stdio, ok := m.ioMap[id]
	if !ok {
		return errors.New("command not found")
	}
	if stdio.Cmd == nil {
		return errors.New("command not running")
	}
	return nil
}

func (m *manager) Control(ctx context.Context, id uint32, cmd protocol.CmdInstructionType, arg byte) error {
	m.m.Lock()
	if cmd == protocol.CmdInstructionType_KillAll {
		var errs []error
		// Kill all commands
		for _, stdio := range m.ioMap {
			if err := stdio.Control(ctx, protocol.CmdInstructionType_Kill, arg, m.output); err != nil {
				errs = append(errs, err)
			}
		}
		clear(m.ioMap) // Clear the map after killing all commands
		m.m.Unlock()
		return errors.Join(errs...)
	}
	stdio, ok := m.ioMap[id]
	if !ok {
		stdio = &stdIO{id: id} // Create a new stdIO if not found
		m.ioMap[id] = stdio
		m.output <- OutputCommand{
			Msg: protocol.LogMessage(protocol.LogLevel_Info, fmt.Sprintf("Starting new command with ID %d", id)),
		}
	}
	m.m.Unlock()
	return stdio.Control(ctx, cmd, arg, m.output)
}

func (m *manager) Output() <-chan OutputCommand {
	return m.output
}

func DispatchMessage(m Manager, msg *protocol.ControlMessage) (bool, error) {
	if msg := msg.CmdlineIn(); msg != nil {
		return true, m.Input(context.Background(), msg.CmdlineId, msg.Cmdline)
	}
	if msg := msg.CmdlineInstr(); msg != nil {
		return true, m.Control(context.Background(), msg.CmdlineId, msg.Instr, msg.Arg)
	}
	if msg := msg.CmdlineResize(); msg != nil {
		return true, m.ResizeWindow(msg.CmdlineId, int(msg.Col), int(msg.Row))
	}
	return false, nil
}
