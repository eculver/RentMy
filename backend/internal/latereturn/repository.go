package latereturn

import (
	"context"
	"errors"
	"fmt"

	"github.com/Brett2thered/RentMy/backend/internal/platform/ulid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all late-return database operations.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given connection pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Insert creates a new late return record.
func (r *Repository) Insert(ctx context.Context, lr LateReturn) (LateReturn, error) {
	if lr.ID == "" {
		lr.ID = ulid.New()
	}
	err := r.pool.QueryRow(ctx,
		`INSERT INTO late_returns (id, transaction_id, renter_id, host_id, scheduled_end, status)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, transaction_id, renter_id, host_id, scheduled_end, actual_end,
		           late_minutes, status, escalation_level, total_fee_charged, confidence,
		           agent_decision_id, created_at, updated_at`,
		lr.ID, lr.TransactionID, lr.RenterID, lr.HostID, lr.ScheduledEnd, StatusMonitoring,
	).Scan(&lr.ID, &lr.TransactionID, &lr.RenterID, &lr.HostID, &lr.ScheduledEnd, &lr.ActualEnd,
		&lr.LateMinutes, &lr.Status, &lr.EscalationLevel, &lr.TotalFeeCharged, &lr.Confidence,
		&lr.AgentDecisionID, &lr.CreatedAt, &lr.UpdatedAt)
	if err != nil {
		return LateReturn{}, fmt.Errorf("insert late return: %w", err)
	}
	return lr, nil
}

// FindByID returns a late return by its ID.
func (r *Repository) FindByID(ctx context.Context, id string) (LateReturn, error) {
	return r.scanOne(ctx, `SELECT `+lateReturnCols+` FROM late_returns WHERE id = $1`, id)
}

// FindByTransactionID returns the late return for a transaction, if any.
func (r *Repository) FindByTransactionID(ctx context.Context, transactionID string) (*LateReturn, error) {
	lr, err := r.scanOne(ctx,
		`SELECT `+lateReturnCols+` FROM late_returns WHERE transaction_id = $1 ORDER BY created_at DESC LIMIT 1`,
		transactionID,
	)
	if errors.Is(err, ErrLateReturnNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &lr, nil
}

// FindActive returns all late returns in MONITORING or CHARGING status.
func (r *Repository) FindActive(ctx context.Context, limit, offset int) ([]LateReturn, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+lateReturnCols+` FROM late_returns
		 WHERE status IN ($1, $2)
		 ORDER BY created_at ASC
		 LIMIT $3 OFFSET $4`,
		StatusMonitoring, StatusCharging, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("query active late returns: %w", err)
	}
	defer rows.Close()
	return r.scanMany(rows)
}

// UpdateStatus updates a late return's status.
func (r *Repository) UpdateStatus(ctx context.Context, id string, status Status) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE late_returns SET status = $1, updated_at = NOW() WHERE id = $2`,
		status, id,
	)
	if err != nil {
		return fmt.Errorf("update late return status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrLateReturnNotFound
	}
	return nil
}

// RecordCharge updates the total fee charged and late minutes.
func (r *Repository) RecordCharge(ctx context.Context, id string, totalFee int64, lateMinutes int) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE late_returns
		 SET total_fee_charged = $1, late_minutes = $2, status = $3, updated_at = NOW()
		 WHERE id = $4`,
		totalFee, lateMinutes, StatusCharging, id,
	)
	if err != nil {
		return fmt.Errorf("record late charge: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrLateReturnNotFound
	}
	return nil
}

// RecordEscalation updates the escalation level and agent decision reference.
func (r *Repository) RecordEscalation(ctx context.Context, id string, level EscalationLevel, confidence float64, agentDecisionID string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE late_returns
		 SET escalation_level = $1, confidence = $2, agent_decision_id = $3, status = $4, updated_at = NOW()
		 WHERE id = $5`,
		level, confidence, agentDecisionID, statusFromEscalation(level), id,
	)
	if err != nil {
		return fmt.Errorf("record escalation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrLateReturnNotFound
	}
	return nil
}

// Resolve marks a late return as resolved with the actual end time.
func (r *Repository) Resolve(ctx context.Context, id string, actualEnd interface{}) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE late_returns SET status = $1, actual_end = $2, updated_at = NOW() WHERE id = $3`,
		StatusResolved, actualEnd, id,
	)
	if err != nil {
		return fmt.Errorf("resolve late return: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrLateReturnNotFound
	}
	return nil
}

