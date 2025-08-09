package main

import (
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"

	"golang.org/x/net/websocket"
)

func mustParseURL(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return u
}

var lbType = flag.String("lbType", "L4LB", "Load balancer type (L4LB or L7LB)")
var serverID = flag.Int("serverID", 1, "Server ID for the load balancer")

func main() {
	flag.Parse()
	consoleURL := fmt.Sprintf("ws://localhost:8080/console?lbType=%s&serverID=%d", *lbType, *serverID)
	ws, err := websocket.Dial(consoleURL, "", "http://localhost:8080")
	if err != nil {
		panic(err)
	}
	defer ws.Close()
	fmt.Print("command> ")
	go func() {
		io.Copy(ws, os.Stdin)
	}()
	io.Copy(os.Stdout, ws)
}
