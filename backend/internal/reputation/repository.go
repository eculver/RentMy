package reputation

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository fetches source data needed for reputation computation.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// FetchUserStats queries all source tables and returns raw statistics for one user.
// Every query is a single SELECT; the method makes one round-trip per query.
func (r *Repository) FetchUserStats(ctx context.Context, userID string) (userStats, error) {
	var s userStats

	// --- User metadata ---
	if err := r.pool.QueryRow(ctx,
		`SELECT created_at, identity_status FROM users WHERE id = $1`,
		userID,
	).Scan(&s.AccountCreatedAt, &s.IdentityStatus); err != nil {
		return userStats{}, fmt.Errorf("reputation: fetch user metadata: %w", err)
	}

	// --- Completed rentals (no open or resolved dispute filed against this user) ---
	// "Completed" = status COMPLETED, no dispute where this user is the non-reporter.
	// We count them once per transaction, regardless of renter/host role.
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM transactions t
		 WHERE (t.renter_id = $1 OR t.host_id = $1)
		   AND t.status = 'COMPLETED'
		   AND NOT EXISTS (
		       SELECT 1 FROM disputes d
		       WHERE d.transaction_id = t.id
		         AND d.reporter_id != $1
		   )`,
		userID,
	).Scan(&s.CompletedRentals); err != nil {
		return userStats{}, fmt.Errorf("reputation: count completed rentals: %w", err)
	}

	// --- Clean rental count (same condition, used for milestones) ---
	s.CleanRentalCount = s.CompletedRentals

	// --- Positive bubbles received ---
	if err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(jsonb_array_length(bubbles)), 0)
		 FROM ratings WHERE to_user_id = $1`,
		userID,
	).Scan(&s.PositiveBubbles); err != nil {
		return userStats{}, fmt.Errorf("reputation: count bubbles: %w", err)
	}

	// --- On-time returns (as renter) ---
	// on-time = actual_end <= scheduled_end + 15 minutes
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM transactions
		 WHERE renter_id = $1
		   AND status = 'COMPLETED'
		   AND actual_end IS NOT NULL
		   AND scheduled_end IS NOT NULL
		   AND actual_end <= scheduled_end + INTERVAL '15 minutes'`,
		userID,
	).Scan(&s.OnTimeReturns); err != nil {
		return userStats{}, fmt.Errorf("reputation: count on-time returns: %w", err)
	}

	// --- Disputes filed against this user ---
	// A dispute is "against" the user when they are NOT the reporter.
	rows, err := r.pool.Query(ctx,
		`SELECT d.created_at FROM disputes d
		 JOIN transactions t ON t.id = d.transaction_id
		 WHERE (t.renter_id = $1 OR t.host_id = $1)
		   AND d.reporter_id != $1`,
		userID,
	)
	if err != nil {
		return userStats{}, fmt.Errorf("reputation: query disputes against: %w", err)
	}
	for rows.Next() {
		var ts time.Time
		if err := rows.Scan(&ts); err != nil {
			rows.Close()
			return userStats{}, fmt.Errorf("reputation: scan dispute ts: %w", err)
		}
		s.DisputesAgainst = append(s.DisputesAgainst, ts)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return userStats{}, fmt.Errorf("reputation: disputes against iter: %w", err)
	}

	// --- Disputes lost (as renter: charge_amount > 0 in a dispute filed against them) ---
	lostRows, err := r.pool.Query(ctx,
		`SELECT d.created_at FROM disputes d
		 JOIN transactions t ON t.id = d.transaction_id
		 WHERE t.renter_id = $1
		   AND d.reporter_id != $1
		   AND d.charge_amount > 0
		   AND d.status IN ('AUTO_RESOLVED', 'RESOLVED')`,
		userID,
	)
	if err != nil {
		return userStats{}, fmt.Errorf("reputation: query disputes lost: %w", err)
	}
	for lostRows.Next() {
		var ts time.Time
		if err := lostRows.Scan(&ts); err != nil {
			lostRows.Close()
			return userStats{}, fmt.Errorf("reputation: scan lost dispute ts: %w", err)
		}
		s.DisputesLost = append(s.DisputesLost, ts)
	}
	lostRows.Close()
	if err := lostRows.Err(); err != nil {
		return userStats{}, fmt.Errorf("reputation: disputes lost iter: %w", err)
	}

	// --- Cancellations as the cancelling party ---
	cancelRows, err := r.pool.Query(ctx,
		`SELECT created_at FROM transactions
		 WHERE status = 'CANCELLED'
		   AND (
		       (renter_id = $1 AND cancelled_by = 'RENTER') OR
		       (host_id   = $1 AND cancelled_by = 'HOST')
		   )`,
		userID,
	)
	if err != nil {
		return userStats{}, fmt.Errorf("reputation: query cancellations: %w", err)
	}
	for cancelRows.Next() {
		var ts time.Time
		if err := cancelRows.Scan(&ts); err != nil {
			cancelRows.Close()
			return userStats{}, fmt.Errorf("reputation: scan cancellation ts: %w", err)
		}
		s.Cancellations = append(s.Cancellations, ts)
	}
	cancelRows.Close()
	if err := cancelRows.Err(); err != nil {
		return userStats{}, fmt.Errorf("reputation: cancellations iter: %w", err)
	}

	// --- Late returns (as renter: actual_end > scheduled_end + 15 min) ---
	lateRows, err := r.pool.Query(ctx,
		`SELECT actual_end FROM transactions
		 WHERE renter_id = $1
		   AND status = 'COMPLETED'
		   AND actual_end IS NOT NULL
		   AND scheduled_end IS NOT NULL
		   AND actual_end > scheduled_end + INTERVAL '15 minutes'`,
		userID,
	)
	if err != nil {
		return userStats{}, fmt.Errorf("reputation: query late returns: %w", err)
	}
	for lateRows.Next() {
		var ts time.Time
		if err := lateRows.Scan(&ts); err != nil {
			lateRows.Close()
			return userStats{}, fmt.Errorf("reputation: scan late return ts: %w", err)
		}
		s.LateReturns = append(s.LateReturns, ts)
	}
	lateRows.Close()
	if err := lateRows.Err(); err != nil {
		return userStats{}, fmt.Errorf("reputation: late returns iter: %w", err)
	}

	// --- Fraud flags (count non-empty entries in risk_flags JSONB array) ---
	if err := r.pool.QueryRow(ctx,
		`SELECT jsonb_array_length(risk_flags) FROM users WHERE id = $1`,
		userID,
	).Scan(&s.FraudFlagCount); err != nil {
		// Non-fatal: defaults to 0 if risk_flags is missing.
		s.FraudFlagCount = 0
	}

	return s, nil
}

// FetchHostStats fetches the host-specific rolling-window metrics needed for
// the monthly host reputation recalculation.
func (r *Repository) FetchHostStats(ctx context.Context, hostID string) (userStats, error) {
	var s userStats
	if err := r.pool.QueryRow(ctx,
		`SELECT
		     COUNT(*) FILTER (WHERE status NOT IN ('REQUESTED')) AS total,
		     COUNT(*) FILTER (WHERE status IN ('ACCEPTED', 'ACTIVE', 'COMPLETED')) AS accepted,
		     COUNT(*) FILTER (WHERE status = 'CANCELLED' AND cancelled_by = 'HOST') AS host_cancels
		 FROM transactions
		 WHERE host_id = $1
		   AND created_at >= NOW() - INTERVAL '90 days'`,
		hostID,
	).Scan(&s.TotalBookings90d, &s.AcceptedBookings90d, &s.HostCancels90d); err != nil {
		return userStats{}, fmt.Errorf("reputation: fetch host stats: %w", err)
	}
	return s, nil
}

// SetScore writes the computed reputation score to users.reputation_score.
// The SQL clamps the value to [0, 1000] as a safety net.
func (r *Repository) SetScore(ctx context.Context, userID string, score int) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET reputation_score = GREATEST(0, LEAST(1000, $2)) WHERE id = $1`,
		userID, score,
	)
	if err != nil {
		return fmt.Errorf("reputation: set score: %w", err)
	}
	return nil
}

