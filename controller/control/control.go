package control

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yzp0n/ncdn/controller/protocol"
	"github.com/yzp0n/ncdn/controller/transport"
)

type ControllerStatus struct {
	L4LBData []*protocol.L4LBControlState
	L7LBData []*protocol.L7LBControlState
}

type LBType string

const (
	LBTypeL4 LBType = "L4LB"
	LBTypeL7 LBType = "L7LB"
)

type DestEntry struct {
	ServerIDs []uint32 `json:"server_ids,omitempty"`
	Broadcast bool     `json:"broadcast,omitempty"` // explicitly, if this true, serverIDs must be empty
	LBType    LBType   `json:"lb_type"`
}

type DestInfo struct {
	DestEntries []DestEntry `json:"dests"`
}

type Controller interface {
	FileTransfer(dest *DestInfo, path string, permission uint16, file ReaderAtCloser) error
	WasmInstall(dest *DestInfo, wasmID uint32, method, path string, file ReaderAtCloser) error
	WasmUninstall(dest *DestInfo, wasmID uint32) error
	ShareKey([]byte)
	Run(context.Context, transport.Listener) error
	Status() *ControllerStatus
	KillAll(typ LBType, serverID uint32) error
	Command(typ LBType, serverID uint32, cmdline string, enablePty bool) (CommandLine, error)
}

func (c *controller) Status() *ControllerStatus {
	s := &ControllerStatus{}
	var l7list *l7list
	var l4list *l4list
	c.withLock(func() {
		l4list = c.l4lblist.Clone()
		l7list = c.l7lbList.Clone()
	})
	for _, l4lb := range l4list.list {
		l4lb.data.WithLock(func(data *protocol.L4LBControlState) {
			s.L4LBData = append(s.L4LBData, data.Clone())
		})
	}
	for _, l7lb := range l7list.list {
		l7lb.data.WithLock(func(data *protocol.L7LBControlState) {
			s.L7LBData = append(s.L7LBData, data.Clone())
		})
	}
	return s
}

func NewController(logger *slog.Logger) Controller {
	c := &controller{
		logger:         logger,
		l4lblist:       &l4list{},
		l7lbList:       &l7list{},
		commandManager: newCommandLineManager(),
	}
	return c
}

type sharedKey struct {
	generation uint64
	key        []byte
}

type ReaderAtCloser interface {
	io.ReaderAt
	io.Closer
	Size() int64
	AddRef()
}

// seqNum has two types:
// 1. entire LBs, used for shared key and L4/L7 LB updates
// 2. per LB connection, used for command line sequence numbers
type message struct {
	seqNum uint64
	data   *protocol.ControlMessage
	reader ReaderAtCloser
}

// invoke the cleanup function
func (m *message) Close(logger *slog.Logger) {
	if m.reader != nil {
		if err := m.reader.Close(); err != nil {
			logger.Error("Failed to close reader", "error", err)
		}
	}
}

type generationList[T comparable] struct {
	generation uint64
	list       []T
}

func (l *generationList[T]) Clone() *generationList[T] {
	clone := &generationList[T]{
		generation: l.generation,
		list:       make([]T, len(l.list)),
	}
	copy(clone.list, l.list)
	return clone
}

func (l *generationList[T]) Sort(compare func(a, b T) int) {
	slices.SortFunc(l.list, compare)
}

func (l *generationList[T]) Add(generation uint64, item T) {
	l.generation = generation
	l.list = append(l.list, item)
}

func (l *generationList[T]) Remove(item T) {
	for i, v := range l.list {
		if v == item {
			l.list = append(l.list[:i], l.list[i+1:]...)
			return
		}
	}
}

type l7list = generationList[*L7LB]
type l4list = generationList[*L4LB]

type controller struct {
	lock      sync.Mutex
	l4lblist  *l4list
	l7lbList  *l7list
	logger    *slog.Logger
	sharedKey sharedKey
	msgSeqNum uint64

	commandManager *commandLineManager
}

