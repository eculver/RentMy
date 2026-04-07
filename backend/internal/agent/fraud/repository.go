package fraud

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// transactionRow is a minimal projection of the transactions table used by
// the FraudAgent to identify the renter and host.
type transactionRow struct {
	RenterID string
	HostID   string
}

// Repository performs all database operations for the FraudAgent.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given connection pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// GetTransaction returns renter_id and host_id for the given transaction.
func (r *Repository) GetTransaction(ctx context.Context, id string) (transactionRow, error) {
	const q = `SELECT renter_id, host_id FROM transactions WHERE id = $1`
	row := r.pool.QueryRow(ctx, q, id)

	var t transactionRow
	if err := row.Scan(&t.RenterID, &t.HostID); err != nil {
		if err == pgx.ErrNoRows {
			return transactionRow{}, fmt.Errorf("fraud: transaction %s not found", id)
		}
		return transactionRow{}, fmt.Errorf("fraud: get transaction: %w", err)
	}
	return t, nil
}

// InsertFraudFlag persists a new fraud flag and appends the flag ID to the
// user's risk_flags JSONB array atomically.
func (r *Repository) InsertFraudFlag(ctx context.Context, flag FraudFlag) error {
	signalsJSON, err := json.Marshal(flag.Signals)
	if err != nil {
		return fmt.Errorf("fraud: marshal signals: %w", err)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("fraud: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const insertFlag = `
		INSERT INTO fraud_flags
		    (id, user_id, signals, total_score, action, agent_decision_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	if _, err := tx.Exec(ctx, insertFlag,
		flag.ID, flag.UserID, signalsJSON, flag.TotalScore,
		string(flag.Action), flag.AgentDecisionID, flag.CreatedAt,
	); err != nil {
		return fmt.Errorf("fraud: insert flag: %w", err)
	}

	// Append the flag ID to the user's risk_flags JSONB array.
	const appendRiskFlag = `
		UPDATE users
		SET risk_flags = risk_flags || to_jsonb($2::text)
		WHERE id = $1`
	if _, err := tx.Exec(ctx, appendRiskFlag, flag.UserID, flag.ID); err != nil {
		return fmt.Errorf("fraud: append risk flag: %w", err)
	}

	return tx.Commit(ctx)
}

// ListFraudFlags returns a paginated list of fraud flags filtered by the
// provided criteria.
func (r *Repository) ListFraudFlags(ctx context.Context, f FraudFlagFilters) ([]FraudFlag, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 || f.Limit > 200 {
		f.Limit = 50
	}
	offset := (f.Page - 1) * f.Limit

	// Build WHERE clause dynamically.
	where := "TRUE"
	args := []any{}
	argN := 1

	if f.Status == "OPEN" {
		where += fmt.Sprintf(" AND resolved_at IS NULL")
	} else if f.Status == "RESOLVED" {
		where += fmt.Sprintf(" AND resolved_at IS NOT NULL")
	}
	if f.Action != "" {
		where += fmt.Sprintf(" AND action = $%d", argN)
		args = append(args, string(f.Action))
		argN++
	}

	countQ := fmt.Sprintf(`SELECT COUNT(*) FROM fraud_flags WHERE %s`, where)
	var total int
	if err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("fraud: count flags: %w", err)
	}

	args = append(args, f.Limit, offset)
	listQ := fmt.Sprintf(`
		SELECT id, user_id, signals, total_score, action, agent_decision_id,
		       resolved_at, resolved_by, resolution_notes, created_at
		FROM fraud_flags
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, argN, argN+1)

	rows, err := r.pool.Query(ctx, listQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("fraud: list flags: %w", err)
	}
	defer rows.Close()

	var flags []FraudFlag
	for rows.Next() {
		f, err := scanFraudFlag(rows)
		if err != nil {
			return nil, 0, err
		}
		flags = append(flags, f)
	}
	return flags, total, rows.Err()
}

// GetFraudFlag returns a single fraud flag by ID.
func (r *Repository) GetFraudFlag(ctx context.Context, id string) (FraudFlag, error) {
	const q = `
		SELECT id, user_id, signals, total_score, action, agent_decision_id,
		       resolved_at, resolved_by, resolution_notes, created_at
		FROM fraud_flags WHERE id = $1`

	row := r.pool.QueryRow(ctx, q, id)
	flag, err := scanFraudFlag(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return FraudFlag{}, ErrFlagNotFound
		}
		return FraudFlag{}, fmt.Errorf("fraud: get flag: %w", err)
	}
	return flag, nil
}

// ResolveFraudFlag marks a flag as resolved and optionally updates
// outcome_correct on the linked agent_decision.
func (r *Repository) ResolveFraudFlag(ctx context.Context, flagID, resolvedBy, outcome, notes string) error {
	now := time.Now().UTC()
	const q = `
		UPDATE fraud_flags
		SET resolved_at = $2, resolved_by = $3, resolution_notes = $4
		WHERE id = $1 AND resolved_at IS NULL`

	tag, err := r.pool.Exec(ctx, q, flagID, now, resolvedBy, notes)
	if err != nil {
		return fmt.Errorf("fraud: resolve flag: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrFlagNotFound
	}

	// Mark the linked decision as verified if outcome indicates it was correct.
	if outcome == "CORRECT" {
		correct := true
		const updateDecision = `
			UPDATE agent_decisions
			SET outcome_correct = $2
			WHERE id = (SELECT agent_decision_id FROM fraud_flags WHERE id = $1)`
		if _, err := r.pool.Exec(ctx, updateDecision, flagID, correct); err != nil {
			// Non-fatal: log and continue.
			_ = err
		}
	}
	return nil
}

// GetUserFraudHistory returns all fraud signals and flags for a user.
func (r *Repository) GetUserFraudHistory(ctx context.Context, userID string) ([]FraudFlag, error) {
	const q = `
		SELECT id, user_id, signals, total_score, action, agent_decision_id,
		       resolved_at, resolved_by, resolution_notes, created_at
		FROM fraud_flags
		WHERE user_id = $1
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("fraud: user history query: %w", err)
	}
	defer rows.Close()

	var flags []FraudFlag
	for rows.Next() {
		f, err := scanFraudFlag(rows)
		if err != nil {
			return nil, err
		}
		flags = append(flags, f)
	}
	return flags, rows.Err()
}

// scanner abstracts pgx.Row and pgx.Rows so scanFraudFlag works for both.
type scanner interface {
	Scan(dest ...any) error
}

func scanFraudFlag(s scanner) (FraudFlag, error) {
	var flag FraudFlag
	var signalsJSON []byte
	var action string
	if err := s.Scan(
		&flag.ID,
		&flag.UserID,
		&signalsJSON,
		&flag.TotalScore,
		&action,
		&flag.AgentDecisionID,
		&flag.ResolvedAt,
		&flag.ResolvedBy,
		&flag.ResolutionNotes,
		&flag.CreatedAt,
	); err != nil {
		return FraudFlag{}, fmt.Errorf("fraud: scan flag: %w", err)
	}
	flag.Action = Action(action)
	if err := json.Unmarshal(signalsJSON, &flag.Signals); err != nil {
		return FraudFlag{}, fmt.Errorf("fraud: unmarshal signals: %w", err)
	}
	return flag, nil
}
