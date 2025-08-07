package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"net/url"
	"time"

	"github.com/yzp0n/ncdn/controller/lbconn"
	"github.com/yzp0n/ncdn/controller/protocol"
	wstransport "github.com/yzp0n/ncdn/controller/transport/websocket"
	"golang.org/x/net/websocket"
)

func mustParseURL(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return u
}

var isL7 = flag.Bool("l7", false, "Use L7 load balancer instead of L4")
var id = flag.Int("id", 1, "Server ID for the load balancer")

func main() {
	flag.Parse()
	conn, err := wstransport.Connect(context.Background(), &websocket.Config{
		Location: mustParseURL("ws://localhost:8080"),
		Origin:   mustParseURL("http://localhost:8080"),
		Version:  websocket.ProtocolVersionHybi13,
	})
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	var clb lbconn.LoadBalancer
	if *isL7 {
		clb, err = lbconn.ConnectL7LB(slog.Default(), conn, &protocol.L7LBData{
			ServerID: uint32(*id),
		}, 20*time.Second, func() (*protocol.L7UpdateInfo, error) {
			return &protocol.L7UpdateInfo{}, nil
		})
	} else {
		clb, err = lbconn.ConnectL4LB(slog.Default(), conn, &protocol.L4LBData{
			ServerID: uint32(*id),
		}, 20*time.Second, func() (*protocol.L4UpdateInfo, error) {
			return &protocol.L4UpdateInfo{}, nil
		})
	}
	if err != nil {
		panic(err)
	}
	defer clb.Close()
	for {
		msg, err := clb.Receive()
		if err != nil {
			log.Printf("Error receiving message: %v", err)
			break
		}
		log.Printf("Received message: %v", msg)
	}
}
