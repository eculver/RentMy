package rating

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/giits/rentmy/backend/internal/agent/risk"
	"github.com/giits/rentmy/backend/internal/platform/ulid"
)

// riskAgent is the subset of risk.Service that the rating service requires.
type riskAgent interface {
	EmitSignal(ctx context.Context, in risk.EmitSignalInput) error
}

// reputationEnqueuer can schedule an async reputation recalculation for a user.
type reputationEnqueuer interface {
	EnqueueRecalc(ctx context.Context, userID string) error
}

// Service contains the rating business logic.
type Service struct {
	repo       *Repository
	riskAgent  riskAgent
	reputation reputationEnqueuer
}

// NewService creates a new rating Service.
func NewService(repo *Repository, riskAgent riskAgent) *Service {
	return &Service{repo: repo, riskAgent: riskAgent}
}

// WithReputation injects the reputation enqueuer dependency.
func (s *Service) WithReputation(r reputationEnqueuer) *Service {
	s.reputation = r
	return s
}

// SubmitRating submits a rating from fromUserID for the given transaction.
//
// Validation:
//   - Transaction must be COMPLETED.
//   - fromUserID must be the renter or the host.
//   - Bubbles must be from the correct set for the submitter's role.
//   - A user may only rate a transaction once (enforced by UNIQUE constraint).
//
// On success, a positive_rating signal is emitted for each bubble, triggering
// reputation score recalculation for the rated user.
func (s *Service) SubmitRating(ctx context.Context, in CreateRatingInput) (*Rating, error) {
	if len(in.Bubbles) == 0 {
		return nil, fmt.Errorf("rating: at least one bubble is required")
	}

	txn, err := s.repo.FindTransactionForRating(ctx, in.TransactionID)
	if err != nil {
		return nil, err
	}

	if txn.Status != "COMPLETED" {
		return nil, ErrTransactionNotCompleted
	}

	// Determine the rated user and validate bubbles based on the submitter's role.
	var toUserID string
	switch in.FromUserID {
	case txn.RenterID:
		if err := ValidateBubblesForRenter(in.Bubbles); err != nil {
			return nil, err
		}
		toUserID = txn.HostID
	case txn.HostID:
		if err := ValidateBubblesForHost(in.Bubbles); err != nil {
			return nil, err
		}
		toUserID = txn.RenterID
	default:
		return nil, ErrNotParticipant
	}

	rt := &Rating{
		ID:            ulid.New(),
		TransactionID: in.TransactionID,
		FromUserID:    in.FromUserID,
		ToUserID:      toUserID,
		Bubbles:       in.Bubbles,
	}

	if err := s.repo.Insert(ctx, rt); err != nil {
		return nil, err
	}

	// Emit one positive_rating signal per bubble for the incremental signal log.
	txnID := in.TransactionID
	for range in.Bubbles {
		if err := s.riskAgent.EmitSignal(ctx, risk.EmitSignalInput{
			UserID:        toUserID,
			SignalType:    risk.SignalPositiveRating,
			TransactionID: &txnID,
		}); err != nil {
			// Non-fatal: signal log is best-effort.
			_ = err
		}
	}

	// Schedule an authoritative source-based reputation recalculation for the
	// rated user.  Non-fatal: the signal log above already updated the score.
	if s.reputation != nil {
		if err := s.reputation.EnqueueRecalc(ctx, toUserID); err != nil {
			slog.Warn("rating: failed to enqueue reputation recalc",
				"userId", toUserID, "error", err)
		}
	}

	return rt, nil
}

// GetRatingsForTransaction returns all ratings for a transaction.
func (s *Service) GetRatingsForTransaction(ctx context.Context, txnID string) ([]Rating, error) {
	ratings, err := s.repo.FindByTransactionID(ctx, txnID)
	if err != nil {
		return nil, fmt.Errorf("rating: get ratings for transaction: %w", err)
	}
	return ratings, nil
}

// GetRatingsForUser returns paginated ratings received by a user.
func (s *Service) GetRatingsForUser(ctx context.Context, userID string, page int) ([]Rating, int, error) {
	const pageSize = 20
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize
	ratings, total, err := s.repo.FindByToUserID(ctx, userID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("rating: get ratings for user: %w", err)
	}
	return ratings, total, nil
}

// GetRatingBubbleSummary returns aggregated bubble counts for a user's received ratings.
func (s *Service) GetRatingBubbleSummary(ctx context.Context, userID string) ([]BubbleSummaryItem, error) {
	items, err := s.repo.BubbleSummary(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("rating: get bubble summary: %w", err)
	}
	return items, nil
}

