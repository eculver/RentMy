package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/rand"
	"sort"
	"time"
)

// repo is the interface the Service uses for data access.
// Defined as an interface so tests can inject a fake.
type repo interface {
	FindNearby(ctx context.Context, q FeedQuery) ([]*RankedListing, error)
	SearchFulltext(ctx context.Context, q SearchQuery) ([]*RankedListing, error)
	FindInBoundingBox(ctx context.Context, q MapQuery) ([]*RankedListing, error)
	GetHostStats(ctx context.Context, hostID string) (HostStats, error)
}

// driveTimer fetches drive-time estimates.
type driveTimer interface {
	Estimate(ctx context.Context, fromLat, fromLng, toLat, toLng float64) (float64, error)
}

// Config holds tunable parameters for the Service.
type Config struct {
	WeightAvailability  float64
	WeightProximity     float64
	WeightReputation    float64
	WeightReliability   float64
	DefaultRadiusMeters int
	MaxFeedLimit        int
	MaxMapLimit         int
}

// Service implements discovery business logic: feed, search, and map.
type Service struct {
	repo      repo
	drivetime driveTimer
	cfg       Config
}

// NewService constructs a Service with the given dependencies.
func NewService(r repo, dt driveTimer, cfg Config) *Service {
	return &Service{repo: r, drivetime: dt, cfg: cfg}
}

// Feed returns nearby active listings ranked by proximity, availability, and host reputation.
func (s *Service) Feed(ctx context.Context, q FeedQuery) ([]*RankedListing, error) {
	if q.RadiusMeters <= 0 {
		q.RadiusMeters = s.cfg.DefaultRadiusMeters
	}
	if q.Limit <= 0 || q.Limit > s.cfg.MaxFeedLimit {
		q.Limit = 20
	}

	listings, err := s.repo.FindNearby(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("feed: %w", err)
	}

	return s.enrich(ctx, listings, q.Lat, q.Lng), nil
}

// Search returns listings matching a keyword query within radius, ranked.
func (s *Service) Search(ctx context.Context, q SearchQuery) ([]*RankedListing, error) {
	if q.RadiusMeters <= 0 {
		q.RadiusMeters = s.cfg.DefaultRadiusMeters
	}
	if q.Limit <= 0 || q.Limit > s.cfg.MaxFeedLimit {
		q.Limit = 20
	}

	listings, err := s.repo.SearchFulltext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	results := s.enrich(ctx, listings, q.Lat, q.Lng)

	// Apply drive-time filter if requested.
	if q.MaxDriveMin != nil && *q.MaxDriveMin > 0 {
		max := float64(*q.MaxDriveMin)
		filtered := results[:0]
		for _, rl := range results {
			if rl.DriveTimeMin <= max {
				filtered = append(filtered, rl)
			}
		}
		results = filtered
	}

	return results, nil
}

// Map returns listings within a bounding box with fuzzed coordinates.
// Results are not ranked (spatial, not relevance-based).
func (s *Service) Map(ctx context.Context, q MapQuery) ([]*RankedListing, error) {
	if q.Limit <= 0 || q.Limit > s.cfg.MaxMapLimit {
		q.Limit = 100
	}

	listings, err := s.repo.FindInBoundingBox(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("map: %w", err)
	}

	for _, rl := range listings {
		lat, lng := exactCoords(rl)
		rl.FuzzedLat, rl.FuzzedLng = fuzzLocation(lat, lng, rl.ID)
	}

	return listings, nil
}