// FindAllHostIDs returns the IDs of all users who have listed at least one item.
func (r *Repository) FindAllHostIDs(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT DISTINCT host_id FROM listings`)
	if err != nil {
		return nil, fmt.Errorf("reputation: find host ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("reputation: scan host id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// FindUsersWithNegativeEventsOlderThan returns user IDs that have at least one
// negative event (dispute, cancellation, late return) older than cutoff.
// These users need a decay recalculation run.
func (r *Repository) FindUsersWithNegativeEventsOlderThan(ctx context.Context, cutoff time.Time) ([]string, error) {
	const q = `
		SELECT DISTINCT u.id FROM users u
		WHERE EXISTS (
		    -- disputes filed against user
		    SELECT 1 FROM disputes d
		    JOIN transactions t ON t.id = d.transaction_id
		    WHERE (t.renter_id = u.id OR t.host_id = u.id)
		      AND d.reporter_id != u.id
		      AND d.created_at <= $1
		) OR EXISTS (
		    -- cancellations as cancelling party
		    SELECT 1 FROM transactions t2
		    WHERE t2.status = 'CANCELLED'
		      AND (
		          (t2.renter_id = u.id AND t2.cancelled_by = 'RENTER') OR
		          (t2.host_id   = u.id AND t2.cancelled_by = 'HOST')
		      )
		      AND t2.created_at <= $1
		) OR EXISTS (
		    -- late returns as renter
		    SELECT 1 FROM transactions t3
		    WHERE t3.renter_id = u.id
		      AND t3.status = 'COMPLETED'
		      AND t3.actual_end IS NOT NULL
		      AND t3.actual_end > t3.scheduled_end + INTERVAL '15 minutes'
		      AND t3.actual_end <= $1
		)`

	rows, err := r.pool.Query(ctx, q, cutoff)
	if err != nil {
		return nil, fmt.Errorf("reputation: find decay candidates: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("reputation: scan decay candidate: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
