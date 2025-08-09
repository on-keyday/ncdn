package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
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
var serverKey = flag.String("serverKey", "", "Path to the server TLS key file")
var serverCert = flag.String("serverCert", "", "Path to the server TLS certificate file")
var enableTLS = flag.Bool("enableTLS", false, "Enable TLS for the controller")
var clientCertRoot = flag.String("clientCertRoot", "", "Path to the client certificate root CA file")

func main() {
	flag.Parse()
	var tlsConfig *tls.Config
	if *enableTLS {
		var err error
		cert, err := tls.LoadX509KeyPair(*serverCert, *serverKey)
		if err != nil {
			log.Fatalf("Failed to load TLS config: %v", err)
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
		if *clientCertRoot != "" {
			caCert, err := os.ReadFile(*clientCertRoot)
			if err != nil {
				log.Fatalf("Failed to read client certificate root CA file: %v", err)
			}
			certPool := x509.NewCertPool()
			if !certPool.AppendCertsFromPEM(caCert) {
				log.Fatalf("Failed to append client certificate root CA")
			}
			tlsConfig.ClientAuth = tls.VerifyClientCertIfGiven
			tlsConfig.ClientCAs = certPool
		}
	}
	lis, err := wstransport.NewWebSocketListener(*port, tlsConfig)
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
	http.HandleFunc("POST /file", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		lbType := query.Get("lbType")
		if lbType == "" {
			http.Error(w, "lbType query parameter is required", http.StatusBadRequest)
			return
		}
		serverID := query.Get("serverID")
		if serverID == "" {
			http.Error(w, "serverID query parameter is required", http.StatusBadRequest)
			return
		}
		path := query.Get("path")
		if path == "" {
			http.Error(w, "path query parameter is required", http.StatusBadRequest)
			return
		}
		permission := query.Get("permission")
		if permission == "" {
			http.Error(w, "permission query parameter is required", http.StatusBadRequest)
			return
		}
		parsedPermission, err := strconv.ParseUint(permission, 8, 16)
		if err != nil {
			http.Error(w, "permission must be an octal number", http.StatusBadRequest)
			return
		}
		parsedServerID, err := strconv.Atoi(serverID)
		if err != nil {
			http.Error(w, "serverID must be an integer", http.StatusBadRequest)
			return
		}
		binaryData, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read file data: "+err.Error(), http.StatusInternalServerError)
			return
		}
		r.Body.Close() // Close the body to prevent resource leaks
		err = controller.FileTransfer(control.LBType(lbType), uint32(parsedServerID), path, uint16(parsedPermission), control.NewSectionReader(io.NewSectionReader(bytes.NewReader(binaryData), 0, int64(len(binaryData)))))
		if err != nil {
			http.Error(w, "Failed to transfer file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	http.HandleFunc("POST /wasm", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		id := query.Get("id")
		if id == "" {
			http.Error(w, "id query parameter is required", http.StatusBadRequest)
			return
		}
		method := query.Get("method")
		if method == "" {
			http.Error(w, "method query parameter is required", http.StatusBadRequest)
			return
		}
		path := query.Get("path")
		if path == "" {
			http.Error(w, "path query parameter is required", http.StatusBadRequest)
			return
		}
		parsedID, err := strconv.Atoi(id)
		if err != nil {
			http.Error(w, "id must be an integer", http.StatusBadRequest)
			return
		}
		binaryData, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body: "+err.Error(), http.StatusInternalServerError)
			return
		}
		r.Body.Close()
		err = controller.WasmInstall(uint32(parsedID), method, path, control.NewSectionReader(io.NewSectionReader(bytes.NewReader(binaryData), 0, int64(len(binaryData)))))
		if err != nil {
			http.Error(w, "Failed to install WASM: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	http.HandleFunc("DELETE /wasm", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		id := query.Get("id")
		if id == "" {
			http.Error(w, "id query parameter is required", http.StatusBadRequest)
			return
		}
		parsedID, err := strconv.Atoi(id)
		if err != nil {
			http.Error(w, "id must be an integer", http.StatusBadRequest)
			return
		}
		err = controller.WasmUninstall(uint32(parsedID))
		if err != nil {
			http.Error(w, "Failed to uninstall WASM: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	if err := controller.Run(ctx, lis); err != nil {
		log.Fatalf("Controller run failed: %v", err)
	}
}
