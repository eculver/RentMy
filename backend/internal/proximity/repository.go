package proximity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles proximity_proofs database operations.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given connection pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Insert creates a new proximity proof record.
func (r *Repository) Insert(ctx context.Context, p ProximityProof) error {
	const q = `
		INSERT INTO proximity_proofs
		    (id, transaction_id, user_id, proof_type, gps_distance,
		     pin, pin_expires_at, verified, method, device_id, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`
	_, err := r.pool.Exec(ctx, q,
		p.ID, p.TransactionID, p.UserID, string(p.ProofType),
		p.GPSDistance, p.PIN, p.PINExpiresAt,
		p.Verified, string(p.Method), p.DeviceID, p.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert proximity proof: %w", err)
	}
	return nil
}

// FindByTransactionID returns all proof records for a transaction, ordered by
// created_at ascending.
func (r *Repository) FindByTransactionID(ctx context.Context, transactionID string) ([]ProximityProof, error) {
	const q = `
		SELECT id, transaction_id, COALESCE(user_id,''), proof_type,
		       gps_distance, COALESCE(pin,''), pin_expires_at,
		       verified, method, COALESCE(device_id,''), created_at
		FROM proximity_proofs
		WHERE transaction_id = $1
		ORDER BY created_at ASC`

	rows, err := r.pool.Query(ctx, q, transactionID)
	if err != nil {
		return nil, fmt.Errorf("query proximity proofs: %w", err)
	}
	defer rows.Close()

	return scanProofs(rows)
}

// FindByTransactionAndType returns the proof record for a specific
// (transaction, proof_type, user) triple.
func (r *Repository) FindByTransactionAndType(ctx context.Context, transactionID string, proofType ProofType, userID string) (*ProximityProof, error) {
	const q = `
		SELECT id, transaction_id, COALESCE(user_id,''), proof_type,
		       gps_distance, COALESCE(pin,''), pin_expires_at,
		       verified, method, COALESCE(device_id,''), created_at
		FROM proximity_proofs
		WHERE transaction_id = $1 AND proof_type = $2 AND user_id = $3`

	var p ProximityProof
	err := r.pool.QueryRow(ctx, q, transactionID, string(proofType), userID).Scan(
		&p.ID, &p.TransactionID, &p.UserID, &p.ProofType,
		&p.GPSDistance, &p.PIN, &p.PINExpiresAt,
		&p.Verified, &p.Method, &p.DeviceID, &p.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrProofNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find proximity proof: %w", err)
	}
	return &p, nil
}

// UpdateVerified sets the gps_distance and verified flag on an existing proof record.
func (r *Repository) UpdateVerified(ctx context.Context, id string, distMeters float64, verified bool) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE proximity_proofs SET gps_distance = $1, verified = $2 WHERE id = $3`,
		distMeters, verified, id,
	)
	if err != nil {
		return fmt.Errorf("update proximity proof verified: %w", err)
	}
	return nil
}

// UpsertPIN stores or refreshes the PIN on the host's check-in proof record.
// The record is identified by (transactionID, ProofTypeCheckIn, hostID).
func (r *Repository) UpsertPIN(ctx context.Context, proofID, pin string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE proximity_proofs SET pin = $1, pin_expires_at = $2 WHERE id = $3`,
		pin, expiresAt, proofID,
	)
	if err != nil {
		return fmt.Errorf("upsert proximity PIN: %w", err)
	}
	return nil
}

// GetListingLocation returns the latitude and longitude of the listing with the
// given ID, extracted from the PostGIS geography column.
func (r *Repository) GetListingLocation(ctx context.Context, listingID string) (lat, lng float64, err error) {
	const q = `
		SELECT ST_Y(location::geometry), ST_X(location::geometry)
		FROM listings
		WHERE id = $1 AND status = 'ACTIVE'`

	err = r.pool.QueryRow(ctx, q, listingID).Scan(&lat, &lng)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, fmt.Errorf("listing not found or not active: %s", listingID)
	}
	if err != nil {
		return 0, 0, fmt.Errorf("get listing location: %w", err)
	}
	return lat, lng, nil
}

// GetTransactionParties returns the renter_id, host_id, and listing_id for a
// transaction, used by the proximity service to resolve party roles.
func (r *Repository) GetTransactionParties(ctx context.Context, transactionID string) (renterID, hostID, listingID string, err error) {
	const q = `SELECT renter_id, host_id, listing_id FROM transactions WHERE id = $1`
	err = r.pool.QueryRow(ctx, q, transactionID).Scan(&renterID, &hostID, &listingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", "", fmt.Errorf("transaction not found: %s", transactionID)
	}
	if err != nil {
		return "", "", "", fmt.Errorf("get transaction parties: %w", err)
	}
	return renterID, hostID, listingID, nil
}

// GetUserPhone returns the phone number for a user, used for SMS fallback.
func (r *Repository) GetUserPhone(ctx context.Context, userID string) (string, error) {
	var phone string
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(phone,'') FROM users WHERE id = $1`, userID,
	).Scan(&phone)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("user not found: %s", userID)
	}
	if err != nil {
		return "", fmt.Errorf("get user phone: %w", err)
	}
	return phone, nil
}

// scanProofs scans a pgx.Rows result into a []ProximityProof slice.
func scanProofs(rows pgx.Rows) ([]ProximityProof, error) {
	var proofs []ProximityProof
	for rows.Next() {
		var p ProximityProof
		if err := rows.Scan(
			&p.ID, &p.TransactionID, &p.UserID, &p.ProofType,
			&p.GPSDistance, &p.PIN, &p.PINExpiresAt,
			&p.Verified, &p.Method, &p.DeviceID, &p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan proximity proof: %w", err)
		}
		proofs = append(proofs, p)
	}
	return proofs, rows.Err()
}
