package booking

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles booking-domain database operations against the transactions table.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given connection pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// FindByID fetches a booking by its transaction ID.
func (r *Repository) FindByID(ctx context.Context, id string) (Booking, error) {
	const q = `
		SELECT id, renter_id, host_id, listing_id,
		       scheduled_start, scheduled_end, status,
		       COALESCE(cancelled_by, ''),
		       ROUND(COALESCE(cancellation_fee, 0) * 100)::bigint,
		       actual_start, actual_end, created_at
		FROM transactions
		WHERE id = $1`

	var b Booking
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&b.ID, &b.RenterID, &b.HostID, &b.ListingID,
		&b.ScheduledStart, &b.ScheduledEnd, &b.Status,
		&b.CancelledBy, &b.CancellationFee,
		&b.ActualStart, &b.ActualEnd, &b.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, ErrBookingNotFound
	}
	if err != nil {
		return Booking{}, fmt.Errorf("find booking by id: %w", err)
	}
	return b, nil
}

// FindByRenterID returns paginated bookings for a renter.
func (r *Repository) FindByRenterID(ctx context.Context, renterID string, limit, offset int) ([]Booking, int, error) {
	if limit <= 0 {
		limit = 20
	}

	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM transactions WHERE renter_id = $1`, renterID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count renter bookings: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, renter_id, host_id, listing_id,
		       scheduled_start, scheduled_end, status,
		       COALESCE(cancelled_by, ''),
		       ROUND(COALESCE(cancellation_fee, 0) * 100)::bigint,
		       actual_start, actual_end, created_at
		FROM transactions
		WHERE renter_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, renterID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query renter bookings: %w", err)
	}
	defer rows.Close()

	bookings, err := scanBookings(rows)
	if err != nil {
		return nil, 0, err
	}
	return bookings, total, nil
}

// FindByHostID returns paginated bookings where the given user is the host.
func (r *Repository) FindByHostID(ctx context.Context, hostID string, limit, offset int) ([]Booking, int, error) {
	if limit <= 0 {
		limit = 20
	}

	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM transactions WHERE host_id = $1`, hostID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count host bookings: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, renter_id, host_id, listing_id,
		       scheduled_start, scheduled_end, status,
		       COALESCE(cancelled_by, ''),
		       ROUND(COALESCE(cancellation_fee, 0) * 100)::bigint,
		       actual_start, actual_end, created_at
		FROM transactions
		WHERE host_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, hostID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query host bookings: %w", err)
	}
	defer rows.Close()

	bookings, err := scanBookings(rows)
	if err != nil {
		return nil, 0, err
	}
	return bookings, total, nil
}

// UpdateStatus sets the status of a booking, using SELECT FOR UPDATE to prevent
// concurrent transitions.
func (r *Repository) UpdateStatus(ctx context.Context, tx pgx.Tx, id string, status Status) error {
	// Lock the row to prevent concurrent state transitions.
	var currentStatus Status
	err := tx.QueryRow(ctx,
		`SELECT status FROM transactions WHERE id = $1 FOR UPDATE`, id,
	).Scan(&currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrBookingNotFound
	}
	if err != nil {
		return fmt.Errorf("lock booking row: %w", err)
	}

	if err := ValidateTransition(currentStatus, status); err != nil {
		return err
	}

	_, err = tx.Exec(ctx,
		`UPDATE transactions SET status = $1 WHERE id = $2`, string(status), id,
	)
	if err != nil {
		return fmt.Errorf("update booking status: %w", err)
	}
	return nil
}

// UpdateCancellation sets the status, cancelled_by, and cancellation_fee atomically.
func (r *Repository) UpdateCancellation(ctx context.Context, tx pgx.Tx, id string, status Status, cancelledBy CancellerRole, feeCents int64) error {
	// Lock the row.
	var currentStatus Status
	err := tx.QueryRow(ctx,
		`SELECT status FROM transactions WHERE id = $1 FOR UPDATE`, id,
	).Scan(&currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrBookingNotFound
	}
	if err != nil {
		return fmt.Errorf("lock booking row: %w", err)
	}

	if err := ValidateTransition(currentStatus, status); err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		UPDATE transactions
		SET status = $1, cancelled_by = $2, cancellation_fee = $3
		WHERE id = $4`,
		string(status), string(cancelledBy), float64(feeCents)/100, id,
	)
	if err != nil {
		return fmt.Errorf("update cancellation: %w", err)
	}
	return nil
}

// BeginTx starts a new database transaction.
func (r *Repository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	return tx, nil
}

// GetUserCreatedAt returns the created_at timestamp for a user.
func (r *Repository) GetUserCreatedAt(ctx context.Context, userID string) (time.Time, error) {
	var t time.Time
	err := r.pool.QueryRow(ctx,
		`SELECT created_at FROM users WHERE id = $1`, userID,
	).Scan(&t)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, fmt.Errorf("user not found: %s", userID)
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("get user created_at: %w", err)
	}
	return t, nil
}

// CountCompletedByUser returns the number of completed transactions for a user
// (as either renter or host).
func (r *Repository) CountCompletedByUser(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM transactions
		WHERE (renter_id = $1 OR host_id = $1) AND status = 'COMPLETED'`, userID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count completed transactions: %w", err)
	}
	return count, nil
}

// SumHostDamageClaimed returns the total damage captured from holds for a host
// in transactions created after the given time, in cents.
func (r *Repository) SumHostDamageClaimed(ctx context.Context, hostID string, since time.Time) (int64, error) {
	var total float64
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM((hold_allocation->>'capturedForDamage')::numeric), 0)
		FROM transactions
		WHERE host_id = $1 AND created_at >= $2`, hostID, since,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("sum host damage claimed: %w", err)
	}
	// hold_allocation values are stored in cents (int64 marshaled directly).
	return int64(total), nil
}

// IsListingAvailable returns true if no overlapping REQUESTED or ACCEPTED booking
// exists for the given listing and time range.
func (r *Repository) IsListingAvailable(ctx context.Context, listingID string, start, end time.Time, excludeID string) (bool, error) {
	const q = `
		SELECT COUNT(*) FROM transactions
		WHERE listing_id = $1
		  AND id != $2
		  AND status IN ('REQUESTED', 'ACCEPTED', 'ACTIVE')
		  AND scheduled_start < $4
		  AND scheduled_end > $3`

	var count int
	err := r.pool.QueryRow(ctx, q, listingID, excludeID, start, end).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check listing availability: %w", err)
	}
	return count == 0, nil
}

// GetListingHostID returns the host_id for an active listing.
func (r *Repository) GetListingHostID(ctx context.Context, listingID string) (string, error) {
	var hostID string
	err := r.pool.QueryRow(ctx,
		`SELECT host_id FROM listings WHERE id = $1 AND status = 'ACTIVE'`, listingID,
	).Scan(&hostID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("listing not found or not active: %s", listingID)
	}
	if err != nil {
		return "", fmt.Errorf("get listing host_id: %w", err)
	}
	return hostID, nil
}

// scanBookings scans all rows into a []Booking slice.
func scanBookings(rows pgx.Rows) ([]Booking, error) {
	var bookings []Booking
	for rows.Next() {
		var b Booking
		if err := rows.Scan(
			&b.ID, &b.RenterID, &b.HostID, &b.ListingID,
			&b.ScheduledStart, &b.ScheduledEnd, &b.Status,
			&b.CancelledBy, &b.CancellationFee,
			&b.ActualStart, &b.ActualEnd, &b.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan booking row: %w", err)
		}
		bookings = append(bookings, b)
	}
	return bookings, rows.Err()
}
