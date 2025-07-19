package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/yzp0n/ncdn/httprps"
	"github.com/yzp0n/ncdn/types"
)

var originURLStr = flag.String("originURL", "http://localhost:8888", "Origin server URL")
var listenAddr = flag.String("listenAddr", ":8889", "Address to listen on")
var nodeId = flag.String("nodeId", "unknown_node", "Name of the node")
var lbNodeId = flag.Int("lbNodeId", 0, "Node ID for load balancer (0 for default)")
var certFile = flag.String("certFile", "ca/cert.pem", "Path to the TLS certificate file")
var keyFile = flag.String("keyFile", "ca/key.pem", "Path to the TLS key file")
var sharedSecret = flag.String("sharedSecret", "shared_secret", "Shared secret for QUIC LB connection ID generation(TODO: move into secure place)")

func main() {
	flag.Parse()

	originURL, err := url.Parse(*originURLStr)
	if err != nil {
		log.Fatalf("Failed to parse origin URL %q: %v", *originURLStr, err)
	}

	start := time.Now()

	mux := http.NewServeMux()
	rps := httprps.NewMiddleware(mux)
	http.Handle("/", rps)

	mux.HandleFunc("/statusz", func(w http.ResponseWriter, r *http.Request) {
		s := types.PoPStatus{
			Id:     *nodeId,
			Uptime: time.Since(start).Seconds(),
			Load:   rps.GetRPS(),
		}
		bs, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			log.Printf("Failed to marshal PoP status: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write(bs)
	})
	mux.HandleFunc("/latencyz", func(w http.ResponseWriter, r *http.Request) {
		// return 204
		w.WriteHeader(http.StatusNoContent)
	})
	mux.Handle("/", &httputil.ReverseProxy{
		// FIXME: actually cache stuff...
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetXForwarded()
			r.Out.Header.Set("X-NCDN-PoPCache-NodeId", *nodeId)
			r.SetURL(originURL)
		},
	})

	cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		log.Fatalf("Failed to load TLS certificate and key: %v", err)
	}

	srv := &http3.Server{
		Addr:    *listenAddr,
		Handler: mux,
	}

	pkt, err := net.ListenPacket("udp4", *listenAddr)

	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", *listenAddr, err)
	}

	if *lbNodeId < 0 || *lbNodeId > 15 {
		log.Fatalf("lbNodeId must be between 0 and 15, got %d", *lbNodeId)
	}

	tr := &quic.Transport{
		Conn:                  pkt,
		ConnectionIDLength:    20,
		ConnectionIDGenerator: NewQUICLBConnIDGenerator(uint8(*lbNodeId), []byte(*sharedSecret)),
	}

	qlis, err := tr.Listen(http3.ConfigureTLSConfig(&tls.Config{Certificates: []tls.Certificate{cert}}), &quic.Config{})
	if err != nil {
		log.Fatalf("Failed to start QUIC listener: %v", err)
	}

	go func() {
		err := srv.ServeListener(qlis)
		if err != nil {
			log.Fatalf("Failed to serve QUIC listener: %v", err)
		}
	}()

	log.Printf("Listening on %s...", *listenAddr)
	if err := http.ListenAndServe(*listenAddr, nil); err != nil {
		log.Fatal(err)
	}
}
