package latereturn

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/Brett2thered/RentMy/backend/internal/agent/decision"
	"github.com/Brett2thered/RentMy/backend/internal/agent/router"
	"github.com/Brett2thered/RentMy/backend/internal/payment"
)

// timeNow is a seam for testing.
var timeNow = time.Now

// Config holds tunable late return parameters.
type Config struct {
	EscalationThresholdHours int // hours overdue before escalation evaluation (default 4)
	DamageReserveRateBPS     int // BPS of hold reserved for damage (default 4000 = 40%)
	ReCheckIntervalMinutes   int // how often to re-check active late returns (default 60)
}

// Service implements the late return domain business logic.
type Service struct {
	repo        *Repository
	decisionSvc *decision.Service
	paymentSvc  *payment.Service
	modelRouter *router.AnthropicRouter
	riverClient *river.Client[pgx.Tx]
	cfg         Config
}

// NewService creates a late return Service with the given dependencies.
func NewService(
	repo *Repository,
	decisionSvc *decision.Service,
	paymentSvc *payment.Service,
	modelRouter *router.AnthropicRouter,
	riverClient *river.Client[pgx.Tx],
	cfg Config,
) *Service {
	if cfg.EscalationThresholdHours == 0 {
		cfg.EscalationThresholdHours = 4
	}
	if cfg.DamageReserveRateBPS == 0 {
		cfg.DamageReserveRateBPS = 4000
	}
	if cfg.ReCheckIntervalMinutes == 0 {
		cfg.ReCheckIntervalMinutes = 60
	}
	return &Service{
		repo:        repo,
		decisionSvc: decisionSvc,
		paymentSvc:  paymentSvc,
		modelRouter: modelRouter,
		riverClient: riverClient,
		cfg:         cfg,
	}
}

