package outcome

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/giits/rentmy/backend/internal/agent/decision"
	"github.com/giits/rentmy/backend/internal/platform/ulid"
)

// Service implements the outcome linking and calibration business logic.
type Service struct {
	repo        *Repository
	decisionSvc *decision.Service
}

// NewService creates a new outcome Service.
func NewService(repo *Repository, decisionSvc *decision.Service) *Service {
	return &Service{
		repo:        repo,
		decisionSvc: decisionSvc,
	}
}

// LinkOutcomes evaluates all agent decisions for a transaction and links them
// to their real-world outcomes. Called 48h after transaction close.
func (s *Service) LinkOutcomes(ctx context.Context, transactionID string) error {
	decisions, err := s.repo.FindDecisionsByTransaction(ctx, transactionID)
	if err != nil {
		return fmt.Errorf("link outcomes: find decisions: %w", err)
	}

	if len(decisions) == 0 {
		slog.Info("outcome: no decisions to link", "transactionId", transactionID)
		return nil
	}

	outcomeID := ulid.New()
	agentTypesEncountered := map[decision.AgentType]bool{}

	for _, d := range decisions {
		// Skip decisions that already have outcomes or are human overrides.
		if d.OutcomeCorrect != nil {
			continue
		}
		if d.AgentType == decision.AgentTypeHumanOverride {
			continue
		}

		correct, reason, err := s.evaluateDecision(ctx, d, transactionID)
		if err != nil {
			slog.Warn("outcome: failed to evaluate decision",
				"decisionId", d.ID,
				"agentType", d.AgentType,
				"error", err,
			)
			continue
		}

		if err := s.decisionSvc.LinkOutcome(ctx, decision.UpdateOutcomeInput{
			DecisionID:     d.ID,
			OutcomeID:      outcomeID,
			OutcomeCorrect: correct,
		}); err != nil {
			slog.Warn("outcome: failed to link",
				"decisionId", d.ID,
				"error", err,
			)
			continue
		}

		slog.Info("outcome: decision linked",
			"decisionId", d.ID,
			"agentType", d.AgentType,
			"correct", correct,
			"reason", reason,
		)
		agentTypesEncountered[d.AgentType] = true
	}

	// Update calibration for each agent type encountered.
	for at := range agentTypesEncountered {
		if err := s.UpdateCalibrationMetrics(ctx, string(at)); err != nil {
			slog.Warn("outcome: failed to update calibration",
				"agentType", at,
				"error", err,
			)
		}
	}

	return nil
}

// evaluateDecision determines if a decision was correct based on agent-specific rules.
func (s *Service) evaluateDecision(ctx context.Context, d *decision.AgentDecision, transactionID string) (bool, string, error) {
	switch d.AgentType {
	case decision.AgentTypeDispute:
		return s.evaluateDispute(ctx, d)
	case decision.AgentTypeLateReturn:
		return s.evaluateLateReturn(ctx, transactionID)
	case decision.AgentTypeAgreement:
		return s.evaluateAgreement(ctx, transactionID)
	case decision.AgentTypeVerification:
		return s.evaluateVerification(ctx, d)
	case decision.AgentTypeAppraisal:
		return s.evaluateAppraisal(ctx, d)
	case decision.AgentTypeRisk:
		return s.evaluateRisk(ctx, d, transactionID)
	case decision.AgentTypeFraud:
		return s.evaluateFraud(ctx, d)
	default:
		return true, "unknown agent type, defaulting to correct", nil
	}
}

// evaluateDispute: correct if not overridden by human reviewer.
func (s *Service) evaluateDispute(ctx context.Context, d *decision.AgentDecision) (bool, string, error) {
	overridden, err := s.repo.HasHumanOverride(ctx, d.ID)
	if err != nil {
		return false, "", fmt.Errorf("check override: %w", err)
	}
	if overridden {
		return false, "decision was overridden by human reviewer", nil
	}
	return true, "decision was not overridden", nil
}

// evaluateLateReturn: correct if escalation was warranted (renter genuinely
// non-responsive) vs premature (renter returned within grace).
func (s *Service) evaluateLateReturn(ctx context.Context, transactionID string) (bool, string, error) {
	warranted, err := s.repo.WasLateReturnEscalationWarranted(ctx, transactionID)
	if err != nil {
		return false, "", fmt.Errorf("check late return: %w", err)
	}
	withinGrace, err := s.repo.DidRenterReturnWithinGrace(ctx, transactionID)
	if err != nil {
		return false, "", fmt.Errorf("check grace: %w", err)
	}

	// If the late return was escalated and the renter didn't return within grace,
	// the escalation was warranted (correct). If the renter returned within grace
	// but was escalated, the escalation was premature (incorrect).
	if warranted {
		return true, "escalation was warranted, renter was non-responsive", nil
	}
	if withinGrace {
		return false, "premature escalation, renter returned within grace", nil
	}
	return true, "no escalation needed and none occurred", nil
}

// evaluateAgreement: correct if no dispute arose from agreement gap.
func (s *Service) evaluateAgreement(ctx context.Context, transactionID string) (bool, string, error) {
	gapDispute, err := s.repo.HasDisputeFromAgreementGap(ctx, transactionID)
	if err != nil {
		return false, "", fmt.Errorf("check agreement gap: %w", err)
	}
	if gapDispute {
		return false, "dispute arose from agreement gap", nil
	}
	return true, "no dispute from agreement gap", nil
}

