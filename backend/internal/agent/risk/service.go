package risk

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Brett2thered/RentMy/backend/internal/agent/decision"
	"github.com/Brett2thered/RentMy/backend/internal/platform/ulid"
)

// jsonMarshal is a package-level alias so repository.go can call it without importing encoding/json.
var jsonMarshal = json.Marshal

// decisionService is the subset of decision.Service the RiskAgent uses.
type decisionService interface {
	RecordDecision(ctx context.Context, in decision.CreateDecisionInput) (*decision.AgentDecision, error)
}

// ComputeRiskInput is the public input for ComputeRiskScore.
type ComputeRiskInput struct {
	TransactionID string
	RenterID      string
	HostID        string
	// Optional overrides — when zero, fetched from DB.
	ItemValueCents int64
	ScheduledStart time.Time
}

// Service is the RiskAgent business logic.
type Service struct {
	repo        *Repository
	decisionSvc decisionService
}

// NewService creates a RiskAgent Service.
func NewService(repo *Repository, decisionSvc decisionService) *Service {
	return &Service{repo: repo, decisionSvc: decisionSvc}
}

// ComputeRiskScore computes and persists the per-transaction risk score.
// Returns the score (0-100), the full breakdown, and any error.
// The booking service calls this during CreateBooking and checks Control == ControlBlock.
func (s *Service) ComputeRiskScore(ctx context.Context, in ComputeRiskInput) (*TransactionRiskScore, error) {
	// Fetch renter profile.
	renter, err := s.repo.FindUserProfile(ctx, in.RenterID)
	if err != nil {
		return nil, fmt.Errorf("risk: fetch renter: %w", err)
	}

	// Fetch host profile.
	host, err := s.repo.FindUserProfile(ctx, in.HostID)
	if err != nil {
		return nil, fmt.Errorf("risk: fetch host: %w", err)
	}

	// Fetch transaction details if not provided inline.
	itemValueCents := in.ItemValueCents
	scheduledStart := in.ScheduledStart
	if itemValueCents == 0 && in.TransactionID != "" {
		td, tdErr := s.repo.FindTransactionDetails(ctx, in.TransactionID)
		if tdErr == nil {
			itemValueCents = td.ItemValueCents
			if scheduledStart.IsZero() {
				scheduledStart = td.ScheduledStart
			}
		}
	}

	// Fetch behavioral history for renter.
	renterHistory, err := s.repo.FindBehavioralHistory(ctx, in.RenterID)
	if err != nil {
		slog.Warn("risk: behavioral history unavailable", "userId", in.RenterID, "error", err)
	}

	renterAge := int(time.Since(renter.CreatedAt).Hours() / 24)
	hostAge := int(time.Since(host.CreatedAt).Hours() / 24)

	input := riskScoreInput{
		RenterID:             in.RenterID,
		RenterIdentityStatus: renter.IdentityStatus,
		RenterAccountAgeDays: renterAge,
		RenterReputation:     renter.ReputationScore,
		HostID:               in.HostID,
		HostReputation:       host.ReputationScore,
		HostAccountAgeDays:   hostAge,
		ItemValueCents:       itemValueCents,
		ScheduledStart:       scheduledStart,
		RenterCancellations60d: renterHistory.Cancellations60d,
		RenterDisputes60d:    renterHistory.Disputes60d,
	}

	score, breakdown, signals := computeScore(input)

	result := &TransactionRiskScore{
		TransactionID: in.TransactionID,
		RiskScore:     score,
		RiskLevel:     levelForScore(score),
		Control:       controlForScore(score),
		Breakdown:     breakdown,
		Signals:       signals,
		ComputedAt:    time.Now().UTC(),
	}

	// Persist the risk score if we have a transaction ID.
	if in.TransactionID != "" {
		if err := s.repo.UpsertRiskScore(ctx, result); err != nil {
			slog.Warn("risk: failed to persist score", "transactionId", in.TransactionID, "error", err)
		}
	}

	// Record the agent decision for auditability.
	s.recordDecision(ctx, in, result)

	slog.Info("risk: score computed",
		"transactionId", in.TransactionID,
		"score", score,
		"level", result.RiskLevel,
		"control", result.Control,
	)
	return result, nil
}

