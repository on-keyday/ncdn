package main

import (
	"context"
	"log"
	"net/url"
	"time"

	"github.com/yzp0n/ncdn/controller/lb"
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

func main() {
	conn, err := wstransport.Connect(context.Background(), &websocket.Config{
		Location: mustParseURL("ws://localhost:8080"),
		Origin:   mustParseURL("http://localhost:8080"),
		Version:  websocket.ProtocolVersionHybi13,
	})
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	lb, err := lb.ConnectL4LB(conn, &protocol.L4LBData{
		ServerID: 1,
	}, 20*time.Second)
	if err != nil {
		panic(err)
	}
	defer lb.Conn.Close()
	for {
		msg, err := lb.Receive()
		if err != nil {
			log.Printf("Error receiving message: %v", err)
			break
		}
		log.Printf("Received message: %v", msg)
	}
}
