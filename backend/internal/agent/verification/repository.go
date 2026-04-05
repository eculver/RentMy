package verification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrAttemptNotFound is returned when no matching verification attempt exists.
var ErrAttemptNotFound = errors.New("verification attempt not found")

// Repository handles Postgres operations for the verification domain.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Insert persists a new verification attempt.
func (r *Repository) Insert(ctx context.Context, a *VerificationAttempt) (*VerificationAttempt, error) {
	fraudJSON, err := json.Marshal(a.FraudIndicators)
	if err != nil {
		return nil, fmt.Errorf("marshalling fraud indicators: %w", err)
	}

	const q = `
		INSERT INTO verification_attempts
		    (id, user_id, stripe_session_id, status, stripe_status, stripe_reason,
		     document_type, selfie_match_score, fraud_indicators, decision, confidence,
		     escalation_reason, model, prompt_version, retry_count, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		RETURNING id, user_id, stripe_session_id, status, stripe_status, stripe_reason,
		          document_type, selfie_match_score, fraud_indicators, decision, confidence,
		          escalation_reason, model, prompt_version, retry_count, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		a.ID, a.UserID, a.StripeSessionID, string(a.Status),
		nilString(a.StripeStatus), nilString(a.StripeReason), nilString(a.DocumentType),
		a.SelfieMatchScore, fraudJSON, a.Decision, a.Confidence,
		a.EscalationReason, a.Model, a.PromptVersion, a.RetryCount,
		a.CreatedAt, a.UpdatedAt,
	)
	return scanAttempt(row)
}

// FindByUserID returns the most recent verification attempt for a user.
// Returns ErrAttemptNotFound if none exists.
func (r *Repository) FindByUserID(ctx context.Context, userID string) (*VerificationAttempt, error) {
	const q = `
		SELECT id, user_id, stripe_session_id, status, stripe_status, stripe_reason,
		       document_type, selfie_match_score, fraud_indicators, decision, confidence,
		       escalation_reason, model, prompt_version, retry_count, created_at, updated_at
		FROM verification_attempts
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 1`

	a, err := scanAttempt(r.pool.QueryRow(ctx, q, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAttemptNotFound
	}
	return a, err
}

// FindBySessionID returns the attempt for a given Stripe session ID.
// Returns ErrAttemptNotFound if none exists.
func (r *Repository) FindBySessionID(ctx context.Context, sessionID string) (*VerificationAttempt, error) {
	const q = `
		SELECT id, user_id, stripe_session_id, status, stripe_status, stripe_reason,
		       document_type, selfie_match_score, fraud_indicators, decision, confidence,
		       escalation_reason, model, prompt_version, retry_count, created_at, updated_at
		FROM verification_attempts
		WHERE stripe_session_id = $1`

	a, err := scanAttempt(r.pool.QueryRow(ctx, q, sessionID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAttemptNotFound
	}
	return a, err
}

// UpdateStatus updates the outcome fields for a verification attempt.
func (r *Repository) UpdateStatus(ctx context.Context, id string, in updateStatusInput) error {
	fraudJSON, err := json.Marshal(in.FraudIndicators)
	if err != nil {
		return fmt.Errorf("marshalling fraud indicators: %w", err)
	}

	const q = `
		UPDATE verification_attempts
		SET status            = $2,
		    stripe_status     = $3,
		    stripe_reason     = $4,
		    document_type     = $5,
		    selfie_match_score = $6,
		    fraud_indicators  = $7,
		    decision          = $8,
		    confidence        = $9,
		    escalation_reason = $10,
		    model             = $11,
		    prompt_version    = $12,
		    updated_at        = $13
		WHERE id = $1`

	_, err = r.pool.Exec(ctx, q,
		id, string(in.Status),
		nilString(in.StripeStatus), nilString(in.StripeReason), nilString(in.DocumentType),
		in.SelfieMatchScore, fraudJSON, in.Decision, in.Confidence,
		in.EscalationReason, in.Model, in.PromptVersion,
		time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("updating verification attempt status: %w", err)
	}
	return nil
}

// IncrementRetryCount adds 1 to retry_count for the given attempt.
func (r *Repository) IncrementRetryCount(ctx context.Context, id string) error {
	const q = `
		UPDATE verification_attempts
		SET retry_count = retry_count + 1, updated_at = $2
		WHERE id = $1`

	_, err := r.pool.Exec(ctx, q, id, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("incrementing retry count: %w", err)
	}
	return nil
}

// scanAttempt reads a verification_attempt row into a VerificationAttempt.
func scanAttempt(row pgx.Row) (*VerificationAttempt, error) {
	var (
		a              VerificationAttempt
		status         string
		stripeStatus   *string
		stripeReason   *string
		documentType   *string
		fraudJSON      []byte
		decision       *string
		model          *string
		promptVersion  *string
		escalation     *string
	)
	err := row.Scan(
		&a.ID, &a.UserID, &a.StripeSessionID, &status,
		&stripeStatus, &stripeReason, &documentType,
		&a.SelfieMatchScore, &fraudJSON,
		&decision, &a.Confidence,
		&escalation, &model, &promptVersion,
		&a.RetryCount, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	a.Status = VerificationStatus(status)
	if stripeStatus != nil {
		a.StripeStatus = *stripeStatus
	}
	if stripeReason != nil {
		a.StripeReason = *stripeReason
	}
	if documentType != nil {
		a.DocumentType = *documentType
	}
	a.Decision = decision
	a.EscalationReason = escalation
	a.Model = model
	a.PromptVersion = promptVersion

	if err := json.Unmarshal(fraudJSON, &a.FraudIndicators); err != nil {
		a.FraudIndicators = []string{}
	}
	return &a, nil
}

// nilString converts an empty string to nil for nullable text columns.
func nilString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
