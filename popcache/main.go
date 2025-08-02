package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/tetratelabs/wazero"
	"github.com/yzp0n/ncdn/httprps"
	"github.com/yzp0n/ncdn/popcache/cache"
	lbconnid "github.com/yzp0n/ncdn/popcache/connid"
	"github.com/yzp0n/ncdn/popcache/edge"
	"github.com/yzp0n/ncdn/tool/util"
	"github.com/yzp0n/ncdn/types"
	"golang.org/x/net/http2"
	"golang.org/x/net/ipv4"
)

var originURLStr = flag.String("originURL", "http://localhost:8888", "Origin server URL")
var listenAddr = flag.String("listenAddr", ":8889", "Address to listen on")
var nodeId = flag.String("nodeId", "unknown_node", "Name of the node")
var lbNodeId = flag.Int("lbNodeId", 0, "Node ID for load balancer (0 for default)")
var certFile = flag.String("certFile", "ca/cert.pem", "Path to the TLS certificate file")
var keyFile = flag.String("keyFile", "ca/key.pem", "Path to the TLS key file")
var sharedSecret = flag.String("sharedSecret", "shared_secret", "Shared secret for QUIC LB connection ID generation(TODO: move into secure place)")

type ObservedPacketConn struct {
	quic.OOBCapablePacketConn
	bt *ipv4.PacketConn
}

func (c *ObservedPacketConn) WriteMsgUDP(b, oob []byte, addr *net.UDPAddr) (n, oobn int, err error) {
	// log.Printf("Write To %s", addr)
	return c.OOBCapablePacketConn.WriteMsgUDP(b, oob, addr)
}

func (c *ObservedPacketConn) ReadBatch(ms []ipv4.Message, flags int) (int, error) {
	return c.bt.ReadBatch(ms, flags)
}

/*
type ObservedListener struct {
	lis net.Listener
}

type ObservedNetConn struct {
	net.Conn
}

func (c *ObservedNetConn) Read(b []byte) (n int, err error) {
	log.Printf("Read from %s", c.RemoteAddr())
	n, err = c.Conn.Read(b)
	if err != nil {
		log.Printf("Read error from %s: %v", c.RemoteAddr(), err)
	} else {
		log.Printf("Read %d bytes from %s", n, c.RemoteAddr())
	}
	return n, err
}

func (l *ObservedListener) Accept() (net.Conn, error) {
	conn, err := l.lis.Accept()
	if err != nil {
		return nil, err
	}
	log.Printf("Accepted connection from %s", conn.RemoteAddr())
	return &ObservedNetConn{Conn: conn}, nil
}

func (l *ObservedListener) Close() error {
	return l.lis.Close()
}

func (l *ObservedListener) Addr() net.Addr {
	return l.lis.Addr()
}
*/

