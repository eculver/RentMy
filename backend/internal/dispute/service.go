package dispute

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/giits/rentmy/backend/internal/agent/decision"
	"github.com/giits/rentmy/backend/internal/agent/router"
	"github.com/giits/rentmy/backend/internal/payment"
)

// Config holds tunable dispute parameters.
type Config struct {
	SLAActiveHours     int // SLA for active rental disputes (default 4h)
	SLAPostReturnHours int // SLA for post-return disputes (default 24h)
}

// reputationEnqueuer can schedule an async reputation recalculation for a user.
type reputationEnqueuer interface {
	EnqueueRecalc(ctx context.Context, userID string) error
}

// Service implements the dispute domain business logic.
type Service struct {
	repo        *Repository
	decisionSvc *decision.Service
	holdSvc     *HoldService
	paymentSvc  *payment.Service
	modelRouter *router.AnthropicRouter
	riverClient *river.Client[pgx.Tx]
	reputation  reputationEnqueuer
	cfg         Config
}

// NewService creates a dispute Service with the given dependencies.
func NewService(
	repo *Repository,
	decisionSvc *decision.Service,
	holdSvc *HoldService,
	paymentSvc *payment.Service,
	modelRouter *router.AnthropicRouter,
	riverClient *river.Client[pgx.Tx],
	cfg Config,
) *Service {
	return &Service{
		repo:        repo,
		decisionSvc: decisionSvc,
		holdSvc:     holdSvc,
		paymentSvc:  paymentSvc,
		modelRouter: modelRouter,
		riverClient: riverClient,
		cfg:         cfg,
	}
}

// WithReputation injects the reputation enqueuer dependency.
func (s *Service) WithReputation(r reputationEnqueuer) *Service {
	s.reputation = r
	return s
}

// FileDispute creates a new dispute, transitions the transaction to DISPUTED,
// and enqueues async processing.
func (s *Service) FileDispute(ctx context.Context, in FileDisputeInput) (Dispute, error) {
	existing, err := s.repo.FindOpenByTransactionID(ctx, in.TransactionID)
	if err != nil {
		return Dispute{}, fmt.Errorf("check existing dispute: %w", err)
	}
	if existing != nil {
		return Dispute{}, ErrAlreadyDisputed
	}

	d, err := s.repo.Insert(ctx, Dispute{
		TransactionID: in.TransactionID,
		ReporterID:    in.ReporterID,
		Reason:        in.Reason,
		Description:   in.Description,
	})
	if err != nil {
		return Dispute{}, fmt.Errorf("insert dispute: %w", err)
	}

	if err := s.paymentSvc.UpdateTransactionStatus(ctx, in.TransactionID, "DISPUTED"); err != nil {
		slog.Warn("dispute: failed to update transaction status", "error", err)
	}

	slaHours := s.cfg.SLAPostReturnHours
	if slaHours == 0 {
		slaHours = 24
	}
	deadline := time.Now().Add(time.Duration(slaHours) * time.Hour)
	_ = s.repo.SetSLADeadline(ctx, d.ID, deadline)

	if s.riverClient != nil {
		_, err = s.riverClient.Insert(ctx, DisputeResolutionJobArgs{
			DisputeID:     d.ID,
			TransactionID: in.TransactionID,
		}, nil)
		if err != nil {
			slog.Warn("dispute: failed to enqueue resolution job", "error", err)
		}
	}

	slog.Info("dispute filed",
		"disputeId", d.ID,
		"transactionId", in.TransactionID,
		"reporterId", in.ReporterID,
	)
	return d, nil
}