// GetTransactionDetails returns fields needed for late fee calculation.
func (r *Repository) GetTransactionDetails(ctx context.Context, transactionID string) (renterID, hostID string, scheduledEnd interface{}, rentalFee, holdAmount, itemValue int64, holdAllocationJSON []byte, status string, err error) {
	err = r.pool.QueryRow(ctx,
		`SELECT renter_id, host_id, scheduled_end,
		        ROUND(rental_fee * 100)::bigint,
		        ROUND(hold_amount * 100)::bigint,
		        ROUND(item_value * 100)::bigint,
		        hold_allocation,
		        status
		 FROM transactions WHERE id = $1`,
		transactionID,
	).Scan(&renterID, &hostID, &scheduledEnd, &rentalFee, &holdAmount, &itemValue, &holdAllocationJSON, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil, 0, 0, 0, nil, "", ErrTransactionNotFound
	}
	return
}

// GetRenterReputationScore returns the renter's reputation score.
func (r *Repository) GetRenterReputationScore(ctx context.Context, renterID string) (int, error) {
	var score int
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(reputation_score, 0) FROM users WHERE id = $1`, renterID,
	).Scan(&score)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("get renter reputation: %w", err)
	}
	return score, nil
}

// CountRecentMessages counts messages from renter in the last N hours for a transaction.
func (r *Repository) CountRecentMessages(ctx context.Context, transactionID, renterID string, withinHours int) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM messages
		 WHERE transaction_id = $1 AND sender_id = $2
		   AND created_at >= NOW() - ($3 || ' hours')::interval`,
		transactionID, renterID, fmt.Sprintf("%d", withinHours),
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count recent messages: %w", err)
	}
	return count, nil
}

// HasConflictingBooking checks if the listing has another ACCEPTED or ACTIVE booking
// overlapping with now.
func (r *Repository) HasConflictingBooking(ctx context.Context, transactionID string) (bool, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM transactions t2
		 WHERE t2.listing_id = (SELECT listing_id FROM transactions WHERE id = $1)
		   AND t2.id != $1
		   AND t2.status IN ('ACCEPTED', 'ACTIVE')
		   AND t2.scheduled_start <= NOW()
		   AND t2.scheduled_end >= NOW()`,
		transactionID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check conflicting booking: %w", err)
	}
	return count > 0, nil
}

const lateReturnCols = `id, transaction_id, renter_id, host_id, scheduled_end, actual_end,
	late_minutes, status, escalation_level, total_fee_charged, confidence,
	agent_decision_id, created_at, updated_at`

func (r *Repository) scanOne(ctx context.Context, query string, args ...interface{}) (LateReturn, error) {
	var lr LateReturn
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&lr.ID, &lr.TransactionID, &lr.RenterID, &lr.HostID, &lr.ScheduledEnd, &lr.ActualEnd,
		&lr.LateMinutes, &lr.Status, &lr.EscalationLevel, &lr.TotalFeeCharged, &lr.Confidence,
		&lr.AgentDecisionID, &lr.CreatedAt, &lr.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return LateReturn{}, ErrLateReturnNotFound
	}
	if err != nil {
		return LateReturn{}, fmt.Errorf("scan late return: %w", err)
	}
	return lr, nil
}

func (r *Repository) scanMany(rows pgx.Rows) ([]LateReturn, error) {
	var results []LateReturn
	for rows.Next() {
		var lr LateReturn
		if err := rows.Scan(
			&lr.ID, &lr.TransactionID, &lr.RenterID, &lr.HostID, &lr.ScheduledEnd, &lr.ActualEnd,
			&lr.LateMinutes, &lr.Status, &lr.EscalationLevel, &lr.TotalFeeCharged, &lr.Confidence,
			&lr.AgentDecisionID, &lr.CreatedAt, &lr.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan late return row: %w", err)
		}
		results = append(results, lr)
	}
	return results, rows.Err()
}

// statusFromEscalation maps an escalation level to the corresponding late return status.
func statusFromEscalation(level EscalationLevel) Status {
	switch level {
	case EscalationEscalateToDispute:
		return StatusEscalatedToDispute
	case EscalationFlagForReview:
		return StatusFlaggedForReview
	default:
		return StatusCharging
	}
}
