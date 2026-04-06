package outcome

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"github.com/giits/rentmy/backend/internal/agent/decision"
)

// Repository handles persistence for outcome linking and calibration data.
type Repository struct {
	pool  *pgxpool.Pool
	redis *goredis.Client
}

// NewRepository creates a new Repository.
func NewRepository(pool *pgxpool.Pool, redis *goredis.Client) *Repository {
	return &Repository{pool: pool, redis: redis}
}

// FindDecisionsByTransaction returns all agent decisions for a transaction.
func (r *Repository) FindDecisionsByTransaction(ctx context.Context, transactionID string) ([]*decision.AgentDecision, error) {
	const q = `
SELECT id, agent_type, transaction_id, user_id, input, decision,
       model, prompt_version, confidence, escalated, escalation_reason,
       reviewed_by, override_of, outcome_id, outcome_correct, created_at
FROM agent_decisions
WHERE transaction_id = $1
ORDER BY created_at ASC`

	rows, err := r.pool.Query(ctx, q, transactionID)
	if err != nil {
		return nil, fmt.Errorf("outcome: query decisions: %w", err)
	}
	defer rows.Close()

	var out []*decision.AgentDecision
	for rows.Next() {
		var d decision.AgentDecision
		var agentType string
		if err := rows.Scan(
			&d.ID, &agentType, &d.TransactionID, &d.UserID, &d.Input, &d.Decision,
			&d.Model, &d.PromptVersion, &d.Confidence, &d.Escalated, &d.EscalationReason,
			&d.ReviewedBy, &d.OverrideOf, &d.OutcomeID, &d.OutcomeCorrect, &d.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("outcome: scanning decision: %w", err)
		}
		d.AgentType = decision.AgentType(agentType)
		out = append(out, &d)
	}
	return out, rows.Err()
}

// HasHumanOverride checks if a specific agent decision was overridden by a human.
func (r *Repository) HasHumanOverride(ctx context.Context, decisionID string) (bool, error) {
	const q = `
SELECT EXISTS(
    SELECT 1 FROM agent_decisions
    WHERE override_of = $1 AND agent_type = 'HUMAN_OVERRIDE'
)`
	var exists bool
	if err := r.pool.QueryRow(ctx, q, decisionID).Scan(&exists); err != nil {
		return false, fmt.Errorf("outcome: check override: %w", err)
	}
	return exists, nil
}

// HasDisputeForTransaction checks if a dispute was filed for a transaction.
func (r *Repository) HasDisputeForTransaction(ctx context.Context, transactionID string) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM disputes WHERE transaction_id = $1)`
	var exists bool
	if err := r.pool.QueryRow(ctx, q, transactionID).Scan(&exists); err != nil {
		return false, fmt.Errorf("outcome: check dispute: %w", err)
	}
	return exists, nil
}

// WasLateReturnEscalationWarranted checks if a late return escalation was warranted
// (renter was genuinely non-responsive, i.e., the late return record ended in
// ESCALATED_TO_DISPUTE rather than RESOLVED with a return).
func (r *Repository) WasLateReturnEscalationWarranted(ctx context.Context, transactionID string) (bool, error) {
	const q = `
SELECT COALESCE(
    (SELECT status = 'ESCALATED_TO_DISPUTE' FROM late_returns
     WHERE transaction_id = $1
     ORDER BY created_at DESC LIMIT 1),
    false
)`
	var warranted bool
	if err := r.pool.QueryRow(ctx, q, transactionID).Scan(&warranted); err != nil {
		return false, fmt.Errorf("outcome: check late return: %w", err)
	}
	return warranted, nil
}

// DidRenterReturnWithinGrace checks if the renter returned within the grace period
// (late return resolved without escalation).
func (r *Repository) DidRenterReturnWithinGrace(ctx context.Context, transactionID string) (bool, error) {
	const q = `
SELECT COALESCE(
    (SELECT status = 'RESOLVED' FROM late_returns
     WHERE transaction_id = $1
     ORDER BY created_at DESC LIMIT 1),
    true
)`
	var withinGrace bool
	if err := r.pool.QueryRow(ctx, q, transactionID).Scan(&withinGrace); err != nil {
		return false, fmt.Errorf("outcome: check grace return: %w", err)
	}
	return withinGrace, nil
}

// HasDisputeFromAgreementGap checks if a dispute was filed with a reason related
// to agreement gaps.
func (r *Repository) HasDisputeFromAgreementGap(ctx context.Context, transactionID string) (bool, error) {
	const q = `
SELECT EXISTS(
    SELECT 1 FROM disputes
    WHERE transaction_id = $1
    AND (reason ILIKE '%agreement%' OR reason ILIKE '%clause%' OR reason ILIKE '%terms%')
)`
	var exists bool
	if err := r.pool.QueryRow(ctx, q, transactionID).Scan(&exists); err != nil {
		return false, fmt.Errorf("outcome: check agreement gap: %w", err)
	}
	return exists, nil
}

// IsUserFraudFlagged checks if a user has been fraud-flagged.
func (r *Repository) IsUserFraudFlagged(ctx context.Context, userID string) (bool, error) {
	const q = `
SELECT COALESCE(
    (SELECT risk_score >= 80 FROM risk_scores WHERE user_id = $1),
    false
)`
	var flagged bool
	if err := r.pool.QueryRow(ctx, q, userID).Scan(&flagged); err != nil {
		return false, fmt.Errorf("outcome: check fraud flag: %w", err)
	}
	return flagged, nil
}

// GetCalibrationStats retrieves raw calibration data for an agent type and
// confidence bucket from the last 90 days.
func (r *Repository) GetCalibrationStats(ctx context.Context, agentType string, bucketLow, bucketHigh float64) (total int, correct int, err error) {
	cutoff := time.Now().Add(-90 * 24 * time.Hour)
	const q = `
