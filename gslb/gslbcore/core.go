package gslbcore

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/yzp0n/ncdn/types"
)

// FetchPoPStatus is a function that fetches PoP status from a PoP.
type FetchPoPStatusFunc func(ctx context.Context, ip netip.Addr) (*types.PoPStatus, error)

type MakeLatencyMeasurerFunc func(proberURL, secret string) LatencyMeasurer

type LatencyMeasurer interface {
	DebugString() string

	// MeasureLatency is a function that measures the latency to the `url`.
	MeasureLatency(ctx context.Context, endpointUrl string) (float64, error)
}

type Config struct {
	Pops         []types.PoPInfo
	Regions      []types.RegionInfo
	ProberSecret string
	HTTPServer   string

	FetchPoPStatus      FetchPoPStatusFunc
	MakeLatencyMeasurer MakeLatencyMeasurerFunc
}

type RegionState struct {
	info       types.RegionInfo
	popLatency []float64
}

type GslbCore struct {
	// shouldn't be changed over lifetime of GslbCore.
	cfg *Config

	// LatencyMeasurer is used to measure latency from a region to a PoP.
	// shouldn't be changed over lifetime of GslbCore.
	latencyMeasurers []LatencyMeasurer

	// pluggable for testing purposes.
	fetchPoPStatus FetchPoPStatusFunc

	// Updated by the `GslbCore.Run()` worker. Access to the fields below should be guarded by `mu`.
	mu       sync.Mutex
	popstate []*types.PoPStatus
	regions  []*RegionState
	serial   uint32

	// TODO: make it as interface?
	geo                *GeoLocationInfo
	popGeoLocations    []*GeoLocation
	regionGeoLocations [][]*GeoLocation
}

func New(cfg *Config) *GslbCore {
	fps := cfg.FetchPoPStatus
	if fps == nil {
		fps = FetchPoPStatusOverHTTP
	}

	mlm := cfg.MakeLatencyMeasurer
	if mlm == nil {
		mlm = func(proberURL, secret string) LatencyMeasurer {
			return ProbeOverJSONRPC{
				ProberURL: proberURL,
				Secret:    secret,
			}
		}
	}

	// TODO: move to config
	geo, err := FetchGeoLocation("secrets/geolite_info.json")
	if err != nil {
		slog.Error("Failed to fetch GeoLocation", slog.String("error", err.Error()))
	} else {
		slog.Info("GeoLocation fetched successfully")
	}

	c := &GslbCore{
		cfg: cfg,

		fetchPoPStatus:   fps,
		latencyMeasurers: make([]LatencyMeasurer, len(cfg.Regions)),

		popstate:           make([]*types.PoPStatus, len(cfg.Pops)),
		regions:            make([]*RegionState, len(cfg.Regions)),
		serial:             0,
		geo:                geo,
		popGeoLocations:    make([]*GeoLocation, len(cfg.Pops)),
		regionGeoLocations: make([][]*GeoLocation, len(cfg.Regions)),
	}
	for i := range c.popstate {
		c.popstate[i] = &types.PoPStatus{
			Error: "not yet available",
		}
	}
	for i := range c.popGeoLocations {
		geoLoc, err := c.geo.GeoLocation(cfg.Pops[i].Ip4)
		if err != nil {
			slog.Warn("Failed to lookup GeoLocation for PoP", slog.String("popId", cfg.Pops[i].Id), slog.String("error", err.Error()))
			c.popGeoLocations[i] = &GeoLocation{
				ASN:  nil,
				City: nil,
			}
		} else {
			slog.Debug("GeoLocation for PoP fetched successfully",
				slog.String("popId", cfg.Pops[i].Id),
				slog.String("continent", geoLoc.City.Continent.Names.English),
				slog.String("country", geoLoc.City.Country.Names.English),
				slog.String("city", geoLoc.City.City.Names.English),
				slog.String("asn", strconv.Itoa(int(geoLoc.ASN.AutonomousSystemNumber))),
				slog.String("asnName", geoLoc.ASN.AutonomousSystemOrganization),
			)
			c.popGeoLocations[i] = geoLoc
		}
	}
	for i, r := range cfg.Regions {
		c.latencyMeasurers[i] = mlm(r.ProberURL, cfg.ProberSecret)

		popLatency := make([]float64, len(c.cfg.Pops))
		for j := range popLatency {
			// initialize to a large value
			popLatency[j] = 10000000
		}

		c.regions[i] = &RegionState{
			info:       r, // copied for convienience
			popLatency: popLatency,
		}
	}

	for i, r := range c.cfg.Regions {
		c.regionGeoLocations[i] = make([]*GeoLocation, len(r.Prefices))
		for j, p := range r.Prefices {
			geoLoc, err := c.geo.GeoLocation(p.Addr())
			if err != nil {
				slog.Warn("Failed to lookup GeoLocation for region", slog.String("regionId", r.Id), slog.String("error", err.Error()))
				c.regionGeoLocations[i][j] = &GeoLocation{
					ASN:  nil,
					City: nil,
				}
			} else {
				slog.Debug("GeoLocation for region fetched successfully",
					slog.String("regionId", r.Id),
					slog.String("continent", geoLoc.City.Continent.Names.English),
					slog.String("country", geoLoc.City.Country.Names.English),
					slog.String("city", geoLoc.City.City.Names.English),
					slog.String("asn", strconv.Itoa(int(geoLoc.ASN.AutonomousSystemNumber))),
					slog.String("asnName", geoLoc.ASN.AutonomousSystemOrganization),
				)
				c.regionGeoLocations[i][j] = geoLoc
			}
		}
	}

	return c
}

