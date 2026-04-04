package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles Postgres queries for the discovery domain.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a Repository backed by the given connection pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// FindNearby returns active listings within the given radius of (lat, lng),
// ordered by ULID descending (newest first), paginated via cursor.
// Returns partial RankedListing values; drive time and rank score are populated
// by the Service layer.
func (r *Repository) FindNearby(ctx context.Context, q FeedQuery) ([]*RankedListing, error) {
	const query = `
		SELECT
			l.id, l.host_id, l.title, l.description,
			l.price_per_day, l.price_per_hour, l.status, l.has_video, l.created_at,
			l.availability,
			ST_Y(l.location::geometry) AS exact_lat,
			ST_X(l.location::geometry) AS exact_lng,
			ST_Distance(l.location, ST_MakePoint($2, $1)::geography) AS distance_meters,
			u.name AS host_name,
			u.reputation_score AS host_reputation,
			COALESCE(m.thumbnail_url, '') AS thumbnail_url
		FROM listings l
		JOIN users u ON l.host_id = u.id
		LEFT JOIN LATERAL (
			SELECT thumbnail_url FROM media
			WHERE listing_id = l.id AND media_type = 'LISTING_PHOTO'
			ORDER BY created_at ASC LIMIT 1
		) m ON true
		WHERE l.status = 'ACTIVE'
		  AND l.location IS NOT NULL
		  AND ST_DWithin(l.location, ST_MakePoint($2, $1)::geography, $3)
		  AND ($4 = '' OR l.id < $4)
		ORDER BY l.id DESC
		LIMIT $5`

	rows, err := r.pool.Query(ctx, query,
		q.Lat, q.Lng, float64(q.RadiusMeters), q.Cursor, q.Limit)
	if err != nil {
		return nil, fmt.Errorf("find nearby: %w", err)
	}
	defer rows.Close()
	return scanRankedListings(rows)
}

// SearchFulltext returns active listings matching a keyword query within radius.
func (r *Repository) SearchFulltext(ctx context.Context, q SearchQuery) ([]*RankedListing, error) {
	const query = `
		SELECT
			l.id, l.host_id, l.title, l.description,
			l.price_per_day, l.price_per_hour, l.status, l.has_video, l.created_at,
			l.availability,
			ST_Y(l.location::geometry) AS exact_lat,
			ST_X(l.location::geometry) AS exact_lng,
			ST_Distance(l.location, ST_MakePoint($2, $1)::geography) AS distance_meters,
			u.name AS host_name,
			u.reputation_score AS host_reputation,
			COALESCE(m.thumbnail_url, '') AS thumbnail_url
		FROM listings l
		JOIN users u ON l.host_id = u.id
		LEFT JOIN LATERAL (
			SELECT thumbnail_url FROM media
			WHERE listing_id = l.id AND media_type = 'LISTING_PHOTO'
			ORDER BY created_at ASC LIMIT 1
		) m ON true
		WHERE l.status = 'ACTIVE'
		  AND l.location IS NOT NULL
		  AND ST_DWithin(l.location, ST_MakePoint($2, $1)::geography, $3)
		  AND (
		      l.search_vector @@ plainto_tsquery('english', $6)
		      OR l.title ILIKE '%' || $6 || '%'
		  )
		  AND ($7::numeric IS NULL OR l.price_per_day >= $7 OR l.price_per_hour >= $7)
		  AND ($8::numeric IS NULL OR l.price_per_day <= $8 OR l.price_per_hour <= $8)
		  AND ($4 = '' OR l.id < $4)
		ORDER BY l.id DESC
		LIMIT $5`

	rows, err := r.pool.Query(ctx, query,
		q.Lat, q.Lng, float64(q.RadiusMeters), q.Cursor, q.Limit,
		q.Query, q.MinPrice, q.MaxPrice)
	if err != nil {
		return nil, fmt.Errorf("search fulltext: %w", err)
	}
	defer rows.Close()
	return scanRankedListings(rows)
}

