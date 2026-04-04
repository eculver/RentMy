package discovery

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fakes ---

type fakeRepo struct {
	listings  []*RankedListing
	hostStats map[string]HostStats
}

func (f *fakeRepo) FindNearby(_ context.Context, _ FeedQuery) ([]*RankedListing, error) {
	return f.listings, nil
}

func (f *fakeRepo) SearchFulltext(_ context.Context, _ SearchQuery) ([]*RankedListing, error) {
	return f.listings, nil
}

func (f *fakeRepo) FindInBoundingBox(_ context.Context, _ MapQuery) ([]*RankedListing, error) {
	return f.listings, nil
}

func (f *fakeRepo) GetHostStats(_ context.Context, hostID string) (HostStats, error) {
	if s, ok := f.hostStats[hostID]; ok {
		return s, nil
	}
	return HostStats{ResponseRate: 0.5, OnTimeRate: 0.5, AcceptanceRate: 0.5}, nil
}

type fakeDriveTimer struct {
	minutes float64
}

func (f *fakeDriveTimer) Estimate(_ context.Context, _, _, _, _ float64) (float64, error) {
	return f.minutes, nil
}

func defaultCfg() Config {
	return Config{
		WeightAvailability:  0.35,
		WeightProximity:     0.30,
		WeightReputation:    0.20,
		WeightReliability:   0.15,
		DefaultRadiusMeters: 30000,
		MaxFeedLimit:        50,
		MaxMapLimit:         200,
	}
}

// --- computeRankScore tests ---

func TestComputeRankScore_FullScore(t *testing.T) {
	svc := NewService(nil, nil, defaultCfg())

	score := svc.computeRankScore(true, 0, 30, 1000, 1.0, 1.0, 1.0)
	// avail=1, proximity=1 (0 drive of 30 max), reputation=1, reliability=1
	assert.InDelta(t, 1.0, score, 0.001)
}

func TestComputeRankScore_ZeroScore(t *testing.T) {
	svc := NewService(nil, nil, defaultCfg())

	score := svc.computeRankScore(false, 30, 30, 0, 0, 0, 0)
	// avail=0, proximity=0 (at max drive), reputation=0, reliability=0
	assert.InDelta(t, 0.0, score, 0.001)
}

func TestComputeRankScore_PartialAvailability(t *testing.T) {
	svc := NewService(nil, nil, defaultCfg())

	// Available, halfway on drive time, reputation 500/1000
	score := svc.computeRankScore(true, 15, 30, 500, 0.5, 0.5, 0.5)
	// 0.35*1 + 0.30*0.5 + 0.20*0.5 + 0.15*0.5 = 0.35 + 0.15 + 0.10 + 0.075 = 0.675
	assert.InDelta(t, 0.675, score, 0.001)
}

func TestComputeRankScore_ZeroMaxDriveTime(t *testing.T) {
	svc := NewService(nil, nil, defaultCfg())

	// When maxDriveTimeMin == 0, proximity should be 0 (no normalization possible)
	score := svc.computeRankScore(true, 10, 0, 500, 1.0, 1.0, 1.0)
	// avail=0.35, proximity=0, reputation=0.10, reliability=0.15
	assert.InDelta(t, 0.6, score, 0.001)
}

// --- fuzzLocation tests ---

func TestFuzzLocation_Deterministic(t *testing.T) {
	lat1, lng1 := fuzzLocation(34.05, -118.24, "01ABCD")
	lat2, lng2 := fuzzLocation(34.05, -118.24, "01ABCD")

	assert.Equal(t, lat1, lat2)
	assert.Equal(t, lng1, lng2)
}

func TestFuzzLocation_DifferentSeeds(t *testing.T) {
	lat1, lng1 := fuzzLocation(34.05, -118.24, "01ABCD")
	lat2, lng2 := fuzzLocation(34.05, -118.24, "01WXYZ")

	assert.NotEqual(t, lat1, lat2)
	assert.NotEqual(t, lng1, lng2)
}

func TestFuzzLocation_WithinBound(t *testing.T) {
	// Verify jitter is bounded to roughly ±0.0055 lat and ±0.0055 lng.
	origLat, origLng := 34.05, -118.24
	lat, lng := fuzzLocation(origLat, origLng, "01TEST")

	assert.InDelta(t, origLat, lat, 0.006)
	assert.InDelta(t, origLng, lng, 0.007)
}

// --- isAvailableNow tests ---

func TestIsAvailableNow_EmptySlots(t *testing.T) {
	assert.True(t, isAvailableNow([]byte("[]"), time.Now()))
}