// GetRiskScore retrieves the stored risk score for a transaction.
func (s *Service) GetRiskScore(ctx context.Context, transactionID string) (*TransactionRiskScore, error) {
	rs, err := s.repo.FindRiskScore(ctx, transactionID)
	if err != nil {
		return nil, fmt.Errorf("risk: get risk score: %w", err)
	}
	return rs, nil
}

// EmitSignal records a reputation signal and triggers score recalculation.
// Idempotent for one-time signals.
func (s *Service) EmitSignal(ctx context.Context, in EmitSignalInput) error {
	pts := in.Points
	if pts == 0 {
		pts = signalPoints[in.SignalType]
	}

	sig := &ReputationSignal{
		ID:            ulid.New(),
		UserID:        in.UserID,
		SignalType:    in.SignalType,
		Points:        pts,
		TransactionID: in.TransactionID,
		EmittedAt:     time.Now().UTC(),
	}

	if oneTimeSignals[in.SignalType] {
		key := string(in.SignalType)
		sig.IdempotencyKey = &key
	}

	if err := s.repo.InsertSignal(ctx, sig); err != nil {
		return fmt.Errorf("risk: insert signal: %w", err)
	}

	// Recalculate and persist reputation score.
	if _, err := s.RecalculateReputation(ctx, in.UserID); err != nil {
		slog.Warn("risk: reputation recalculation failed after signal", "userId", in.UserID, "error", err)
	}

	return nil
}

// RecalculateReputation computes the reputation score from all signals,
// applies 180-day decay, persists the result, and returns the new score.
func (s *Service) RecalculateReputation(ctx context.Context, userID string) (int, error) {
	signals, err := s.repo.FindSignalsByUserID(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("risk: fetch signals: %w", err)
	}

	now := time.Now().UTC()
	decayCutoff := now.AddDate(0, 0, -decayWindowDays)

	score := 0
	for _, sig := range signals {
		if negativeSignals[sig.SignalType] && sig.EmittedAt.Before(decayCutoff) {
			// 50% decay for negative signals older than 180 days.
			score += sig.Points / 2
		} else {
			score += sig.Points
		}
	}

	score = clamp(0, 1000, score)

	if err := s.repo.SetReputationScore(ctx, userID, score); err != nil {
		return score, fmt.Errorf("risk: set reputation score: %w", err)
	}

	return score, nil
}

// GetReputationSignals returns all reputation signals for a user.
func (s *Service) GetReputationSignals(ctx context.Context, userID string) ([]ReputationSignal, error) {
	signals, err := s.repo.FindSignalsByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("risk: get signals: %w", err)
	}
	return signals, nil
}

// RecalculateHostSignals recomputes the monthly host-specific reputation signals
// (response rate, acceptance rate, zero-cancellation bonus) for a single host.
func (s *Service) RecalculateHostSignals(ctx context.Context, hostID string) error {
	metrics, err := s.repo.FindHostMetrics(ctx, hostID)
	if err != nil {
		return fmt.Errorf("risk: find host metrics: %w", err)
	}

	if metrics.TotalBookings90d == 0 {
		return nil // no data yet
	}

	acceptanceRate := float64(metrics.AcceptedBookings90d) / float64(metrics.TotalBookings90d)

	// High acceptance rate (>= 80%) signal.
	if acceptanceRate >= 0.80 {
		if err := s.EmitSignal(ctx, EmitSignalInput{
			UserID:     hostID,
			SignalType: SignalHighAcceptanceRate,
		}); err != nil {
			slog.Warn("risk: failed to emit high acceptance signal", "hostId", hostID, "error", err)
		}
	} else if acceptanceRate < 0.50 {
		// Low acceptance rate (< 50%) penalty.
		if err := s.EmitSignal(ctx, EmitSignalInput{
			UserID:     hostID,
			SignalType: SignalLowAcceptanceRate,
		}); err != nil {
			slog.Warn("risk: failed to emit low acceptance signal", "hostId", hostID, "error", err)
		}
	}

	// Zero-cancellation bonus.
	if metrics.HostCancellations90d == 0 {
		if err := s.EmitSignal(ctx, EmitSignalInput{
			UserID:     hostID,
			SignalType: SignalZeroCancellations90,
		}); err != nil {
			slog.Warn("risk: failed to emit zero cancellations signal", "hostId", hostID, "error", err)
		}
	}

	return nil
}

