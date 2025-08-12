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

	"github.com/yzp0n/ncdn/controller/api"
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
	teeWriter   io.Writer
}

func (lw *lockedWriter) Write(p []byte) (n int, err error) {
	if lw.teeWriter != nil {
		n, err = lw.teeWriter.Write(p)
		if err != nil {
			return n, err
		}
	}
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
var logFile = flag.String("logFile", "", "Path to the log file")

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
	if *logFile != "" {
		file, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.Fatalf("Failed to open log file: %v", err)
		}
		lw.teeWriter = file
		defer file.Close()
	}
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

	uploaderManager := api.NewUploaderManager()
	http.HandleFunc("POST /upload", func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read payload: "+err.Error(), http.StatusBadRequest)
			return
		}
		id := uploaderManager.AddPayload(payload)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]uint64{"id": id}); err != nil {
			http.Error(w, "Failed to encode response: "+err.Error(), http.StatusInternalServerError)
			return
		}
	})
	http.HandleFunc("POST /file/transfer", func(w http.ResponseWriter, r *http.Request) {
		info := &api.FileUploadBody{}
		if err := json.NewDecoder(r.Body).Decode(info); err != nil {
			http.Error(w, "Failed to decode request body: "+err.Error(), http.StatusBadRequest)
			return
		}
		file, exists := uploaderManager.GetPayload(info.FileID)
		if !exists {
			http.Error(w, "File not found or expired", http.StatusNotFound)
			return
		}
		if err := controller.FileTransfer(&info.Dest, info.Path, info.Mode, control.NewReaderAtCloser(io.NewSectionReader(bytes.NewReader(file), 0, int64(len(file))))); err != nil {
			http.Error(w, "Failed to transfer file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	http.HandleFunc("POST /wasm/install", func(w http.ResponseWriter, r *http.Request) {
		info := &api.WasmInstallBody{}
		if err := json.NewDecoder(r.Body).Decode(info); err != nil {
			http.Error(w, "Failed to decode request body: "+err.Error(), http.StatusBadRequest)
			return
		}
		file, exists := uploaderManager.GetPayload(info.FileID)
		if !exists {
			http.Error(w, "File not found or expired", http.StatusNotFound)
			return
		}
		if err := controller.WasmInstall(&info.Dest, info.WasmID, info.Method, info.Path, control.NewReaderAtCloser(io.NewSectionReader(bytes.NewReader(file), 0, int64(len(file))))); err != nil {
			http.Error(w, "Failed to install WASM: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	http.HandleFunc("POST /wasm/uninstall", func(w http.ResponseWriter, r *http.Request) {
		info := &api.WasmInstallBody{}
		if err := json.NewDecoder(r.Body).Decode(info); err != nil {
			http.Error(w, "Failed to decode request body: "+err.Error(), http.StatusBadRequest)
			return
		}
		err = controller.WasmUninstall(&info.Dest, info.WasmID)
		if err != nil {
			http.Error(w, "Failed to uninstall WASM: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	http.HandleFunc("POST /vip/update", func(w http.ResponseWriter, r *http.Request) {
		var update api.VIPUpdate
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			http.Error(w, "Failed to decode request body: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err := controller.UpdateVIP(&update.Dest, update.VIP); err != nil {
			http.Error(w, "Failed to update VIP: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	if err := controller.Run(ctx, lis); err != nil {
		log.Fatalf("Controller run failed: %v", err)
	}
}
