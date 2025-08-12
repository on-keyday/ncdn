package transport

import (
	"io"
	"time"
)

type Connection interface {
	Send([]byte) error
	SendReader(control []byte, size int, reader io.ReaderAt) error
	Receive() ([]byte, error)
	ReceiveSize(size int) ([]byte, error)
	SetReadDeadline(time.Time) error
	Close() error
	RemoteAddr() string
	MutualSecured() bool // Indicates if the connection is secured with mutual TLS
}

type Listener interface {
	Accept() (Connection, error)
	Close() error
	LocalAddr() string
}
