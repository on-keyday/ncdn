package lb

import (
	"time"

	"github.com/yzp0n/ncdn/controller/protocol"
	"github.com/yzp0n/ncdn/controller/stat"
	"github.com/yzp0n/ncdn/controller/transport"
)

type LoadBalancer interface {
	KeepAlive(t time.Duration) error
	Receive() (*protocol.ControlMessage, error)
	Close() error
}

type LBState[T any, U any] struct {
	Data          *T
	Conn          transport.Connection
	makeKeepAlive func(t time.Duration, m *protocol.MachineStat, data U) (*protocol.ControlMessage, error)
}

func connectLB[T any, U any](conn transport.Connection,
	hello func(data *T, machine *protocol.MachineData) *protocol.ControlMessage,
	getAppStats func() (U, error),
	makeKeepAlive func(t time.Duration, machine *protocol.MachineStat, data U) (*protocol.ControlMessage, error),
	data *T,
	keepalive time.Duration) (*LBState[T, U], error) {
	mdata, err := stat.GetMachineData()
	if err != nil {
		return nil, err
	}
	enc, err := hello(data, mdata).Encode()
	if err != nil {
		return nil, err
	}
	err = conn.Send(enc)
	if err != nil {
		return nil, err
	}
	lb := &LBState[T, U]{Data: data, Conn: conn, makeKeepAlive: makeKeepAlive}
	go func() {
		if keepalive < 10*time.Second {
			keepalive = 10 * time.Second
		}
		ticker := time.NewTicker(keepalive - 5*time.Second)
		defer ticker.Stop()
		for range ticker.C {
			m, err := stat.GetMachineStat()
			if err != nil {
				conn.Close()
				return
			}
			u, err := getAppStats()
			if err != nil {
				conn.Close()
				return
			}
			if err := lb.keepAlive(keepalive, m, u); err != nil {
				conn.Close()
				return
			}
		}
	}()
	return lb, nil
}

func makeL4LBKeepAlive(t time.Duration, m *protocol.MachineStat, data *protocol.L4UpdateInfo) (*protocol.ControlMessage, error) {
	return protocol.L4LBKeepAlive(t, m, data), nil
}

func makeL7LBKeepAlive(t time.Duration, m *protocol.MachineStat, data *protocol.L7UpdateInfo) (*protocol.ControlMessage, error) {
	return protocol.L7LBKeepAlive(t, m, data), nil
}

func ConnectL4LB(conn transport.Connection, data *protocol.L4LBData, keepalive time.Duration, appStat func() (*protocol.L4UpdateInfo, error)) (*LBState[protocol.L4LBData, *protocol.L4UpdateInfo], error) {
	return connectLB(conn, protocol.L4LBHello, appStat, makeL4LBKeepAlive, data, keepalive)
}

func ConnectL7LB(conn transport.Connection, data *protocol.L7LBData, keepalive time.Duration, appStat func() (*protocol.L7UpdateInfo, error)) (*LBState[protocol.L7LBData, *protocol.L7UpdateInfo], error) {
	return connectLB(conn, protocol.L7LBHello, appStat, makeL7LBKeepAlive, data, keepalive)
}

func (lb *LBState[T, U]) keepAlive(t time.Duration, m *protocol.MachineStat, u U) error {
	msg, err := lb.makeKeepAlive(t, m, u)
	if err != nil {
		return err
	}
	enc, err := msg.Encode()
	if err != nil {
		return err
	}
	return lb.Conn.Send(enc)
}

func (lb *LBState[T, U]) Receive() (*protocol.ControlMessage, error) {
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

func (lb *LBState[T, U]) Close() error {
	if lb.Conn == nil {
		return nil
	}
	err := lb.Conn.Close()
	lb.Conn = nil // Prevent double close
	return err
}
