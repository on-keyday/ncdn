package geoloc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"os"
	"runtime"
	"slices"

	"github.com/google/go-github/github"
	"github.com/oschwald/geoip2-golang/v2"
)

type GeoLocation struct {
	ASN  *geoip2.ASN  `json:"asn,omitempty"`
	City *geoip2.City `json:"city,omitempty"`
}

type FetchInfo struct {
	Name      string `json:"name"`
	Url       string `json:"url"`
	SavedPath string `json:"saved_path,omitempty"`
}

type GeoLocationInfo struct {
	asn  *geoip2.Reader
	city *geoip2.Reader

	originalInfo []*FetchInfo
}

func (g *GeoLocationInfo) GeoLocation(ip netip.Addr) (*GeoLocation, error) {
	asn, err := g.ASN(ip)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup ASN for %s: %w", ip, err)
	}

	city, err := g.City(ip)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup City for %s: %w", ip, err)
	}

	return &GeoLocation{
		ASN:  asn,
		City: city,
	}, nil
}

func (g *GeoLocationInfo) ASN(ip netip.Addr) (*geoip2.ASN, error) {
	if g.asn == nil {
		return nil, fmt.Errorf("ASN database not loaded")
	}
	record, err := g.asn.ASN(ip)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup ASN for %s: %w", ip, err)
	}
	return record, nil
}

func (g *GeoLocationInfo) City(ip netip.Addr) (*geoip2.City, error) {
	if g.city == nil {
		return nil, fmt.Errorf("City database not loaded")
	}
	record, err := g.city.City(ip)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup City for %s: %w", ip, err)
	}
	return record, nil
}

func (g *GeoLocationInfo) Close() {
	if g.asn != nil {
		g.asn.Close()
	}
	if g.city != nil {
		g.city.Close()
	}
}

