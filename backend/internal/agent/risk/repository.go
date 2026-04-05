package risk

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles database operations for the risk domain.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// InsertSignal persists a new reputation signal.
// Returns nil for one-time signals that have already been emitted (idempotent).
func (r *Repository) InsertSignal(ctx context.Context, s *ReputationSignal) error {
	const q = `
		INSERT INTO reputation_signals
		    (id, user_id, signal_type, points, idempotency_key, transaction_id, emitted_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id, idempotency_key)
		    WHERE idempotency_key IS NOT NULL
		DO NOTHING`

	_, err := r.pool.Exec(ctx, q,
		s.ID, s.UserID, string(s.SignalType), s.Points,
		s.IdempotencyKey, s.TransactionID, s.EmittedAt,
	)
	if err != nil {
		return fmt.Errorf("risk: insert signal: %w", err)
	}
	return nil
}

// FindSignalsByUserID returns all reputation signals for a user,
// ordered oldest-first so callers can apply decay in chronological order.
func (r *Repository) FindSignalsByUserID(ctx context.Context, userID string) ([]ReputationSignal, error) {
	const q = `
		SELECT id, user_id, signal_type, points, idempotency_key, transaction_id, emitted_at
		FROM reputation_signals
		WHERE user_id = $1
		ORDER BY emitted_at ASC`

	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("risk: query signals: %w", err)
	}
	defer rows.Close()

	var signals []ReputationSignal
	for rows.Next() {
		var s ReputationSignal
		if err := rows.Scan(
			&s.ID, &s.UserID, &s.SignalType, &s.Points,
			&s.IdempotencyKey, &s.TransactionID, &s.EmittedAt,
		); err != nil {
			return nil, fmt.Errorf("risk: scan signal: %w", err)
		}
		signals = append(signals, s)
	}
	return signals, rows.Err()
}

// userProfile is the subset of user data needed for risk computation.
type userProfile struct {
	ID             string
	IdentityStatus string
	ReputationScore int
	CreatedAt      time.Time
	DeviceID       *string
}

// FindUserProfile fetches the identity status, reputation score and account age for a user.
func (r *Repository) FindUserProfile(ctx context.Context, userID string) (userProfile, error) {
	const q = `
		SELECT id, identity_status, reputation_score, created_at
		FROM users WHERE id = $1`

	var p userProfile
	err := r.pool.QueryRow(ctx, q, userID).Scan(
		&p.ID, &p.IdentityStatus, &p.ReputationScore, &p.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return userProfile{}, ErrUserNotFound
	}
	if err != nil {
		return userProfile{}, fmt.Errorf("risk: find user profile: %w", err)
	}
	return p, nil
}

// SetReputationScore writes the fully-computed reputation score to the users table.
// The value is clamped to [0, 1000] in SQL.
func (r *Repository) SetReputationScore(ctx context.Context, userID string, score int) error {
	const q = `
		UPDATE users
		SET reputation_score = GREATEST(0, LEAST(1000, $2))
		WHERE id = $1`

	_, err := r.pool.Exec(ctx, q, userID, score)
	if err != nil {
		return fmt.Errorf("risk: set reputation score: %w", err)
	}
	return nil
}

// transactionDetails holds the minimal transaction data needed for risk computation.
type transactionDetails struct {
	ID             string
	RenterID       string
	HostID         string
	ItemValueCents int64
	ScheduledStart time.Time
}

// FindTransactionDetails fetches item value and scheduling data for risk computation.
// Joins transactions with listings to get the item value.
func (r *Repository) FindTransactionDetails(ctx context.Context, transactionID string) (transactionDetails, error) {
	const q = `
		SELECT t.id, t.renter_id, t.host_id,
		       COALESCE(l.price_per_hour_cents, 0) AS item_value_cents,
		       t.scheduled_start
		FROM transactions t
		JOIN listings l ON l.id = t.listing_id
		WHERE t.id = $1`

	var d transactionDetails
	err := r.pool.QueryRow(ctx, q, transactionID).Scan(
		&d.ID, &d.RenterID, &d.HostID, &d.ItemValueCents, &d.ScheduledStart,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return transactionDetails{}, ErrTransactionNotFound
	}
	if err != nil {
		return transactionDetails{}, fmt.Errorf("risk: find transaction details: %w", err)
	}
	return d, nil
}

// behavioralHistory holds aggregated counts for a user's recent activity.
type behavioralHistory struct {
	Cancellations60d int
	Disputes60d      int
}

// FindBehavioralHistory returns the count of cancellations and disputes for a
// user in the past 60 days.
func (r *Repository) FindBehavioralHistory(ctx context.Context, userID string) (behavioralHistory, error) {
	const q = `
		SELECT
		    COUNT(*) FILTER (
		        WHERE (renter_id = $1 OR host_id = $1)
		          AND status = 'CANCELLED'
		          AND created_at >= NOW() - INTERVAL '60 days'
		    ) AS cancellations_60d,
		    COUNT(*) FILTER (
		        WHERE (renter_id = $1 OR host_id = $1)
		          AND status = 'DISPUTED'
		          AND created_at >= NOW() - INTERVAL '60 days'
		    ) AS disputes_60d
		FROM transactions`

	var h behavioralHistory
	if err := r.pool.QueryRow(ctx, q, userID).Scan(
		&h.Cancellations60d, &h.Disputes60d,
	); err != nil {
		return behavioralHistory{}, fmt.Errorf("risk: find behavioral history: %w", err)
	}
	return h, nil
}