// GatherEvidence assembles the evidence package from existing tables.
func (s *Service) GatherEvidence(ctx context.Context, transactionID string) (EvidencePackage, error) {
	renterID, hostID, err := s.repo.GetTransactionParties(ctx, transactionID)
	if err != nil {
		return EvidencePackage{}, fmt.Errorf("get parties: %w", err)
	}

	txnRef, agreementJSON, err := s.repo.GatherTransactionEvidence(ctx, transactionID)
	if err != nil {
		return EvidencePackage{}, fmt.Errorf("gather transaction: %w", err)
	}

	checkIn, checkOut, err := s.repo.GatherMediaEvidence(ctx, transactionID)
	if err != nil {
		return EvidencePackage{}, fmt.Errorf("gather media: %w", err)
	}

	msgs, err := s.repo.GatherMessages(ctx, transactionID)
	if err != nil {
		return EvidencePackage{}, fmt.Errorf("gather messages: %w", err)
	}

	proofs, err := s.repo.GatherProximityProofs(ctx, transactionID)
	if err != nil {
		return EvidencePackage{}, fmt.Errorf("gather proximity: %w", err)
	}

	diffResult, diffConf, err := s.repo.GatherPhotoDiff(ctx, transactionID)
	if err != nil {
		return EvidencePackage{}, fmt.Errorf("gather photo diff: %w", err)
	}

	reporterScore, otherScore, err := s.repo.GatherReputationScores(ctx, renterID, hostID)
	if err != nil {
		slog.Warn("dispute: failed to get reputation scores", "error", err)
	}

	hasFraud, err := s.repo.HasFraudFlags(ctx, renterID, hostID)
	if err != nil {
		slog.Warn("dispute: failed to check fraud flags", "error", err)
	}

	return EvidencePackage{
		AgreementSnapshot:  agreementJSON,
		CheckInMedia:       checkIn,
		CheckOutMedia:      checkOut,
		Messages:           msgs,
		ProximityProofs:    proofs,
		PhotoDiffResult:    diffResult,
		PhotoDiffConf:      diffConf,
		TransactionData:    txnRef,
		ReporterReputation: reporterScore,
		OtherReputation:    otherScore,
		HasFraudFlags:      hasFraud,
	}, nil
}

