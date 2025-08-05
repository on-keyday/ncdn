package transport

import (
	"io"
	"time"
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