// UpsertRiskScore inserts or updates the risk score for a transaction.
func (r *Repository) UpsertRiskScore(ctx context.Context, s *TransactionRiskScore) error {
	const q = `
		INSERT INTO risk_scores (transaction_id, risk_score, risk_level, breakdown, computed_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (transaction_id) DO UPDATE
		    SET risk_score  = EXCLUDED.risk_score,
		        risk_level  = EXCLUDED.risk_level,
		        breakdown   = EXCLUDED.breakdown,
		        computed_at = EXCLUDED.computed_at`

	breakdownJSON := buildBreakdownJSON(s)
	_, err := r.pool.Exec(ctx, q,
		s.TransactionID, s.RiskScore, string(s.RiskLevel), breakdownJSON, s.ComputedAt,
	)
	if err != nil {
		return fmt.Errorf("risk: upsert risk score: %w", err)
	}
	return nil
}

// FindRiskScore retrieves the stored risk score for a transaction.
func (r *Repository) FindRiskScore(ctx context.Context, transactionID string) (*TransactionRiskScore, error) {
	const q = `
		SELECT transaction_id, risk_score, risk_level, breakdown, computed_at
		FROM risk_scores
		WHERE transaction_id = $1`

	var (
		s            TransactionRiskScore
		breakdownRaw []byte
	)
	err := r.pool.QueryRow(ctx, q, transactionID).Scan(
		&s.TransactionID, &s.RiskScore, &s.RiskLevel, &breakdownRaw, &s.ComputedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTransactionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("risk: find risk score: %w", err)
	}
	s.Control = controlForScore(s.RiskScore)
	return &s, nil
}

// FindAllHostIDs returns the IDs of all users who have at least one listing.
// Used by the monthly reputation recalculation job.
func (r *Repository) FindAllHostIDs(ctx context.Context) ([]string, error) {
	const q = `SELECT DISTINCT host_id FROM listings`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("risk: find host ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("risk: scan host id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// HostMetrics contains host-specific metrics needed for monthly reputation recalculation.
type HostMetrics struct {
	// Total bookings in last 90 days (for response/acceptance rates)
	TotalBookings90d    int
	AcceptedBookings90d int
	// Cancellations in last 90 days (host-initiated)
	HostCancellations90d int
}

// FindHostMetrics returns response and acceptance rate metrics for a host.
func (r *Repository) FindHostMetrics(ctx context.Context, hostID string) (HostMetrics, error) {
	const q = `
		SELECT
		    COUNT(*) FILTER (WHERE status NOT IN ('REQUESTED')) AS total_bookings_90d,
		    COUNT(*) FILTER (WHERE status IN ('ACCEPTED', 'ACTIVE', 'COMPLETED')) AS accepted_90d,
		    COUNT(*) FILTER (
		        WHERE status = 'CANCELLED'
		          AND cancelled_by = $1
		    ) AS host_cancellations_90d
		FROM transactions
		WHERE host_id = $1
		  AND created_at >= NOW() - INTERVAL '90 days'`

	var m HostMetrics
	if err := r.pool.QueryRow(ctx, q, hostID).Scan(
		&m.TotalBookings90d, &m.AcceptedBookings90d, &m.HostCancellations90d,
	); err != nil {
		return HostMetrics{}, fmt.Errorf("risk: find host metrics: %w", err)
	}
	return m, nil
}

// FindUsersWithNegativeSignalsOlderThan returns user IDs that have at least one
// negative reputation signal emitted before the given cutoff that hasn't been
// decayed yet (i.e., emitted_at is exactly at the decay boundary).
func (r *Repository) FindUsersWithNegativeSignalsOlderThan(ctx context.Context, cutoff time.Time) ([]string, error) {
	const q = `
		SELECT DISTINCT user_id
		FROM reputation_signals
		WHERE points < 0
		  AND emitted_at <= $1`

	rows, err := r.pool.Query(ctx, q, cutoff)
	if err != nil {
		return nil, fmt.Errorf("risk: find users with decaying signals: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("risk: scan user id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// buildBreakdownJSON marshals the RiskBreakdown into a JSON byte slice.
// Falls back to an empty object on marshal failure (never happens in practice).
func buildBreakdownJSON(s *TransactionRiskScore) []byte {
	type breakdown struct {
		BaseRisk         int `json:"base_risk"`
		TransactionRisk  int `json:"transaction_risk"`
		CounterpartyRisk int `json:"counterparty_risk"`
		BehavioralRisk   int `json:"behavioral_risk"`
		FraudSignals     int `json:"fraud_signals"`
	}
	b, _ := jsonMarshal(breakdown{
		BaseRisk:         s.Breakdown.BaseRisk,
		TransactionRisk:  s.Breakdown.TransactionRisk,
		CounterpartyRisk: s.Breakdown.CounterpartyRisk,
		BehavioralRisk:   s.Breakdown.BehavioralRisk,
		FraudSignals:     s.Breakdown.FraudSignals,
	})
	return b
}
