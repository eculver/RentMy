package dispute

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Brett2thered/RentMy/backend/internal/platform/ulid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all dispute-domain database operations.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given connection pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Insert creates a new dispute record.
func (r *Repository) Insert(ctx context.Context, d Dispute) (Dispute, error) {
	if d.ID == "" {
		d.ID = ulid.New()
	}
	err := r.pool.QueryRow(ctx,
		`INSERT INTO disputes (id, transaction_id, reporter_id, reason, description, status)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, transaction_id, reporter_id, reason, description, status, created_at, updated_at`,
		d.ID, d.TransactionID, d.ReporterID, d.Reason, d.Description, StatusPending,
	).Scan(&d.ID, &d.TransactionID, &d.ReporterID, &d.Reason, &d.Description,
		&d.Status, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return Dispute{}, fmt.Errorf("insert dispute: %w", err)
	}
	return d, nil
}

// FindByID returns a dispute by its ID.
func (r *Repository) FindByID(ctx context.Context, id string) (Dispute, error) {
	return r.scanOne(ctx, `SELECT `+disputeCols+` FROM disputes WHERE id = $1`, id)
}

// FindByTransactionID returns all disputes for a transaction, newest first.
func (r *Repository) FindByTransactionID(ctx context.Context, transactionID string) ([]Dispute, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+disputeCols+` FROM disputes WHERE transaction_id = $1 ORDER BY created_at DESC`,
		transactionID,
	)
	if err != nil {
		return nil, fmt.Errorf("query disputes: %w", err)
	}
	defer rows.Close()
	return r.scanMany(rows)
}

// FindOpenByTransactionID returns an open dispute (not resolved) for a transaction, if any.
func (r *Repository) FindOpenByTransactionID(ctx context.Context, transactionID string) (*Dispute, error) {
	d, err := r.scanOne(ctx,
		`SELECT `+disputeCols+` FROM disputes
		 WHERE transaction_id = $1 AND status NOT IN ('RESOLVED', 'AUTO_RESOLVED', 'AUDIT_QUEUED')
		 LIMIT 1`,
		transactionID,
	)
	if errors.Is(err, ErrDisputeNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// FindPendingReview returns disputes awaiting human review, ordered by SLA deadline.
func (r *Repository) FindPendingReview(ctx context.Context, limit, offset int) ([]Dispute, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+disputeCols+` FROM disputes
		 WHERE status = $1
		 ORDER BY sla_deadline ASC NULLS LAST
		 LIMIT $2 OFFSET $3`,
		StatusHumanReview, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("query pending review: %w", err)
	}
	defer rows.Close()
	return r.scanMany(rows)
}

// UpdateStatus updates a dispute's status.
func (r *Repository) UpdateStatus(ctx context.Context, id string, status Status) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE disputes SET status = $1, updated_at = NOW() WHERE id = $2`,
		status, id,
	)
	if err != nil {
		return fmt.Errorf("update dispute status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrDisputeNotFound
	}
	return nil
}

// UpdateDecision records the agent's decision on a dispute.
func (r *Repository) UpdateDecision(ctx context.Context, id string, route EscalationRoute, chargeAmount int64, confidence float64, agentDecisionID string, evidence json.RawMessage) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE disputes
		 SET route = $1, charge_amount = $2, confidence = $3,
		     agent_decision_id = $4, evidence = $5, updated_at = NOW()
		 WHERE id = $6`,
		route, chargeAmount, confidence, agentDecisionID, evidence, id,
	)
	if err != nil {
		return fmt.Errorf("update dispute decision: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrDisputeNotFound
	}
	return nil
}

// UpdateReview records a human reviewer's action.
func (r *Repository) UpdateReview(ctx context.Context, id string, status Status, reviewerID string, notes string, chargeAmount *int64) error {
	var err error
	if chargeAmount != nil {
		_, err = r.pool.Exec(ctx,
			`UPDATE disputes
			 SET status = $1, reviewer_id = $2, reviewer_notes = $3, charge_amount = $4, updated_at = NOW()
			 WHERE id = $5`,
			status, reviewerID, notes, *chargeAmount, id,
		)
	} else {
		_, err = r.pool.Exec(ctx,
			`UPDATE disputes
			 SET status = $1, reviewer_id = $2, reviewer_notes = $3, updated_at = NOW()
			 WHERE id = $4`,
			status, reviewerID, notes, id,
		)
	}
	if err != nil {
		return fmt.Errorf("update dispute review: %w", err)
	}
	return nil
}

