package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// DriveTimeClient fetches drive-time estimates from a self-hosted OSRM instance.
// Responses are cached in Redis to avoid redundant routing calls.
type DriveTimeClient struct {
	baseURL    string
	httpClient *http.Client
	cache      *redis.Client
	cacheTTL   time.Duration
}

// NewDriveTimeClient constructs a DriveTimeClient.
func NewDriveTimeClient(baseURL string, cache *redis.Client) *DriveTimeClient {
	return &DriveTimeClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 3 * time.Second},
		cache:      cache,
		cacheTTL:   60 * time.Minute,
	}
}

// osrmResponse is the JSON shape returned by OSRM's route endpoint.
type osrmResponse struct {
	Routes []struct {
		Duration float64 `json:"duration"` // seconds
	} `json:"routes"`
}

// Estimate returns the drive time in minutes between two geographic points.
// Responses are cached in Redis keyed on a ~1 km grid cell.
// Returns 0 on any error (OSRM unavailable, no route found) so discovery
// continues to function without drive-time data in development.
func (c *DriveTimeClient) Estimate(
	ctx context.Context,
	fromLat, fromLng, toLat, toLng float64,
) (float64, error) {
	cacheKey := driveTimeCacheKey(fromLat, fromLng, toLat, toLng)

	// Try cache first.
	if cached, err := c.cache.Get(ctx, cacheKey).Float64(); err == nil {
		return cached, nil
	}

	url := fmt.Sprintf(
		"%s/route/v1/driving/%.6f,%.6f;%.6f,%.6f?overview=false",
		c.baseURL, fromLng, fromLat, toLng, toLat,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		slog.Debug("drivetime: build request failed", "error", err)
		return 0, nil
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Debug("drivetime: osrm unreachable", "error", err)
		return 0, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Debug("drivetime: read body failed", "error", err)
		return 0, nil
	}

	var osrm osrmResponse
	if err := json.Unmarshal(body, &osrm); err != nil || len(osrm.Routes) == 0 {
		slog.Debug("drivetime: parse failed or no route", "error", err)
		return 0, nil
	}

	minutes := osrm.Routes[0].Duration / 60.0

	// Cache the result.
	_ = c.cache.Set(ctx, cacheKey, minutes, c.cacheTTL).Err()

	return minutes, nil
}

// driveTimeCacheKey produces a Redis key for a route between two points,
// rounded to a ~1 km grid (2 decimal places ≈ 1.1 km at the equator).
func driveTimeCacheKey(fromLat, fromLng, toLat, toLng float64) string {
	return fmt.Sprintf("drivetime:%.2f,%.2f;%.2f,%.2f",
		math.Round(fromLat*100)/100, math.Round(fromLng*100)/100,
		math.Round(toLat*100)/100, math.Round(toLng*100)/100,
	)
}