func main() {
	flag.Parse()
	log.Printf("QUIC_GO_LOG_LEVEL=%s", os.Getenv("QUIC_GO_LOG_LEVEL"))

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
	ec := edge.NewEdgeComputing(wazero.NewRuntime(context.Background()))
	c := cache.NewCache()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		reqID, err := ec.StartRequest(r.Context(), r)
		var handleRequest func(*http.Request)
		var handleResponse func(*http.Response)
		if err != nil {
			if !errors.Is(err, edge.NoEdgeFunctionError) {
				log.Printf("Failed to start request: %v", err)
			}
			handleRequest = func(req *http.Request) {}
			handleResponse = func(resp *http.Response) {}
		} else {
			defer func() {
				if err := ec.FinishRequest(r.Context(), reqID); err != nil {
					log.Printf("Failed to finish request: %v", err)
				}
			}()
			handleRequest = func(req *http.Request) {
				err := ec.ProcessRequest(r.Context(), reqID, req)
				if err != nil {
					log.Printf("Failed to process request: %v", err)
				}
			}
			handleResponse = func(resp *http.Response) {
				err := ec.ProcessResponse(r.Context(), reqID, resp)
				if err != nil {
					log.Printf("Failed to process response: %v", err)
				}
			}
		}
		isCacheable := r.Method == http.MethodGet || r.Method == http.MethodHead
		if isCacheable {
			key := r.URL.String()
			if cached, found := c.Get(key); found {
				handleRequest(r)
				log.Printf("Cache hit for %s", key)
				for k, v := range cached.Header {
					w.Header()[k] = v
				}
				resp := &http.Response{
					StatusCode: cached.StatusCode,
					Header:     w.Header(),
					Body:       io.NopCloser(bytes.NewReader(cached.Body)),
				}
				handleResponse(resp)
				w.Header().Set("X-NCDN-PoPCache-Hit", "true")
				w.WriteHeader(resp.StatusCode)
				_, _ = w.Write(cached.Body)
				return
			}
		}
		// Handle GET and HEAD requests
		reverseProxy := &httputil.ReverseProxy{
			Rewrite: func(r *httputil.ProxyRequest) {
				handleRequest(r.Out)
				r.SetXForwarded()
				r.Out.Header.Set("X-NCDN-PoPCache-NodeId", *nodeId)
				r.SetURL(originURL)
			},
			ModifyResponse: func(resp *http.Response) error {
				cacheControl := resp.Header.Get("Cache-Control")
				if isCacheable && !strings.Contains(cacheControl, "no-store") {
					body, err := io.ReadAll(resp.Body)
					if err != nil {
						log.Printf("Failed to read response body: %v", err)
						return err
					}
					resp.Body.Close()                               // Close the original body
					resp.Body = io.NopCloser(bytes.NewReader(body)) // Create a new body reader
					// Cache the response
					c.Set(r.URL.String(), &cache.CacheEntry{
						StatusCode: resp.StatusCode,
						Header:     resp.Header.Clone(),
						Body:       body,
						StoredAt:   time.Now(),
					})
				}
				handleResponse(resp)
				resp.Header.Set("X-NCDN-PoPCache-Hit", "false")
				return nil
			},
		}
		reverseProxy.ServeHTTP(w, r)
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

	oobcap, ok := pkt.(quic.OOBCapablePacketConn)
	if !ok {
		log.Fatalf("PacketConn %T does not implement OOBCapablePacketConn", pkt)
	}

	pkt = &ObservedPacketConn{OOBCapablePacketConn: oobcap, bt: ipv4.NewPacketConn(pkt)}

	if *lbNodeId < 0 || *lbNodeId > 15 {
		log.Fatalf("lbNodeId must be between 0 and 15, got %d", *lbNodeId)
	}

	derivedKey, err := util.DeriveKey([]byte(*sharedSecret), "quic-lb")
	if err != nil {
		log.Fatalf("Failed to derive key: %v", err)
	}

	tr := &quic.Transport{
		Conn:                  pkt,
		ConnectionIDLength:    20,
		ConnectionIDGenerator: lbconnid.NewQUICLBConnIDGenerator(uint32(*lbNodeId), derivedKey, 17),
	}

	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{cert},
		VerifyConnection: func(cs tls.ConnectionState) error {
			return nil
		},
	}

	qlis, err := tr.Listen(http3.ConfigureTLSConfig(tlsConf), &quic.Config{})
	if err != nil {
		log.Fatalf("Failed to start QUIC listener: %v", err)
	}

	go func() {
		err := srv.ServeListener(qlis)
		if err != nil {
			log.Fatalf("Failed to serve QUIC listener: %v", err)
		}
	}()

	tlsServ := &http.Server{
		Addr:      *listenAddr,
		TLSConfig: tlsConf,
		ConnState: func(conn net.Conn, state http.ConnState) {
			switch state {
			case http.StateNew:
				log.Printf("New connection from %s", conn.RemoteAddr())
			case http.StateClosed:
				log.Printf("Connection closed from %s", conn.RemoteAddr())
			}
		},
	}

	lis, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", *listenAddr, err)
	}

	lis = tls.NewListener(lis, tlsConf)

	err = http2.ConfigureServer(tlsServ, &http2.Server{})
	if err != nil {
		log.Fatalf("Failed to configure HTTP/2 server: %v", err)
	}

	log.Printf("Listening on %s...", *listenAddr)
	if err := tlsServ.Serve(lis); err != nil {
		log.Fatal(err)
	}
}
