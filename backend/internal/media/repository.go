package media

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a media record does not exist.
var ErrNotFound = errors.New("media not found")

// Repository handles Postgres persistence for the media domain.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a Repository backed by the given connection pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Insert persists a new Media record and returns it with CreatedAt populated.
func (r *Repository) Insert(ctx context.Context, m *Media) (*Media, error) {
	const q = `
		INSERT INTO media (
			id, listing_id, transaction_id, media_type,
			original_url, thumbnail_url,
			orientation_roll, orientation_pitch, orientation_yaw,
			gps_lat, gps_lng, device_id, captured_at, created_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6,
			$7, $8, $9,
			$10, $11, $12, $13, NOW()
		)
		RETURNING created_at`

	err := r.pool.QueryRow(ctx, q,
		m.ID, m.ListingID, m.TransactionID, string(m.MediaType),
		m.OriginalURL, m.ThumbnailURL,
		m.OrientationRoll, m.OrientationPitch, m.OrientationYaw,
		m.GpsLat, m.GpsLng, m.DeviceID, m.CapturedAt,
	).Scan(&m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert media: %w", err)
	}
	return m, nil
}

// FindByID fetches a single media record by its ULID.
func (r *Repository) FindByID(ctx context.Context, id string) (*Media, error) {
	const q = `
		SELECT id, listing_id, transaction_id, media_type,
		       original_url, thumbnail_url,
		       orientation_roll, orientation_pitch, orientation_yaw,
		       gps_lat, gps_lng, device_id, captured_at, created_at
		FROM media
		WHERE id = $1`

	m := &Media{}
	var mediaType string
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&m.ID, &m.ListingID, &m.TransactionID, &mediaType,
		&m.OriginalURL, &m.ThumbnailURL,
		&m.OrientationRoll, &m.OrientationPitch, &m.OrientationYaw,
		&m.GpsLat, &m.GpsLng, &m.DeviceID, &m.CapturedAt, &m.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find media by id: %w", err)
	}
	m.MediaType = MediaType(mediaType)
	return m, nil
}

// FindByListingID returns all media records attached to a listing.
func (r *Repository) FindByListingID(ctx context.Context, listingID string) ([]*Media, error) {
	const q = `
		SELECT id, listing_id, transaction_id, media_type,
		       original_url, thumbnail_url,
		       orientation_roll, orientation_pitch, orientation_yaw,
		       gps_lat, gps_lng, device_id, captured_at, created_at
		FROM media
		WHERE listing_id = $1
		ORDER BY created_at ASC`

	rows, err := r.pool.Query(ctx, q, listingID)
	if err != nil {
		return nil, fmt.Errorf("find media by listing: %w", err)
	}
	defer rows.Close()

	var results []*Media
	for rows.Next() {
		m := &Media{}
		var mediaType string
		if err := rows.Scan(
			&m.ID, &m.ListingID, &m.TransactionID, &mediaType,
			&m.OriginalURL, &m.ThumbnailURL,
			&m.OrientationRoll, &m.OrientationPitch, &m.OrientationYaw,
			&m.GpsLat, &m.GpsLng, &m.DeviceID, &m.CapturedAt, &m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan media row: %w", err)
		}
		m.MediaType = MediaType(mediaType)
		results = append(results, m)
	}
	return results, rows.Err()
}

// FindByTransactionID returns all media records attached to a transaction.
func (r *Repository) FindByTransactionID(ctx context.Context, transactionID string) ([]*Media, error) {
	const q = `
		SELECT id, listing_id, transaction_id, media_type,
		       original_url, thumbnail_url,
		       orientation_roll, orientation_pitch, orientation_yaw,
		       gps_lat, gps_lng, device_id, captured_at, created_at
		FROM media
		WHERE transaction_id = $1
		ORDER BY created_at ASC`

	rows, err := r.pool.Query(ctx, q, transactionID)
	if err != nil {
		return nil, fmt.Errorf("find media by transaction: %w", err)
	}
	defer rows.Close()

	var results []*Media
	for rows.Next() {
		m := &Media{}
		var mediaType string
		if err := rows.Scan(
			&m.ID, &m.ListingID, &m.TransactionID, &mediaType,
			&m.OriginalURL, &m.ThumbnailURL,
			&m.OrientationRoll, &m.OrientationPitch, &m.OrientationYaw,
			&m.GpsLat, &m.GpsLng, &m.DeviceID, &m.CapturedAt, &m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan media row: %w", err)
		}
		m.MediaType = MediaType(mediaType)
		results = append(results, m)
	}
	return results, rows.Err()
}
