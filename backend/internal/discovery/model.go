// Package discovery implements the RentMy discovery domain: feed, search, and map queries.
package discovery

import "time"

// FeedQuery represents parameters for the nearby listings feed.
type FeedQuery struct {
	Lat          float64 // user latitude
	Lng          float64 // user longitude
	RadiusMeters int     // defaults to config.DefaultFeedRadiusMeters
	Cursor       string  // ULID cursor for pagination (exclusive)
	Limit        int     // defaults to 20, capped at config.MaxFeedLimit
}

// SearchQuery represents parameters for keyword + geo search.
type SearchQuery struct {
	Query        string
	Lat          float64
	Lng          float64
	RadiusMeters int
	MinPrice     *float64
	MaxPrice     *float64
	MaxDriveMin  *int
	Cursor       string
	Limit        int
}

// MapQuery represents parameters for map bounding-box discovery.
type MapQuery struct {
	SWLat float64 // southwest corner latitude
	SWLng float64 // southwest corner longitude
	NELat float64 // northeast corner latitude
	NELng float64 // northeast corner longitude
	Limit int     // defaults to 100, capped at config.MaxMapLimit
}

// HostStats holds aggregated transaction statistics for a host.
// Used in the ranking formula.
type HostStats struct {
	ResponseRate   float64 // fraction of requests that were not auto-declined
	OnTimeRate     float64 // fraction of completed rentals returned on time
	AcceptanceRate float64 // fraction of non-cancelled requests that were accepted
}

// RankedListing is a listing enriched with discovery metadata.
// Fuzzed coordinates are used for display; exact coordinates are never returned.
type RankedListing struct {
	// Core listing fields
	ID          string    `json:"id"`
	HostID      string    `json:"hostId"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	PricePerDay *float64  `json:"pricePerDay,omitempty"`
	PricePerHour *float64 `json:"pricePerHour,omitempty"`
	Status      string    `json:"status"`
	HasVideo    bool      `json:"hasVideo"`
	CreatedAt   time.Time `json:"createdAt"`

	// Availability (raw JSONB, forwarded to clients)
	Availability []byte `json:"availability"`

	// Host info
	HostName       string `json:"hostName"`
	HostReputation int    `json:"hostReputation"`

	// Discovery-computed fields
	DistanceMeters float64 `json:"distanceMeters"`
	DriveTimeMin   float64 `json:"driveTimeMin"`
	RankScore      float64 `json:"rankScore"`

	// Fuzzed coordinates for display (~500 m jitter from actual location)
	FuzzedLat float64 `json:"lat"`
	FuzzedLng float64 `json:"lng"`

	// First listing photo thumbnail (empty string if no media)
	ThumbnailURL string `json:"thumbnailUrl"`
}

// TimeSlot is an availability window within a week.
// DayOfWeek: 0 = Sunday, 6 = Saturday. Hours in 0–23 range.
type TimeSlot struct {
	DayOfWeek int `json:"dayOfWeek"`
	StartHour int `json:"startHour"`
	EndHour   int `json:"endHour"`
}