// SetSLADeadline sets the SLA deadline for a dispute.
func (r *Repository) SetSLADeadline(ctx context.Context, id string, deadline interface{}) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE disputes SET sla_deadline = $1, updated_at = NOW() WHERE id = $2`,
		deadline, id,
	)
	if err != nil {
		return fmt.Errorf("set sla deadline: %w", err)
	}
	return nil
}

// FindSLABreaching returns disputes in HUMAN_REVIEW that are past or near their SLA deadline.
func (r *Repository) FindSLABreaching(ctx context.Context, warningThreshold float64) ([]Dispute, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+disputeCols+` FROM disputes
		 WHERE status = $1
		   AND sla_deadline IS NOT NULL
		   AND NOW() >= sla_deadline - (sla_deadline - created_at) * (1 - $2::double precision)
		 ORDER BY sla_deadline ASC`,
		StatusHumanReview, warningThreshold,
	)
	if err != nil {
		return nil, fmt.Errorf("query sla breaching: %w", err)
	}
	defer rows.Close()
	return r.scanMany(rows)
}

// GatherTransactionEvidence reads transaction data needed for evidence assembly.
func (r *Repository) GatherTransactionEvidence(ctx context.Context, transactionID string) (TransactionRef, json.RawMessage, error) {
	var ref TransactionRef
	var agreementJSON []byte
	err := r.pool.QueryRow(ctx,
		`SELECT
			ROUND(rental_fee * 100)::bigint,
			ROUND(hold_amount * 100)::bigint,
			ROUND(item_value * 100)::bigint,
			scheduled_start, scheduled_end, status,
			agreement_snapshot
		 FROM transactions WHERE id = $1`,
		transactionID,
	).Scan(&ref.RentalFee, &ref.HoldAmount, &ref.ItemValue,
		&ref.ScheduledStart, &ref.ScheduledEnd, &ref.Status,
		&agreementJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return TransactionRef{}, nil, ErrTransactionNotFound
	}
	if err != nil {
		return TransactionRef{}, nil, fmt.Errorf("gather transaction evidence: %w", err)
	}
	return ref, agreementJSON, nil
}

// GatherMediaEvidence reads check-in and check-out media for a transaction.
func (r *Repository) GatherMediaEvidence(ctx context.Context, transactionID string) (checkIn []MediaRef, checkOut []MediaRef, err error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, media_type, original_url, gps_lat, gps_lng
		 FROM media
		 WHERE transaction_id = $1 AND media_type IN ('CHECK_IN', 'CHECK_OUT')
		 ORDER BY created_at ASC`,
		transactionID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("query media evidence: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var m MediaRef
		if err := rows.Scan(&m.ID, &m.MediaType, &m.URL, &m.GpsLat, &m.GpsLng); err != nil {
			return nil, nil, fmt.Errorf("scan media: %w", err)
		}
		switch m.MediaType {
		case "CHECK_IN":
			checkIn = append(checkIn, m)
		case "CHECK_OUT":
			checkOut = append(checkOut, m)
		}
	}
	return checkIn, checkOut, rows.Err()
}

// GatherMessages reads messages for a transaction (limited to last 50).
func (r *Repository) GatherMessages(ctx context.Context, transactionID string) ([]MessageRef, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT sender_id, content, created_at
		 FROM messages
		 WHERE transaction_id = $1
		 ORDER BY created_at DESC
		 LIMIT 50`,
		transactionID,
	)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var msgs []MessageRef
	for rows.Next() {
		var m MessageRef
		if err := rows.Scan(&m.SenderID, &m.Content, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// GatherProximityProofs reads proximity proofs for a transaction.
func (r *Repository) GatherProximityProofs(ctx context.Context, transactionID string) ([]ProximityRef, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT proof_type, method, gps_distance, verified
		 FROM proximity_proofs
		 WHERE transaction_id = $1
		 ORDER BY created_at ASC`,
		transactionID,
	)
	if err != nil {
		return nil, fmt.Errorf("query proximity proofs: %w", err)
	}
	defer rows.Close()

	var proofs []ProximityRef
	for rows.Next() {
		var p ProximityRef
		if err := rows.Scan(&p.ProofType, &p.Method, &p.GPSDistance, &p.Verified); err != nil {
			return nil, fmt.Errorf("scan proximity proof: %w", err)
		}
		proofs = append(proofs, p)
	}
	return proofs, rows.Err()
}