func (c *GslbCore) Run(ctx context.Context) error {
	if c.cfg.HTTPServer != "" {
		if err := c.spawnHTTPServer(ctx); err != nil {
			return err
		}
	}

	for {
		ctxU, cancel := context.WithTimeout(ctx, 10*time.Second)
		c.UpdatePoPStatus(ctxU)
		cancel()

		ctxUL, cancel := context.WithTimeout(ctx, 30*time.Second)
		c.UpdateLatency(ctxUL)
		cancel()

		// sleep for 30 seconds, or stop running if the context is done
		select {
		case <-time.After(30 * time.Second):
			// continue

		case <-ctx.Done():
			err := ctx.Err()
			if !errors.Is(err, context.Canceled) {
				return err
			}
			return nil
		}
	}
}

func (c *GslbCore) UpdatePoPStatus(ctx context.Context) {
	slog.Info("UpdatePoPStatus start")
	start := time.Now()
	defer func() {
		slog.Info("UpdatePoPStatus done", slog.Duration("took", time.Since(start)))
	}()

	newstate := make([]*types.PoPStatus, len(c.cfg.Pops))
	for i, pop := range c.cfg.Pops {
		slog.Info("Fetching PoP status", slog.String("pop.Id", pop.Id))
		ps, err := c.fetchPoPStatus(ctx, pop.Ip4)
		if err != nil {
			slog.Error("PoP status fetch failed with error", slog.String("pop.Id", pop.Id), slog.String("error", err.Error()))
			newstate[i] = &types.PoPStatus{
				Error: err.Error(),
			}
			continue
		}

		newstate[i] = ps
	}

	c.mu.Lock()
	c.popstate = newstate
	c.serial++
	c.mu.Unlock()
}

func (c *GslbCore) UpdateLatency(ctx context.Context) {
	slog.Info("UpdateLatency start")
	start := time.Now()
	defer func() {
		slog.Info("UpdateLatency done", slog.Duration("took", time.Since(start)))
	}()

	for i, lm := range c.latencyMeasurers {
		slog.Info("Measuring latency from prober", slog.String("latencyMeasurer", lm.DebugString()))

		popLatency := make([]float64, len(c.cfg.Pops))
		for j := range popLatency {
			lat, err := lm.MeasureLatency(ctx, c.cfg.Pops[j].LatencyEndpointUrl)
			if err != nil {
				slog.Error("Failed to measure latency",
					slog.String("latencyMeasurer", lm.DebugString()),
					slog.String("error", err.Error()))
				lat = 20000000 // random long latancy
			}
			popLatency[j] = lat
		}

		c.mu.Lock()
		c.regions[i].popLatency = popLatency
		c.serial++
		c.mu.Unlock()
	}
}

