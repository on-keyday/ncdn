package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io"
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

type Metrics struct {
	ID                        int           `json:"id"`
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
	fmt.Println("Root CA Path:", *rootCA)
	rootCertPool := x509.NewCertPool()
	rootCABytes, err := os.ReadFile(*rootCA)
	if err != nil {
		panic(err)
	}
	if !rootCertPool.AppendCertsFromPEM(rootCABytes) {
		panic("Failed to append root CA")
	}
	tr := &http3.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: rootCertPool,
		},
		QUICConfig: &quic.Config{},
	}
	client := &http.Client{
		Transport: tr,
	}

	fmt.Print("Starting request to QUIC LB...")

	// client.Timeout = 10 * time.Second
	targetURL := fmt.Sprintf("https://%s/statusz", *serverAddress)
	var mu sync.Mutex
	var metrics []*Metrics

	request := func(i int, wg *sync.WaitGroup, fence chan struct{}) {
		if wg != nil {
			defer wg.Done()
		}
		if fence != nil {
			fence <- struct{}{}        // Acquire a slot in the fence
			defer func() { <-fence }() // Release the slot in the fence
		}
		req, _ := http.NewRequest(http.MethodGet, targetURL, nil)
		var dnsend time.Time
		var connectend time.Time
		var requestend time.Time
		var firstbyte time.Time
		req = req.WithContext(httptrace.WithClientTrace(req.Context(), &httptrace.ClientTrace{
			DNSDone: func(httptrace.DNSDoneInfo) {
				dnsend = time.Now()
			},
			GotConn: func(httptrace.GotConnInfo) {
				connectend = time.Now()
			},
			WroteRequest: func(httptrace.WroteRequestInfo) {
				requestend = time.Now()
			},
			GotFirstResponseByte: func() {
				firstbyte = time.Now()
			},
		}))

		start := time.Now()
		resp, err := client.Do(req)

		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			panic("Unexpected status code: " + resp.Status)
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		end := time.Now()
		dnsDuration := dnsend.Sub(start)
		if dnsDuration < 0 {
			dnsDuration = 0
		}
		connectDuration := connectend.Sub(start)
		requestDuration := requestend.Sub(start)
		firstByteDuration := firstbyte.Sub(start)
		fullDuration := end.Sub(start)
		mu.Lock()
		metrics = append(metrics, &Metrics{
			ID:                        i,
			StartTime:                 start,
			DNSDuration:               dnsDuration,
			ConnectDuration:           connectDuration,
			RequestSentDuration:       requestDuration,
			ResponseFirstByteDuration: firstByteDuration,
			FullDuration:              fullDuration,
			ResponseBody:              string(data),
		})
		mu.Unlock()
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
	}
	var varianceMetrics Metrics // 絶対値バージョン
	abs := func(d time.Duration) time.Duration {
		if d < 0 {
			return -d
		}
		return d
	}
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