// TODO: 定期更新するようにする
func FetchGeoLocation(fetchInfoPath string) (*GeoLocationInfo, error) {
	var oldFetchInfo []*FetchInfo
	if fetchInfoPath != "" {
		fetchInfoRaw, err := os.ReadFile(fetchInfoPath)
		if err != nil {
			if os.IsNotExist(err) {
				slog.Warn("Fetch info file not found, skipping", slog.String("path", fetchInfoPath))
				goto SKIPPED
			}
			return nil, fmt.Errorf("failed to read fetch info file: %w", err)
		}
		var fetchInfo []*FetchInfo
		if err := json.Unmarshal(fetchInfoRaw, &fetchInfo); err != nil {
			return nil, fmt.Errorf("failed to unmarshal fetch info: %w", err)
		}
		slog.Info("Loaded fetch info from file", slog.String("path", fetchInfoPath), slog.Int("count", len(fetchInfo)))
		oldFetchInfo = fetchInfo
	}
SKIPPED:

	// GitHub クライアントの初期化
	ctx := context.Background()
	var httpClient *http.Client
	/*
		cloned := http.DefaultTransport.(*http.Transport).Clone()
		defaultDialContext := cloned.DialContext
		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			Resolver: &net.Resolver{
				Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
					return defaultDialContext(ctx, network, "1.1.1.1:53")
				},
			},
		}
		cloned.DialContext = dialer.DialContext
	*/
	//httpClient = &http.Client{}
	client := github.NewClient(httpClient)

	release, _, err := client.Repositories.ListReleases(ctx, "P3TERX", "GeoLite.mmdb", &github.ListOptions{
		Page:    1,
		PerPage: 1,
	})
	var useCandidates []*FetchInfo
	var removeOldInfo []*FetchInfo
	if err != nil {
		slog.Error("Failed to fetch releases from GitHub", slog.String("error", err.Error()))
		if len(oldFetchInfo) == 0 {
			return nil, fmt.Errorf("failed to fetch releases from GitHub: %w", err)
		}
		slog.Info("Using old fetch info", slog.Int("count", len(oldFetchInfo)))
		useCandidates = oldFetchInfo
	} else {

		var fetchCandidate = []string{"GeoLite2-ASN.mmdb", "GeoLite2-City.mmdb"}

		var fetchCandidates []*FetchInfo

		for _, rel := range release {
			for _, asset := range rel.Assets {
				if slices.Contains(fetchCandidate, asset.GetName()) {
					fetchCandidates = append(fetchCandidates, &FetchInfo{
						Name: asset.GetName(),
						Url:  asset.GetBrowserDownloadURL(),
					})
				}
			}
		}

		if len(oldFetchInfo) > 0 {
			var newFetchCandidates []*FetchInfo
		OUTER:
			for _, c := range fetchCandidates {
				for _, old := range oldFetchInfo {
					if old.Url == c.Url { // if the URL matches, we assume it's the same file
						slog.Info("Found existing fetch info, reusing", slog.String("name", old.Name), slog.String("url", old.Url))
						useCandidates = append(useCandidates, old)
						continue OUTER
					} else if old.Name == c.Name { // if the name matches but the URL does not, we consider it old
						removeOldInfo = append(removeOldInfo, old) // if the URL does not match, we consider it old
					}
				}
				newFetchCandidates = append(newFetchCandidates, c)
			}
			fetchCandidates = newFetchCandidates
		}

		for _, c := range fetchCandidates {
			slog.Info("Downloading GeoLite.mmdb from GitHub", slog.String("url", c.Url), slog.String("name", c.Name))

			// http.DefaultClient.Timeout = 10 * 60 // 10 minutes
			resp, err := http.Get(c.Url)
			if err != nil {
				return nil, fmt.Errorf("failed to download GeoLite.mmdb: %w", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("failed to download GeoLite.mmdb: %s", resp.Status)
			}
			// save into temporary file
			tmpFile, err := os.CreateTemp("", c.Name)
			if err != nil {
				return nil, fmt.Errorf("failed to create temporary file: %w", err)
			}
			// defer os.Remove(tmpFile.Name())

			_, err = io.Copy(tmpFile, resp.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to save GeoLite.mmdb: %w", err)
			}

			slog.Info("downloaded successfully", slog.String("file", tmpFile.Name()))
			c.SavedPath = tmpFile.Name()

			tmpFile.Close()

			useCandidates = append(useCandidates, c)
		}
	}

	dbs := &GeoLocationInfo{}

	for _, c := range useCandidates {
		slog.Info("Loading GeoLite.mmdb", slog.String("file", c.SavedPath))

		db, err := geoip2.Open(c.SavedPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open GeoLite.mmdb: %w", err)
		}

		slog.Info("db loaded successfully", slog.String("file", c.SavedPath))

		switch c.Name {
		case "GeoLite2-ASN.mmdb":
			if dbs.asn != nil {
				slog.Warn("ASN database already loaded, skipping", slog.String("file", c.SavedPath))
				continue
			}
			dbs.asn = db
			slog.Info("ASN database loaded", slog.String("file", c.SavedPath))
		case "GeoLite2-City.mmdb":
			if dbs.city != nil {
				slog.Warn("City database already loaded, skipping", slog.String("file", c.SavedPath))
				continue
			}
			dbs.city = db
			slog.Info("City database loaded", slog.String("file", c.SavedPath))
		default:
			slog.Warn("Unknown GeoLite.mmdb file", slog.String("file", c.Name))
			continue
		}
		dbs.originalInfo = append(dbs.originalInfo, c)
	}
	if dbs.asn == nil || dbs.city == nil {
		return nil, fmt.Errorf("failed to load all GeoLite.mmdb files: ASN=%v, City=%v", dbs.asn != nil, dbs.city != nil)
	}

	runtime.AddCleanup(dbs, func(s struct{}) {
		dbs.Close()
	}, struct{}{})

	if len(removeOldInfo) > 0 {
		slog.Info("Removing old fetch info files", slog.Int("count", len(removeOldInfo)))
		for _, old := range removeOldInfo {
			if old.SavedPath != "" {
				if err := os.Remove(old.SavedPath); err != nil {
					slog.Error("Failed to remove old fetch info file", slog.String("file", old.SavedPath), slog.String("error", err.Error()))
				} else {
					slog.Info("Removed old fetch info file", slog.String("file", old.SavedPath))
				}
			}
		}
	}

	if fetchInfoPath != "" && (len(oldFetchInfo) == 0 || len(removeOldInfo) > 0) {
		slog.Info("Saving fetch info to file", slog.String("path", fetchInfoPath))
		fetchInfoRaw, err := json.MarshalIndent(useCandidates, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal fetch info: %w", err)
		}
		if err := os.WriteFile(fetchInfoPath, fetchInfoRaw, 0644); err != nil {
			return nil, fmt.Errorf("failed to write fetch info file: %w", err)
		}
		slog.Info("Fetch info saved successfully")
	} else {
		slog.Info("No fetch info file specified or no old info to remove, skipping save")
	}

	runtime.KeepAlive(dbs)

	return dbs, nil
}