// multi producer, single consumer message channel
// because latestSeqPerEvent is only updated by the consumer,
// it is not necessary to lock it
type messageChannel struct {
	messageChan       chan message
	ctx               context.Context
	cancel            context.CancelCauseFunc
	cancelLock        sync.RWMutex
	closed            sync.Once
	latestSeqPerEvent map[protocol.ControlMessageType]uint64
	senderWg          sync.WaitGroup
	logger            *slog.Logger
}

func (c *messageChannel) Logger() *slog.Logger {
	return c.logger
}

func (c *messageChannel) CloseChannel() {
	c.closed.Do(func() {
		c.cancelLock.Lock()
		c.cancel(ErrChannelClosed) // Cancel the context to stop the goroutine
		c.cancelLock.Unlock()
		c.senderWg.Wait()
		close(c.messageChan) // Close the message channel after all senders are done
	})
}

var ErrChannelClosed = errors.New("message channel closed")

func (c *messageChannel) ReceiveMessage() (message, error) {
	for msg := range c.messageChan {
		if latest, ok := c.latestSeqPerEvent[msg.data.Header.MessageType]; ok && msg.seqNum <= latest {
			continue // Skip messages with sequence number less than or equal to the latest
		}
		c.latestSeqPerEvent[msg.data.Header.MessageType] = msg.seqNum
		return msg, nil
	}
	return message{}, ErrChannelClosed // Return error if the channel is closed
}

func (c *messageChannel) SendMessage(msg message) error {
	c.cancelLock.RLock()
	select {
	case <-c.ctx.Done():
		c.cancelLock.RUnlock()
		msg.Close(c.logger) // Clean up the message if the context is done
		return c.ctx.Err()
	default:
	}
	c.senderWg.Add(1)
	c.cancelLock.RUnlock()
	go func() {
		defer c.senderWg.Done()
		select {
		case c.messageChan <- msg:
		case <-c.ctx.Done():
			msg.Close(c.logger) // Clean up the message if the context is done
			return
		}
	}()
	return nil
}

func (c *messageChannel) SendMessageBlocking(msg message) error {
	c.cancelLock.RLock()
	select {
	case <-c.ctx.Done():
		c.cancelLock.RUnlock()
		msg.Close(c.logger) // Clean up the message if the context is done
		return c.ctx.Err()
	default:
	}
	c.senderWg.Add(1)
	c.cancelLock.RUnlock()
	defer c.senderWg.Done()
	select {
	case c.messageChan <- msg:
	case <-c.ctx.Done():
		msg.Close(c.logger) // Clean up the message if the context is done
		return c.ctx.Err()
	}
	return nil
}

type WithLockedData[T any] struct {
	data *T
	lock sync.Mutex
}

func NewWithLockedData[T any](data *T) *WithLockedData[T] {
	return &WithLockedData[T]{
		data: data,
	}
}

func (wd *WithLockedData[T]) WithLock(fn func(*T)) {
	wd.lock.Lock()
	defer wd.lock.Unlock()
	fn(wd.data)
}

type LBConn[T any] struct {
	conn transport.Connection
	data *WithLockedData[T]
	messageChannel
	seq atomic.Uint64 // for command line sequence numbers
}

// sequence number per lbconnection
func (lb *LBConn[T]) GetSeqNum() uint64 {
	return lb.seq.Add(1) // Increment and return the sequence number
}

type L4LB = LBConn[protocol.L4LBControlState]
type L7LB = LBConn[protocol.L7LBControlState]

func (l *LBConn[T]) Close() error {
	l.CloseChannel()
	return l.conn.Close()
}

func NewLBConn[T any](ctx context.Context, conn transport.Connection, data *T, logger *slog.Logger) *LBConn[T] {
	ctx, cancel := context.WithCancelCause(ctx)
	return &LBConn[T]{
		conn: conn,
		data: NewWithLockedData(data),
		messageChannel: messageChannel{
			messageChan:       make(chan message, 100),
			ctx:               ctx,
			cancel:            cancel,
			latestSeqPerEvent: make(map[protocol.ControlMessageType]uint64),
			logger:            logger,
		},
	}
}