// evaluateVerification: correct if verified user not later fraud-flagged.
func (s *Service) evaluateVerification(ctx context.Context, d *decision.AgentDecision) (bool, string, error) {
	if d.UserID == nil {
		return true, "no user associated, defaulting to correct", nil
	}
	flagged, err := s.repo.IsUserFraudFlagged(ctx, *d.UserID)
	if err != nil {
		return false, "", fmt.Errorf("check fraud flag: %w", err)
	}
	if flagged {
		return false, "verified user was later fraud-flagged", nil
	}
	return true, "verified user has no fraud flags", nil
}

// evaluateAppraisal: correct if no host override occurred. Full value accuracy
// requires damage claim data which may not be available for every transaction.
func (s *Service) evaluateAppraisal(ctx context.Context, d *decision.AgentDecision) (bool, string, error) {
	overridden, err := s.repo.HasHumanOverride(ctx, d.ID)
	if err != nil {
		return false, "", fmt.Errorf("check override: %w", err)
	}
	if overridden {
		return false, "appraisal was overridden by host/admin", nil
	}
	return true, "appraisal was accepted without override", nil
}

// evaluateRisk: for transactions that completed, low-risk passes are correct if
// no incident occurred.
func (s *Service) evaluateRisk(ctx context.Context, d *decision.AgentDecision, transactionID string) (bool, string, error) {
	hasDispute, err := s.repo.HasDisputeForTransaction(ctx, transactionID)
	if err != nil {
		return false, "", fmt.Errorf("check dispute: %w", err)
	}

	// If the decision was to escalate (high risk) and no dispute occurred,
	// the block may have been overly cautious.
	if d.Escalated && !hasDispute {
		return false, "high-risk block but no incident occurred", nil
	}
	// If no escalation (low risk) and a dispute/incident occurred, it was a miss.
	if !d.Escalated && hasDispute {
		return false, "low-risk pass but incident occurred", nil
	}
	return true, "risk assessment matched outcome", nil
}

// evaluateFraud: correct if flagged accounts were confirmed fraudulent.
func (s *Service) evaluateFraud(ctx context.Context, d *decision.AgentDecision) (bool, string, error) {
	if d.UserID == nil {
		return true, "no user associated", nil
	}
	flagged, err := s.repo.IsUserFraudFlagged(ctx, *d.UserID)
	if err != nil {
		return false, "", fmt.Errorf("check fraud: %w", err)
	}
	// For a fraud agent that flagged a user: correct if user is confirmed fraudulent.
	// For a fraud agent that didn't flag: correct if user is not fraudulent.
	if d.Escalated && flagged {
		return true, "flagged user confirmed as fraudulent", nil
	}
	if d.Escalated && !flagged {
		return false, "flagged user was not fraudulent (false positive)", nil
	}
	if !d.Escalated && flagged {
		return false, "unflagged user later confirmed as fraudulent (false negative)", nil
	}
	return true, "correct non-flag on clean user", nil
}

// UpdateCalibrationMetrics recalculates per-confidence-bucket accuracy for an
// agent type using a rolling 90-day window. Results are stored in Redis.
func (s *Service) UpdateCalibrationMetrics(ctx context.Context, agentType string) error {
	for _, b := range CalibrationBuckets {
		total, correct, err := s.repo.GetCalibrationStats(ctx, agentType, b.Low, b.High)
		if err != nil {
			return fmt.Errorf("calibration stats for %s [%.1f-%.1f]: %w", agentType, b.Low, b.High, err)
		}

		bucket := buildCalibrationBucket(agentType, b.Low, b.High, total, correct)
		if err := s.repo.StoreCalibrationBucket(ctx, bucket); err != nil {
			return fmt.Errorf("store bucket: %w", err)
		}
	}

	slog.Info("outcome: calibration metrics updated",
		"agentType", agentType,
	)
	return nil
}

// GetCalibration returns calibration data for all agent types.
func (s *Service) GetCalibration(ctx context.Context) ([]CalibrationReport, error) {
	return s.repo.GetAllAgentCalibration(ctx)
}

// GetCalibrationForAgent returns calibration data for a specific agent type.
func (s *Service) GetCalibrationForAgent(ctx context.Context, agentType string) (*CalibrationReport, error) {
	buckets, err := s.repo.GetCalibrationBuckets(ctx, agentType)
	if err != nil {
		return nil, fmt.Errorf("get buckets: %w", err)
	}

	report := &CalibrationReport{
		AgentType:   agentType,
		Buckets:     buckets,
		GeneratedAt: buckets[0].UpdatedAt,
	}

	var totalCalErr float64
	correctDecisions := 0
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

	return report, nil
}

// GetDecisions returns paginated agent decisions with outcome data.
func (s *Service) GetDecisions(ctx context.Context, agentType string, outcomeFilter *bool, limit, offset int) ([]DecisionWithOutcome, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.repo.GetPaginatedDecisions(ctx, agentType, outcomeFilter, limit, offset)
}