// CheckAndCharge is the core late return logic. It verifies the rental is still
// ACTIVE, calculates late duration, computes the late fee, captures from hold
// (respecting the damage reserve cap), and logs an AgentDecision.
func (s *Service) CheckAndCharge(ctx context.Context, transactionID string) error {
	renterID, hostID, scheduledEndRaw, rentalFee, holdAmount, _, holdAllocJSON, txnStatus, err := s.repo.GetTransactionDetails(ctx, transactionID)
	if err != nil {
		return fmt.Errorf("get transaction: %w", err)
	}

	if txnStatus != "ACTIVE" {
		slog.Info("latereturn: rental not active, skipping",
			"transactionId", transactionID,
			"status", txnStatus,
		)
		return nil
	}

	scheduledEnd, ok := scheduledEndRaw.(time.Time)
	if !ok {
		return fmt.Errorf("invalid scheduled_end type for transaction %s", transactionID)
	}

	now := timeNow()
	if now.Before(scheduledEnd) {
		return nil // not yet late
	}

	lateMinutes := int(now.Sub(scheduledEnd).Minutes())
	if lateMinutes < 1 {
		return nil
	}

	// Parse hold allocation to get remaining.
	var holdAlloc payment.HoldAllocation
	if holdAllocJSON != nil {
		if err := json.Unmarshal(holdAllocJSON, &holdAlloc); err != nil {
			return fmt.Errorf("unmarshal hold allocation: %w", err)
		}
	} else {
		holdAlloc = payment.HoldAllocation{
			TotalAuthorized: holdAmount,
			Remaining:       holdAmount,
		}
	}

	// Calculate hourly rate from rental fee. Minimum 1 hour basis.
	durationHours := scheduledEnd.Sub(scheduledEnd.Add(-time.Duration(rentalFee))).Hours()
	_ = durationHours // placeholder — we compute hourly rate from rental fee / scheduled duration
	hourlyRate := computeHourlyRate(rentalFee, scheduledEnd, scheduledEnd) // will be overridden below

	// Re-derive scheduled_start to compute hourly rate properly.
	hourlyRate = s.computeHourlyRateFromFee(ctx, transactionID, rentalFee)

	// Double rate if there's a conflicting booking.
	hasConflict, err := s.repo.HasConflictingBooking(ctx, transactionID)
	if err != nil {
		slog.Warn("latereturn: failed to check conflicting booking", "error", err)
	}
	if hasConflict {
		hourlyRate *= 2
	}

	// Calculate fee for this charging period.
	lateHours := float64(lateMinutes) / 60.0
	totalLateFee := int64(lateHours * float64(hourlyRate))
	if totalLateFee < 1 {
		totalLateFee = 1 // minimum 1 cent
	}

	// Enforce damage reserve cap: late fees cannot exceed holdAmount * (1 - damageReserveRate).
	maxLateFee := s.maxLateFeeCap(holdAmount)
	if totalLateFee > maxLateFee {
		totalLateFee = maxLateFee
	}

	// Calculate incremental capture: total desired minus what's already been captured.
	existingLR, err := s.repo.FindByTransactionID(ctx, transactionID)
	if err != nil {
		return fmt.Errorf("find existing late return: %w", err)
	}

	var lrID string
	var previousFee int64
	if existingLR != nil {
		lrID = existingLR.ID
		previousFee = existingLR.TotalFeeCharged
	} else {
		// Create a new late return record.
		lr, err := s.repo.Insert(ctx, LateReturn{
			TransactionID: transactionID,
			RenterID:      renterID,
			HostID:        hostID,
			ScheduledEnd:  scheduledEnd,
		})
		if err != nil {
			return fmt.Errorf("insert late return: %w", err)
		}
		lrID = lr.ID
	}

	incrementalFee := totalLateFee - previousFee
	if incrementalFee <= 0 {
		// Already charged up to the cap — just update minutes.
		if err := s.repo.RecordCharge(ctx, lrID, totalLateFee, lateMinutes); err != nil {
			return fmt.Errorf("record charge: %w", err)
		}
		return nil
	}

	// Ensure we don't exceed remaining hold.
	if incrementalFee > holdAlloc.Remaining {
		incrementalFee = holdAlloc.Remaining
	}
	if incrementalFee <= 0 {
		slog.Warn("latereturn: no remaining hold to capture",
			"transactionId", transactionID,
		)
		return s.repo.RecordCharge(ctx, lrID, previousFee, lateMinutes)
	}

	// Capture from hold.
	chargeID, err := s.paymentSvc.CaptureFromHold(ctx, transactionID, incrementalFee, payment.CaptureReasonLateFee)
	if err != nil {
		return fmt.Errorf("capture late fee: %w", err)
	}

	actualFeeCharged := previousFee + incrementalFee
	slog.Info("latereturn: charged late fee",
		"transactionId", transactionID,
		"incrementalFee", incrementalFee,
		"totalFee", actualFeeCharged,
		"chargeId", chargeID,
		"lateMinutes", lateMinutes,
	)

	if err := s.repo.RecordCharge(ctx, lrID, actualFeeCharged, lateMinutes); err != nil {
		return fmt.Errorf("record charge: %w", err)
	}

	// Schedule re-check if still within escalation threshold.
	if s.riverClient != nil {
		reCheckAt := now.Add(time.Duration(s.cfg.ReCheckIntervalMinutes) * time.Minute)
		_, err := s.riverClient.Insert(ctx, LateReturnCheckJobArgs{
			TransactionID: transactionID,
		}, &river.InsertOpts{
			ScheduledAt: reCheckAt,
		})
		if err != nil {
			slog.Warn("latereturn: failed to schedule re-check", "error", err)
		}
	}

	// If past escalation threshold, schedule escalation evaluation.
	hoursOverdue := float64(lateMinutes) / 60.0
	if hoursOverdue >= float64(s.cfg.EscalationThresholdHours) {
		if s.riverClient != nil {
			_, err := s.riverClient.Insert(ctx, LateReturnEscalationJobArgs{
				TransactionID: transactionID,
			}, nil)
			if err != nil {
				slog.Warn("latereturn: failed to schedule escalation", "error", err)
			}
		}
	}

	return nil
}

