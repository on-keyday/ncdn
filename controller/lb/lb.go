package lb

import (
	"time"

	"github.com/yzp0n/ncdn/controller/protocol"
	"github.com/yzp0n/ncdn/controller/transport"
)

type LBState[T any] struct {
	Data *T
	Conn transport.Connection
}

func connectLB[T any](conn transport.Connection, hello func(data *T) *protocol.ControlMessage, data *T, keepalive time.Duration) (*LBState[T], error) {
	enc, err := hello(data).Encode()
	if err != nil {
		return nil, err
	}
	err = conn.Send(enc)
	if err != nil {
		return nil, err
	}
	lb := &LBState[T]{Data: data, Conn: conn}
	go func() {
		if keepalive < 10*time.Second {
			keepalive = 10 * time.Second
		}
		ticker := time.NewTicker(keepalive - 5*time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := lb.KeepAlive(keepalive); err != nil {
				conn.Close()
				return
			}
		}
	}()
	return lb, nil
}

func ConnectL4LB(conn transport.Connection, data *protocol.L4LBData, keepalive time.Duration) (*LBState[protocol.L4LBData], error) {
	return connectLB(conn, protocol.L4LBHello, data, keepalive)
}

func ConnectL7LB(conn transport.Connection, data *protocol.L7LBData, keepalive time.Duration) (*LBState[protocol.L7LBData], error) {
	return connectLB(conn, protocol.L7LBHello, data, keepalive)
}

func (lb *LBState[T]) KeepAlive(t time.Duration) error {
	enc, err := protocol.KeepAlive(t).Encode()
	if err != nil {
		return err
	}
	return lb.Conn.Send(enc)
}

func (lb *LBState[T]) Receive() (*protocol.ControlMessage, error) {
	enc, err := lb.Conn.Receive()
	if err != nil {
		return nil, err
	}
	msg := &protocol.ControlMessage{}
	err = msg.DecodeExact(enc)
	if err != nil {
		return nil, err
	}
	return msg, nil
}
