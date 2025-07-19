package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/quic-go/quic-go/http3"
)

func main() {
	fmt.Println("Starting QUIC LB client...")
	rootCA := "/workspaces/ncdn/ca/certs/20250719194728/root_ca.crt"
	rootCertPool := x509.NewCertPool()
	rootCABytes, err := os.ReadFile(rootCA)
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
	}
	client := &http.Client{
		Transport: tr,
	}

	fmt.Print("Starting request to QUIC LB...")

	client.Timeout = 10 * time.Second

	resp, err := client.Get("https://192.0.2.10:8889/statusz")

	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		panic("Unexpected status code: " + resp.Status)
	}

	_, err = io.Copy(os.Stdout, resp.Body)
	if err != nil {
		panic(err)
	}
	fmt.Println("\nRequest completed successfully")
}