// EvaluateEscalation runs the LLM to decide whether to escalate a late return.
func (s *Service) EvaluateEscalation(ctx context.Context, transactionID string) error {
	renterID, _, scheduledEndRaw, _, holdAmount, itemValue, holdAllocJSON, txnStatus, err := s.repo.GetTransactionDetails(ctx, transactionID)
	if err != nil {
		return fmt.Errorf("get transaction: %w", err)
	}

	if txnStatus != "ACTIVE" {
		slog.Info("latereturn: rental not active, skipping escalation",
			"transactionId", transactionID,
		)
		return nil
	}

	scheduledEnd, ok := scheduledEndRaw.(time.Time)
	if !ok {
		return fmt.Errorf("invalid scheduled_end type")
	}

	now := timeNow()
	lateMinutes := int(now.Sub(scheduledEnd).Minutes())

	var holdAlloc payment.HoldAllocation
	if holdAllocJSON != nil {
		_ = json.Unmarshal(holdAllocJSON, &holdAlloc)
	}

	existingLR, err := s.repo.FindByTransactionID(ctx, transactionID)
	if err != nil {
		return fmt.Errorf("find late return: %w", err)
	}
	if existingLR == nil {
		return fmt.Errorf("no late return record for transaction %s", transactionID)
	}

	// Skip if already escalated.
	if existingLR.Status == StatusEscalatedToDispute || existingLR.Status == StatusFlaggedForReview || existingLR.Status == StatusResolved {
		return nil
	}

	renterReputation, _ := s.repo.GetRenterReputationScore(ctx, renterID)
	renterMsgCount, _ := s.repo.CountRecentMessages(ctx, transactionID, renterID, 2)
	hasConflict, _ := s.repo.HasConflictingBooking(ctx, transactionID)

	input := LateReturnInput{
		TransactionID:         transactionID,
		RenterID:              renterID,
		HostID:                existingLR.HostID,
		MinutesOverdue:        lateMinutes,
		HoursOverdue:          float64(lateMinutes) / 60.0,
		ItemValue:             itemValue,
		HoldAmount:            holdAmount,
		HoldRemaining:         holdAlloc.Remaining,
		TotalLateFeesSoFar:    existingLR.TotalFeeCharged,
		RenterReputation:      renterReputation,
		RenterMessageCount:    renterMsgCount,
		HasConflictingBooking: hasConflict,
		TimeOfDay:             now.Format("15:04 MST"),
	}

	// Call LLM for escalation decision.
	agentOutput, modelOutput, err := s.runEscalationAgent(ctx, input)
	if err != nil {
		slog.Warn("latereturn: agent call failed, defaulting to WARNING",
			"transactionId", transactionID,
			"error", err,
		)
		agentOutput = &EscalationDecisionOutput{
			EscalationLevel: EscalationWarning,
			Confidence:      0.0,
			Reasoning:       "Agent call failed, defaulting to WARNING",
		}
		modelOutput = nil
	}

	// Record agent decision.
	escalated := agentOutput.EscalationLevel == EscalationEscalateToDispute ||
		agentOutput.EscalationLevel == EscalationFlagForReview
	var escalationReason *string
	if escalated {
		reason := fmt.Sprintf("level=%s, confidence=%.2f, hoursOverdue=%.1f, renterMsgs=%d",
			agentOutput.EscalationLevel, agentOutput.Confidence, input.HoursOverdue, renterMsgCount)
		escalationReason = &reason
	}

	var model string
	var pv string
	if modelOutput != nil {
		model = modelOutput.Model
		pv = promptVersion
	} else {
		model = "FALLBACK"
		pv = "fallback"
	}
	confidence := agentOutput.Confidence
	agentDecision, err := s.decisionSvc.RecordDecision(ctx, decision.CreateDecisionInput{
		AgentType:        decision.AgentTypeLateReturn,
		TransactionID:    &transactionID,
		Input:            input,
		Decision:         agentOutput,
		Model:            &model,
		PromptVersion:    &pv,
		Confidence:       &confidence,
		Escalated:        escalated,
		EscalationReason: escalationReason,
	})
	if err != nil {
		return fmt.Errorf("record decision: %w", err)
	}

	if err := s.repo.RecordEscalation(ctx, existingLR.ID, agentOutput.EscalationLevel, agentOutput.Confidence, agentDecision.ID); err != nil {
		return fmt.Errorf("record escalation: %w", err)
	}

	// Execute escalation.
	switch agentOutput.EscalationLevel {
	case EscalationEscalateToDispute:
		return s.escalateToDispute(ctx, transactionID, existingLR, holdAmount)
	case EscalationFlagForReview:
		slog.Error("latereturn: FLAGGED FOR REVIEW — potential theft",
			"transactionId", transactionID,
			"hoursOverdue", input.HoursOverdue,
			"renterMsgCount", renterMsgCount,
		)
		return nil
	default:
		// CHARGING or WARNING — schedule another check.
		if s.riverClient != nil {
			reCheckAt := timeNow().Add(time.Duration(s.cfg.ReCheckIntervalMinutes) * time.Minute)
			_, _ = s.riverClient.Insert(ctx, LateReturnEscalationJobArgs{
				TransactionID: transactionID,
			}, &river.InsertOpts{
				ScheduledAt: reCheckAt,
			})
		}
		return nil
	}
}

