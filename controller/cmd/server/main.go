package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"io"
	"log"
	"log/slog"
	"net/http"
	"strconv"
	"sync"

	"github.com/yzp0n/ncdn/controller/control"
	wstransport "github.com/yzp0n/ncdn/controller/transport/websocket"
	"golang.org/x/net/websocket"
)

type subscriber struct {
	conn   http.ResponseWriter
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
		_, err := sub.conn.Write([]byte("data: " + text + "\n"))
		if err != nil {
			sub.cancel() // Cancel the subscriber context if sending fails
			lw.removeSubscriber(sub)
			continue
		}
		http.NewResponseController(sub.conn).Flush()
	}
}

func (lw *lockedWriter) SubscribeAndWait(ctx context.Context, conn http.ResponseWriter) {
	lw.l.Lock()
	for _, line := range lw.lines {
		io.WriteString(conn, "data: "+line+"\n")
	}
	http.NewResponseController(conn).Flush()
	ctx, cancel := context.WithCancel(ctx)
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
	http.HandleFunc("GET /logs", func(w http.ResponseWriter, r *http.Request) {
		ctl := http.NewResponseController(w)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		ctl.Flush()
		lw.SubscribeAndWait(ctx, w)
	})
	http.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		status := controller.Status()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(status); err != nil {
			http.Error(w, "Failed to encode status", http.StatusInternalServerError)
			return
		}
	})
	http.Handle("GET /console", websocket.Handler(func(c *websocket.Conn) {
		query := c.Request().URL.Query()
		lbType := query.Get("lbType")
		if lbType == "" {
			c.Write([]byte("Error: lbType query parameter is required\n"))
			return
		}
		serverID := query.Get("serverID")
		if serverID == "" {
			c.Write([]byte("Error: serverID query parameter is required\n"))
			return
		}
		parsedServerID, err := strconv.Atoi(serverID)
		if err != nil {
			c.Write([]byte("Error: serverID must be an integer\n"))
			return
		}
		textScanner := bufio.NewScanner(c)
		if !textScanner.Scan() {
			c.Write([]byte("Error: Failed to read command\n"))
			return
		}
		cmdline := textScanner.Text()
		cmd, err := controller.Command(control.LBType(lbType), uint32(parsedServerID), cmdline, true)
		if err != nil {
			io.WriteString(c, "Error: "+err.Error()+"\n")
			return
		}
		defer cmd.Kill() // Ensure the command is killed when done
		go func() {
			stdin := cmd.Stdin()
			defer cmd.Kill() // Ensure the command is killed when done
			_, err = io.Copy(stdin, c)
			if err != nil {
				h.Error("Failed to copy input to command stdin", slog.String("error", err.Error()))
			}
		}()
		for out := range cmd.Stdout() {
			c.Write([]byte(out))
		}
	}))
	if err := controller.Run(ctx, lis); err != nil {
		log.Fatalf("Controller run failed: %v", err)
	}
}
