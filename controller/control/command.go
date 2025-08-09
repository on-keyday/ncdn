package control

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/yzp0n/ncdn/controller/protocol"
)

func (c *controller) ShareKey(key []byte) {
	var l4lblist *l4list
	var l7lbList *l7list
	var keyGeneration uint64
	c.withLock(func() {
		c.sharedKey.generation = c.msgSeqNum
		c.sharedKey.key = key
		keyGeneration = c.sharedKey.generation
		c.msgSeqNum++
		l4lblist = c.l4lblist.Clone()
		l7lbList = c.l7lbList.Clone()
	})
	for _, l4lb := range l4lblist.list {
		l4lb.SendMessage(message{
			seqNum: keyGeneration,
			data:   protocol.KeyShare(protocol.KeyType_Quiclb, key),
		})
	}
	for _, l7lb := range l7lbList.list {
		l7lb.SendMessage(message{
			seqNum: keyGeneration,
			data:   protocol.KeyShare(protocol.KeyType_Quiclb, key),
		})
	}
	c.logger.Info("Shared key with L4LB and L7LBs", "generation", keyGeneration)
}

const chunkingThreshold = 65535 - 8 // 8 bytes for header

func NewSectionReader(s *io.SectionReader) *SecttionReader {
	return &SecttionReader{SectionReader: s}
}

type SecttionReader struct {
	*io.SectionReader
}

func (*SecttionReader) Close() error { return nil }
func (*SecttionReader) AddRef()      {}

// file should have reference before calling this function
func sendWithChunk(lb lbSender, makeMsg func(protocol.ChunkInfo) *protocol.ControlMessage, file ReaderAtCloser) error {
	size := file.Size()
	if size <= chunkingThreshold {
		if err := lb.SendMessageBlocking(message{
			seqNum: lb.GetSeqNum(),
			data:   makeMsg(protocol.MakeChunkInfo(false, uint32(size))),
			reader: file,
		}); err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}
		return nil
	}

	chunkID := uint32(lb.GetSeqNum() & 0x7FFFFFFF) // Use lower 31 bits for chunk ID

	if err := lb.SendMessageBlocking(message{
		seqNum: lb.GetSeqNum(),
		data:   makeMsg(protocol.MakeChunkInfo(true, chunkID)),
	}); err != nil {
		return fmt.Errorf("failed to send initial chunk message: %w", err)
	}

	offset := int64(0)
	eof := false
	for offset < size {
		chunkSize := chunkingThreshold
		if size-offset < int64(chunkSize) {
			chunkSize = int(size - offset)
			eof = true // Last chunk
		}

		buf := make([]byte, chunkSize)
		if _, err := file.ReadAt(buf, offset); err != nil {
			return fmt.Errorf("failed to read file at offset %d: %w", offset, err)
		}

		if err := lb.SendMessageBlocking(message{
			seqNum: lb.GetSeqNum(),
			data:   protocol.LargeChunk(chunkID, uint32(len(buf)), eof),
			reader: &SecttionReader{SectionReader: io.NewSectionReader(file, offset, int64(len(buf)))},
		}); err != nil {
			return fmt.Errorf("failed to send chunk at offset %d: %w", offset, err)
		}

		offset += int64(len(buf))
	}
	return nil
}

func (c *controller) FileTransfer(typ LBType, serverID uint32, path string, permission uint16, file ReaderAtCloser) error {
	if len(path) > 65535-8 { // 8 bytes for permission and file size
		return errors.New("file path exceeds maximum length of 65535 bytes")
	}
	var lb lbSender
	switch typ {
	case LBTypeL7:
		c.withLock(func() {
			for _, l7lb := range c.l7lbList.list {
				if l7lb.data.data.Data.Data.ServerID == serverID {
					lb = l7lb
					break
				}
			}
		})
	case LBTypeL4:
		c.withLock(func() {
			for _, l4lb := range c.l4lblist.list {
				if l4lb.data.data.Data.Data.ServerID == serverID {
					lb = l4lb
					break
				}
			}
		})
	}
	if lb == nil {
		return fmt.Errorf("Load Balancer with server ID %d not found", serverID)
	}
	file.AddRef()
	return sendWithChunk(lb, func(chunkInfo protocol.ChunkInfo) *protocol.ControlMessage {
		return protocol.TransferFile(path, permission, chunkInfo)
	}, file)
}

func (c *controller) WasmInstall(id uint32, method, path string, file ReaderAtCloser) error {
	var l7lblist *l7list
	c.withLock(func() {
		l7lblist = c.l7lbList.Clone()
	})
	for _, l7lb := range l7lblist.list {
		file.AddRef() // Ensure the file remains open until the message is sent
		go func(l7lb *L7LB) {
			if err := sendWithChunk(l7lb, func(chunkInfo protocol.ChunkInfo) *protocol.ControlMessage {
				return protocol.WasmInstall(id, method, path, chunkInfo)
			}, file); err != nil {
				l7lb.logger.Error("Failed to send WasmInstall message", "error", err, "id", id, "method", method, "path", path)
			}
		}(l7lb)
	}
	return nil
}

