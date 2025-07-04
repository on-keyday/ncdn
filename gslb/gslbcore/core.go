package gslbcore

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/netip"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/oschwald/geoip2-golang/v2"
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

func formatFloatPtr(f *float64) string {
	if f == nil {
		return "nil"
	}
	return fmt.Sprintf("%f", *f)
}

func calculateLocationDistance(loc1, loc2 *geoip2.Location) float64 {
	if loc1 == nil || loc2 == nil {
		return math.MaxFloat64 // return a large value if either location is nil
	}
	if loc1.Latitude == nil || loc1.Longitude == nil || loc2.Latitude == nil || loc2.Longitude == nil {
		return math.MaxFloat64
	}
	// Haversine formula to calculate distance between two points on the Earth
	const R = 6371e3                                      // Earth radius in meters
	lat1 := *loc1.Latitude * (3.141592653589793 / 180.0)  // Convert degrees to radians
	lon1 := *loc1.Longitude * (3.141592653589793 / 180.0) // Convert degrees to radians
	lat2 := *loc2.Latitude * (3.141592653589793 / 180.0)  // Convert degrees to radians
	lon2 := *loc2.Longitude * (3.141592653589793 / 180.0) // Convert degrees to radians
	dlat := lat2 - lat1
	dlon := lon2 - lon1
	a := (math.Sin(dlat/2) * math.Sin(dlat/2))
	b := math.Cos(lat1) * math.Cos(lat2) * (math.Sin(dlon/2) * math.Sin(dlon/2))
	c := 2 * math.Atan2(math.Sqrt(a+b), math.Sqrt(1-a-b))
	distance := R * c
	return distance // in meters
}

func debugLogGeoLocation(msg string, geoLoc *GeoLocation, srcIP netip.Addr, optionalAttrs ...any) {
	if geoLoc == nil {
		slog.Debug(msg, slog.String("srcIP", srcIP.String()), slog.String("geoLocation", "nil"))
		return
	}
	slog.Debug(msg,
		append([]any{
			slog.String("srcIP", srcIP.String()),
			slog.String("continent", geoLoc.City.Continent.Names.English),
			slog.String("country", geoLoc.City.Country.Names.English),
			slog.String("city", geoLoc.City.City.Names.English),
			slog.String("asn", strconv.Itoa(int(geoLoc.ASN.AutonomousSystemNumber))),
			slog.String("asnName", geoLoc.ASN.AutonomousSystemOrganization),
			slog.String("latitude", formatFloatPtr(geoLoc.City.Location.Latitude)),
			slog.String("longitude", formatFloatPtr(geoLoc.City.Location.Longitude)),
		}, // add the default attributes
			optionalAttrs...)...,
	)
}

func (c *GslbCore) collectCandidateRegions(srcIP netip.Addr, geoLoc *GeoLocation) []*RegionState {
	var candidateRegions []*RegionState
	for i, region := range c.regionGeoLocations {
		for _, regionLoc := range region {
			if geoLoc.City.Continent.Names.English == regionLoc.City.Continent.Names.English &&
				geoLoc.City.Country.Names.English == regionLoc.City.Country.Names.English &&
				geoLoc.City.City.Names.English == regionLoc.City.City.Names.English {
				debugLogGeoLocation("Matched region by continent, country and city", geoLoc, srcIP)
				candidateRegions = append(candidateRegions, c.regions[i])
				break
			}
		}
	}
	if len(candidateRegions) == 0 {
		// next, try to match by continent and country
		for i, region := range c.regionGeoLocations {
			for _, regionLoc := range region {
				if geoLoc.City.Continent.Names.English == regionLoc.City.Continent.Names.English &&
					geoLoc.City.Country.Names.English == regionLoc.City.Country.Names.English {
					debugLogGeoLocation("Matched region by continent and country", geoLoc, srcIP)
					candidateRegions = append(candidateRegions, c.regions[i])
					break
				}
			}
		}
	}
	if len(candidateRegions) == 0 {
		// finally, try to match by continent only
		for i, region := range c.regionGeoLocations {
			for _, regionLoc := range region {
				if geoLoc.City.Continent.Names.English == regionLoc.City.Continent.Names.English {
					debugLogGeoLocation("Matched region by continent", geoLoc, srcIP)
					candidateRegions = append(candidateRegions, c.regions[i])
					break
				}
			}
		}
	}
	return candidateRegions
}

