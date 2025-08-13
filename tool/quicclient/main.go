package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"os"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

var requestCount = flag.Int("requestCount", 1, "Number of requests to send")
var serverAddress = flag.String("serverAddress", "localhost:4433", "Server address to connect to")
var rootCA = flag.String("rootCA", "path/to/rootCA.pem", "Path to the root CA certificate")
var parallelRequests = flag.Bool("parallel", false, "Send requests in parallel")
var metricsFile = flag.String("metricsFile", "metrics.json", "File to save metrics")
var parallelLimit = flag.Int("parallelLimit", 0, "Maximum number of parallel requests (0 for no limit)")
var mode = flag.String("mode", "QUIC", "mode of client (TCP or QUIC)")
var debug = flag.Bool("debug", false, "Enable debug output")
var path = flag.String("path", "/statusz", "Path to request on the server")
var showBody = flag.Bool("showBody", false, "Show response body in metrics output")
var showHeaders = flag.Bool("showHeaders", false, "Include response headers in metrics output")
var noRedirect = flag.Bool("noRedirect", false, "Disable HTTP redirects")

type Metrics struct {
	ID                        int           `json:"id"`
	StatusCode                int           `json:"status_code,omitempty"`
	ResponseHeaders           http.Header   `json:"response_headers,omitempty"`
	StartTime                 time.Time     `json:"start_time,omitempty"`
	DNSDuration               time.Duration `json:"dns_duration,omitempty"`
	ConnectDuration           time.Duration `json:"connect_duration,omitempty"`
	RequestSentDuration       time.Duration `json:"request_sent_duration,omitempty"`
	ResponseFirstByteDuration time.Duration `json:"response_first_byte_duration,omitempty"`
	FullDuration              time.Duration `json:"full_duration,omitempty"`
	ResponseBody              string        `json:"response_body,omitempty"`
}

type metricsOutput struct {
	Data          []*Metrics `json:"data"`
	Summary       Metrics    `json:"summary"`
	Variance      Metrics    `json:"variance"`
	StartTime     time.Time  `json:"start_time"`
	EndTime       time.Time  `json:"end_time"`
	ParallelLimit *int       `json:"parallel_limit,omitempty"`
}

