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
	if len(os.Args) < 3 {
		fmt.Println("Usage: quicclient <server address> <certificate path>")
		return
	}
	fmt.Println("Starting QUIC LB client...")
	serverAddress := os.Args[1]
	fmt.Println("Server Address:", serverAddress)
	rootCA := os.Args[2]
	fmt.Println("Root CA Path:", rootCA)
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

	resp, err := client.Get("https://" + serverAddress + "/statusz")

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