// GatherPhotoDiff reads the photo diff result for a transaction.
func (r *Repository) GatherPhotoDiff(ctx context.Context, transactionID string) (*string, *float64, error) {
	var result *string
	var confidence *float64
	err := r.pool.QueryRow(ctx,
		`SELECT photo_diff_result, photo_diff_confidence FROM transactions WHERE id = $1`,
		transactionID,
	).Scan(&result, &confidence)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrTransactionNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("gather photo diff: %w", err)
	}
	return result, confidence, nil
}

// GatherReputationScores reads reputation scores for two users.
func (r *Repository) GatherReputationScores(ctx context.Context, userID1, userID2 string) (score1, score2 int, err error) {
	err = r.pool.QueryRow(ctx,
		`SELECT COALESCE(reputation_score, 0) FROM users WHERE id = $1`, userID1,
	).Scan(&score1)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, fmt.Errorf("get reputation score: %w", err)
	}
	err = r.pool.QueryRow(ctx,
		`SELECT COALESCE(reputation_score, 0) FROM users WHERE id = $1`, userID2,
	).Scan(&score2)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, fmt.Errorf("get reputation score: %w", err)
	}
	return score1, score2, nil
}

// HasFraudFlags checks whether either party has active fraud flags (reputation signals).
func (r *Repository) HasFraudFlags(ctx context.Context, renterID, hostID string) (bool, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM reputation_signals
		 WHERE user_id IN ($1, $2) AND signal_type = 'fraud_flag'`,
		renterID, hostID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check fraud flags: %w", err)
	}
	return count > 0, nil
}

// GetTransactionParties returns the renter_id and host_id for a transaction.
func (r *Repository) GetTransactionParties(ctx context.Context, transactionID string) (renterID, hostID string, err error) {
	err = r.pool.QueryRow(ctx,
		`SELECT renter_id, host_id FROM transactions WHERE id = $1`, transactionID,
	).Scan(&renterID, &hostID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrTransactionNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("get transaction parties: %w", err)
	}
	return renterID, hostID, nil
}

const disputeCols = `id, transaction_id, reporter_id, reason, description, status,
	COALESCE(route, ''), COALESCE(charge_amount, 0), COALESCE(confidence, 0),
	agent_decision_id, reviewer_id, reviewer_notes, sla_deadline, evidence,
	created_at, updated_at`

func (r *Repository) scanOne(ctx context.Context, query string, args ...interface{}) (Dispute, error) {
	var d Dispute
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&d.ID, &d.TransactionID, &d.ReporterID, &d.Reason, &d.Description, &d.Status,
		&d.Route, &d.ChargeAmount, &d.Confidence,
		&d.AgentDecisionID, &d.ReviewerID, &d.ReviewerNotes, &d.SLADeadline, &d.Evidence,
		&d.CreatedAt, &d.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Dispute{}, ErrDisputeNotFound
	}
	if err != nil {
		return Dispute{}, fmt.Errorf("scan dispute: %w", err)
	}
	return d, nil
}

func (r *Repository) scanMany(rows pgx.Rows) ([]Dispute, error) {
	var disputes []Dispute
	for rows.Next() {
		var d Dispute
		if err := rows.Scan(
			&d.ID, &d.TransactionID, &d.ReporterID, &d.Reason, &d.Description, &d.Status,
			&d.Route, &d.ChargeAmount, &d.Confidence,
			&d.AgentDecisionID, &d.ReviewerID, &d.ReviewerNotes, &d.SLADeadline, &d.Evidence,
			&d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan dispute row: %w", err)
		}
		disputes = append(disputes, d)
	}
	return disputes, rows.Err()
}