// escalateToDispute captures remaining hold (minus damage reserve) and notifies host.
func (s *Service) escalateToDispute(ctx context.Context, transactionID string, lr *LateReturn, holdAmount int64) error {
	maxCapture := s.maxLateFeeCap(holdAmount)
	alreadyCaptured := lr.TotalFeeCharged
	remaining := maxCapture - alreadyCaptured

	if remaining > 0 {
		_, err := s.paymentSvc.CaptureFromHold(ctx, transactionID, remaining, payment.CaptureReasonLateFee)
		if err != nil {
			slog.Warn("latereturn: failed to capture remaining for escalation", "error", err)
		}
	}

	// Update transaction status to DISPUTED so DisputeAgent can take over.
	if err := s.paymentSvc.UpdateTransactionStatus(ctx, transactionID, "DISPUTED"); err != nil {
		slog.Warn("latereturn: failed to update transaction to DISPUTED", "error", err)
	}

	slog.Info("latereturn: escalated to dispute",
		"transactionId", transactionID,
		"lateReturnId", lr.ID,
	)
	return nil
}

// runEscalationAgent calls the LLM for an escalation decision.
func (s *Service) runEscalationAgent(ctx context.Context, input LateReturnInput) (*EscalationDecisionOutput, *router.RouteOutput, error) {
	if s.modelRouter == nil {
		return nil, nil, fmt.Errorf("model router not configured")
	}

	userPrompt := buildUserPrompt(input)
	output, err := s.modelRouter.Route(ctx, router.RouteInput{
		Task:         router.TaskEscalationDecision,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		MaxTokens:    1024,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("model route: %w", err)
	}

	var agentOutput EscalationDecisionOutput
	if err := json.Unmarshal([]byte(output.Content), &agentOutput); err != nil {
		return nil, &output, fmt.Errorf("unmarshal escalation output: %w", err)
	}

	return &agentOutput, &output, nil
}

// maxLateFeeCap returns the maximum amount that can be captured for late fees,
// respecting the damage reserve. Formula: holdAmount * (1 - damageReserveRate).
func (s *Service) maxLateFeeCap(holdAmount int64) int64 {
	reserveRateBPS := s.cfg.DamageReserveRateBPS
	if reserveRateBPS <= 0 || reserveRateBPS >= 10000 {
		reserveRateBPS = 4000 // default 40%
	}
	return holdAmount * int64(10000-reserveRateBPS) / 10000
}

// computeHourlyRateFromFee derives the hourly late fee rate from the rental fee
// and scheduled duration.
func (s *Service) computeHourlyRateFromFee(ctx context.Context, transactionID string, rentalFee int64) int64 {
	var scheduledStart, scheduledEnd time.Time
	_ = s.repo.pool.QueryRow(ctx,
		`SELECT scheduled_start, scheduled_end FROM transactions WHERE id = $1`,
		transactionID,
	).Scan(&scheduledStart, &scheduledEnd)

	return computeHourlyRate(rentalFee, scheduledStart, scheduledEnd)
}

// computeHourlyRate derives the hourly rate from the total rental fee and scheduled duration.
func computeHourlyRate(rentalFee int64, scheduledStart, scheduledEnd time.Time) int64 {
	duration := scheduledEnd.Sub(scheduledStart)
	if duration <= 0 {
		duration = time.Hour // fallback: assume 1 hour minimum
	}
	hours := duration.Hours()
	if hours < 1 {
		hours = 1
	}
	rate := int64(float64(rentalFee) / hours)
	if rate < 100 { // minimum $1/hour
		rate = 100
	}
	return rate
}

// GetActiveLateReturns returns paginated active late returns (for admin).
func (s *Service) GetActiveLateReturns(ctx context.Context, limit, offset int) ([]LateReturn, error) {
	return s.repo.FindActive(ctx, limit, offset)
}

// GetLateReturn returns a late return by ID.
func (s *Service) GetLateReturn(ctx context.Context, id string) (LateReturn, error) {
	return s.repo.FindByID(ctx, id)
}
