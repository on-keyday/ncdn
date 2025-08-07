package lbconn

import (
	"log"
	"log/slog"
	"time"

	"github.com/yzp0n/ncdn/controller/protocol"
	"github.com/yzp0n/ncdn/controller/stat"
	"github.com/yzp0n/ncdn/controller/transport"
)

type LoadBalancer interface {
	Receive() (*protocol.ControlMessage, error)
	Close() error
}

type LBState[T any, U any] struct {
	Data          T
	Conn          transport.Connection
	makeKeepAlive func(t time.Duration, m *protocol.MachineStat, data U) (*protocol.ControlMessage, error)
}

func connectLB[T any, U any](logger *slog.Logger, conn transport.Connection,
	hello func(data T, machine *protocol.MachineData) *protocol.ControlMessage,
	getAppStats func() (U, error),
	makeKeepAlive func(t time.Duration, machine *protocol.MachineStat, data U) (*protocol.ControlMessage, error),
	data T,
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
		doSendKeepAlive := func() bool {
			m, err := stat.GetMachineStat()
			if err != nil {
				logger.Info("Failed to get machine stat", "error", err)
				conn.Close()
				return false
			}
			u, err := getAppStats()
			if err != nil {
				logger.Info("Failed to get app stats", "error", err)
				conn.Close()
				return false
			}
			if err := lb.keepAlive(keepalive, m, u); err != nil {
				log.Printf("Failed to send keepalive: %v", err)
				conn.Close()
				return false
			}
			return true
		}
		if !doSendKeepAlive() {
			return
		}
		ticker := time.NewTicker(keepalive - 5*time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if !doSendKeepAlive() {
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

func ConnectL4LB(logger *slog.Logger, conn transport.Connection, data *protocol.L4LBData, keepalive time.Duration, appStat func() (*protocol.L4UpdateInfo, error)) (*LBState[*protocol.L4LBData, *protocol.L4UpdateInfo], error) {
	return connectLB(logger, conn, protocol.L4LBHello, appStat, makeL4LBKeepAlive, data, keepalive)
}

func ConnectL7LB(logger *slog.Logger, conn transport.Connection, data *protocol.L7LBData, keepalive time.Duration, appStat func() (*protocol.L7UpdateInfo, error)) (*LBState[*protocol.L7LBData, *protocol.L7UpdateInfo], error) {
	return connectLB(logger, conn, protocol.L7LBHello, appStat, makeL7LBKeepAlive, data, keepalive)
}

type LBConnConnector[T any, U any] func(logger *slog.Logger, conn transport.Connection, data T, keepalive time.Duration, appStat func() (U, error)) (*LBState[T, U], error)

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

type RetriableLBConn[T any, U any] struct {
	connect    LBConnConnector[T, U]
	conn       *LBState[T, U]
	createConn func() (transport.Connection, error)
	appStat    func() (U, error)
	keepalive  time.Duration
	logger     *slog.Logger
	retryWait  time.Duration
	data       T
}

func ConnectRetriableLB[T any, U any](logger *slog.Logger, retryWait time.Duration, connect LBConnConnector[T, U], data T, keepalive time.Duration, createConn func() (transport.Connection, error), appStat func() (U, error)) *RetriableLBConn[T, U] {
	// lazy initialization of conn
	return &RetriableLBConn[T, U]{connect: connect, conn: nil, createConn: createConn, appStat: appStat, keepalive: keepalive, logger: logger, retryWait: retryWait, data: data}
}

func (r *RetriableLBConn[T, U]) Receive() (*protocol.ControlMessage, error) {
	for {
		if r.conn == nil {
			conn, err := r.createConn()
			if err != nil {
				r.logger.Error("Failed to create connection", "error", err)
				time.Sleep(r.retryWait)
				continue
			}
			r.conn, err = r.connect(r.logger, conn, r.data, r.keepalive, r.appStat)
			if err != nil {
				r.logger.Error("Failed to connect to load balancer", "error", err)
				conn.Close()
				time.Sleep(r.retryWait)
				continue
			}
			continue
		}
		recved, err := r.conn.Receive()
		if err != nil {
			if r.conn.Conn != nil {
				r.conn.Close()
			}
			r.conn = nil // Reset connection to allow retry
			r.logger.Error("Failed to receive message from load balancer", "error", err)
			time.Sleep(r.retryWait)
			continue
		}
		return recved, nil
	}
}

func (r *RetriableLBConn[T, U]) Close() error {
	if r.conn == nil {
		return nil
	}
	err := r.conn.Close()
	r.conn = nil // Prevent double close
	return err
}