func (c *controller) withLock(fn func()) {
	c.lock.Lock()
	defer c.lock.Unlock()
	fn()
}

func (c *controller) Run(ctx context.Context, lis transport.Listener) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(errors.New("controller stopped"))
	go func() {
		<-ctx.Done()
		if err := lis.Close(); err != nil {
			c.logger.Error("Failed to close listener", "error", err)
		}
	}()
	c.logger.Info("Controller started, waiting for connections", "addr", lis.LocalAddr())
	for {
		conn, err := lis.Accept()
		if err != nil {
			c.logger.Error("Failed to accept connection", "error", err)
			continue
		}
		go c.handleConnection(ctx, conn)
	}
}

func sendKeyShare[T any](lbConn *LBConn[T], key []byte, seqNum uint64) {
	if err := lbConn.SendMessage(message{
		seqNum: seqNum,
		data:   protocol.KeyShare(protocol.KeyType_Quiclb, key),
	}); err != nil {
		lbConn.logger.Error("Failed to send KeyShare message", "error", err)
	}
}

func sendCommandLineReset[T any](lbConn *LBConn[T]) {
	if err := lbConn.SendMessage(message{
		seqNum: lbConn.GetSeqNum(),
		data:   protocol.CommandLineInstruction(0, protocol.CmdInstructionType_KillAll, 0),
	}); err != nil {
		lbConn.logger.Error("Failed to send CommandLineKill message", "error", err)
	}
}

func sendL4L7LBUpdate(lbConn *L4LB, data []*protocol.L7LBData, seqNum uint64) {
	if err := lbConn.SendMessage(message{
		seqNum: seqNum,
		data:   protocol.L4L7LBUpdate(data),
	}); err != nil {
		lbConn.logger.Error("Failed to send L4L7LBUpdate message", "error", err)
	}
}

func readControlMessage(conn transport.Connection) (*protocol.ControlMessage, int, error) {
	data, err := conn.Receive()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to receive data: %w", err)
	}
	msg := &protocol.ControlMessage{}
	if err := msg.DecodeExact(data); err != nil {
		return nil, 0, fmt.Errorf("failed to decode control message: %w", err)
	}
	return msg, len(data), nil
}