// RunDisputeAgent calls the LLM with the evidence package and returns a structured decision.
func (s *Service) RunDisputeAgent(ctx context.Context, evidence EvidencePackage) (*AgentDecisionOutput, *router.RouteOutput, error) {
	if s.modelRouter == nil {
		return nil, nil, fmt.Errorf("model router not configured")
	}

	userPrompt := buildUserPrompt(evidence)
	output, err := s.modelRouter.Route(ctx, router.RouteInput{
		Task:         router.TaskEvidenceAnalysis,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		MaxTokens:    2048,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("model route: %w", err)
	}

	var agentOutput AgentDecisionOutput
	if err := json.Unmarshal([]byte(output.Content), &agentOutput); err != nil {
		return nil, &output, fmt.Errorf("unmarshal agent output: %w", err)
	}

	return &agentOutput, &output, nil
}

// RouteAndExecute applies the escalation gate to an agent decision and executes
// or queues accordingly.
func (s *Service) RouteAndExecute(ctx context.Context, disputeID string, transactionID string, agentOutput *AgentDecisionOutput, evidence EvidencePackage, modelOutput *router.RouteOutput) error {
	photoDiffResult := ""
	if evidence.PhotoDiffResult != nil {
		photoDiffResult = *evidence.PhotoDiffResult
	}

	route := RouteDecision(
		agentOutput.Confidence,
		agentOutput.ChargeAmount,
		photoDiffResult,
		evidence.HasFraudFlags,
	)

	escalated := route == RouteHumanReview
	var escalationReason *string
	if escalated {
		reason := fmt.Sprintf("route=%s, confidence=%.2f, charge=%d, photoDiff=%s, fraudFlags=%v",
			route, agentOutput.Confidence, agentOutput.ChargeAmount, photoDiffResult, evidence.HasFraudFlags)
		escalationReason = &reason
	}

	model := modelOutput.Model
	pv := promptVersion
	confidence := agentOutput.Confidence
	agentDecision, err := s.decisionSvc.RecordDecision(ctx, decision.CreateDecisionInput{
		AgentType:        decision.AgentTypeDispute,
		TransactionID:    &transactionID,
		Input:            evidence,
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

	evidenceJSON, _ := json.Marshal(evidence)
	if err := s.repo.UpdateDecision(ctx, disputeID, route, agentOutput.ChargeAmount, agentOutput.Confidence, agentDecision.ID, evidenceJSON); err != nil {
		return fmt.Errorf("update dispute decision: %w", err)
	}

	switch route {
	case RouteAutoResolve:
		return s.executeDecision(ctx, disputeID, transactionID, agentOutput)
	case RouteAutoResolveAudit:
		if err := s.executeDecision(ctx, disputeID, transactionID, agentOutput); err != nil {
			return err
		}
		return s.repo.UpdateStatus(ctx, disputeID, StatusAuditQueued)
	case RouteHumanReview:
		return s.repo.UpdateStatus(ctx, disputeID, StatusHumanReview)
	default:
		return fmt.Errorf("unknown route: %s", route)
	}
}

// executeDecision captures from hold or releases based on the agent's verdict.
func (s *Service) executeDecision(ctx context.Context, disputeID string, transactionID string, agentOutput *AgentDecisionOutput) error {
	if agentOutput.ChargeAmount == 0 {
		if err := s.holdSvc.ReleaseRemaining(ctx, transactionID); err != nil {
			slog.Warn("dispute: failed to release hold", "error", err)
		}
		if err := s.repo.UpdateStatus(ctx, disputeID, StatusAutoResolved); err != nil {
			return fmt.Errorf("update status: %w", err)
		}
		s.enqueueReputationRecalc(ctx, transactionID)
		return nil
	}

	txn, err := s.paymentSvc.GetTransaction(ctx, transactionID)
	if err != nil {
		return fmt.Errorf("get transaction: %w", err)
	}

	if agentOutput.ChargeAmount <= txn.HoldAllocation.Remaining {
		if _, err := s.holdSvc.CaptureForDamage(ctx, transactionID, agentOutput.ChargeAmount); err != nil {
			return fmt.Errorf("capture for damage: %w", err)
		}
	} else {
		if err := s.holdSvc.CaptureAndEscalate(ctx, transactionID, agentOutput.ChargeAmount); err != nil {
			return fmt.Errorf("capture and escalate: %w", err)
		}
	}

	if err := s.repo.UpdateStatus(ctx, disputeID, StatusAutoResolved); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	// Schedule authoritative reputation recalculation for both parties.
	s.enqueueReputationRecalcForParties(ctx, txn.RenterID, txn.HostID)
	return nil
}

// enqueueReputationRecalc fetches the transaction parties and enqueues recalc for both.
func (s *Service) enqueueReputationRecalc(ctx context.Context, transactionID string) {
	if s.reputation == nil {
		return
	}
	renterID, hostID, err := s.repo.GetTransactionParties(ctx, transactionID)
	if err != nil {
		slog.Warn("dispute: failed to get parties for reputation recalc", "error", err)
		return
	}
	s.enqueueReputationRecalcForParties(ctx, renterID, hostID)
}

// enqueueReputationRecalcForParties enqueues reputation recalc jobs for both
// the renter and the host.  Errors are logged but do not fail the caller.
func (s *Service) enqueueReputationRecalcForParties(ctx context.Context, renterID, hostID string) {
	if s.reputation == nil {
		return
	}
	for _, userID := range []string{renterID, hostID} {
		if err := s.reputation.EnqueueRecalc(ctx, userID); err != nil {
			slog.Warn("dispute: failed to enqueue reputation recalc",
				"userId", userID, "error", err)
		}
	}
}

// ProcessDispute runs the full dispute resolution pipeline: gather evidence, run agent, route & execute.
func (s *Service) ProcessDispute(ctx context.Context, disputeID, transactionID string) error {
	if err := s.repo.UpdateStatus(ctx, disputeID, StatusGathering); err != nil {
		return fmt.Errorf("set gathering status: %w", err)
	}

	evidence, err := s.GatherEvidence(ctx, transactionID)
	if err != nil {
		return fmt.Errorf("gather evidence: %w", err)
	}

	if err := s.repo.UpdateStatus(ctx, disputeID, StatusAnalyzing); err != nil {
		return fmt.Errorf("set analyzing status: %w", err)
	}

	agentOutput, modelOutput, err := s.RunDisputeAgent(ctx, evidence)
	if err != nil {
		slog.Warn("dispute: agent call failed, escalating to human review",
			"disputeId", disputeID,
			"error", err,
		)
		return s.repo.UpdateStatus(ctx, disputeID, StatusHumanReview)
	}

	return s.RouteAndExecute(ctx, disputeID, transactionID, agentOutput, evidence, modelOutput)
}

// ResolveByHuman handles a human reviewer's action on a dispute.
func (s *Service) ResolveByHuman(ctx context.Context, in ResolveInput) error {
	d, err := s.repo.FindByID(ctx, in.DisputeID)
	if err != nil {
		return fmt.Errorf("find dispute: %w", err)
	}

	if d.Status != StatusHumanReview {
		return ErrInvalidStatus
	}

	chargeAmount := d.ChargeAmount
	if in.Action == "OVERRIDE" && in.ChargeAmount != nil {
		chargeAmount = *in.ChargeAmount
	}

	overrideOf := d.AgentDecisionID
	confidence := 1.0
	model := "HUMAN"
	pv := "human_review"
	_, err = s.decisionSvc.RecordDecision(ctx, decision.CreateDecisionInput{
		AgentType:     decision.AgentTypeHumanOverride,
		TransactionID: &d.TransactionID,
		Input:         d.Evidence,
		Decision: map[string]interface{}{
			"action":       in.Action,
			"chargeAmount": chargeAmount,
			"notes":        in.Notes,
		},
		Model:         &model,
		PromptVersion: &pv,
		Confidence:    &confidence,
		OverrideOf:    overrideOf,
	})
	if err != nil {
		return fmt.Errorf("record human decision: %w", err)
	}

	if chargeAmount > 0 {
		txn, err := s.paymentSvc.GetTransaction(ctx, d.TransactionID)
		if err != nil {
			return fmt.Errorf("get transaction: %w", err)
		}
		if chargeAmount <= txn.HoldAllocation.Remaining {
			if _, err := s.holdSvc.CaptureForDamage(ctx, d.TransactionID, chargeAmount); err != nil {
				return fmt.Errorf("capture for damage: %w", err)
			}
		} else {
			if err := s.holdSvc.CaptureAndEscalate(ctx, d.TransactionID, chargeAmount); err != nil {
				return fmt.Errorf("capture and escalate: %w", err)
			}
		}
	} else {
		if err := s.holdSvc.ReleaseRemaining(ctx, d.TransactionID); err != nil {
			slog.Warn("dispute: failed to release hold after human review", "error", err)
		}
	}

	if err := s.repo.UpdateReview(ctx, in.DisputeID, StatusResolved, in.ReviewerID, in.Notes, in.ChargeAmount); err != nil {
		return fmt.Errorf("update review: %w", err)
	}

	return nil
}

// GetDispute returns a dispute by ID.
func (s *Service) GetDispute(ctx context.Context, id string) (Dispute, error) {
	return s.repo.FindByID(ctx, id)
}

// GetDisputesByTransaction returns all disputes for a transaction.
func (s *Service) GetDisputesByTransaction(ctx context.Context, transactionID string) ([]Dispute, error) {
	return s.repo.FindByTransactionID(ctx, transactionID)
}

// GetReviewQueue returns paginated disputes awaiting human review.
func (s *Service) GetReviewQueue(ctx context.Context, limit, offset int) ([]Dispute, error) {
	return s.repo.FindPendingReview(ctx, limit, offset)
}