// FindInBoundingBox returns active listings within the given lat/lng bounding box.
// No cursor pagination — map results are not paged.
func (r *Repository) FindInBoundingBox(ctx context.Context, q MapQuery) ([]*RankedListing, error) {
	const query = `
		SELECT
			l.id, l.host_id, l.title, l.description,
			l.price_per_day, l.price_per_hour, l.status, l.has_video, l.created_at,
			l.availability,
			ST_Y(l.location::geometry) AS exact_lat,
			ST_X(l.location::geometry) AS exact_lng,
			0.0 AS distance_meters,
			u.name AS host_name,
			u.reputation_score AS host_reputation,
			COALESCE(m.thumbnail_url, '') AS thumbnail_url
		FROM listings l
		JOIN users u ON l.host_id = u.id
		LEFT JOIN LATERAL (
			SELECT thumbnail_url FROM media
			WHERE listing_id = l.id AND media_type = 'LISTING_PHOTO'
			ORDER BY created_at ASC LIMIT 1
		) m ON true
		WHERE l.status = 'ACTIVE'
		  AND l.location IS NOT NULL
		  AND ST_Within(
		      l.location::geometry,
		      ST_MakeEnvelope($1, $2, $3, $4, 4326)
		  )
		ORDER BY l.id DESC
		LIMIT $5`

	rows, err := r.pool.Query(ctx, query,
		q.SWLng, q.SWLat, q.NELng, q.NELat, q.Limit)
	if err != nil {
		return nil, fmt.Errorf("find in bounding box: %w", err)
	}
	defer rows.Close()
	return scanRankedListings(rows)
}

// GetHostStats returns aggregated transaction metrics for a host.
// Returns default values (0.5 for all rates) when no transactions exist.
func (r *Repository) GetHostStats(ctx context.Context, hostID string) (HostStats, error) {
	const query = `
		SELECT
			COALESCE(
				COUNT(*) FILTER (WHERE status != 'AUTO_DECLINED')::float / NULLIF(COUNT(*), 0),
				0.5
			) AS response_rate,
			COALESCE(
				COUNT(*) FILTER (WHERE status = 'COMPLETED' AND actual_end <= scheduled_end)::float /
					NULLIF(COUNT(*) FILTER (WHERE status = 'COMPLETED'), 0),
				0.5
			) AS on_time_rate,
			COALESCE(
				COUNT(*) FILTER (WHERE status IN ('ACCEPTED', 'ACTIVE', 'COMPLETED'))::float /
					NULLIF(COUNT(*) FILTER (WHERE status NOT IN ('REQUESTED')), 0),
				0.5
			) AS acceptance_rate
		FROM transactions
		WHERE host_id = $1`

	var stats HostStats
	err := r.pool.QueryRow(ctx, query, hostID).Scan(
		&stats.ResponseRate,
		&stats.OnTimeRate,
		&stats.AcceptanceRate,
	)
	if err != nil {
		return HostStats{ResponseRate: 0.5, OnTimeRate: 0.5, AcceptanceRate: 0.5},
			fmt.Errorf("get host stats: %w", err)
	}
	return stats, nil
}

// scanRankedListings scans a pgx.Rows result into a slice of RankedListing.
// The exact_lat and exact_lng columns are stored privately for fuzzing by the service.
func scanRankedListings(rows pgx.Rows) ([]*RankedListing, error) {
	var results []*RankedListing
	for rows.Next() {
		rl, err := scanRankedListing(rows)
		if err != nil {
			return nil, fmt.Errorf("scan ranked listing: %w", err)
		}
		results = append(results, rl)
	}
	return results, rows.Err()
}

// scanRankedListing scans a single row into a RankedListing.
// FuzzedLat/FuzzedLng are initially set to the exact coordinates; the service
// applies jitter before returning to callers.
func scanRankedListing(row pgx.Row) (*RankedListing, error) {
	var (
		rl        RankedListing
		priceDay  *float64
		priceHour *float64
		avail     []byte
		status    string
		exactLat  float64
		exactLng  float64
	)

	err := row.Scan(
		&rl.ID, &rl.HostID, &rl.Title, &rl.Description,
		&priceDay, &priceHour, &status, &rl.HasVideo, &rl.CreatedAt,
		&avail,
		&exactLat, &exactLng,
		&rl.DistanceMeters,
		&rl.HostName, &rl.HostReputation,
		&rl.ThumbnailURL,
	)
	if err != nil {
		return nil, err
	}

	rl.Status = status
	rl.PricePerDay = priceDay
	rl.PricePerHour = priceHour
	rl.Availability = avail
	if rl.Availability == nil {
		rl.Availability = []byte("[]")
	}

	// Store exact coords in Fuzzed fields temporarily; service applies jitter.
	rl.FuzzedLat = exactLat
	rl.FuzzedLng = exactLng

	return &rl, nil
}

// exactCoords is a helper used internally during ranking to extract the stored
// exact coordinates from a RankedListing before fuzzing is applied.
// After the service calls fuzzLocation, FuzzedLat/FuzzedLng become the display coords.
func exactCoords(rl *RankedListing) (lat, lng float64) {
	return rl.FuzzedLat, rl.FuzzedLng
}

// nowUTC returns the current UTC time for availability checks.
// Extracted so tests can override it via a package-level variable.
var nowUTC = func() time.Time { return time.Now().UTC() }
