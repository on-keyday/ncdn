package control

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/yzp0n/ncdn/controller/protocol"
	"github.com/yzp0n/ncdn/controller/transport"
)

type ControllerStatus struct {
	L4LBData []*protocol.L4LBControlState
	L7LBData []*protocol.L7LBControlState
}

type Controller interface {
	ShareKey([]byte)
	Run(context.Context, transport.Listener) error
	Status() *ControllerStatus
}

func (c *controller) Status() *ControllerStatus {
	s := &ControllerStatus{}
	var l7list []*protocol.L7LBControlState
	var l4list []*protocol.L4LBControlState
	c.withLock(func() {
		l7list = make([]*protocol.L7LBControlState, len(c.l7lbList.list))
		for i, v := range c.l7lbList.list {
			v.data.WithLock(func(data *protocol.L7LBControlState) {
				l7list[i] = data.Clone()
			})
		}
		l4list = make([]*protocol.L4LBControlState, len(c.l4lblist.list))
		for i, v := range c.l4lblist.list {
			v.data.WithLock(func(data *protocol.L4LBControlState) {
				l4list[i] = data.Clone()
			})
		}
	})
	s.L4LBData = l4list
	s.L7LBData = l7list
	return s
}

func NewController(logger *slog.Logger) Controller {
	c := &controller{
		logger:   logger,
		l4lblist: &l4list{},
		l7lbList: &l7list{},
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
	c.senderWg.Add(1)
	go func() {
		defer c.senderWg.Done()
		select {
		case c.messageChan <- msg:
		case <-c.ctx.Done():
			return
		}
	}()
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
	logger *slog.Logger
}

type L4LB = LBConn[protocol.L4LBControlState]
type L7LB = LBConn[protocol.L7LBControlState]

func (l *LBConn[T]) Close() error {
	l.CloseChannel(l.logger)
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
	var l4lblist *l4list
	var l7lbList *l7list
	c.withLock(func() {
		c.sharedKey.generation = c.msgSeqNum
		c.sharedKey.key = key
		c.msgSeqNum++
		l4lblist = c.l4lblist.Clone()
		l7lbList = c.l7lbList.Clone()
	})
	for _, l4lb := range l4lblist.list {
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

func (c *controller) Run(ctx context.Context, lis transport.Listener) error {
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

func sendL4L7LBUpdate(lbConn *L4LB, data []*protocol.L7LBData, seqNum uint64) {
	if err := lbConn.SendMessage(message{
		seqNum: seqNum,
		data:   protocol.L4L7LBUpdate(data),
	}); err != nil {
		lbConn.logger.Error("Failed to send L4L7LBUpdate message", "error", err)
	}
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

func handleLBConn[T any](lbType string, lbConn *LBConn[T], handleKeepAlive func(msg *protocol.ControlMessage) (time.Duration, error), clean func()) {
	lbConn.logger.Info("LB Connected", "data", lbConn.data, "type", lbType)
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
			enc, err := msg.data.Encode()
			if err != nil {
				lbConn.logger.Error("Failed to encode message from controller", "error", err)
				return
			}
			if err := lbConn.conn.Send(enc); err != nil {
				lbConn.logger.Error("Failed to send message to LB", "error", err)
				return
			}
			lbConn.logger.Info("Sent message to LB", "message_type", msg.data.Header.MessageType, "seq_num", msg.seqNum)
		}
	}()
	for {
		data, err := lbConn.conn.Receive()
		if err != nil {
			lbConn.logger.Error("Failed to receive data from LB", "error", err)
			return
		}
		msg := &protocol.ControlMessage{}
		err = msg.DecodeExact(data)
		if err != nil {
			lbConn.logger.Error("Failed to decode message from LB", "error", err)
			return
		}
		lbConn.logger.Info("Received message from LB", "message_type", msg.Header.MessageType)
		if nextPeriod, err := handleKeepAlive(msg); err == nil {
			lbConn.conn.SetReadDeadline(time.Now().Add(time.Duration(nextPeriod)))
		} else {
			lbConn.logger.Error("Failed to handle keep alive", "error", err)
			return
		}
	}
}

func (c *controller) handleL7LB(l7lb *L7LB) {
	handleLBConn("L7LB", l7lb, func(msg *protocol.ControlMessage) (time.Duration, error) {
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
	handleLBConn("L4LB", l4lb, func(msg *protocol.ControlMessage) (time.Duration, error) {
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