func (c *GslbCore) collectPoPPhysicalDistance(srcIP netip.Addr, geoLoc *GeoLocation) ([]float64, int) {
	var candidatePopIndex []float64
	var smallestDistance = math.MaxFloat64
	var smallestIndex = -1
	for i, popGeoLoc := range c.popGeoLocations {
		if popGeoLoc == nil || geoLoc == nil {
			continue // skip if either is nil
		}
		distance := calculateLocationDistance(&popGeoLoc.City.Location, &geoLoc.City.Location)
		candidatePopIndex = append(candidatePopIndex, distance)
		if distance < smallestDistance {
			smallestDistance = distance
			smallestIndex = i
		}
	}
	return candidatePopIndex, smallestIndex
}

func (c *GslbCore) collectRegionPhysicalDistance(srcIP netip.Addr, geoLoc *GeoLocation) ([][]float64, int, int) {
	var candidateRegionIndex [][]float64
	var smallestDistance = math.MaxFloat64
	var smallestIndex = -1
	var candidateRegionIndexSmallestIndex = -1
	for i, region := range c.regionGeoLocations {
		var distances []float64
		for j, regionLoc := range region {
			if regionLoc == nil || geoLoc == nil {
				distances = append(distances, math.MaxFloat64) // append MaxFloat64 if either is nil
				continue                                       // skip if either is nil
			}
			distance := calculateLocationDistance(&regionLoc.City.Location, &geoLoc.City.Location)
			distances = append(distances, distance)
			if distance < smallestDistance {
				smallestDistance = distance
				smallestIndex = i
				candidateRegionIndexSmallestIndex = j
			}
		}
		candidateRegionIndex = append(candidateRegionIndex, distances)
	}
	return candidateRegionIndex, smallestIndex, candidateRegionIndexSmallestIndex
}

// TODO: make it configurable
const physicalDistanceConfidenceThreshold = 1000000    // 1,000,000 meters (1,000 km)
const proberConfidenceHighDistanceThreshold = 100000   // 100,000 meters (100 km)
const proberConfidenceMediumDistanceThreshold = 500000 // 500,000 meters (500 km)

const (
	proberConfidenceHigh   = "high"
	proberConfidenceMedium = "medium"
	proberConfidenceLow    = "low"
)