// enrich adds drive time, availability, rank score, and fuzzed location to each listing.
// Listings from hosts with response_rate < 0.30 are filtered out.
func (s *Service) enrich(ctx context.Context, listings []*RankedListing, userLat, userLng float64) []*RankedListing {
	now := nowUTC()
	maxDriveTime := 0.0

	type enriched struct {
		rl    *RankedListing
		stats HostStats
		avail bool
	}

	// First pass: fetch host stats, compute drive time.
	var candidates []enriched
	for _, rl := range listings {
		stats, err := s.repo.GetHostStats(ctx, rl.HostID)
		if err != nil {
			continue
		}
		if stats.ResponseRate < 0.30 {
			continue
		}

		exactLat, exactLng := exactCoords(rl)
		driveMin, _ := s.drivetime.Estimate(ctx, userLat, userLng, exactLat, exactLng)
		rl.DriveTimeMin = driveMin
		if driveMin > maxDriveTime {
			maxDriveTime = driveMin
		}

		candidates = append(candidates, enriched{
			rl:    rl,
			stats: stats,
			avail: isAvailableNow(rl.Availability, now),
		})
	}

	// Second pass: compute rank scores with normalised maxDriveTime, apply fuzz.
	result := make([]*RankedListing, 0, len(candidates))
	for _, c := range candidates {
		exactLat, exactLng := exactCoords(c.rl)
		c.rl.RankScore = s.computeRankScore(
			c.avail,
			c.rl.DriveTimeMin,
			maxDriveTime,
			c.rl.HostReputation,
			c.stats.ResponseRate,
			c.stats.OnTimeRate,
			c.stats.AcceptanceRate,
		)
		c.rl.FuzzedLat, c.rl.FuzzedLng = fuzzLocation(exactLat, exactLng, c.rl.ID)
		result = append(result, c.rl)
	}

	// Sort descending by rank score.
	sort.Slice(result, func(i, j int) bool {
		return result[i].RankScore > result[j].RankScore
	})

	return result
}

// computeRankScore implements the ranking formula from PRD section 13.
// All inputs are normalized to [0, 1] before weighting.
func (s *Service) computeRankScore(
	availableNow bool,
	driveTimeMin float64,
	maxDriveTimeMin float64,
	hostReputation int,
	responseRate float64,
	onTimeRate float64,
	acceptanceRate float64,
) float64 {
	avail := 0.0
	if availableNow {
		avail = 1.0
	}

	proximity := 0.0
	if maxDriveTimeMin > 0 {
		proximity = 1.0 - (driveTimeMin / maxDriveTimeMin)
		if proximity < 0 {
			proximity = 0
		}
	}

	reputation := float64(hostReputation) / 1000.0
	reliability := (responseRate + onTimeRate + acceptanceRate) / 3.0

	return s.cfg.WeightAvailability*avail +
		s.cfg.WeightProximity*proximity +
		s.cfg.WeightReputation*reputation +
		s.cfg.WeightReliability*reliability
}

// fuzzLocation adds a deterministic random offset (~500 m) to exact coordinates.
// The same listing ID always produces the same fuzzed location so pins don't
// jump on repeated loads.
func fuzzLocation(lat, lng float64, seed string) (float64, float64) {
	h := fnv.New64a()
	h.Write([]byte(seed))
	//nolint:gosec // deterministic PRNG seeded by listing ID, not used for security
	rng := rand.New(rand.NewSource(int64(h.Sum64())))

	// ~500 m in degrees: ±0.0045° latitude, ±0.0055° longitude.
	latOffset := (rng.Float64() - 0.5) * 0.009
	lngOffset := (rng.Float64() - 0.5) * 0.011
	return lat + latOffset, lng + lngOffset
}

// isAvailableNow returns true if t falls within any availability window
// in the listing's availability JSON array, or if the array is empty
// (treated as always available).
func isAvailableNow(availability []byte, t time.Time) bool {
	if len(availability) == 0 {
		return true
	}
	var slots []TimeSlot
	if err := json.Unmarshal(availability, &slots); err != nil || len(slots) == 0 {
		return true
	}
	dow := int(t.Weekday())
	hour := t.Hour()
	for _, slot := range slots {
		if slot.DayOfWeek == dow && hour >= slot.StartHour && hour < slot.EndHour {
			return true
		}
	}
	return false
}
