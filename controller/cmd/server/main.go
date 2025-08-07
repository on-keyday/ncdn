package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yzp0n/ncdn/controller/control"
	wstransport "github.com/yzp0n/ncdn/controller/transport/websocket"
	"golang.org/x/net/websocket"
)

type subscriber struct {
	conn   *websocket.Conn
	cancel context.CancelFunc
}

type lockedWriter struct {
	l           sync.Mutex
	lines       []string
	subscribers []*subscriber
}

func (lw *lockedWriter) Write(p []byte) (n int, err error) {
	lw.l.Lock()
	defer lw.l.Unlock()
	line := string(p)
	lw.lines = append(lw.lines, line)
	if len(lw.lines) > 100 {
		lw.lines = lw.lines[len(lw.lines)-100:] // Keep only the last 100 lines
	}
	lw.broadcast(line)
	return len(p), nil
}

func (lw *lockedWriter) removeSubscriber(conn *subscriber) {
	for i, sub := range lw.subscribers {
		if sub == conn {
			lw.subscribers = append(lw.subscribers[:i], lw.subscribers[i+1:]...)
			break
		}
	}
}

func (lw *lockedWriter) broadcast(text string) {
	for _, sub := range lw.subscribers {
		if err := websocket.Message.Send(sub.conn, text); err != nil {
			log.Printf("Failed to send log to subscriber: %v", err)
			sub.conn.Close()
			sub.cancel()
			lw.removeSubscriber(sub)
		}
	}
}

func (lw *lockedWriter) SubscribeAndWait(conn *websocket.Conn) {
	lw.l.Lock()
	currentText := strings.Join(lw.lines, "")
	if err := websocket.Message.Send(conn, currentText); err != nil {
		lw.l.Unlock()
		log.Printf("Failed to send initial log: %v", err)
		conn.Close()
		return
	}
	ctx, cancel := context.WithCancel(conn.Request().Context())
	lw.subscribers = append(lw.subscribers, &subscriber{
		conn:   conn,
		cancel: cancel,
	})
	lw.l.Unlock()
	<-ctx.Done() // Wait for the context to be done before closing the connection
}

var port = flag.String("port", ":8080", "Port to run the controller on")

func main() {
	flag.Parse()
	lis, err := wstransport.NewWebSocketListener(*port)
	if err != nil {
		log.Fatalf("Failed to create WebSocket listener: %v", err)
	}
	lw := &lockedWriter{}
	h := slog.New(slog.NewTextHandler(lw, nil))
	controller := control.NewController(h)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	http.Handle("GET /log", websocket.Handler(func(c *websocket.Conn) {
		defer c.Close()
		lw.SubscribeAndWait(c)
	}))
	http.Handle("GET /status", websocket.Handler(func(c *websocket.Conn) {
		for {
			status := controller.Status()
			data, err := json.Marshal(status)
			if err != nil {
				log.Printf("Failed to marshal status: %v", err)
				return
			}
			if err := websocket.Message.Send(c, string(data)); err != nil {
				log.Printf("Failed to send status: %v", err)
				return
			}
			time.Sleep(5 * time.Second) // Send status every 5 seconds
		}
	}))
	if err := controller.Run(ctx, lis); err != nil {
		log.Fatalf("Controller run failed: %v", err)
	}
}
