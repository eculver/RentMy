package verification

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/riverqueue/river"

	"github.com/giits/rentmy/backend/internal/platform/ulid"
)

const (
	// maxVerificationRetries is the number of times we retry a stalled session
	// before escalating to human review.
	maxVerificationRetries = 3
	// timeoutReschedule is the delay before re-checking after a retry.
	timeoutReschedule = 10 * time.Minute
)

// VerificationTimeoutJobArgs holds the arguments for the verification timeout job.
type VerificationTimeoutJobArgs struct {
	AttemptID string `json:"attemptId"`
	UserID    string `json:"userId"`
	SessionID string `json:"sessionId"`
}

// Kind returns the job type identifier registered with River.
func (VerificationTimeoutJobArgs) Kind() string { return "verification_timeout" }

// VerificationTimeoutWorker handles verification sessions that stall without
// receiving a Stripe webhook within the expected window.
type VerificationTimeoutWorker struct {
	river.WorkerDefaults[VerificationTimeoutJobArgs]
	repo   attemptRepository
	svc    *Service
}

// NewVerificationTimeoutWorker creates a VerificationTimeoutWorker.
func NewVerificationTimeoutWorker(repo attemptRepository, svc *Service) *VerificationTimeoutWorker {
	return &VerificationTimeoutWorker{repo: repo, svc: svc}
}

// Work checks whether the session is still PENDING.
// - If PENDING and retry count < maxVerificationRetries: creates a new Stripe session and increments retry.
// - If PENDING and retry count >= maxVerificationRetries: escalates to human review.
// - If not PENDING: the webhook arrived; nothing to do.
func (w *VerificationTimeoutWorker) Work(ctx context.Context, job *river.Job[VerificationTimeoutJobArgs]) error {
	args := job.Args
	slog.Info("verification timeout: checking session",
		"attemptId", args.AttemptID, "sessionId", args.SessionID)

	attempt, err := w.repo.FindBySessionID(ctx, args.SessionID)
	if err != nil {
		if errors.Is(err, ErrAttemptNotFound) {
			slog.Warn("verification timeout: attempt not found, skipping", "sessionId", args.SessionID)
			return nil
		}
		return fmt.Errorf("verification timeout: finding attempt: %w", err)
	}

	if attempt.Status != VerificationStatusPending {
		// Webhook already resolved this session — nothing to do.
		slog.Info("verification timeout: session already resolved",
			"status", attempt.Status, "sessionId", args.SessionID)
		return nil
	}

	if attempt.RetryCount >= maxVerificationRetries {
		// Escalate to human review after too many retries.
		reason := "timeout_max_retries"
		in := updateStatusInput{
			Status:           VerificationStatusEscalated,
			StripeStatus:     "timeout",
			FraudIndicators:  []string{},
			EscalationReason: &reason,
		}
		if err := w.repo.UpdateStatus(ctx, attempt.ID, in); err != nil {
			return fmt.Errorf("verification timeout: escalating attempt: %w", err)
		}
		slog.Warn("verification timeout: escalated after max retries",
			"userId", attempt.UserID, "attemptId", attempt.ID)
		return nil
	}

	// Retry: create a new Stripe session for this user.
	newSession, err := w.svc.stripe.CreateVerificationSession(ctx, attempt.UserID)
	if err != nil {
		return fmt.Errorf("verification timeout: creating retry session: %w", err)
	}

	// Mark old attempt as canceled and insert a new one.
	canceledStatus := updateStatusInput{
		Status:          VerificationStatusCanceled,
		StripeStatus:    "timeout_retry",
		FraudIndicators: []string{},
	}
	if err := w.repo.UpdateStatus(ctx, attempt.ID, canceledStatus); err != nil {
		return fmt.Errorf("verification timeout: canceling stale attempt: %w", err)
	}

	now := time.Now().UTC()
	newAttempt := &VerificationAttempt{
		ID:              ulid.New(),
		UserID:          attempt.UserID,
		StripeSessionID: newSession.SessionID,
		Status:          VerificationStatusPending,
		FraudIndicators: []string{},
		RetryCount:      attempt.RetryCount + 1,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if _, err := w.repo.Insert(ctx, newAttempt); err != nil {
		return fmt.Errorf("verification timeout: inserting retry attempt: %w", err)
	}

	// Schedule the next timeout check for the new attempt.
	if w.svc.riverClient == nil {
		slog.Warn("verification timeout: river client nil, skipping reschedule", "attemptId", newAttempt.ID)
		return nil
	}
	if _, err := w.svc.riverClient.Insert(ctx, VerificationTimeoutJobArgs{
		AttemptID: newAttempt.ID,
		UserID:    attempt.UserID,
		SessionID: newSession.SessionID,
	}, &river.InsertOpts{
		ScheduledAt: time.Now().Add(timeoutReschedule),
	}); err != nil {
		slog.Warn("verification timeout: failed to schedule next check",
			"attemptId", newAttempt.ID, "error", err)
	}

	slog.Info("verification timeout: retried session",
		"userId", attempt.UserID,
		"oldSession", args.SessionID,
		"newSession", newSession.SessionID,
		"retryCount", newAttempt.RetryCount,
	)
	return nil
}

