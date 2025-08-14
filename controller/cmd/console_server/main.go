package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/url"
	"time"

	"github.com/yzp0n/ncdn/controller/chunk"
	"github.com/yzp0n/ncdn/controller/file"
	"github.com/yzp0n/ncdn/controller/lbconn"
	"github.com/yzp0n/ncdn/controller/protocol"
	"github.com/yzp0n/ncdn/controller/remoteshell"
	"github.com/yzp0n/ncdn/controller/transport"
	wstransport "github.com/yzp0n/ncdn/controller/transport/websocket"
	"github.com/yzp0n/ncdn/tool/util"
	"golang.org/x/net/websocket"
)

var controlPlaneAddress = flag.String("control-plane", "ws://localhost:8080/", "Control plane address")
var serverID = flag.Uint("server-id", 1, "Server ID")

func main() {
	flag.Parse()
	parsed, err := url.Parse(*controlPlaneAddress)
	if err != nil {
		log.Fatalf("Failed to parse control plane address: %v", err)
	}
	if parsed.Scheme != "ws" && parsed.Scheme != "wss" {
		log.Fatalf("Invalid control plane address scheme: %s", parsed.Scheme)
	}
	httpOrigin := *parsed
	httpOrigin.Scheme = "http"
	if parsed.Scheme == "wss" {
		httpOrigin.Scheme = "https"
	}

	ifname, _, addr, hardAddr, err := util.GetSelfIPv4Address("")
	if err != nil {
		log.Panicf("Failed to get IPv4 address for interface %s: %v", ifname, err)
	}
	retryConn := lbconn.ConnectRetriable(slog.Default(), 5*time.Second, lbconn.ConnectConsole,
		&protocol.ConsoleData{
			ServerID:   uint32(*serverID),
			Address:    addr,
			MacAddress: [6]byte(hardAddr),
		}, 20*time.Second, func() (transport.Connection, error) {
			return wstransport.Connect(context.Background(), &websocket.Config{
				Location: parsed,
				Origin:   &httpOrigin,
				Version:  websocket.ProtocolVersionHybi13,
			})
		}, func() (struct{}, error) { return struct{}{}, nil })

	cmdMgr := remoteshell.NewManager()

	chunkedMap := chunk.NewChunkMap()

	go func() {
		for r := range cmdMgr.Output() {
			if err := retryConn.SendCommandline(r); err != nil {
				slog.Error("Failed to send command line", slog.String("error", err.Error()))
			}
		}
	}()

	for {
		msg, err := retryConn.Receive()
		if err != nil {
			slog.Error("Failed to receive connection", slog.String("error", err.Error()))
			return // this is fatal, we cannot continue without a connection
		}
		if handled, err := remoteshell.DispatchMessage(cmdMgr, msg); err != nil {
			retryConn.Send(&lbconn.LogMsg{
				Level:   protocol.LogLevel_Error,
				Message: fmt.Sprintf("Failed to dispatch command line message: %v", err),
			})
		} else if handled {
			continue // Handled by cmdline, no need to process further
		}
		chunked, err := chunkedMap.ReadChunked(retryConn, msg)
		if err != nil {
			retryConn.Send(&lbconn.LogMsg{
				Level:   protocol.LogLevel_Error,
				Message: fmt.Sprintf("Failed to read chunked data: %v", err),
			})
			continue
		}
		if chunked != nil {
			if handled, path, perm, err := file.MaySaveFile(chunked); err != nil {
				retryConn.Send(&lbconn.LogMsg{
					Level:   protocol.LogLevel_Error,
					Message: fmt.Sprintf("Failed to save file: %v", err),
				})
			} else if handled {
				retryConn.Send(&lbconn.LogMsg{
					Level:   protocol.LogLevel_Info,
					Message: fmt.Sprintf("File saved to %s with permission %o", path, perm),
				})
			} else {
				retryConn.Send(&lbconn.LogMsg{
					Level:   protocol.LogLevel_Error,
					Message: fmt.Sprintf("Unhandled chunked data: %v", chunked.Msg.Header.MessageType),
				})
			}
		}
	}

}