func (c *controller) handleConnection(ctx context.Context, conn transport.Connection) {
	defer conn.Close()
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(errors.New("connection closed"))
	go func() {
		<-ctx.Done()
		if err := conn.Close(); err != nil {
			c.logger.Error("Failed to close connection", "error", err)
		}
	}()
	c.logger.Info("New connection established", "remote_addr", conn.RemoteAddr())
	if err := conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		c.logger.Error("Failed to set deadline", "error", err)
		return
	}
	msg, _, err := readControlMessage(conn)
	if err != nil {
		c.logger.Error("Failed to read control message", "error", err)
		return
	}
	switch msg.Header.MessageType {
	case protocol.ControlMessageType_L4LbHello:
		l4lbInfo := msg.L4LbHello()
		var err error
		l4lb := NewLBConn(ctx, conn, protocol.L4LBHelloToControlState(l4lbInfo),
			c.logger.With("server_id", l4lbInfo.Info.ServerId, "remote_addr", conn.RemoteAddr()))
		var updateInfo []*protocol.L7LBData
		var l7generation uint64
		var sharedKey sharedKey
		c.withLock(func() {
			for _, v := range c.l4lblist.list {
				if v.data.data.Data.Data.ServerID == l4lb.data.data.Data.Data.ServerID {
					err = errors.New("L4LB with this ServerID already exists")
					return
				}
			}
			c.l4lblist.Add(c.msgSeqNum, l4lb)
			c.msgSeqNum++
			for _, v := range c.l7lbList.list {
				updateInfo = append(updateInfo, &v.data.data.Data.Data)
			}
			l7generation = c.l7lbList.generation
			sharedKey = c.sharedKey
		})
		if err != nil {
			l4lb.logger.Error("Failed to set L4LB", "error", err)
			return
		}
		sendCommandLineReset(l4lb) // Reset command line state
		sendKeyShare(l4lb, sharedKey.key, sharedKey.generation)
		sendL4L7LBUpdate(l4lb, updateInfo, l7generation)
		c.handleL4LB(l4lb)
	case protocol.ControlMessageType_L7LbHello:
		l7lbInfo := msg.L7LbHello()
		l7lb := NewLBConn(ctx, conn, protocol.L7LBHelloToControlState(l7lbInfo),
			c.logger.With("server_id", l7lbInfo.Info.ServerId, "remote_addr", conn.RemoteAddr()))
		var err error
		var data []*protocol.L7LBData
		var sharedKey sharedKey
		var l4lbList *l4list
		var l7generation uint64
		c.withLock(func() {
			for _, v := range c.l7lbList.list {
				if v.data.data.Data.Data.ServerID == l7lb.data.data.Data.Data.ServerID {
					err = errors.New("L7LB with this ServerID already exists")
					return
				}
			}
			c.l7lbList.Add(c.msgSeqNum, l7lb)
			c.msgSeqNum++
			c.l7lbList.Sort(func(a, b *L7LB) int {
				return int(a.data.data.Data.Data.ServerID) - int(b.data.data.Data.Data.ServerID)
			})
			for _, v := range c.l7lbList.list {
				data = append(data, &v.data.data.Data.Data)
			}
			l4lbList = c.l4lblist.Clone()
			l7generation = c.l7lbList.generation
			sharedKey = c.sharedKey
		})
		if err != nil {
			l7lb.logger.Error("Failed to set L7LB", "error", err)
			return
		}
		sendCommandLineReset(l7lb) // Reset command line state
		sendKeyShare(l7lb, sharedKey.key, sharedKey.generation)
		for _, l4lb := range l4lbList.list {
			sendL4L7LBUpdate(l4lb, data, l7generation)
		}
		c.handleL7LB(l7lb)
	default:
		c.logger.Error("Unknown message type", "type", msg.Header.MessageType)
		return
	}
}

func handleLBConn[T any](c *controller, lbType string, lbConn *LBConn[T], handleKeepAlive func(msg *protocol.ControlMessage) (time.Duration, error), clean func()) {
	lbConn.logger.Info("LB Connected", "type", lbType)
	defer lbConn.Close()
	defer clean()
	go func() {
		defer lbConn.Close()
		for {
			msg, err := lbConn.ReceiveMessage()
			if err != nil {
				if errors.Is(err, ErrChannelClosed) {
					lbConn.logger.Info("Controller message channel closed")
				} else {
					lbConn.logger.Error("Failed to receive message from controller", "error", err)
				}
				return
			}
			handle := func() bool {
				defer msg.Close(lbConn.logger) // Ensure the reader is closed after sending
				enc, err := msg.data.Encode()
				if err != nil {
					lbConn.logger.Error("Failed to encode message from controller", "error", err)
					return false
				}
				if msg.reader != nil {
					if err := lbConn.conn.SendReader(enc, int(msg.reader.Size()), msg.reader); err != nil {
						lbConn.logger.Error("Failed to send message with reader to LB", "error", err)
						return false
					}
				} else {
					if err := lbConn.conn.Send(enc); err != nil {
						lbConn.logger.Error("Failed to send message to LB", "error", err)
						return false
					}
				}
				return true
			}
			if !handle() {
				return
			}
			lbConn.logger.Info("Sent message to LB", "message_type", msg.data.Header.MessageType, "seq_num", msg.seqNum)
		}
	}()
	for {
		msg, len, err := readControlMessage(lbConn.conn)
		if err != nil {
			lbConn.logger.Error("Failed to read message from LB", "error", err)
			return
		}
		lbConn.logger.Info("Received message from LB", "message_type", msg.Header.MessageType, "len", len)
		if msg := msg.Message(); msg != nil {
			msgStr := string(msg.Msg)
			switch msg.Level {
			case protocol.LogLevel_Trace:
				lbConn.logger.Debug("Received message from LB", "message", msgStr)
			case protocol.LogLevel_Debug:
				lbConn.logger.Debug("Received message from LB", "message", msgStr)
			case protocol.LogLevel_Info:
				lbConn.logger.Info("Received message from LB", "message", msgStr)
			case protocol.LogLevel_Warn:
				lbConn.logger.Warn("Received message from LB", "message", msgStr)
			case protocol.LogLevel_Error:
				lbConn.logger.Error("Received message from LB", "message", msgStr)
			default:
				lbConn.logger.Error("Received message from LB with unknown log level", "message", msgStr, "level", msg.Level)
			}
			continue
		}
		if msg := msg.CmdlineOut(); msg != nil {
			if msg.ChunkInfo.IsChunkd() {
				lbConn.logger.Error("Received chunked command line output from LB, but chunked output is not supported in this version")
				return
			}
			data, err := lbConn.conn.ReceiveSize(int(msg.ChunkInfo.LenOrId()))
			if err != nil {
				lbConn.logger.Error("Failed to receive command line output from LB", "error", err)
				return
			}
			err = c.commandManager.HandleOutput(lbConn.logger, msg.CmdlineId, msg.OutputType, data)
			if err != nil {
				lbConn.logger.Error("Failed to handle command line output", "error", err, "cmdline_id", msg.CmdlineId, "output_type", msg.OutputType)
				return
			}
			continue
		}
		if msg := msg.CmdlineExit(); msg != nil {
			err := c.commandManager.HandleExit(lbConn.logger, msg.CmdlineId, int(msg.ExitCode))
			if err != nil {
				lbConn.logger.Error("Failed to handle command line exit", "error", err, "cmdline_id", msg.CmdlineId, "exit_code", msg.ExitCode)
				return
			}
			continue
		}
		if nextPeriod, err := handleKeepAlive(msg); err == nil {
			lbConn.conn.SetReadDeadline(time.Now().Add(time.Duration(nextPeriod)))
		} else {
			lbConn.logger.Error("Failed to handle keep alive", "error", err)
			return
		}
	}
}

