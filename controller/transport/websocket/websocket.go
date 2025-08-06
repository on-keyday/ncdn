package wstransport

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/yzp0n/ncdn/controller/transport"
	"golang.org/x/net/websocket"
)

// A compile-time check to ensure that *WebSocketConn implements the control.Connection interface.
var _ transport.Connection = (*WebSocketConn)(nil)

// WebSocketConn is a connection that uses a WebSocket for communication.
type WebSocketConn struct {
	conn       *websocket.Conn
	remoteAddr string
	cancel     context.CancelFunc
}

// NewWebSocketConn creates a new WebSocketConn.
func NewWebSocketConn(conn *websocket.Conn, cancel context.CancelFunc) *WebSocketConn {
	return &WebSocketConn{
		conn:       conn,
		remoteAddr: conn.Request().RemoteAddr,
		cancel:     cancel,
	}
}

// Send writes a message to the WebSocket connection.
func (c *WebSocketConn) Send(p []byte) error {
	return websocket.Message.Send(c.conn, p)
}

// SendReader sends a control message and data from a reader.
// The data from the reader will be read into memory and sent as a single message
// after the control message. The recipient must be prepared to handle these two separate messages.
func (c *WebSocketConn) SendReader(control []byte, size int, reader io.ReaderAt) error {
	// Send the control part first.
	if err := websocket.Message.Send(c.conn, control); err != nil {
		return err
	}

	// Read data from the reader and send it.
	buf := make([]byte, size)
	if _, err := reader.ReadAt(buf, 0); err != nil {
		return err
	}
	return websocket.Message.Send(c.conn, buf)
}

// Receive reads a message from the WebSocket connection.
func (c *WebSocketConn) Receive() ([]byte, error) {
	var p []byte
	err := websocket.Message.Receive(c.conn, &p)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// ReceiveReader is not implemented for WebSockets as they are message-based, not stream-based.
// It will return an error as this operation is not supported.
func (c *WebSocketConn) ReceiveReader(size int) (io.Reader, error) {
	return nil, errors.New("ReceiveReader is not supported for WebSocket connections")
}

// SetReadDeadline sets the read deadline for future read calls.
func (c *WebSocketConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

// Close closes the WebSocket connection.
func (c *WebSocketConn) Close() error {
	c.cancel()
	return c.conn.Close()
}

// RemoteAddr returns the remote network address.
func (c *WebSocketConn) RemoteAddr() string {
	return c.remoteAddr
}

// WebSocketListener implements the control.Listener interface for WebSockets.
type WebSocketListener struct {
	httpServer *http.Server
	connChan   chan transport.Connection
	once       sync.Once
}

var _ transport.Listener = (*WebSocketListener)(nil)

// NewWebSocketListener creates and starts a new WebSocketListener.
func NewWebSocketListener(addr string) (*WebSocketListener, error) {
	listener := &WebSocketListener{
		connChan: make(chan transport.Connection),
	}

	handler := websocket.Handler(func(ws *websocket.Conn) {
		ctx, cancel := context.WithCancel(ws.Request().Context())
		listener.connChan <- NewWebSocketConn(ws, cancel)
		<-ctx.Done() // Wait for the context to be done before closing the connection
	})

	listener.httpServer = &http.Server{Addr: addr}
	http.Handle("/", handler)

	go func() {
		if err := listener.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server failed: %v", err)
		}
	}()

	return listener, nil
}

// Accept waits for and returns the next connection.
func (l *WebSocketListener) Accept() (transport.Connection, error) {
	conn, ok := <-l.connChan
	if !ok {
		return nil, errors.New("listener is closed")
	}
	return conn, nil
}

// Close closes the listener.
func (l *WebSocketListener) Close() error {
	l.once.Do(func() {
		close(l.connChan)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return l.httpServer.Shutdown(ctx)
}

func Connect(ctx context.Context, conf *websocket.Config) (transport.Connection, error) {
	ws, err := conf.DialContext(ctx)
	if err != nil {
		return nil, err
	}
	cancel := func() {}
	return NewWebSocketConn(ws, cancel), nil
}