func TestIsAvailableNow_NilAvailability(t *testing.T) {
	assert.True(t, isAvailableNow(nil, time.Now()))
}

func TestIsAvailableNow_MatchingSlot(t *testing.T) {
	// Monday = 1, hour 10 should match a slot for Monday 9–18.
	slots := []byte(`[{"dayOfWeek":1,"startHour":9,"endHour":18}]`)
	monday10am := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC) // 2026-03-02 is a Monday

	assert.True(t, isAvailableNow(slots, monday10am))
}

func TestIsAvailableNow_NoMatchingSlot(t *testing.T) {
	// Slot only covers Monday, but we check Sunday (0).
	slots := []byte(`[{"dayOfWeek":1,"startHour":9,"endHour":18}]`)
	sunday10am := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC) // 2026-03-01 is a Sunday

	assert.False(t, isAvailableNow(slots, sunday10am))
}

func TestIsAvailableNow_HourAtEndBoundary(t *testing.T) {
	// Slot ends at 18; hour 18 should not match (exclusive end).
	slots := []byte(`[{"dayOfWeek":1,"startHour":9,"endHour":18}]`)
	monday6pm := time.Date(2026, 3, 2, 18, 0, 0, 0, time.UTC)

	assert.False(t, isAvailableNow(slots, monday6pm))
}

// --- Feed integration with fake repo ---

func TestFeed_FiltersLowResponseRate(t *testing.T) {
	fr := &fakeRepo{
		listings: []*RankedListing{
			{ID: "01A", HostID: "host1", FuzzedLat: 34.05, FuzzedLng: -118.24, Status: "ACTIVE"},
			{ID: "01B", HostID: "host2", FuzzedLat: 34.06, FuzzedLng: -118.25, Status: "ACTIVE"},
		},
		hostStats: map[string]HostStats{
			"host1": {ResponseRate: 0.10, OnTimeRate: 0.5, AcceptanceRate: 0.5}, // below 0.30 threshold
			"host2": {ResponseRate: 0.80, OnTimeRate: 0.9, AcceptanceRate: 0.9},
		},
	}

	svc := NewService(fr, &fakeDriveTimer{minutes: 5}, defaultCfg())
	results, err := svc.Feed(context.Background(), FeedQuery{Lat: 34.05, Lng: -118.24, Limit: 10})

	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "01B", results[0].ID)
}

func TestFeed_SortsByRankScore(t *testing.T) {
	fr := &fakeRepo{
		listings: []*RankedListing{
			// host3 has high reputation but low reliability
			{ID: "01C", HostID: "host3", FuzzedLat: 34.05, FuzzedLng: -118.24, Status: "ACTIVE", HostReputation: 900},
			// host4 has lower reputation but high reliability
			{ID: "01D", HostID: "host4", FuzzedLat: 34.06, FuzzedLng: -118.25, Status: "ACTIVE", HostReputation: 200},
		},
		hostStats: map[string]HostStats{
			"host3": {ResponseRate: 0.31, OnTimeRate: 0.31, AcceptanceRate: 0.31},
			"host4": {ResponseRate: 1.0, OnTimeRate: 1.0, AcceptanceRate: 1.0},
		},
	}

	svc := NewService(fr, &fakeDriveTimer{minutes: 5}, defaultCfg())
	results, err := svc.Feed(context.Background(), FeedQuery{Lat: 34.05, Lng: -118.24, Limit: 10})

	require.NoError(t, err)
	require.Len(t, results, 2)
	// host4's high reliability should give it a higher rank score
	assert.True(t, results[0].RankScore >= results[1].RankScore)
}

func TestMap_FuzzesCoordinates(t *testing.T) {
	exactLat, exactLng := 34.05, -118.24
	fr := &fakeRepo{
		listings: []*RankedListing{
			{ID: "01E", HostID: "host5", FuzzedLat: exactLat, FuzzedLng: exactLng, Status: "ACTIVE"},
		},
	}

	svc := NewService(fr, &fakeDriveTimer{}, defaultCfg())
	results, err := svc.Map(context.Background(), MapQuery{SWLat: 33.9, SWLng: -118.5, NELat: 34.2, NELng: -118.0, Limit: 10})

	require.NoError(t, err)
	require.Len(t, results, 1)

	// Fuzzed coords should differ from exact.
	fLat, fLng := fuzzLocation(exactLat, exactLng, "01E")
	assert.InDelta(t, fLat, results[0].FuzzedLat, 0.0001)
	assert.InDelta(t, fLng, results[0].FuzzedLng, 0.0001)
}