func (c *controller) WasmUninstall(id uint32) error {
	msg := protocol.WasmUninstall(id)
	var seqNum uint64
	var l7lblist *l7list
	c.withLock(func() {
		seqNum = c.msgSeqNum
		c.msgSeqNum++
		l7lblist = c.l7lbList.Clone()
	})
	for _, l7lb := range l7lblist.list {
		l7lb.SendMessage(message{
			seqNum: seqNum,
			data:   msg,
		})
	}
	return nil
}

type CommandLine interface {
	Stdin() io.WriteCloser
	Stderr() <-chan []byte
	Stdout() <-chan []byte
	ResizeWindow(x, y int) error
	Kill() error
	Signal(signal int) error // argument is signal number
}

type lbSender interface {
	GetSeqNum() uint64
	SendMessageBlocking(msg message) error
}

type commandLine struct {
	c            *controller
	lb           lbSender
	id           uint32
	outbuf       chan []byte
	errbuf       chan []byte
	closeOutOnce sync.Once
}

type stdinWriter struct {
	id uint32
	s  lbSender
}

func (s *stdinWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil // No data to write
	}
	const inputLimit = 65535 - 6 // 6 bytes for command ID and length
	for len(p) > inputLimit {
		chunk := p[:inputLimit]
		p = p[inputLimit:]
		if err := s.s.SendMessageBlocking(message{
			seqNum: s.s.GetSeqNum(),
			data:   protocol.CommandLineIn(s.id, string(chunk)),
		}); err != nil {
			return 0, err
		}
	}
	if len(p) > 0 {
		if err := s.s.SendMessageBlocking(message{
			seqNum: s.s.GetSeqNum(),
			data:   protocol.CommandLineIn(s.id, string(p)),
		}); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func (c *stdinWriter) Close() error {
	if err := c.s.SendMessageBlocking(message{
		seqNum: c.s.GetSeqNum(),
		data:   protocol.CommandLineInstruction(c.id, protocol.CmdInstructionType_CloseStdin, 0),
	}); err != nil {
		return err
	}
	return nil
}

func (c *commandLine) Stdout() <-chan []byte {
	return c.outbuf
}
func (c *commandLine) Stderr() <-chan []byte {
	return c.errbuf
}

func (c *commandLine) Stdin() io.WriteCloser {
	return &stdinWriter{s: c.lb, id: c.id}
}

func (c *commandLine) ResizeWindow(col, row int) error {
	if col <= 0 || row <= 0 {
		return fmt.Errorf("invalid window size: %dx%d", col, row)
	}
	if col > 65535 || row > 65535 {
		return fmt.Errorf("window size exceeds maximum: %dx%d", col, row)
	}
	if err := c.lb.SendMessageBlocking(message{
		seqNum: c.lb.GetSeqNum(),
		data:   protocol.CommandLineResize(c.id, uint16(col), uint16(row)),
	}); err != nil {
		return fmt.Errorf("failed to send resize command: %w", err)
	}
	return nil
}

func (c *commandLine) Signal(signal int) error {
	if signal < 0 || signal > 127 {
		return fmt.Errorf("invalid signal number: %d", signal)
	}
	if err := c.lb.SendMessageBlocking(message{
		seqNum: c.lb.GetSeqNum(),
		data:   protocol.CommandLineInstruction(c.id, protocol.CmdInstructionType_Kill, byte(signal+1)), // signal + 1 to match protocol
	}); err != nil {
		return fmt.Errorf("failed to send signal command: %w", err)
	}
	return nil
}

func (c *commandLine) Kill() error {
	// before killing, close stdin to ensure no more input is sent
	if err := c.lb.SendMessageBlocking(message{
		seqNum: c.lb.GetSeqNum(),
		data:   protocol.CommandLineInstruction(c.id, protocol.CmdInstructionType_CloseStdin, 0),
	}); err != nil {
		return fmt.Errorf("failed to send kill command: %w", err)
	}
	if err := c.lb.SendMessageBlocking(message{
		seqNum: c.lb.GetSeqNum(),
		data:   protocol.CommandLineInstruction(c.id, protocol.CmdInstructionType_Kill, 0),
	}); err != nil {
		return fmt.Errorf("failed to send kill command: %w", err)
	}
	return nil
}

func (c *commandLine) closeStdouterr() {
	c.closeOutOnce.Do(func() {
		close(c.outbuf)
		close(c.errbuf)
	})
}

type commandLineManager struct {
	mu       sync.Mutex
	cmdlines map[uint32]*commandLine
	seqNum   uint32
}

func newCommandLineManager() *commandLineManager {
	return &commandLineManager{
		cmdlines: make(map[uint32]*commandLine),
		seqNum:   1, // Start from 1 to avoid zero ID
	}
}

func (c *commandLineManager) HandleOutput(logger *slog.Logger, id uint32, outType protocol.OutputType, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cmd, ok := c.cmdlines[id]; ok {
		switch outType {
		case protocol.OutputType_Stdout:
			cmd.outbuf <- data
		case protocol.OutputType_Stderr:
			cmd.errbuf <- data
		default:
			return fmt.Errorf("unknown output type: %v", outType)
		}
		return nil
	}
	logger.Warn("Command line not found", "id", id, "output_type", outType)
	return nil // not found is not error (for resetting)
}

func (c *commandLineManager) HandleExit(logger *slog.Logger, id uint32, exitCode int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cmd, ok := c.cmdlines[id]; ok {
		cmd.closeStdouterr() // Close the output channels
		delete(c.cmdlines, id)
		logger.Info("Command line exited", "id", id, "exit_code", exitCode)
		return nil
	}
	logger.Warn("Command line not found for exit", "id", id, "exit_code", exitCode)
	return nil // not found is not error (for resetting)
}

func (c *commandLineManager) command(ctl *controller, serverID uint32, lb lbSender, cmdline string, enablePty bool) (CommandLine, error) {
	c.mu.Lock()
	cmd := &commandLine{
		c:      ctl,
		lb:     lb,
		id:     c.seqNum,
		outbuf: make(chan []byte),
		errbuf: make(chan []byte),
	}
	c.cmdlines[c.seqNum] = cmd
	cmd.id = c.seqNum
	c.seqNum++
	c.mu.Unlock()
	if enablePty {
		if err := lb.SendMessageBlocking(message{
			seqNum: lb.GetSeqNum(),
			data:   protocol.CommandLineInstruction(cmd.id, protocol.CmdInstructionType_UsePty, 0),
		}); err != nil {
			return nil, fmt.Errorf("failed to send use pty command: %w", err)
		}
	}
	lb.SendMessageBlocking(message{
		seqNum: lb.GetSeqNum(),
		data:   protocol.CommandLineIn(cmd.id, cmdline),
	})
	return cmd, nil
}

func (c *controller) Command(typ LBType, serverID uint32, cmdline string, enablePty bool) (CommandLine, error) {
	var lb lbSender
	switch typ {
	case LBTypeL7:
		var l7lb *L7LB
		c.withLock(func() {
			for _, l7lbConn := range c.l7lbList.list {
				if l7lbConn.data.data.Data.Data.ServerID == serverID {
					l7lb = l7lbConn
					break
				}
			}
		})
		if l7lb == nil {
			return nil, fmt.Errorf("L7 Load Balancer with server ID %d not found", serverID)
		}
		lb = l7lb
	case LBTypeL4:
		var l4lb *L4LB
		c.withLock(func() {
			for _, l4lbConn := range c.l4lblist.list {
				if l4lbConn.data.data.Data.Data.ServerID == serverID {
					l4lb = l4lbConn
					break
				}
			}
		})
		if l4lb == nil {
			return nil, fmt.Errorf("L4 Load Balancer with server ID %d not found", serverID)
		}
		lb = l4lb
	default:
		return nil, fmt.Errorf("unknown Load Balancer type: %v", typ)
	}
	return c.commandManager.command(c, serverID, lb, cmdline, enablePty)
}

func (c *controller) KillAll(typ LBType, serverID uint32) error {
	var lb lbSender
	switch typ {
	case LBTypeL7:
		var l7lb *L7LB
		c.withLock(func() {
			for _, l7lbConn := range c.l7lbList.list {
				if l7lbConn.data.data.Data.Data.ServerID == serverID {
					l7lb = l7lbConn
					break
				}
			}
		})
		if l7lb == nil {
			return fmt.Errorf("L7 Load Balancer with server ID %d not found", serverID)
		}
		lb = l7lb
	case LBTypeL4:
		var l4lb *L4LB
		c.withLock(func() {
			for _, l4lbConn := range c.l4lblist.list {
				if l4lbConn.data.data.Data.Data.ServerID == serverID {
					l4lb = l4lbConn
					break
				}
			}
		})
		if l4lb == nil {
			return fmt.Errorf("L4 Load Balancer with server ID %d not found", serverID)
		}
		lb = l4lb
	default:
		return fmt.Errorf("unknown Load Balancer type: %v", typ)
	}
	return lb.SendMessageBlocking(message{
		seqNum: lb.GetSeqNum(),
		data:   protocol.CommandLineInstruction(0, protocol.CmdInstructionType_KillAll, 0),
	})
}
