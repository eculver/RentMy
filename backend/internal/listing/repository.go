package listing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a listing record does not exist.
var ErrNotFound = errors.New("listing not found")

// Repository handles Postgres persistence for the listing domain.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a Repository backed by the given connection pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Insert persists a new listing row and returns the record with DB-populated fields.
func (r *Repository) Insert(ctx context.Context, l *Listing) (*Listing, error) {
	const q = `
		INSERT INTO listings (
			id, host_id, title, description,
			price_per_hour, price_per_day,
			min_duration, max_duration,
			location,
			availability, host_declared_value,
			status, created_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6,
			$7, $8,
			CASE WHEN $9::float8 IS NULL THEN NULL
			     ELSE ST_SetSRID(ST_MakePoint($10::float8, $9::float8), 4326)::geography
			END,
			$11, $12,
			$13, NOW()
		)
		RETURNING id, host_id, title, description,
		          ai_generated_tags, estimated_value, host_declared_value, value_justification,
		          price_per_hour, price_per_day,
		          min_duration, max_duration,
		          ST_Y(location::geometry) AS loc_lat, ST_X(location::geometry) AS loc_lng,
		          availability, has_video, status, appraisal_status, created_at`

	var (
		lat *float64
		lng *float64
	)
	if l.Location != nil {
		lat = &l.Location.Lat
		lng = &l.Location.Lng
	}

	row := r.pool.QueryRow(ctx, q,
		l.ID, l.HostID, l.Title, l.Description,
		l.PricePerHour, l.PricePerDay,
		durationToInterval(l.MinDuration), durationToInterval(l.MaxDuration),
		lat, lng,
		availabilityBytes(l.Availability), l.HostDeclaredValue,
		string(l.Status),
	)
	return scanListing(row)
}

// FindByID retrieves a listing by primary key. Returns ErrNotFound if absent.
func (r *Repository) FindByID(ctx context.Context, id string) (*Listing, error) {
	const q = `
		SELECT id, host_id, title, description,
		       ai_generated_tags, estimated_value, host_declared_value, value_justification,
		       price_per_hour, price_per_day,
		       min_duration, max_duration,
		       ST_Y(location::geometry) AS loc_lat, ST_X(location::geometry) AS loc_lng,
		       availability, has_video, status, appraisal_status, created_at
		FROM listings
		WHERE id = $1`

	l, err := scanListing(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find listing by id: %w", err)
	}
	return l, nil
}