// computeScore applies the deterministic risk rules engine from PRD §8.
// Returns the clamped score, the component breakdown, and a list of signal details.
// signals is always non-nil (may be empty).
func computeScore(in riskScoreInput) (int, RiskBreakdown, []RiskSignalDetail) {
	signals := make([]RiskSignalDetail, 0)
	addSignal := func(desc string, weight int, flag bool) {
		signals = append(signals, RiskSignalDetail{Signal: desc, Weight: weight, Flag: flag})
	}

	// --- Base risk ---
	baseRisk := 0

	if in.RenterIdentityStatus == "PENDING" {
		baseRisk += 20
		addSignal("identity_not_verified", 20, true)
	}
	if in.RenterAccountAgeDays < 7 {
		baseRisk += 15
		addSignal("new_account_under_7_days", 15, true)
	}
	if in.RenterReputation < 100 {
		baseRisk += 15
		addSignal("low_reputation_under_100", 15, true)
	} else if in.RenterReputation < 300 {
		baseRisk += 5
		addSignal("low_reputation_under_300", 5, false)
	}

	// --- Transaction risk ---
	txRisk := 0

	const highValue = 100_000  // $1,000 in cents
	const veryHighValue = 300_000 // $3,000 in cents
	if in.ItemValueCents >= veryHighValue {
		txRisk += 25
		addSignal("high_value_item_over_3000", 25, true)
	} else if in.ItemValueCents >= highValue {
		txRisk += 15
		addSignal("high_value_item_over_1000", 15, false)
	}

	// Night booking: 00:00–05:00 local time.
	if !in.ScheduledStart.IsZero() {
		h := in.ScheduledStart.Hour()
		if h >= 0 && h < 5 {
			txRisk += 10
			addSignal("night_booking_0000_0500", 10, false)
		}
	}

	// --- Counterparty risk ---
	counterpartyRisk := 0

	if in.RenterReputation < 50 && in.HostReputation < 50 {
		counterpartyRisk += 30
		addSignal("both_parties_low_reputation", 30, true)
	}
	if in.HostAccountAgeDays < 14 {
		counterpartyRisk += 10
		addSignal("new_host_under_14_days", 10, false)
	}

	// --- Behavioral risk ---
	behavioralRisk := 0

	if in.RenterCancellations60d >= 2 {
		behavioralRisk += 20
		addSignal("multiple_cancellations_60d", 20, true)
	}
	if in.RenterDisputes60d >= 1 {
		behavioralRisk += 25
		addSignal("prior_dispute_60d", 25, true)
	}
	if in.GeoMismatch {
		behavioralRisk += 10
		addSignal("geo_outside_usual_radius", 10, false)
	}

	// --- Fraud signals ---
	fraudRisk := 0

	if in.DeviceFingerprintMatch {
		fraudRisk += 50
		addSignal("device_fingerprint_match", 50, true)
	}
	if in.CompoundNetworkSignal {
		fraudRisk += 30
		addSignal("compound_network_signal", 30, true)
	}
	if in.VelocityFlag {
		fraudRisk += 20
		addSignal("velocity_threshold_exceeded", 20, true)
	}

	total := clamp(0, 100, baseRisk+txRisk+counterpartyRisk+behavioralRisk+fraudRisk)
	breakdown := RiskBreakdown{
		BaseRisk:         baseRisk,
		TransactionRisk:  txRisk,
		CounterpartyRisk: counterpartyRisk,
		BehavioralRisk:   behavioralRisk,
		FraudSignals:     fraudRisk,
		Total:            total,
	}

	return total, breakdown, signals
}

func (s *Service) recordDecision(ctx context.Context, in ComputeRiskInput, result *TransactionRiskScore) {
	if s.decisionSvc == nil {
		return
	}

	conf := 1.0 // deterministic rules, always 100% confident
	txID := in.TransactionID
	var txIDPtr *string
	if txID != "" {
		txIDPtr = &txID
	}

	if _, err := s.decisionSvc.RecordDecision(ctx, decision.CreateDecisionInput{
		AgentType:     decision.AgentTypeRisk,
		TransactionID: txIDPtr,
		Input: map[string]any{
			"renter_id": in.RenterID,
			"host_id":   in.HostID,
		},
		Decision: map[string]any{
			"risk_score": result.RiskScore,
			"risk_level": string(result.RiskLevel),
			"control":    string(result.Control),
			"breakdown":  result.Breakdown,
		},
		Confidence: &conf,
	}); err != nil {
		slog.Warn("risk: failed to record agent decision", "error", err)
	}
}
