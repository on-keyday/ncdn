package main

import (
	"log"

	"github.com/yzp0n/ncdn/tool/geoloc"
)

func main() {
	_, err := geoloc.FetchGeoLocation("/tmp/geoloc.json")
	if err != nil {
		log.Fatalf("Failed to fetch GeoLocation: %v", err)
	}
}
