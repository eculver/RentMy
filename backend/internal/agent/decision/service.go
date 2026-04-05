package decision

import (
	"context"
	"fmt"
	"log/slog"
)

// Service provides the business logic for recording and linking agent decisions.
type Service struct {
	repo *Repository
}

// NewService creates a new decision Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// RecordDecision persists a new agent decision and logs it.
// Every agent calls this after making a model call (or a deterministic decision).
func (s *Service) RecordDecision(ctx context.Context, in CreateDecisionInput) (*AgentDecision, error) {
	d, err := s.repo.Insert(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("decision service: recording: %w", err)
	}
	slog.Info("agent decision recorded",
		"id", d.ID,
		"agent_type", d.AgentType,
		"model", d.Model,
		"prompt_version", d.PromptVersion,
		"escalated", d.Escalated,
	)
	return d, nil
}

// LinkOutcome sets the outcome on an existing decision. Called by the learning
// loop River job 48 hours after a transaction closes.
func (s *Service) LinkOutcome(ctx context.Context, in UpdateOutcomeInput) error {
	if err := s.repo.UpdateOutcome(ctx, in); err != nil {
		return fmt.Errorf("decision service: linking outcome: %w", err)
	}
	slog.Info("agent decision outcome linked",
		"decision_id", in.DecisionID,
		"outcome_id", in.OutcomeID,
		"outcome_correct", in.OutcomeCorrect,
	)
	return nil
}

// FindByTransactionID returns all decisions for a transaction.
func (s *Service) FindByTransactionID(ctx context.Context, transactionID string) ([]*AgentDecision, error) {
	return s.repo.FindByTransactionID(ctx, transactionID)
}

// FindByUserID returns all decisions for a user.
func (s *Service) FindByUserID(ctx context.Context, userID string) ([]*AgentDecision, error) {
	return s.repo.FindByUserID(ctx, userID)
}