func (c *controller) handleL7LB(l7lb *L7LB) {
	handleLBConn(c, "L7LB", l7lb, func(msg *protocol.ControlMessage) (time.Duration, error) {
		kl := msg.L7LbKeepAlive()
		if kl != nil {
			l7lb.data.WithLock(func(data *protocol.L7LBControlState) {
				protocol.UpdateL7WithKeepAlive(kl, data)
			})
			return time.Duration(kl.Info.NextPeriod), nil
		}
		return 0, fmt.Errorf("unexpected message for L7LB: %v", msg.Header.MessageType)
	}, func() {
		var data []*protocol.L7LBData
		var l4lblist *l4list
		var l7generation uint64
		c.withLock(func() {
			c.l7lbList.Remove(l7lb)
			for _, v := range c.l7lbList.list {
				data = append(data, &v.data.data.Data.Data)
			}
			c.l7lbList.generation = c.msgSeqNum
			c.msgSeqNum++
			l7generation = c.l7lbList.generation
			l4lblist = c.l4lblist.Clone()
		})
		l7lb.logger.Info("L7LB disconnected")
		for _, l4lb := range l4lblist.list {
			sendL4L7LBUpdate(l4lb, data, l7generation)
		}
	})
}

func (c *controller) handleL4LB(l4lb *L4LB) {
	handleLBConn(c, "L4LB", l4lb, func(msg *protocol.ControlMessage) (time.Duration, error) {
		kl := msg.L4LbKeepAlive()
		if kl != nil {
			l4lb.data.WithLock(func(data *protocol.L4LBControlState) {
				protocol.UpdateL4WithKeepAlive(kl, data)
			})
			return time.Duration(kl.Info.NextPeriod), nil
		}
		return 0, fmt.Errorf("unexpected message for L4LB: %v", msg.Header.MessageType)
	}, func() {
		c.withLock(func() {
			c.l4lblist.Remove(l4lb) // Remove the L4LB from the list
		})
		l4lb.logger.Info("L4LB disconnected", "remote_addr", l4lb.conn.RemoteAddr())
	})
}
