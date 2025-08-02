package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/netip"
	"strings"

	"github.com/yzp0n/ncdn/httprps"
	"github.com/yzp0n/ncdn/tool/geoloc"
)

var nodeId = flag.String("nodeId", "unknown_node", "Name of the node")
var listenAddr = flag.String("listenAddr", ":8888", "Address to listen on")

type requestInfo struct {
	RemoteAddr string
	PopCacheId string
	OriginId   string
	ASN        string
	ISP        string
	GeoIP      string
}

func dumpRequestInfo(r *http.Request) requestInfo {
	addr := r.RemoteAddr
	xforwardedAddr := r.Header.Get("X-Forwarded-For")
	if xforwardedAddr != "" {
		// Use the first IP address from the X-Forwarded-For header
		parts := strings.Split(xforwardedAddr, ",")
		if len(parts) > 0 {
			addr = strings.TrimSpace(parts[0])
		}
	}
	result := requestInfo{
		RemoteAddr: addr,
		PopCacheId: r.Header.Get("X-NCDN-PoPCache-NodeId"),
		OriginId:   *nodeId,
	}
	parsedAddr, err := netip.ParseAddr(addr)
	if err != nil {
		log.Printf("Failed to parse remote address %s: %v", addr, err)
		return result
	}
	// Fetch GeoLocation information
	geoLoc, err := geoLoc.GeoLocation(parsedAddr)
	if err != nil {
		log.Printf("Failed to fetch GeoLocation for %s: %v", addr, err)
		return result
	}
	result.ASN = fmt.Sprintf("%d", geoLoc.ASN.AutonomousSystemNumber)
	result.ISP = geoLoc.ASN.AutonomousSystemOrganization
	if geoLoc.City != nil {
		result.GeoIP = fmt.Sprintf("%s, %s, %s", geoLoc.City.City.Names.English, geoLoc.City.Country.ISOCode, geoLoc.City.Continent.Code)
	} else {
		result.GeoIP = "Unknown"
	}
	return result
}

var geoLoc *geoloc.GeoLocationInfo

func serveIndexHTMLInternal(w http.ResponseWriter, r *http.Request) error {
	tmpl, err := template.New("index.html.gotmpl").ParseFiles("./templates/index.html.gotmpl")
	if err != nil {
		return fmt.Errorf("Failed to parse index.html template: %w", err)
	}

	ri := dumpRequestInfo(r)

	var buf bytes.Buffer
	if err = tmpl.Execute(&buf, &ri); err != nil {
		return fmt.Errorf("Failed to execute index.html template: %w", err)
	}

	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	_, err = w.Write(buf.Bytes())
	if err != nil {
		log.Printf("Failed to write response: %v", err)
		return nil // since it is too late to recover
	}

	return nil
}

func serveIndexHTML(w http.ResponseWriter, r *http.Request) {
	err := serveIndexHTMLInternal(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func serveJsonInternal(w http.ResponseWriter, r *http.Request) error {
	ri := dumpRequestInfo(r)

	bs, err := json.MarshalIndent(ri, "", "  ")
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	_, err = w.Write(bs)
	if err != nil {
		log.Printf("Failed to write response: %v", err)
		return nil // since it is too late to recover
	}

	return nil
}

func serveJson(w http.ResponseWriter, r *http.Request) {
	err := serveJsonInternal(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	flag.Parse()

	geolocInfo, err := geoloc.FetchGeoLocation("/tmp/geoloc.json")
	if err != nil {
		log.Fatalf("Failed to fetch GeoLocation: %v", err)
	}
	geoLoc = geolocInfo
	fs := http.FileServer(http.Dir("./static"))

	mux := http.NewServeMux()
	mux.HandleFunc("/index.html", serveIndexHTML)
	mux.HandleFunc("/json", serveJson)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			log.Printf("Redirecting to /index.html")
			// redirect to index.html
			http.Redirect(w, r, "/index.html", http.StatusPermanentRedirect)
			return
		}

		fs.ServeHTTP(w, r)
	})

	rps := httprps.NewMiddleware(mux)
	http.Handle("/", rps)
	mux.HandleFunc("/rps", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "RPS: %.2f\n", rps.GetRPS())
	})

	log.Printf("Listening on %s...\n", *listenAddr)
	err = http.ListenAndServe(*listenAddr, nil)
	if err != nil {
		log.Fatal(err)
	}
}
