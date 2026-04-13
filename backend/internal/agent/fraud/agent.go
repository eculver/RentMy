package fraud

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Brett2thered/RentMy/backend/internal/agent/decision"
	"github.com/Brett2thered/RentMy/backend/internal/platform/ulid"
)

// Agent orchestrates fraud detection: per-transaction signal evaluation and
// scheduled cross-platform pattern scans.
type Agent struct {
	repo        *Repository
	decisionSvc *decision.Service
	threshold   int // total score above which a FraudFlag is created; default 80
}

// New creates a FraudAgent with the default score threshold.
func New(repo *Repository, decisionSvc *decision.Service) *Agent {
	return &Agent{repo: repo, decisionSvc: decisionSvc, threshold: ScoreThreshold}
}

// EvaluateTransaction runs signal detection for both the renter and host
// involved in a transaction.  An AgentDecision is written for each evaluation.
// If a user's score exceeds the threshold, a FraudFlag is created.
func (a *Agent) EvaluateTransaction(ctx context.Context, transactionID string) error {
	txn, err := a.repo.GetTransaction(ctx, transactionID)
	if err != nil {
		return fmt.Errorf("fraud: get transaction %s: %w", transactionID, err)
	}

	for _, userID := range []string{txn.RenterID, txn.HostID} {
		if err := a.EvaluateUser(ctx, userID, &transactionID); err != nil {
			// Log and continue — one user's failure must not block the other.
			slog.Warn("fraud: user evaluation failed", "userId", userID, "transactionId", transactionID, "error", err)
		}
	}
	return nil
}

// EvaluateUser runs the full signal + pattern bundle for a single user and
// creates a FraudFlag when the effective score exceeds the threshold.
// transactionID may be nil for standalone evaluations (e.g., scheduled scans).
func (a *Agent) EvaluateUser(ctx context.Context, userID string, transactionID *string) error {
	bundle := RunAllSignals(ctx, a.repo, userID)
	score := bundle.TotalScore()

	inputJSON, _ := json.Marshal(map[string]any{
		"userId":        userID,
		"transactionId": transactionID,
		"signals":       bundle.Signals,
	})
	decisionJSON, _ := json.Marshal(map[string]any{
		"totalScore":           score,
		"hasNonCompoundSignal": bundle.HasNonCompoundSignal,
		"action":               actionFromScore(score),
	})

	conf := float64(score) / 100.0
	if conf > 1.0 {
		conf = 1.0
	}

	decInput := decision.CreateDecisionInput{
		AgentType:     decision.AgentTypeFraud,
		TransactionID: transactionID,
		UserID:        &userID,
		Input:         json.RawMessage(inputJSON),
		Decision:      json.RawMessage(decisionJSON),
		Confidence:    &conf,
	}

	d, err := a.decisionSvc.RecordDecision(ctx, decInput)
	if err != nil {
		return fmt.Errorf("fraud: record decision for user %s: %w", userID, err)
	}

	action := actionFromScore(score)
	if score < a.threshold && action == ActionMonitor {
		// Below threshold — record decision but skip flag creation.
		slog.Info("fraud: evaluation below threshold", "userId", userID, "score", score)
		return nil
	}

	flag := FraudFlag{
		ID:              ulid.New(),
		UserID:          userID,
		Signals:         bundle.Signals,
		TotalScore:      score,
		Action:          action,
		AgentDecisionID: &d.ID,
		CreatedAt:       time.Now().UTC(),
	}

	if err := a.repo.InsertFraudFlag(ctx, flag); err != nil {
		return fmt.Errorf("fraud: insert flag for user %s: %w", userID, err)
	}

	slog.Info("fraud: flag created",
		"userId", userID,
		"score", score,
		"action", action,
		"flagId", flag.ID,
	)
	return nil
}

// RunScheduledScan runs cross-platform pattern analysis and flags new
// detections.  Called by the River periodic job every 6 hours.
func (a *Agent) RunScheduledScan(ctx context.Context) error {
	slog.Info("fraud: scheduled scan starting")

	signals := RunPatternAnalysis(ctx, a.repo)

	// Group signals by user and evaluate each one.
	byUser := make(map[string][]FraudSignal)
	for _, sig := range signals {
		byUser[sig.UserID] = append(byUser[sig.UserID], sig)
	}

	var flagged int
	for userID, sigs := range byUser {
		score := 0
		hasNonCompound := false
		for _, s := range sigs {
			if !s.IsCompoundOnly {
				hasNonCompound = true
				score += s.Score
			} else if hasNonCompound {
				score += s.Score
			}
		}

		if score < a.threshold {
			continue
		}

		inputJSON, _ := json.Marshal(map[string]any{"userId": userID, "signals": sigs, "source": "scheduled_scan"})
		decisionJSON, _ := json.Marshal(map[string]any{"totalScore": score, "action": actionFromScore(score)})
		conf := float64(score) / 100.0
		if conf > 1.0 {
			conf = 1.0
		}

		d, err := a.decisionSvc.RecordDecision(ctx, decision.CreateDecisionInput{
			AgentType:  decision.AgentTypeFraud,
			UserID:     &userID,
			Input:      json.RawMessage(inputJSON),
			Decision:   json.RawMessage(decisionJSON),
			Confidence: &conf,
		})
		if err != nil {
			slog.Warn("fraud: scheduled scan decision error", "userId", userID, "error", err)
			continue
		}

		action := actionFromScore(score)
		flag := FraudFlag{
			ID:              ulid.New(),
			UserID:          userID,
			Signals:         sigs,
			TotalScore:      score,
			Action:          action,
			AgentDecisionID: &d.ID,
			CreatedAt:       time.Now().UTC(),
		}
		if err := a.repo.InsertFraudFlag(ctx, flag); err != nil {
			slog.Warn("fraud: scheduled scan flag insert error", "userId", userID, "error", err)
			continue
		}
		flagged++
	}

	slog.Info("fraud: scheduled scan complete",
		"patterns_detected", len(signals),
		"users_flagged", flagged,
	)
	return nil
}
