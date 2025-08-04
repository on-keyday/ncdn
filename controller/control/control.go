package control

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/yzp0n/ncdn/controller/protocol"
)

type Connection interface {
	Send([]byte) error
	Receive() ([]byte, error)
	SetDeadline(time.Time) error
	Close() error
	RemoteAddr() string
}
type Listener interface {
	Accept() (Connection, error)
	Close() error
}

type Controller struct {
	lock     sync.RWMutex
	l4lb     *L4LB // currently only one L4LB is supported
	l7lbList []*L7LB
	loger    slog.Logger
}

func (c *Controller) withLock(fn func()) {
	c.lock.Lock()
	defer c.lock.Unlock()
	fn()
}

type L4LB struct {
	conn Connection
	data *protocol.L4LBData
}

type L7LB struct {
	conn Connection
	data *protocol.L7LBData
}

func (c *Controller) Run(lis Listener) error {
	for {
		conn, err := lis.Accept()
		if err != nil {
			// Handle error (e.g., log it, close listener, etc.)
			continue
		}
		go c.handleConnection(conn)
	}
}

func (c *Controller) handleConnection(conn Connection) {
	defer conn.Close()
	c.loger.Info("New connection established", "remote_addr", conn.RemoteAddr())
	if err := conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		c.loger.Error("Failed to set deadline", "error", err)
		return
	}
	data, err := conn.Receive()
	if err != nil {
		c.loger.Error("Failed to receive data", "error", err)
		return
	}
	msg := &protocol.ControlMessage{}
	err = msg.DecodeExact(data)
	if err != nil {
		c.loger.Error("Failed to decode message", "error", err)
		return
	}
	switch msg.Header.MessageType {
	case protocol.ControlMessageType_L4LbHello:
		l4lbData := msg.L4LbHello()
		var err error
		c.withLock(func() {
			if c.l4lb != nil {
				err = errors.New("L4LB already exists")
				return
			}
			c.l4lb = &L4LB{
				conn: conn,
				data: protocol.L4LBInfoToData(l4lbData),
			}
		})
		if err != nil {
			c.loger.Error("Failed to set L4LB", "error", err)
			return
		}
	case protocol.ControlMessageType_L7LbHello:
		l7lbData := msg.L7LbHello()
		l7lb := &L7LB{
			conn: conn,
			data: protocol.L7LBInfoToData(l7lbData),
		}
		c.withLock(func() {
			c.l7lbList = append(c.l7lbList, l7lb)
		})
	default:
		c.loger.Error("Unknown message type", "type", msg.Header.MessageType)
		return
	}
}

func (c *Controller) handleL4LB(l4lb *L4LB) {
	// Handle L4LB specific logic here
	c.loger.Info("Handling L4LB", "data", l4lb.data)
	defer func() {
		c.withLock(func() {
			c.l4lb = nil // Clear L4LB after handling
		})
	}()
	for {
		data, err := l4lb.conn.Receive()
		if err != nil {
			c.loger.Error("Failed to receive data from L4LB", "error", err)
			return
		}
		msg := &protocol.ControlMessage{}
		err = msg.DecodeExact(data)
		if err != nil {
			c.loger.Error("Failed to decode message from L4LB", "error", err)
			return
		}
		c.loger.Info("Received message from L4LB", "message_type", msg.Header.MessageType)
		if msg.Header.MessageType != protocol.ControlMessageType_Keepalive {
			c.loger.Error("Unexpected message type from L4LB", "type", msg.Header.MessageType)
			return
		}
	}
}