func (c *GslbCore) Query(srcIP netip.Addr) []netip.Addr {
	slog.Info("Query", slog.String("srcIP", srcIP.String()))

	c.mu.Lock()
	defer c.mu.Unlock()

	geoLoc, err := c.geo.GeoLocation(srcIP)

	if err != nil {
		slog.Warn("Failed to lookup GeoLocation for srcIP", slog.String("srcIP", srcIP.String()), slog.String("error", err.Error()))
	} else {
		debugLogGeoLocation("GeoLocation for srcIP fetched successfully", geoLoc, srcIP)
		// first, try to calculate physical distance between geoLoc and popGeoLocations

		candidateRegions := c.collectCandidateRegions(srcIP, geoLoc)
		_, smallestPIndex := c.collectPoPPhysicalDistance(srcIP, geoLoc)
		regionPhysicalDistance, smallestRIndex, smallestSubIndex := c.collectRegionPhysicalDistance(srcIP, geoLoc)

		confidence := proberConfidenceLow // default to low confidence

		if smallestRIndex >= 0 && smallestPIndex >= 0 {
			mostSmall := regionPhysicalDistance[smallestRIndex][smallestSubIndex]

			switch {
			case mostSmall < proberConfidenceHighDistanceThreshold:
				confidence = proberConfidenceHigh
			case mostSmall < proberConfidenceMediumDistanceThreshold:
				confidence = proberConfidenceMedium
			}
		}

		var popByDistance *types.PoPInfo

		if smallestPIndex >= 0 {
			popByDistance = &c.cfg.Pops[smallestPIndex]

			debugLogGeoLocation("Selected PoP by physical distance", c.popGeoLocations[smallestPIndex], srcIP,
				slog.Float64("distance", regionPhysicalDistance[smallestRIndex][smallestSubIndex]),
			)
		}

		var (
			lowestLatency   = float64(10000000) // random long latency
			regionState     *RegionState
			popByLatency    *types.PoPInfo
			popLatencyIndex int
		)
		for _, region := range candidateRegions {
			popIndex := make([]int, len(c.cfg.Pops))
			for j := range c.cfg.Pops {
				popIndex[j] = j
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
					regionState = region
					popByLatency = &c.cfg.Pops[popIndex[0]]
					popLatencyIndex = popIndex[0]
				}
			}
		}
		if popByLatency != nil {
			debugLogGeoLocation("Selected PoP by latency", c.popGeoLocations[popLatencyIndex], srcIP,
				slog.Float64("latency", lowestLatency),
				slog.String("latency_confidence", confidence),
				slog.String("regionId", regionState.info.Id),
				slog.String("regionProberURL", regionState.info.ProberURL),
			)
		}

		if popByDistance != nil && popByLatency != nil {
			if popByDistance == popByLatency { // if both are the same, return it
				debugLogGeoLocation("Selected PoP by both physical distance and latency", c.popGeoLocations[smallestPIndex], srcIP,
					slog.Float64("latency", lowestLatency),
					slog.String("latency_confidence", confidence),
					slog.String("regionId", regionState.info.Id),
					slog.String("regionProberURL", regionState.info.ProberURL),
				)
				return []netip.Addr{popByDistance.Ip4}
			}
			// TODO: make it configurable and more efficient
			if confidence == proberConfidenceLow {
				debugLogGeoLocation("Selected PoP by physical distance due to low confidence in latency", c.popGeoLocations[smallestPIndex], srcIP,
					slog.Float64("latency", lowestLatency),
					slog.String("latency_confidence", confidence),
					slog.String("regionId", regionState.info.Id),
					slog.String("regionProberURL", regionState.info.ProberURL),
				)
				return []netip.Addr{popByDistance.Ip4}
			} else {
				debugLogGeoLocation("Selected PoP by latency due to higher confidence", c.popGeoLocations[popLatencyIndex], srcIP,
					slog.Float64("latency", lowestLatency),
					slog.String("latency_confidence", confidence),
					slog.String("regionId", regionState.info.Id),
					slog.String("regionProberURL", regionState.info.ProberURL),
				)
				return []netip.Addr{popByLatency.Ip4}
			}
		}
		if popByDistance != nil {
			debugLogGeoLocation("Selected PoP by physical distance", c.popGeoLocations[smallestPIndex], srcIP)
			return []netip.Addr{popByDistance.Ip4}
		}
		if popByLatency != nil {
			debugLogGeoLocation("Selected PoP by latency", c.popGeoLocations[popLatencyIndex], srcIP,
				slog.Float64("latency", lowestLatency),
				slog.String("latency_confidence", confidence),
				slog.String("regionId", regionState.info.Id),
				slog.String("regionProberURL", regionState.info.ProberURL),
			)
			return []netip.Addr{popByLatency.Ip4}
		}
		// no PoP found by distance or latency, fallback to the first PoP
	}
	// TODO: make it configurable or use a better fallback strategy
	slog.Warn("falling back to the first PoP", slog.String("srcIP", srcIP.String()), slog.String("popId", c.cfg.Pops[0].Id), slog.String("popIP", c.cfg.Pops[0].Ip4.String()))
	return []netip.Addr{c.cfg.Pops[0].Ip4}
}
