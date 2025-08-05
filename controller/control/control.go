package control

import (
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
	SetDeadline(time.Time) error
	Close() error
	RemoteAddr() string
}

type Listener interface {
	Accept() (Connection, error)
	Close() error
}

type Controller interface {
	ShareKey([]byte)
	Run(Listener) error
}

func NewController(logger slog.Logger) Controller {
	c := &controller{
		logger: logger,
	}
	return c
}

type sharedKey struct {
	generation uint64
	key        []byte
}

type message struct {
	seqNum uint64
	data   *protocol.ControlMessage
}

type l7List struct {
	generation uint64
	list       []*L7LB
}

type controller struct {
	lock      sync.Mutex
	l4lb      *L4LB // currently only one L4LB is supported
	l7lbList  l7List
	logger    slog.Logger
	sharedKey sharedKey
	msgSeqNum uint64
}

type messageChannel struct {
	messageChan       chan message
	closedChan        chan struct{}
	closed            sync.Once
	latestSeqPerEvent map[protocol.ControlMessageType]uint64
}

func (c *messageChannel) CloseChannel() {
	c.closed.Do(func() {
		close(c.closedChan)
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
		case <-c.closedChan:
			return message{}, ErrChannelClosed
		}
	}
}

func (c *messageChannel) SendMessage(msg message) error {
	select {
	case c.messageChan <- msg:
	case <-c.closedChan:
		return ErrChannelClosed
	}
	return nil
}

type LBConn[T any] struct {
	conn Connection
	data *T
	messageChannel
}

type L4LB = LBConn[protocol.L4LBData]
type L7LB = LBConn[protocol.L7LBData]

func (l *LBConn[T]) Close() error {
	l.CloseChannel()
	return l.conn.Close()
}

func NewL4LB(conn Connection, data *protocol.L4LBData) *L4LB {
	return &L4LB{
		conn: conn,
		data: data,
		messageChannel: messageChannel{
			messageChan:       make(chan message, 100),
			closedChan:        make(chan struct{}),
			latestSeqPerEvent: make(map[protocol.ControlMessageType]uint64),
		},
	}
}

func NewL7LB(conn Connection, data *protocol.L7LBData) *L7LB {
	return &L7LB{
		conn: conn,
		data: data,
		messageChannel: messageChannel{
			messageChan:       make(chan message, 100),
			closedChan:        make(chan struct{}),
			latestSeqPerEvent: make(map[protocol.ControlMessageType]uint64),
		},
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

func (c *controller) Run(lis Listener) error {
	for {
		conn, err := lis.Accept()
		if err != nil {
			c.logger.Error("Failed to accept connection", "error", err)
			continue
		}
		go c.handleConnection(conn)
	}
}

func (c *controller) handleConnection(conn Connection) {
	defer conn.Close()
	c.logger.Info("New connection established", "remote_addr", conn.RemoteAddr())
	if err := conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
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
		l4lb := NewL4LB(conn, protocol.L4LBInfoToData(l4lbInfo))
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
			c.logger.Error("Failed to set L4LB", "error", err)
			return
		}
		l4lb.SendMessage(message{
			seqNum: sharedKey.generation,
			data:   protocol.KeyShare(protocol.KeyType_Quiclb, sharedKey.key),
		})
		l4lb.SendMessage(message{
			seqNum: l7generation,
			data:   protocol.L4L7LBUpdate(updateInfo),
		})
		c.handleL4LB(l4lb)
	case protocol.ControlMessageType_L7LbHello:
		l7lbInfo := msg.L7LbHello()
		l7lb := NewL7LB(conn, protocol.L7LBInfoToData(l7lbInfo))
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
			c.logger.Error("Failed to set L7LB", "error", err)
			return
		}
		l7lb.SendMessage(message{
			seqNum: l7generation,
			data:   protocol.KeyShare(protocol.KeyType_Quiclb, sharedKey.key),
		})
		if l4lb != nil {
			l4lb.SendMessage(message{
				seqNum: l7generation,
				data:   protocol.L4L7LBUpdate(data),
			})
		}
		c.handleL7LB(l7lb)
	default:
		c.logger.Error("Unknown message type", "type", msg.Header.MessageType)
		return
	}
}

func handleLBConn[T any](c *controller, lbType string, lbConn *LBConn[T], clean func()) {
	c.logger.Info("LB Connected", "data", lbConn.data, "type", lbType)
	defer lbConn.Close()
	defer clean()
	go func() {
		defer lbConn.Close()
		for {
			msg, err := lbConn.ReceiveMessage()
			if err != nil {
				if errors.Is(err, ErrChannelClosed) {
					c.logger.Info("L7LB message channel closed")
				} else {
					c.logger.Error("Failed to receive message from L7LB", "error", err)
				}
				return
			}
			enc, err := msg.data.Encode()
			if err != nil {
				c.logger.Error("Failed to encode message from L7LB", "error", err)
				return
			}
			if err := lbConn.conn.Send(enc); err != nil {
				c.logger.Error("Failed to send message to L7LB", "error", err)
				return
			}
			c.logger.Info("Sent message to L7LB", "message_type", msg.data.Header.MessageType, "seq_num", msg.seqNum)
		}
	}()
	for {
		data, err := lbConn.conn.Receive()
		if err != nil {
			c.logger.Error("Failed to receive data from L7LB", "error", err)
			return
		}
		msg := &protocol.ControlMessage{}
		err = msg.DecodeExact(data)
		if err != nil {
			c.logger.Error("Failed to decode message from L7LB", "error", err)
			return
		}
		c.logger.Info("Received message from L7LB", "message_type", msg.Header.MessageType)
		if kl := msg.KeepAlive(); kl != nil {
			lbConn.conn.SetDeadline(time.Now().Add(time.Duration(kl.NextPeriod)))
		} else {
			c.logger.Error("Unexpected message type from L7LB", "type", msg.Header.MessageType)
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
		c.logger.Info("L7LB disconnected", "remote_addr", l7lb.conn.RemoteAddr())
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
		c.logger.Info("L4LB disconnected", "remote_addr", l4lb.conn.RemoteAddr())
	})
}
