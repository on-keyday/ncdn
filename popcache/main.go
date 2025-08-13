package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"os"
	"time"

	_ "embed"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/spaolacci/murmur3"
	"github.com/tetratelabs/wazero"
	"github.com/yzp0n/ncdn/controller/chunk"
	"github.com/yzp0n/ncdn/controller/file"
	"github.com/yzp0n/ncdn/controller/lbconn"
	"github.com/yzp0n/ncdn/controller/protocol"
	"github.com/yzp0n/ncdn/controller/remoteshell"
	"github.com/yzp0n/ncdn/controller/transport"
	wstransport "github.com/yzp0n/ncdn/controller/transport/websocket"
	"github.com/yzp0n/ncdn/httprps"
	"github.com/yzp0n/ncdn/popcache/cache"
	lbconnid "github.com/yzp0n/ncdn/popcache/connid"
	"github.com/yzp0n/ncdn/popcache/edge"
	"github.com/yzp0n/ncdn/popcache/vip"
	"github.com/yzp0n/ncdn/tool/util"
	"github.com/yzp0n/ncdn/types"
	"golang.org/x/net/http2"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/websocket"
)

var controlPlaneAddr = flag.String("controlPlane", "ws://localhost:8080", "Control plane address")
var originURLStr = flag.String("originURL", "http://localhost:8888", "Origin server URL")
var secureListenAddr = flag.String("listenAddr", ":8889", "Address to listen on")
var httpListenAddr = flag.String("insecureListenAddr", ":8890", "HTTP server address to listen on")
var nodeId = flag.String("nodeId", "unknown_node", "Name of the node")
var interfaceName = flag.String("interface", "", "Network interface name to use for IPv4 address (default: first non-loopback interface)")
var lbNodeId = flag.Uint("lbNodeId", 0, "LB node ID (if 0, derived from nodeId)")
var certFile = flag.String("certFile", "", "Path to the TLS certificate file")
var keyFile = flag.String("keyFile", "", "Path to the TLS key file")
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

///var edgeApp []byte

type appStat struct{}

func (s *appStat) GetStat() (*protocol.L7UpdateInfo, error) {
	return &protocol.L7UpdateInfo{}, nil
}

