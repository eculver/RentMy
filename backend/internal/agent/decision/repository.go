package decision

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Brett2thered/RentMy/backend/internal/platform/ulid"
)

// Repository handles persistence for AgentDecision records.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository backed by the provided pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Insert persists a new AgentDecision and returns it with its generated ID.
func (r *Repository) Insert(ctx context.Context, in CreateDecisionInput) (*AgentDecision, error) {
	inputJSON, err := json.Marshal(in.Input)
	if err != nil {
		return nil, fmt.Errorf("decision: marshaling input: %w", err)
	}
	decisionJSON, err := json.Marshal(in.Decision)
	if err != nil {
		return nil, fmt.Errorf("decision: marshaling decision: %w", err)
	}

	id := ulid.New()
	now := time.Now().UTC()

	const q = `
INSERT INTO agent_decisions (
    id, agent_type, transaction_id, user_id, input, decision,
    model, prompt_version, confidence, escalated, escalation_reason, override_of, created_at
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11, $12, $13
)`
	_, err = r.pool.Exec(ctx, q,
		id,
		string(in.AgentType),
		in.TransactionID,
		in.UserID,
		inputJSON,
		decisionJSON,
		in.Model,
		in.PromptVersion,
		in.Confidence,
		in.Escalated,
		in.EscalationReason,
		in.OverrideOf,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("decision: inserting: %w", err)
	}

	return &AgentDecision{
		ID:               id,
		AgentType:        in.AgentType,
		TransactionID:    in.TransactionID,
		UserID:           in.UserID,
		Input:            inputJSON,
		Decision:         decisionJSON,
		Model:            in.Model,
		PromptVersion:    in.PromptVersion,
		Confidence:       in.Confidence,
		Escalated:        in.Escalated,
		EscalationReason: in.EscalationReason,
		OverrideOf:       in.OverrideOf,
		CreatedAt:        now,
	}, nil
}

// FindByID returns a single AgentDecision by its ID.
func (r *Repository) FindByID(ctx context.Context, id string) (*AgentDecision, error) {
	const q = `
SELECT id, agent_type, transaction_id, user_id, input, decision,
       model, prompt_version, confidence, escalated, escalation_reason,
       reviewed_by, override_of, outcome_id, outcome_correct, created_at
FROM agent_decisions
WHERE id = $1`
	return r.scanOne(ctx, q, id)
}

// FindByTransactionID returns all decisions for a transaction, newest first.
func (r *Repository) FindByTransactionID(ctx context.Context, transactionID string) ([]*AgentDecision, error) {
	const q = `
SELECT id, agent_type, transaction_id, user_id, input, decision,
       model, prompt_version, confidence, escalated, escalation_reason,
       reviewed_by, override_of, outcome_id, outcome_correct, created_at
FROM agent_decisions
WHERE transaction_id = $1
ORDER BY created_at DESC`
	return r.scanMany(ctx, q, transactionID)
}

// FindByUserID returns all decisions for a user, newest first.
func (r *Repository) FindByUserID(ctx context.Context, userID string) ([]*AgentDecision, error) {
	const q = `
SELECT id, agent_type, transaction_id, user_id, input, decision,
       model, prompt_version, confidence, escalated, escalation_reason,
       reviewed_by, override_of, outcome_id, outcome_correct, created_at
FROM agent_decisions
WHERE user_id = $1
ORDER BY created_at DESC`
	return r.scanMany(ctx, q, userID)
}

// FindByAgentType returns decisions of a given type within an optional date range.
func (r *Repository) FindByAgentType(ctx context.Context, agentType AgentType, limit int) ([]*AgentDecision, error) {
	const q = `
SELECT id, agent_type, transaction_id, user_id, input, decision,
       model, prompt_version, confidence, escalated, escalation_reason,
       reviewed_by, override_of, outcome_id, outcome_correct, created_at
FROM agent_decisions
WHERE agent_type = $1
ORDER BY created_at DESC
LIMIT $2`
	return r.scanMany(ctx, q, string(agentType), limit)
}

// UpdateOutcome links a decision to its verified outcome for the learning loop.
func (r *Repository) UpdateOutcome(ctx context.Context, in UpdateOutcomeInput) error {
	const q = `
UPDATE agent_decisions
SET outcome_id = $2, outcome_correct = $3
WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, in.DecisionID, in.OutcomeID, in.OutcomeCorrect)
	if err != nil {
		return fmt.Errorf("decision: updating outcome: %w", err)
	}
	return nil
}

func (r *Repository) scanOne(ctx context.Context, query string, args ...any) (*AgentDecision, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("decision: query: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("decision: iterating: %w", err)
		}
		return nil, fmt.Errorf("decision: not found")
	}
	d, err := scanDecision(rows)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (r *Repository) scanMany(ctx context.Context, query string, args ...any) ([]*AgentDecision, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("decision: query: %w", err)
	}
	defer rows.Close()

	var out []*AgentDecision
	for rows.Next() {
		d, err := scanDecision(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

type scannable interface {
	Scan(dest ...any) error
}

func scanDecision(row scannable) (*AgentDecision, error) {
	var d AgentDecision
	var agentType string
	if err := row.Scan(
		&d.ID, &agentType, &d.TransactionID, &d.UserID, &d.Input, &d.Decision,
		&d.Model, &d.PromptVersion, &d.Confidence, &d.Escalated, &d.EscalationReason,
		&d.ReviewedBy, &d.OverrideOf, &d.OutcomeID, &d.OutcomeCorrect, &d.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("decision: scanning row: %w", err)
	}
	d.AgentType = AgentType(agentType)
	return &d, nil
}
