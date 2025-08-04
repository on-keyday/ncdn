package control

import (
	"errors"
	"log/slog"
	"slices"
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
	lock             sync.RWMutex
	l4lb             *L4LB // currently only one L4LB is supported
	l7lbList         []*L7LB
	loger            slog.Logger
	notifyL4L7Update chan []*protocol.L7LBData
}

func (c *Controller) handleNotify() {
	for data := range c.notifyL4L7Update {
		msg := protocol.L4L7LBUpdate(data)
		enc, err := msg.Encode()
		if err != nil {
			c.loger.Error("Failed to encode L4L7LBUpdate message", "error", err)
			continue
		}
		var l4lb *L4LB
		c.withLock(func() {
			l4lb = c.l4lb
		})
		if l4lb != nil {
			if err := l4lb.conn.Send(enc); err != nil {
				c.loger.Error("Failed to send L4L7LBUpdate message", "error", err)
			} else {
				c.loger.Info("Sent L4L7LBUpdate message", "data", data)
			}
		} else {
			c.loger.Warn("No L4LB connection available to send update")
		}
	}
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
			c.loger.Error("Failed to accept connection", "error", err)
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
		l4lb := &L4LB{
			conn: conn,
			data: protocol.L4LBInfoToData(l4lbData),
		}
		var updateInfo []*protocol.L7LBData
		c.withLock(func() {
			if c.l4lb != nil {
				err = errors.New("L4LB already exists")
				return
			}
			c.l4lb = l4lb
			for _, v := range c.l7lbList {
				updateInfo = append(updateInfo, v.data)
			}
		})
		if err != nil {
			c.loger.Error("Failed to set L4LB", "error", err)
			return
		}
		c.notifyL4L7Update <- updateInfo
		c.handleL4LB(l4lb)
	case protocol.ControlMessageType_L7LbHello:
		l7lbData := msg.L7LbHello()
		l7lb := &L7LB{
			conn: conn,
			data: protocol.L7LBInfoToData(l7lbData),
		}
		var err error
		var data []*protocol.L7LBData
		c.withLock(func() {
			for _, v := range c.l7lbList {
				if v.data.ServerID == l7lb.data.ServerID {
					err = errors.New("L7LB with this ServerID already exists")
					return
				}
			}
			c.l7lbList = append(c.l7lbList, l7lb)
			slices.SortFunc(c.l7lbList, func(a, b *L7LB) int {
				return int(a.data.ServerID) - int(b.data.ServerID)
			})
			for _, v := range c.l7lbList {
				data = append(data, v.data)
			}
		})
		if err != nil {
			c.loger.Error("Failed to set L7LB", "error", err)
			return
		}
		c.notifyL4L7Update <- data
		c.handleL7LB(l7lb)
	default:
		c.loger.Error("Unknown message type", "type", msg.Header.MessageType)
		return
	}
}

func (c *Controller) handleL7LB(l7lb *L7LB) {
	// Handle L7LB specific logic here
	c.loger.Info("L7LB Connected", "data", l7lb.data)
	defer func() {
		c.withLock(func() {
			for i, v := range c.l7lbList {
				if v == l7lb {
					c.l7lbList = append(c.l7lbList[:i], c.l7lbList[i+1:]...)
					break
				}
			}
		})
		c.loger.Info("L7LB disconnected", "remote_addr", l7lb.conn.RemoteAddr())
	}()
	for {
		data, err := l7lb.conn.Receive()
		if err != nil {
			c.loger.Error("Failed to receive data from L7LB", "error", err)
			return
		}
		msg := &protocol.ControlMessage{}
		err = msg.DecodeExact(data)
		if err != nil {
			c.loger.Error("Failed to decode message from L7LB", "error", err)
			return
		}
		c.loger.Info("Received message from L7LB", "message_type", msg.Header.MessageType)
		if kl := msg.KeepAlive(); kl != nil {
			l7lb.conn.SetDeadline(time.Now().Add(time.Duration(kl.NextPeriod)))
		} else {
			c.loger.Error("Unexpected message type from L7LB", "type", msg.Header.MessageType)
			return
		}
	}
}

func (c *Controller) handleL4LB(l4lb *L4LB) {
	// Handle L4LB specific logic here
	c.loger.Info("L4LB Connected", "data", l4lb.data)
	defer func() {
		c.withLock(func() {
			c.l4lb = nil // Clear L4LB after handling
		})
		c.loger.Info("L4LB disconnected", "remote_addr", l4lb.conn.RemoteAddr())
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
		kl := msg.KeepAlive()
		if kl == nil {
			c.loger.Error("Unexpected message type from L4LB", "type", msg.Header.MessageType)
			return
		}
		l4lb.conn.SetDeadline(time.Now().Add(time.Duration(kl.NextPeriod)))
	}
}