SELECT
    COUNT(*),
    COUNT(*) FILTER (WHERE outcome_correct = true)
FROM agent_decisions
WHERE agent_type = $1
  AND confidence >= $2 AND confidence < $3
  AND outcome_correct IS NOT NULL
  AND created_at >= $4`

	if err := r.pool.QueryRow(ctx, q, agentType, bucketLow, bucketHigh, cutoff).Scan(&total, &correct); err != nil {
		return 0, 0, fmt.Errorf("outcome: calibration stats: %w", err)
	}
	return total, correct, nil
}

// StoreCalibrationBucket stores a calibration bucket in Redis with a 90-day TTL.
func (r *Repository) StoreCalibrationBucket(ctx context.Context, bucket CalibrationBucket) error {
	key := fmt.Sprintf("calibration:%s:%.1f-%.1f", bucket.AgentType, bucket.BucketLow, bucket.BucketHigh)
	data, err := json.Marshal(bucket)
	if err != nil {
		return fmt.Errorf("outcome: marshal bucket: %w", err)
	}
	if err := r.redis.Set(ctx, key, data, 90*24*time.Hour).Err(); err != nil {
		return fmt.Errorf("outcome: store bucket: %w", err)
	}
	return nil
}

// GetCalibrationBuckets retrieves all calibration buckets for an agent type from Redis.
func (r *Repository) GetCalibrationBuckets(ctx context.Context, agentType string) ([]CalibrationBucket, error) {
	var buckets []CalibrationBucket
	for _, b := range CalibrationBuckets {
		key := fmt.Sprintf("calibration:%s:%.1f-%.1f", agentType, b.Low, b.High)
		data, err := r.redis.Get(ctx, key).Result()
		if err != nil {
			continue // bucket has no data yet
		}
		var bucket CalibrationBucket
		if err := json.Unmarshal([]byte(data), &bucket); err != nil {
			continue
		}
		buckets = append(buckets, bucket)
	}
	return buckets, nil
}

// GetAllAgentCalibration returns calibration reports for all agent types.
func (r *Repository) GetAllAgentCalibration(ctx context.Context) ([]CalibrationReport, error) {
	agentTypes := []decision.AgentType{
		decision.AgentTypeDispute,
		decision.AgentTypeRisk,
		decision.AgentTypeAppraisal,
		decision.AgentTypeLateReturn,
		decision.AgentTypeAgreement,
		decision.AgentTypeVerification,
		decision.AgentTypeFraud,
	}

	var reports []CalibrationReport
	for _, at := range agentTypes {
		buckets, err := r.GetCalibrationBuckets(ctx, string(at))
		if err != nil {
			continue
		}
		if len(buckets) == 0 {
			continue
		}

		report := CalibrationReport{
			AgentType:   string(at),
			Buckets:     buckets,
			GeneratedAt: time.Now().UTC(),
		}

		var totalCalErr float64
		var correctDecisions int
		for _, b := range buckets {
			report.TotalDecisions += b.TotalDecisions
			correctDecisions += b.CorrectDecisions
			totalCalErr += b.CalibrationError
		}
		if report.TotalDecisions > 0 {
			report.OverallAccuracy = float64(correctDecisions) / float64(report.TotalDecisions)
		}
		if len(buckets) > 0 {
			report.MeanCalibration = totalCalErr / float64(len(buckets))
		}
		reports = append(reports, report)
	}
	return reports, nil
}

// CorrectDecisions is a field on CalibrationReport not in the struct — add it:
// (Already tracked via Buckets summation in GetAllAgentCalibration)

// GetPaginatedDecisions returns paginated agent decisions with outcome data.
func (r *Repository) GetPaginatedDecisions(ctx context.Context, agentType string, outcomeFilter *bool, limit, offset int) ([]DecisionWithOutcome, error) {
	q := `
SELECT id, agent_type, transaction_id, confidence, escalated,
       outcome_correct, outcome_id, created_at
FROM agent_decisions
WHERE 1=1`
	args := []any{}
	argIdx := 1

	if agentType != "" {
		q += fmt.Sprintf(" AND agent_type = $%d", argIdx)
		args = append(args, agentType)
		argIdx++
	}
	if outcomeFilter != nil {
		q += fmt.Sprintf(" AND outcome_correct = $%d", argIdx)
		args = append(args, *outcomeFilter)
		argIdx++
	}

	q += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("outcome: query decisions: %w", err)
	}
	defer rows.Close()

	var out []DecisionWithOutcome
	for rows.Next() {
		var d DecisionWithOutcome
		if err := rows.Scan(
			&d.ID, &d.AgentType, &d.TransactionID, &d.Confidence,
			&d.Escalated, &d.OutcomeCorrect, &d.OutcomeID, &d.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("outcome: scanning: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// buildCalibrationBucket computes a single calibration bucket from DB stats.
func buildCalibrationBucket(agentType string, low, high float64, total, correct int) CalibrationBucket {
	expected := (low + high) / 2
	var actual float64
	if total > 0 {
		actual = float64(correct) / float64(total)
	}
	return CalibrationBucket{
		AgentType:        agentType,
		BucketLow:        low,
		BucketHigh:       high,
		ExpectedAccuracy: expected,
		ActualAccuracy:   actual,
		CalibrationError: math.Abs(expected - actual),
		TotalDecisions:   total,
		CorrectDecisions: correct,
		UpdatedAt:        time.Now().UTC(),
	}
}