func (c *GslbCore) Serial() uint32 {
	c.mu.Lock()
	ret := c.serial
	c.mu.Unlock()
	return ret
}

func (c *GslbCore) PopIdFromIP(ip netip.Addr) string {
	for _, pop := range c.cfg.Pops {
		if pop.Ip4.Compare(ip) == 0 {
			return pop.Id
		}
	}

	return "<not found>"
}

func (c *GslbCore) Query(srcIP netip.Addr) []netip.Addr {
	slog.Info("Query", slog.String("srcIP", srcIP.String()))

	c.mu.Lock()
	defer c.mu.Unlock()

	geoLoc, err := c.geo.GeoLocation(srcIP)

	if err != nil {
		slog.Warn("Failed to lookup GeoLocation for srcIP", slog.String("srcIP", srcIP.String()), slog.String("error", err.Error()))
	} else {
		var candidateRegions []*RegionState
		for i, region := range c.regionGeoLocations {
			for _, regeionLoc := range region {
				if geoLoc.City.Continent.Names.English == regeionLoc.City.Continent.Names.English {
					slog.Debug("Matched region by continent",
						slog.String("srcIP", srcIP.String()),
						slog.String("regionId", c.cfg.Regions[i].Id),
						slog.String("continent", geoLoc.City.Continent.Names.English))
					candidateRegions = append(candidateRegions, c.regions[i])
					break
				}
			}
		}
		var (
			currentCandidate netip.Addr = c.cfg.Pops[0].Ip4 // default to the first PoP
			lowestLatency               = float64(10000000) // random long latency
			regeionState     *RegionState
			popInfo          *types.PoPInfo
		)
		for _, region := range candidateRegions {
			var popIndex []int
			for j := range c.cfg.Pops {
				popIndex = append(popIndex, j)
			}
			// sort popIndex by latency
			slices.SortFunc(popIndex, func(a, b int) int {
				if region.popLatency[a] < region.popLatency[b] {
					return -1
				} else if region.popLatency[a] > region.popLatency[b] {
					return 1
				}
				return 0
			})
			if len(popIndex) > 0 {
				if lowestLatency > region.popLatency[popIndex[0]] {
					lowestLatency = region.popLatency[popIndex[0]]
					currentCandidate = c.cfg.Pops[popIndex[0]].Ip4
					regeionState = region
					popInfo = &c.cfg.Pops[popIndex[0]]
				} else {
					slog.Debug("Skipping candidate region",
						slog.String("srcIP", srcIP.String()),
						slog.String("regionId", region.info.Id),
						slog.Float64("latency", region.popLatency[popIndex[0]]))
				}
			}
		}
		if regeionState != nil && popInfo != nil {
			slog.Info("Selected candidate region",
				slog.String("srcIP", srcIP.String()),
				slog.String("regionId", regeionState.info.Id),
				slog.String("popId", popInfo.Id),
				slog.String("popIP", currentCandidate.String()),
				slog.Float64("latency", lowestLatency),
				slog.String("continent", geoLoc.City.Continent.Names.English),
				slog.String("country", geoLoc.City.Country.Names.English),
				slog.String("city", geoLoc.City.City.Names.English),
				slog.String("asn", strconv.Itoa(int(geoLoc.ASN.AutonomousSystemNumber))),
				slog.String("asnName", geoLoc.ASN.AutonomousSystemOrganization),
			)
		} else {
			slog.Warn("No suitable region found, falling back to the first PoP",
				slog.String("srcIP", srcIP.String()),
				slog.String("popId", c.cfg.Pops[0].Id),
				slog.String("popIP", currentCandidate.String()))
		}
		return []netip.Addr{currentCandidate}
	}
	slog.Warn("GeoLocation lookup failed, falling back to the first PoP", slog.String("srcIP", srcIP.String()), slog.String("popId", c.cfg.Pops[0].Id), slog.String("popIP", c.cfg.Pops[0].Ip4.String()))
	return []netip.Addr{c.cfg.Pops[0].Ip4}
}