// FindByHostID returns a paginated slice of listings owned by the given host.
func (r *Repository) FindByHostID(ctx context.Context, hostID string, page, limit int) ([]*Listing, int, error) {
	const countQ = `SELECT COUNT(*) FROM listings WHERE host_id = $1`
	var total int
	if err := r.pool.QueryRow(ctx, countQ, hostID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count listings: %w", err)
	}

	const q = `
		SELECT id, host_id, title, description,
		       ai_generated_tags, estimated_value, host_declared_value, value_justification,
		       price_per_hour, price_per_day,
		       min_duration, max_duration,
		       ST_Y(location::geometry) AS loc_lat, ST_X(location::geometry) AS loc_lng,
		       availability, has_video, status, appraisal_status, created_at
		FROM listings
		WHERE host_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	offset := (page - 1) * limit
	rows, err := r.pool.Query(ctx, q, hostID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list listings by host: %w", err)
	}
	defer rows.Close()

	var results []*Listing
	for rows.Next() {
		l, err := scanListing(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan listing row: %w", err)
		}
		results = append(results, l)
	}
	return results, total, rows.Err()
}

// Update applies non-nil fields from UpdateListingInput to the listing row.
// Returns ErrNotFound if the listing does not exist.
func (r *Repository) Update(ctx context.Context, id string, in UpdateListingInput) (*Listing, error) {
	const q = `
		UPDATE listings
		SET title         = COALESCE($2, title),
		    description   = COALESCE($3, description),
		    price_per_hour = COALESCE($4, price_per_hour),
		    price_per_day  = COALESCE($5, price_per_day),
		    min_duration   = COALESCE($6, min_duration),
		    max_duration   = COALESCE($7, max_duration),
		    availability   = COALESCE($8, availability)
		WHERE id = $1
		RETURNING id, host_id, title, description,
		          ai_generated_tags, estimated_value, host_declared_value, value_justification,
		          price_per_hour, price_per_day,
		          min_duration, max_duration,
		          ST_Y(location::geometry) AS loc_lat, ST_X(location::geometry) AS loc_lng,
		          availability, has_video, status, appraisal_status, created_at`

	var minDurInterval, maxDurInterval interface{}
	if in.MinDuration != nil {
		iv := durationToInterval(in.MinDuration)
		minDurInterval = iv
	}
	if in.MaxDuration != nil {
		iv := durationToInterval(in.MaxDuration)
		maxDurInterval = iv
	}

	row := r.pool.QueryRow(ctx, q,
		id, in.Title, in.Description, in.PricePerHour, in.PricePerDay,
		minDurInterval, maxDurInterval,
		availabilityBytes(in.Availability),
	)
	l, err := scanListing(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update listing: %w", err)
	}
	return l, nil
}

// AttachMedia updates the listing_id of the given media records.
// It performs a bulk UPDATE in a single query.
func (r *Repository) AttachMedia(ctx context.Context, listingID string, mediaIDs []string) error {
	// Build a parameterised IN clause.
	args := make([]interface{}, 0, len(mediaIDs)+1)
	args = append(args, listingID)
	placeholders := make([]byte, 0, len(mediaIDs)*5)
	for i, id := range mediaIDs {
		if i > 0 {
			placeholders = append(placeholders, ',')
		}
		args = append(args, id)
		placeholders = append(placeholders, []byte(fmt.Sprintf("$%d", i+2))...)
	}

	q := fmt.Sprintf(
		"UPDATE media SET listing_id = $1 WHERE id IN (%s)",
		string(placeholders),
	)
	if _, err := r.pool.Exec(ctx, q, args...); err != nil {
		return fmt.Errorf("attach media to listing: %w", err)
	}
	return nil
}

// scanListing reads a listing row from a pgx Row or Rows scanner.
// It handles the PostGIS lat/lng columns and INTERVAL duration columns.
func scanListing(row pgx.Row) (*Listing, error) {
	var (
		l               Listing
		tags            []byte
		avail           []byte
		status          string
		appraisalStatus string
		minDur          pgtype.Interval
		maxDur          pgtype.Interval
		locLat          *float64
		locLng          *float64
		estVal          *float64
		hostVal         *float64
		valJust         *string
		priceHour       *float64
		priceDay        *float64
	)
	err := row.Scan(
		&l.ID, &l.HostID, &l.Title, &l.Description,
		&tags, &estVal, &hostVal, &valJust,
		&priceHour, &priceDay,
		&minDur, &maxDur,
		&locLat, &locLng,
		&avail, &l.HasVideo, &status, &appraisalStatus, &l.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	l.Status = ListingStatus(status)
	l.AppraisalStatus = AppraisalStatus(appraisalStatus)
	l.AIGeneratedTags = jsonOrDefault(tags, "[]")
	l.Availability = jsonOrDefault(avail, "[]")
	l.EstimatedValue = estVal
	l.HostDeclaredValue = hostVal
	l.ValueJustification = valJust
	l.PricePerHour = priceHour
	l.PricePerDay = priceDay
	l.MinDuration = intervalToDuration(minDur)
	l.MaxDuration = intervalToDuration(maxDur)

	if locLat != nil && locLng != nil {
		l.Location = &Location{Lat: *locLat, Lng: *locLng}
	}
	return &l, nil
}

// UpdateAppraisalFields merges AI-generated fields into a listing row.
// Existing non-empty title/description and non-null numeric fields are preserved via COALESCE/CASE.
func (r *Repository) UpdateAppraisalFields(ctx context.Context, id string, in AppraisalFieldsUpdate) error {
	// Convert cent integers to NUMERIC(10,2) dollar values.
	var estValDollars *float64
	if in.EstimatedValueCents != nil {
		v := float64(*in.EstimatedValueCents) / 100.0
		estValDollars = &v
	}
	var pricePerHourDollars *float64
	if in.SuggestedPricePerHourCents != nil {
		v := float64(*in.SuggestedPricePerHourCents) / 100.0
		pricePerHourDollars = &v
	}
	var pricePerDayDollars *float64
	if in.SuggestedPricePerDayCents != nil {
		v := float64(*in.SuggestedPricePerDayCents) / 100.0
		pricePerDayDollars = &v
	}

	const q = `
		UPDATE listings SET
			ai_generated_tags    = COALESCE($2, ai_generated_tags),
			estimated_value      = COALESCE(estimated_value, $3),
			title                = CASE WHEN title = '' THEN COALESCE($4, title) ELSE title END,
			description          = CASE WHEN description = '' THEN COALESCE($5, description) ELSE description END,
			price_per_hour       = COALESCE(price_per_hour, $6),
			price_per_day        = COALESCE(price_per_day, $7),
			appraisal_status     = $8
		WHERE id = $1`

	_, err := r.pool.Exec(ctx, q,
		id,
		in.AIGeneratedTags,
		estValDollars,
		in.SuggestedTitle,
		in.SuggestedDescription,
		pricePerHourDollars,
		pricePerDayDollars,
		string(in.AppraisalStatus),
	)
	if err != nil {
		return fmt.Errorf("update appraisal fields: %w", err)
	}
	return nil
}

// durationToInterval converts a *Duration to a pgtype.Interval for pgx.
// Returns a null interval when d is nil.
func durationToInterval(d *Duration) pgtype.Interval {
	if d == nil {
		return pgtype.Interval{Valid: false}
	}
	return pgtype.Interval{
		Microseconds: time.Duration(*d).Microseconds(),
		Valid:        true,
	}
}

// intervalToDuration converts a pgtype.Interval back to a *Duration.
// Returns nil for null intervals.
func intervalToDuration(iv pgtype.Interval) *Duration {
	if !iv.Valid {
		return nil
	}
	total := time.Duration(iv.Microseconds)*time.Microsecond +
		time.Duration(iv.Days)*24*time.Hour +
		time.Duration(iv.Months)*30*24*time.Hour
	d := Duration(total)
	return &d
}

// availabilityBytes returns raw JSONB bytes, defaulting to [] when nil.
func availabilityBytes(raw []byte) []byte {
	if raw == nil {
		return []byte("[]")
	}
	return raw
}

// jsonOrDefault returns raw when non-nil, otherwise the fallback JSON literal.
func jsonOrDefault(raw []byte, fallback string) []byte {
	if raw == nil {
		return []byte(fallback)
	}
	return raw
}