func main() {
	flag.Parse()
	if *requestCount <= 0 {
		fmt.Println("Request count must be a positive integer")
		return
	}
	if *parallelLimit < 0 {
		fmt.Println("Parallel limit must be a non-negative integer")
		return
	}
	fmt.Println("Starting QUIC LB client...")
	fmt.Println("Request Count:", *requestCount)
	fmt.Println("Server Address:", *serverAddress)
	var rootCertPool *x509.CertPool
	if *rootCA != "" {
		fmt.Println("Root CA Path:", *rootCA)
		rootCertPool = x509.NewCertPool()
		rootCABytes, err := os.ReadFile(*rootCA)
		if err != nil {
			panic(err)
		}
		if !rootCertPool.AppendCertsFromPEM(rootCABytes) {
			panic("Failed to append root CA")
		}
	}
	var client *http.Client
	switch *mode {
	case "TCP":
		proto := &http.Protocols{}
		proto.SetHTTP1(true)
		proto.SetHTTP2(true)
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: rootCertPool,
			},
			Protocols: proto,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, addr)
			},
		}
		client = &http.Client{
			Transport: tr,
		}
	case "QUIC":
		tr := &http3.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: rootCertPool,
			},
			QUICConfig: &quic.Config{},
		}
		client = &http.Client{
			Transport: tr,
		}
	default:
		fmt.Println("Unknown mode:", *mode)
		return
	}
	if *noRedirect {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Prevents following redirects
		}
	}
	fmt.Printf("Running %s test\n", *mode)
	fmt.Printf("Starting request to %s LB...\n", *mode)

	// client.Timeout = 10 * time.Second
	targetURL := fmt.Sprintf("https://%s%s", *serverAddress, *path)
	var mu sync.Mutex
	var metrics []*Metrics

	abs := func(d time.Duration) time.Duration {
		if d < 0 {
			return -d
		}
		return d
	}

	request := func(i int, wg *sync.WaitGroup, fence chan struct{}) {
		if wg != nil {
			defer wg.Done()
		}
		if fence != nil {
			fence <- struct{}{}        // Acquire a slot in the fence
			defer func() { <-fence }() // Release the slot in the fence
		}
		if *debug {
			fmt.Printf("DEBUG: Request %d started\n", i)
		}
		req, _ := http.NewRequest(http.MethodGet, targetURL, nil)
		var dnsend time.Time
		var connectend time.Time
		var requestend time.Time
		var firstbyte time.Time
		req = req.WithContext(httptrace.WithClientTrace(req.Context(), &httptrace.ClientTrace{
			DNSDone: func(httptrace.DNSDoneInfo) {
				dnsend = time.Now()
				if *debug {
					fmt.Printf("DEBUG: DNS lookup done for request %d at %s\n", i, dnsend)
				}
			},
			TLSHandshakeStart: func() {
				if *debug {
					fmt.Printf("DEBUG: TLS handshake started for request %d\n", i)
				}
			},
			TLSHandshakeDone: func(tls.ConnectionState, error) {
				if *debug {
					fmt.Printf("DEBUG: TLS handshake done for request %d at %s\n", i, time.Now())
				}
			},
			GotConn: func(httptrace.GotConnInfo) {
				connectend = time.Now()
				if *debug {
					fmt.Printf("DEBUG: Connection established for request %d at %s\n", i, connectend)
				}
			},
			WroteRequest: func(httptrace.WroteRequestInfo) {
				requestend = time.Now()
				if *debug {
					fmt.Printf("DEBUG: Request sent for request %d at %s\n", i, requestend)
				}
			},
			GotFirstResponseByte: func() {
				firstbyte = time.Now()
				if *debug {
					fmt.Printf("DEBUG: First response byte received for request %d at %s\n", i, firstbyte)
				}
			},
		}))

		start := time.Now()
		resp, err := client.Do(req)

		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		end := time.Now()
		clampZero := func(d time.Duration) time.Duration {
			if d < 0 {
				return 0
			}
			return d
		}
		dnsDuration := clampZero(dnsend.Sub(start))
		connectDuration := clampZero(connectend.Sub(start))
		requestDuration := clampZero(requestend.Sub(start))
		firstByteDuration := clampZero(firstbyte.Sub(start))
		fullDuration := clampZero(end.Sub(start))
		mu.Lock()
		metrics = append(metrics, &Metrics{
			ID:                        i,
			StatusCode:                resp.StatusCode,
			ResponseHeaders:           resp.Header.Clone(),
			StartTime:                 start,
			DNSDuration:               dnsDuration,
			ConnectDuration:           connectDuration,
			RequestSentDuration:       requestDuration,
			ResponseFirstByteDuration: firstByteDuration,
			FullDuration:              fullDuration,
			ResponseBody:              string(data),
		})
		mu.Unlock()
		if *debug {
			fmt.Printf("DEBUG: Request %d completed: DNS=%s, Connect=%s, RequestSent=%s, FirstByte=%s, Full=%s\n",
				i, dnsDuration, connectDuration, requestDuration, firstByteDuration, fullDuration)
		}
	}

	var wg sync.WaitGroup
	var fence chan struct{}
	if *parallelLimit > 0 {
		fence = make(chan struct{}, *parallelLimit)
	}

	start := time.Now()

	for i := 0; i < *requestCount; i++ {
		if *parallelRequests {
			wg.Add(1)
			go request(i, &wg, fence)
		} else {
			request(i, nil, nil)
		}
	}
	if *parallelRequests {
		wg.Wait()
	}
	end := time.Now()
	fmt.Println("\nRequest completed successfully")
	totalRequest := len(metrics)
	var summaryMetics Metrics
	for _, m := range metrics {
		fmt.Printf("Request ID: %d, Start Time: %s, DNS Duration: %s, Connect Duration: %s, Request Sent Duration: %s, Response First Byte Duration: %s, Full Duration: %s\n",
			m.ID, m.StartTime.Format(time.RFC3339), m.DNSDuration, m.ConnectDuration, m.RequestSentDuration, m.ResponseFirstByteDuration, m.FullDuration)
		summaryMetics.DNSDuration += m.DNSDuration
		summaryMetics.ConnectDuration += m.ConnectDuration
		summaryMetics.RequestSentDuration += m.RequestSentDuration
		summaryMetics.ResponseFirstByteDuration += m.ResponseFirstByteDuration
		summaryMetics.FullDuration += m.FullDuration
		if *showHeaders {
			fmt.Printf("Response Headers for Request ID %d:\n", m.ID)
			for k, v := range m.ResponseHeaders {
				fmt.Printf("  %s: %s\n", k, v)
			}
		}
		if *showBody {
			fmt.Printf("Response Body for Request ID %d:\n%s\n", m.ID, m.ResponseBody)
		}
	}
	var varianceMetrics Metrics // 絶対値バージョン

	for _, m := range metrics {
		varianceMetrics.DNSDuration += abs(m.DNSDuration - summaryMetics.DNSDuration/time.Duration(totalRequest))
		varianceMetrics.ConnectDuration += abs(m.ConnectDuration - summaryMetics.ConnectDuration/time.Duration(totalRequest))
		varianceMetrics.RequestSentDuration += abs(m.RequestSentDuration - summaryMetics.RequestSentDuration/time.Duration(totalRequest))
		varianceMetrics.ResponseFirstByteDuration += abs(m.ResponseFirstByteDuration - summaryMetics.ResponseFirstByteDuration/time.Duration(totalRequest))
		varianceMetrics.FullDuration += abs(m.FullDuration - summaryMetics.FullDuration/time.Duration(totalRequest))
	}
	fmt.Println("total: ", len(metrics), "requests")
	// mean
	fmt.Printf("Summary Metrics: DNS Duration: %s, Connect Duration: %s, Request Sent Duration: %s, Response First Byte Duration: %s, Full Duration: %s\n",
		summaryMetics.DNSDuration/time.Duration(totalRequest),
		summaryMetics.ConnectDuration/time.Duration(totalRequest),
		summaryMetics.RequestSentDuration/time.Duration(totalRequest),
		summaryMetics.ResponseFirstByteDuration/time.Duration(totalRequest),
		summaryMetics.FullDuration/time.Duration(totalRequest))
	// variance
	fmt.Printf("Variance Metrics: DNS Duration: %s, Connect Duration: %s, Request Duration: %s, First Byte Duration: %s, Full Duration: %s\n",
		varianceMetrics.DNSDuration/time.Duration(totalRequest),
		varianceMetrics.ConnectDuration/time.Duration(totalRequest),
		varianceMetrics.RequestSentDuration/time.Duration(totalRequest),
		varianceMetrics.ResponseFirstByteDuration/time.Duration(totalRequest),
		varianceMetrics.FullDuration/time.Duration(totalRequest))
	if *metricsFile != "" {
		file, err := os.Create(*metricsFile)
		if err != nil {
			fmt.Printf("Failed to create metrics file: %v\n", err)
			return
		}
		defer file.Close()

		summaryMetics.DNSDuration /= time.Duration(totalRequest)
		summaryMetics.ConnectDuration /= time.Duration(totalRequest)
		summaryMetics.RequestSentDuration /= time.Duration(totalRequest)
		summaryMetics.ResponseFirstByteDuration /= time.Duration(totalRequest)
		summaryMetics.FullDuration /= time.Duration(totalRequest)
		varianceMetrics.DNSDuration /= time.Duration(totalRequest)
		varianceMetrics.ConnectDuration /= time.Duration(totalRequest)
		varianceMetrics.RequestSentDuration /= time.Duration(totalRequest)
		varianceMetrics.ResponseFirstByteDuration /= time.Duration(totalRequest)
		varianceMetrics.FullDuration /= time.Duration(totalRequest)
		output := metricsOutput{
			Data:      metrics,
			Summary:   summaryMetics,
			Variance:  varianceMetrics,
			StartTime: start,
			EndTime:   end,
		}
		if *parallelRequests {
			output.ParallelLimit = parallelLimit
		} else {
			output.ParallelLimit = nil
		}
		enc := json.NewEncoder(file)
		if err := enc.Encode(output); err != nil {
			fmt.Printf("Failed to encode metrics to JSON: %v\n", err)
			return
		}
		fmt.Println("Metrics saved to", *metricsFile)
	}
}