func main() {
	flag.Parse()
	log.Printf("QUIC_GO_LOG_LEVEL=%s", os.Getenv("QUIC_GO_LOG_LEVEL"))

	originURL, err := url.Parse(*originURLStr)
	if err != nil {
		log.Fatalf("Failed to parse origin URL %q: %v", *originURLStr, err)
	}

	_, devIndex, addr, hardAddr, err := util.GetSelfIPv4Address(*interfaceName)
	if err != nil {
		log.Fatalf("Failed to get IPv4 address for interface %s: %v", *interfaceName, err)
	}

	controlPlaneURL, err := url.Parse(*controlPlaneAddr)
	if err != nil {
		log.Fatalf("Failed to parse control plane address %q: %v", *controlPlaneAddr, err)
	}
	if controlPlaneURL.Scheme != "ws" && controlPlaneURL.Scheme != "wss" {
		log.Fatalf("Control plane address must use ws or wss scheme, got %s", controlPlaneURL.Scheme)
	}
	httpOrigin := *controlPlaneURL
	httpOrigin.Scheme = "http"

	start := time.Now()
	conf := wazero.NewRuntimeConfig().WithCloseOnContextDone(true)
	rt := wazero.NewRuntimeWithConfig(context.Background(), conf)
	ec := edge.NewEdgeComputing(rt, 100*time.Millisecond)
	/*err = ec.Register(context.Background(), 0, "GET", "/index.html", edgeApp)
	if err != nil {
		log.Fatalf("Failed to register edge function: %v", err)
	}
	*/
	c := cache.NewCache()

	appStat := &appStat{}
	var serverID uint32
	if *lbNodeId == 0 {
		serverID = murmur3.Sum32([]byte(*nodeId)) // Use MurmurHash3 for consistent hashing
	} else {
		serverID = uint32(*lbNodeId)
	}

	retryConn := lbconn.ConnectRetriable(slog.Default(), 5*time.Second, lbconn.ConnectL7LB,
		&protocol.L7LBData{
			ServerID:   serverID,
			Address:    addr,
			MacAddress: [6]byte(hardAddr),
		}, 20*time.Second, func() (transport.Connection, error) {
			return wstransport.Connect(context.Background(), &websocket.Config{
				Location: controlPlaneURL,
				Origin:   &httpOrigin,
				Version:  websocket.ProtocolVersionHybi13,
			})
		}, appStat.GetStat)

	cmdMgr := remoteshell.NewManager()
	chunkedMap := chunk.NewChunkMap()
	vipmgr, err := vip.NewVIPManager(addr, devIndex)
	if err != nil {
		log.Fatalf("Failed to create VIP manager: %v", err)
	}

	go func() {
		for {
			msg, err := retryConn.Receive()
			if err != nil {
				log.Printf("Failed to receive connection: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}
			log.Printf("Received message: %s", msg.Header.MessageType)
			if handled, err := remoteshell.DispatchMessage(cmdMgr, msg); err != nil {
				log.Printf("Failed to dispatch message: %v", err)
				continue
			} else if handled {
				continue
			}
			chunked, err := chunkedMap.ReadChunked(retryConn, msg)
			if err != nil {
				log.Printf("Failed to read chunked data: %v", err)
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
				} else if wasm := chunked.Msg.WasmInstall(); wasm != nil {
					err := ec.Register(context.Background(), wasm.Id, string(wasm.Method), string(wasm.Path), chunked.Data)
					if err != nil {
						retryConn.Send(&lbconn.LogMsg{
							Level:   protocol.LogLevel_Error,
							Message: fmt.Sprintf("Failed to register WASM module %q: %v", wasm.Path, err),
						})
					} else {
						retryConn.Send(&lbconn.LogMsg{
							Level:   protocol.LogLevel_Info,
							Message: fmt.Sprintf("WASM module %d:%s %s registered successfully", wasm.Id, wasm.Method, wasm.Path),
						})
					}
				} else if wasm := chunked.Msg.WasmUninstall(); wasm != nil {
					err := ec.Unregister(wasm.Id)
					if err != nil {
						retryConn.Send(&lbconn.LogMsg{
							Level:   protocol.LogLevel_Error,
							Message: fmt.Sprintf("Failed to unregister WASM module %d: %v", wasm.Id, err),
						})
					} else {
						retryConn.Send(&lbconn.LogMsg{
							Level:   protocol.LogLevel_Info,
							Message: fmt.Sprintf("WASM module %d unregistered successfully", wasm.Id),
						})
					}
				} else if vipUpdate := chunked.Msg.VipUpdate(); vipUpdate != nil {
					if err := vipmgr.UpdateVIP(netip.PrefixFrom(protocol.IPFromAddress(vipUpdate.VirtualAddress), int(vipUpdate.Prefix)), slog.Default()); err != nil {
						retryConn.Send(&lbconn.LogMsg{
							Level:   protocol.LogLevel_Error,
							Message: fmt.Sprintf("Failed to update VIP: %v", err),
						})
					} else {
						retryConn.Send(&lbconn.LogMsg{
							Level:   protocol.LogLevel_Info,
							Message: fmt.Sprintf("VIP %s updated successfully", netip.PrefixFrom(protocol.IPFromAddress(vipUpdate.VirtualAddress), int(vipUpdate.Prefix))),
						})
					}
				} else if l7l4update := chunked.Msg.L7LbL4LbUpdate(); l7l4update != nil {
					var remoteList []*protocol.L4LBData
					for _, r := range l7l4update.Info {
						remoteList = append(remoteList, &protocol.L4LBData{
							Address:    protocol.IPFromAddress(r.Address),
							MacAddress: r.MacAddress,
							ServerID:   r.ServerId,
						})
					}
					if err := vipmgr.UpdateRemote(remoteList, slog.Default()); err != nil {
						retryConn.Send(&lbconn.LogMsg{
							Level:   protocol.LogLevel_Error,
							Message: fmt.Sprintf("Failed to update remote list: %v", err),
						})
					} else {
						retryConn.Send(&lbconn.LogMsg{
							Level:   protocol.LogLevel_Info,
							Message: fmt.Sprintf("Remote list updated successfully with %d entries", len(remoteList)),
						})
					}
				} else {
					retryConn.Send(&lbconn.LogMsg{
						Level:   protocol.LogLevel_Info,
						Message: fmt.Sprintf("Received unhandled data %s", chunked.Msg.Header.MessageType),
					})
				}
			}
		}
	}()

	go func() {
		for cmd := range cmdMgr.Output() {
			if err := retryConn.SendCommandline(cmd); err != nil {
				slog.Error("Failed to send command line message", slog.String("error", err.Error()))
			}
		}
	}()

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

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		reqID, err := ec.StartRequest(r.Context(), r)
		var handleRequest func(*http.Request)
		var handleResponse func(*http.Response)
		routing := edge.Routing_Cont
		if err != nil {
			if !errors.Is(err, edge.ErrNoEdgeFunction) {
				log.Printf("Failed to start request: %v", err)
			}
			log.Printf("No edge function registered for %s, serving directly from origin", r.URL.Path)
			handleRequest = func(req *http.Request) {}
			handleResponse = func(resp *http.Response) {}
		} else {
			log.Printf("Edge function started for %s with request ID %d", r.URL.Path, reqID)
			defer func() {
				if err := ec.FinishRequest(r.Context(), reqID); err != nil {
					log.Printf("Failed to finish request: %v", err)
				}
			}()
			handleRequest = func(req *http.Request) {
				err := ec.ProcessRequest(r.Context(), reqID, serverID, req)
				if err != nil {
					log.Printf("Failed to process request: %v", err)
				}
				cloned := req.Clone(r.Context())
				changeRoute := edge.Routing_Cont
				err = ec.ModifyRequest(r.Context(), reqID, func(d []edge.DiffData) error {
					for i := range d {
						if r := d[i].Routing(); r != nil {
							changeRoute = *r
							continue
						}
						err := edge.DefaultModifyRequest(&d[i], cloned)
						if err != nil {
							return err
						}
					}
					return nil
				}, true)
				if err != nil {
					return
				}
				*req = *cloned
				routing = changeRoute
			}
			handleResponse = func(resp *http.Response) {
				// first, apply the changes from on_request
				err := ec.ModifyResponse(r.Context(), reqID, func(d []edge.DiffData) error {
					for i := range d {
						if r := d[i].Routing(); r != nil {
							routing = *r
							continue
						}
						err := edge.DefaultModifyResponse(&d[i], resp)
						if err != nil {
							return err
						}
					}
					return nil
				}, true)
				if err != nil {
					log.Printf("Failed to modify response: %v", err)
					return
				}
				err = ec.ProcessResponse(r.Context(), reqID, resp)
				if err != nil {
					log.Printf("Failed to process response: %v", err)
				}
				// then, apply the changes from on_response
				err = ec.ModifyResponse(r.Context(), reqID, func(d []edge.DiffData) error {
					for i := range d {
						if r := d[i].Routing(); r != nil {
							routing = *r
							continue
						}
						err := edge.DefaultModifyResponse(&d[i], resp)
						if err != nil {
							return err
						}
					}
					return nil
				}, true)
				if err != nil {
					log.Printf("Failed to modify response: %v", err)
					return
				}
			}
		}
		doAbort := func() {
			log.Printf("Request %s %s is aborted by edge function, disconnect", r.Method, r.URL.Path)
			hijacked, ok := w.(http.Hijacker)
			if !ok {
				log.Printf("Response writer does not support hijacking, cannot abort connection")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			conn, _, err := hijacked.Hijack()
			if err != nil {
				log.Printf("Failed to hijack connection: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			log.Printf("Hijacking connection from %s", conn.RemoteAddr())
			conn.Close() // Close the connection to abort

		}
		handleRequest(r)
		if routing == edge.Routing_Abort {
			doAbort()
			return
		}
		if routing == edge.Routing_Deny {
			resp := &http.Response{
				StatusCode: http.StatusForbidden,
				Header:     http.Header{"Content-Type": []string{"text/plain"}},
				Body:       io.NopCloser(bytes.NewBufferString("Access denied by edge function")),
			}
			handleResponse(resp)
			w.Header().Set("X-NCDN-PoPCache-Hit", "false")
			w.WriteHeader(resp.StatusCode)
			if _, err := io.Copy(w, resp.Body); err != nil {
				log.Printf("Failed to write response body: %v", err)
			}
			return
		}
		isCacheable := r.Method == http.MethodGet || r.Method == http.MethodHead
		var cacheKey string
		if isCacheable && !(routing == edge.Routing_NoCache ||
			routing == edge.Routing_SkipCache) {
			key, resp, found := c.HasCache(r)
			if found {
				log.Printf("Cache hit for %s", key)
				if routing == edge.Routing_SkipResponseExecIfCached {
					handleResponse(resp)
				}
				w.Header().Set("X-NCDN-PoPCache-Hit", "true")
				w.WriteHeader(resp.StatusCode)
				if _, err := io.Copy(w, resp.Body); err != nil {
					log.Printf("Failed to write cached response body: %v", err)
				}
				return
			}
			cacheKey = key
		}
		// Handle GET and HEAD requests
		reverseProxy := &httputil.ReverseProxy{
			Rewrite: func(r *httputil.ProxyRequest) {
				r.SetXForwarded()
				r.Out.Header.Set("X-NCDN-PoPCache-NodeId", *nodeId)
				r.SetURL(originURL)
			},
			ModifyResponse: func(resp *http.Response) error {
				handleResponse(resp)
				if routing == edge.Routing_Abort {
					doAbort()
					return nil
				}
				if routing != edge.Routing_NoCache {
					if cacheData, ok := c.IsCacheable(resp); ok || routing == edge.Routing_ForceCache {
						if cacheKey == "" {
							cacheKey = c.GenerateCacheKey(resp.Request)
						}
						c.SetCache(cacheKey, r, resp, cacheData)
					}
				}
				resp.Header.Set("X-NCDN-PoPCache-Hit", "false")
				return nil
			},
		}
		reverseProxy.ServeHTTP(w, r)
	})

	log.Printf("Listening on HTTP %s...", *httpListenAddr)
	httpServ := &http.Server{
		Addr:    *httpListenAddr,
		Handler: mux,
		ConnState: func(conn net.Conn, state http.ConnState) {
			switch state {
			case http.StateNew:
				log.Printf("New HTTP connection from %s", conn.RemoteAddr())
			case http.StateClosed:
				log.Printf("HTTP connection closed from %s", conn.RemoteAddr())
			}
		},
	}
	go func() {
		if err := httpServ.ListenAndServe(); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()
	if *certFile == "" || *keyFile == "" {
		for {
			time.Sleep(10 * time.Second)
		}
	}

	cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		log.Fatalf("Failed to load TLS certificate and key: %v", err)
	}

	srv := &http3.Server{
		Addr:    *secureListenAddr,
		Handler: mux,
	}

	pkt, err := net.ListenPacket("udp4", *secureListenAddr)

	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", *secureListenAddr, err)
	}

	oobcap, ok := pkt.(quic.OOBCapablePacketConn)
	if !ok {
		log.Fatalf("PacketConn %T does not implement OOBCapablePacketConn", pkt)
	}

	pkt = &ObservedPacketConn{OOBCapablePacketConn: oobcap, bt: ipv4.NewPacketConn(pkt)}

	derivedKey, err := util.DeriveKey([]byte(*sharedSecret), "quic-lb")
	if err != nil {
		log.Fatalf("Failed to derive key: %v", err)
	}

	tr := &quic.Transport{
		Conn:                  pkt,
		ConnectionIDLength:    20,
		ConnectionIDGenerator: lbconnid.NewQUICLBConnIDGenerator(serverID, derivedKey, 17),
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
		Addr:      *secureListenAddr,
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

	lis, err := net.Listen("tcp", *secureListenAddr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", *secureListenAddr, err)
	}

	lis = tls.NewListener(lis, tlsConf)

	err = http2.ConfigureServer(tlsServ, &http2.Server{})
	if err != nil {
		log.Fatalf("Failed to configure HTTP/2 server: %v", err)
	}

	log.Printf("Listening on %s...", *secureListenAddr)

	go func() {
		if err := tlsServ.Serve(lis); err != nil {
			log.Fatal(err)
		}
	}()

	for {
		time.Sleep(10 * time.Second)
	}

}
