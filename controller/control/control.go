package control

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/yzp0n/ncdn/controller/protocol"
)

type Connection interface {
	Send([]byte) error
	SendReader(control []byte, size int, reader io.ReaderAt) error
	Receive() ([]byte, error)
	ReceiveReader(size int) (io.Reader, error)
	SetReadDeadline(time.Time) error
	Close() error
	RemoteAddr() string
}

type Listener interface {
	Accept() (Connection, error)
	Close() error
}

type Controller interface {
	ShareKey([]byte)
	Run(context.Context, Listener) error
}

func NewController(logger *slog.Logger) Controller {
	c := &controller{
		logger: logger,
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
}
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

type l7List struct {
	generation uint64
	list       []*L7LB
}

type controller struct {
	lock      sync.Mutex
	l4lb      *L4LB // currently only one L4LB is supported
	l7lbList  l7List
	logger    *slog.Logger
	sharedKey sharedKey
	msgSeqNum uint64
}

// multi producer, single consumer message channel
// because latestSeqPerEvent is only updated by the consumer,
// it is not necessary to lock it
type messageChannel struct {
	messageChan       chan message
	ctx               context.Context
	cancel            context.CancelCauseFunc
	closed            sync.Once
	latestSeqPerEvent map[protocol.ControlMessageType]uint64
	senderWg          sync.WaitGroup
}

func (c *messageChannel) CloseChannel(logger *slog.Logger) {
	c.closed.Do(func() {
		c.cancel(ErrChannelClosed) // Cancel the context to stop the goroutine
		c.senderWg.Wait()
		close(c.messageChan) // Close the message channel after all senders are done
		for r := range c.messageChan {
			r.Close(logger)
		}
	})
}

var ErrChannelClosed = errors.New("message channel closed")

func (c *messageChannel) ReceiveMessage() (message, error) {
	for {
		select {
		case msg := <-c.messageChan:
			if latest, ok := c.latestSeqPerEvent[msg.data.Header.MessageType]; ok && msg.seqNum <= latest {
				continue // Skip messages with sequence number less than or equal to the latest
			}
			c.latestSeqPerEvent[msg.data.Header.MessageType] = msg.seqNum
			return msg, nil
		case <-c.ctx.Done():
			return message{}, ErrChannelClosed
		}
	}
}

func (c *messageChannel) SendMessage(msg message) error {
	c.senderWg.Add(1)
	defer c.senderWg.Done()
	select {
	case <-c.ctx.Done():
		return c.ctx.Err()
	default:
	}
	select {
	case c.messageChan <- msg:
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
	return nil
}

type LBConn[T any] struct {
	conn Connection
	data *T
	messageChannel
	logger *slog.Logger
}

type L4LB = LBConn[protocol.L4LBData]
type L7LB = LBConn[protocol.L7LBData]

func (l *LBConn[T]) Close() error {
	l.CloseChannel(l.logger)
	return l.conn.Close()
}

func NewLBConn[T any](ctx context.Context, conn Connection, data *T, logger *slog.Logger) *LBConn[T] {
	ctx, cancel := context.WithCancelCause(ctx)
	return &LBConn[T]{
		conn: conn,
		data: data,
		messageChannel: messageChannel{
			messageChan:       make(chan message, 100),
			ctx:               ctx,
			cancel:            cancel,
			latestSeqPerEvent: make(map[protocol.ControlMessageType]uint64),
		},
		logger: logger,
	}
}

func (c *controller) withLock(fn func()) {
	c.lock.Lock()
	defer c.lock.Unlock()
	fn()
}

func (c *controller) ShareKey(key []byte) {
	var l4lb *L4LB
	var l7lbList l7List
	c.withLock(func() {
		c.sharedKey.generation = c.msgSeqNum
		c.sharedKey.key = key
		c.msgSeqNum++
		l4lb = c.l4lb
		l7lbList.list = append([]*L7LB{}, c.l7lbList.list...)
		l7lbList.generation = c.l7lbList.generation
	})
	if l4lb != nil {
		l4lb.SendMessage(message{
			seqNum: c.sharedKey.generation,
			data:   protocol.KeyShare(protocol.KeyType_Quiclb, c.sharedKey.key),
		})
	}
	for _, l7lb := range l7lbList.list {
		l7lb.SendMessage(message{
			seqNum: c.sharedKey.generation,
			data:   protocol.KeyShare(protocol.KeyType_Quiclb, c.sharedKey.key),
		})
	}
	c.logger.Info("Shared key with L4LB and L7LBs", "key", c.sharedKey.key, "generation", c.sharedKey.generation)
}

func (c *controller) Run(ctx context.Context, lis Listener) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(errors.New("controller stopped"))
	go func() {
		<-ctx.Done()
		if err := lis.Close(); err != nil {
			c.logger.Error("Failed to close listener", "error", err)
		}
	}()
	c.logger.Info("Controller started, waiting for connections")
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

func sendL4L7LBUpdate[T any](lbConn *LBConn[T], data []*protocol.L7LBData, seqNum uint64) {
	if err := lbConn.SendMessage(message{
		seqNum: seqNum,
		data:   protocol.L4L7LBUpdate(data),
	}); err != nil {
		lbConn.logger.Error("Failed to send L4L7LBUpdate message", "error", err)
	}
}

func (c *controller) handleConnection(ctx context.Context, conn Connection) {
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
	data, err := conn.Receive()
	if err != nil {
		c.logger.Error("Failed to receive data", "error", err)
		return
	}
	msg := &protocol.ControlMessage{}
	err = msg.DecodeExact(data)
	if err != nil {
		c.logger.Error("Failed to decode message", "error", err)
		return
	}
	switch msg.Header.MessageType {
	case protocol.ControlMessageType_L4LbHello:
		l4lbInfo := msg.L4LbHello()
		var err error
		l4lb := NewLBConn(ctx, conn, protocol.L4LBInfoToData(l4lbInfo),
			c.logger.With("server_id", l4lbInfo.ServerId, "remote_addr", conn.RemoteAddr()))
		var updateInfo []*protocol.L7LBData
		var l7generation uint64
		var sharedKey sharedKey
		c.withLock(func() {
			if c.l4lb != nil {
				err = errors.New("L4LB already exists")
				return
			}
			c.l4lb = l4lb
			for _, v := range c.l7lbList.list {
				updateInfo = append(updateInfo, v.data)
			}
			l7generation = c.l7lbList.generation
			sharedKey = c.sharedKey
		})
		if err != nil {
			l4lb.logger.Error("Failed to set L4LB", "error", err)
			return
		}
		sendKeyShare(l4lb, sharedKey.key, sharedKey.generation)
		sendL4L7LBUpdate(l4lb, updateInfo, l7generation)
		c.handleL4LB(l4lb)
	case protocol.ControlMessageType_L7LbHello:
		l7lbInfo := msg.L7LbHello()
		l7lb := NewLBConn(ctx, conn, protocol.L7LBInfoToData(l7lbInfo),
			c.logger.With("server_id", l7lbInfo.ServerId, "remote_addr", conn.RemoteAddr()))
		var err error
		var data []*protocol.L7LBData
		var sharedKey sharedKey
		var l7generation uint64
		var l4lb *L4LB
		c.withLock(func() {
			for _, v := range c.l7lbList.list {
				if v.data.ServerID == l7lb.data.ServerID {
					err = errors.New("L7LB with this ServerID already exists")
					return
				}
			}
			c.l7lbList.list = append(c.l7lbList.list, l7lb)
			c.l7lbList.generation = c.msgSeqNum
			c.msgSeqNum++
			slices.SortFunc(c.l7lbList.list, func(a, b *L7LB) int {
				return int(a.data.ServerID) - int(b.data.ServerID)
			})
			for _, v := range c.l7lbList.list {
				data = append(data, v.data)
			}
			l4lb = c.l4lb
			l7generation = c.l7lbList.generation
			sharedKey = c.sharedKey
		})
		if err != nil {
			l7lb.logger.Error("Failed to set L7LB", "error", err)
			return
		}
		sendKeyShare(l7lb, sharedKey.key, sharedKey.generation)
		if l4lb != nil {
			sendL4L7LBUpdate(l4lb, data, l7generation)
		}
		c.handleL7LB(l7lb)
	default:
		c.logger.Error("Unknown message type", "type", msg.Header.MessageType)
		return
	}
}

func handleLBConn[T any](c *controller, lbType string, lbConn *LBConn[T], clean func()) {
	lbConn.logger.Info("LB Connected", "data", lbConn.data, "type", lbType)
	defer lbConn.Close()
	defer clean()
	go func() {
		defer lbConn.Close()
		for {
			msg, err := lbConn.ReceiveMessage()
			if err != nil {
				if errors.Is(err, ErrChannelClosed) {
					lbConn.logger.Info("L7LB message channel closed")
				} else {
					lbConn.logger.Error("Failed to receive message from L7LB", "error", err)
				}
				return
			}
			enc, err := msg.data.Encode()
			if err != nil {
				lbConn.logger.Error("Failed to encode message from L7LB", "error", err)
				return
			}
			if err := lbConn.conn.Send(enc); err != nil {
				lbConn.logger.Error("Failed to send message to L7LB", "error", err)
				return
			}
			lbConn.logger.Info("Sent message to L7LB", "message_type", msg.data.Header.MessageType, "seq_num", msg.seqNum)
		}
	}()
	for {
		data, err := lbConn.conn.Receive()
		if err != nil {
			lbConn.logger.Error("Failed to receive data from L7LB", "error", err)
			return
		}
		msg := &protocol.ControlMessage{}
		err = msg.DecodeExact(data)
		if err != nil {
			lbConn.logger.Error("Failed to decode message from L7LB", "error", err)
			return
		}
		lbConn.logger.Info("Received message from L7LB", "message_type", msg.Header.MessageType)
		if kl := msg.KeepAlive(); kl != nil {
			lbConn.conn.SetReadDeadline(time.Now().Add(time.Duration(kl.NextPeriod)))
		} else {
			lbConn.logger.Error("Unexpected message type from L7LB", "type", msg.Header.MessageType)
			return
		}
	}
}

func (c *controller) handleL7LB(l7lb *L7LB) {
	handleLBConn(c, "L7LB", l7lb, func() {
		var data []*protocol.L7LBData
		var l4lb *L4LB
		var l7generation uint64
		c.withLock(func() {
			for i, v := range c.l7lbList.list {
				if v == l7lb {
					c.l7lbList.list = append(c.l7lbList.list[:i], c.l7lbList.list[i+1:]...)
					break
				}
			}
			for _, v := range c.l7lbList.list {
				data = append(data, v.data)
			}
			c.l7lbList.generation = c.msgSeqNum
			c.msgSeqNum++
			l7generation = c.l7lbList.generation
			l4lb = c.l4lb
		})
		l7lb.logger.Info("L7LB disconnected")
		if l4lb != nil {
			l4lb.SendMessage(message{
				seqNum: l7generation,
				data:   protocol.L4L7LBUpdate(data),
			})
		}
	})
}

func (c *controller) handleL4LB(l4lb *L4LB) {
	handleLBConn(c, "L4LB", l4lb, func() {
		c.withLock(func() {
			c.l4lb = nil // Clear L4LB after handling
		})
		l4lb.logger.Info("L4LB disconnected", "remote_addr", l4lb.conn.RemoteAddr())
	})
}
