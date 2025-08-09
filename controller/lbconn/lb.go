package lbconn

import (
	"bytes"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/yzp0n/ncdn/controller/cmdline"
	"github.com/yzp0n/ncdn/controller/protocol"
	"github.com/yzp0n/ncdn/controller/stat"
	"github.com/yzp0n/ncdn/controller/transport"
)

type LoadBalancer interface {
	Send(msg *LogMsg) error
	Receive() (*protocol.ControlMessage, error)
	Close() error
}

type LogMsg struct {
	Level   protocol.LogLevel
	Message string
}

type LBState[T any, U any] struct {
	Data          T
	Conn          transport.Connection
	CmdlineMsg    chan cmdline.OutputCommand
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
	lb := &LBState[T, U]{
		Data:          data,
		Conn:          conn,
		makeKeepAlive: makeKeepAlive,
		CmdlineMsg:    make(chan cmdline.OutputCommand, 100),
	}
	go func() {
		if keepalive < 10*time.Second {
			keepalive = 10 * time.Second
		}
		doSendKeepAlive := func() bool {
			m, err := stat.GetMachineStat()
			if err != nil {
				logger.Error("Failed to get machine stat", "error", err)
				conn.Close()
				return false
			}
			u, err := getAppStats()
			if err != nil {
				logger.Error("Failed to get app stats", "error", err)
				conn.Close()
				return false
			}
			msg, err := lb.makeKeepAlive(keepalive, m, u)
			if err != nil {
				logger.Error("Failed to make keepalive message", "error", err)
				conn.Close()
				return false
			}
			enc, err := msg.Encode()
			if err != nil {
				logger.Error("Failed to encode keepalive message", "error", err)
				conn.Close()
				return false
			}
			if err := conn.Send(enc); err != nil {
				logger.Error("Failed to send keepalive message", "error", err)
				return false
			}
			return true
		}
		if !doSendKeepAlive() {
			return
		}
		ticker := time.NewTicker(keepalive - 5*time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if !doSendKeepAlive() {
					return
				}
			case c := <-lb.CmdlineMsg:
				if enc, err := c.Msg.Encode(); err != nil {
					logger.Error("Failed to encode command line message", "error", err, "msg", c.Msg.Header.MessageType.String())
				} else {
					if len(c.Data) > 0 {
						if err := conn.SendReader(enc, len(c.Data), bytes.NewReader(c.Data)); err != nil {
							logger.Error("Failed to send command line data", "error", err)
						}
					} else {
						if err := conn.Send(enc); err != nil {
							logger.Error("Failed to send command line message", "error", err, "msg", c.Msg.Header.MessageType.String())
						}
					}
				}
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

func (lb *LBState[T, U]) Send(msg *LogMsg) error {
	if lb.Conn == nil {
		return errors.New("connection is nil")
	}
	lb.CmdlineMsg <- cmdline.OutputCommand{
		Msg: protocol.LogMessage(msg.Level, msg.Message),
	}
	return nil
}

func (lb *LBState[T, U]) SendCommandline(msg cmdline.OutputCommand) error {
	if lb.Conn == nil {
		return errors.New("connection is nil")
	}
	lb.CmdlineMsg <- msg
	return nil
}

type RetriableLBConn[T any, U any] struct {
	connect    LBConnConnector[T, U]
	connLock   sync.Mutex // Ensure thread-safe access to conn
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
		r.connLock.Lock()
		if r.conn == nil {
			r.connLock.Unlock()
			conn, err := r.createConn()
			if err != nil {
				r.logger.Error("Failed to create connection", "error", err)
				time.Sleep(r.retryWait)
				continue
			}
			newConn, err := r.connect(r.logger, conn, r.data, r.keepalive, r.appStat)
			if err != nil {
				r.logger.Error("Failed to connect to load balancer", "error", err)
				conn.Close()
				time.Sleep(r.retryWait)
				continue
			}
			r.connLock.Lock() // Re-lock to set the new connection
			if r.conn != nil {
				r.conn.Close() // Close the old connection if it exists
			}
			r.conn = newConn
			r.connLock.Unlock() // Unlock after setting the new connection
			continue
		}
		conn := r.conn
		r.connLock.Unlock()
		recved, err := conn.Receive()
		if err != nil {
			r.connLock.Lock()
			r.conn.Close()
			r.conn = nil // Reset connection to allow retry
			r.connLock.Unlock()
			r.logger.Error("Failed to receive message from load balancer", "error", err)
			time.Sleep(r.retryWait)
			continue
		}
		return recved, nil
	}
}

func (r *RetriableLBConn[T, U]) Close() error {
	return r.conn.Close()
}

func (r *RetriableLBConn[T, U]) Send(msg *LogMsg) error {
	r.connLock.Lock()
	defer r.connLock.Unlock()
	if r.conn == nil {
		return errors.New("connection is nil")
	}
	return r.conn.Send(msg)
}

func (r *RetriableLBConn[T, U]) SendCommandline(msg cmdline.OutputCommand) error {
	r.connLock.Lock()
	defer r.connLock.Unlock()
	if r.conn == nil {
		return errors.New("connection is nil")
	}
	return r.conn.SendCommandline(msg)
}
