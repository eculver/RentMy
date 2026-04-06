package reputation

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/riverqueue/river"
)

// ReputationRecalcJobArgs is the payload for an event-triggered reputation
// recalculation.  It is enqueued by the rating, dispute, and booking services
// after any event that affects a user's reputation.
type ReputationRecalcJobArgs struct {
	UserID string `json:"userId"`
}

// Kind implements river.JobArgs.
func (ReputationRecalcJobArgs) Kind() string { return "reputation_recalc" }

// ReputationRecalcWorker processes a single-user reputation recalculation.
type ReputationRecalcWorker struct {
	river.WorkerDefaults[ReputationRecalcJobArgs]
	svc *Service
}

// NewReputationRecalcWorker creates the worker.
func NewReputationRecalcWorker(svc *Service) *ReputationRecalcWorker {
	return &ReputationRecalcWorker{svc: svc}
}

// Work runs the full source-based reputation recalculation for the user.
func (w *ReputationRecalcWorker) Work(ctx context.Context, job *river.Job[ReputationRecalcJobArgs]) error {
	slog.Info("reputation: recalc job started", "userId", job.Args.UserID)
	if _, err := w.svc.Recalculate(ctx, job.Args.UserID); err != nil {
		return fmt.Errorf("reputation: recalc job: %w", err)
	}
	return nil
}

// MonthlyHostReputationJobArgs is the payload for the monthly host-reputation job.
// It has no fields; the worker iterates all host IDs itself.
type MonthlyHostReputationJobArgs struct{}

// Kind implements river.JobArgs.
func (MonthlyHostReputationJobArgs) Kind() string { return "reputation_monthly_host" }

// MonthlyHostReputationWorker recalculates host-specific signals for every host.
// It is scheduled via River's periodic job support (1st of each month).
type MonthlyHostReputationWorker struct {
	river.WorkerDefaults[MonthlyHostReputationJobArgs]
	svc *Service
}

// NewMonthlyHostReputationWorker creates the worker.
func NewMonthlyHostReputationWorker(svc *Service) *MonthlyHostReputationWorker {
	return &MonthlyHostReputationWorker{svc: svc}
}

// Work iterates all hosts and applies host-specific reputation signals.
func (w *MonthlyHostReputationWorker) Work(ctx context.Context, _ *river.Job[MonthlyHostReputationJobArgs]) error {
	slog.Info("reputation: monthly host job started")

	hostIDs, err := w.svc.repo.FindAllHostIDs(ctx)
	if err != nil {
		return fmt.Errorf("reputation: monthly host: fetch ids: %w", err)
	}

	var errs int
	for _, hostID := range hostIDs {
		if err := w.svc.RecalculateHostSignals(ctx, hostID); err != nil {
			slog.Warn("reputation: monthly host: recalc failed", "hostId", hostID, "error", err)
			errs++
		}
	}

	slog.Info("reputation: monthly host job complete",
		"hosts_processed", len(hostIDs), "errors", errs)
	return nil
}

// NegativeDecayJobArgs is the payload for the daily negative-signal decay job.
type NegativeDecayJobArgs struct{}

// Kind implements river.JobArgs.
func (NegativeDecayJobArgs) Kind() string { return "reputation_negative_decay" }

// NegativeDecayWorker finds users whose negative events have crossed the 180-day
// decay boundary and recalculates their reputation scores.
// It is scheduled via River's periodic job support (daily).
type NegativeDecayWorker struct {
	river.WorkerDefaults[NegativeDecayJobArgs]
	svc *Service
}

// NewNegativeDecayWorker creates the worker.
func NewNegativeDecayWorker(svc *Service) *NegativeDecayWorker {
	return &NegativeDecayWorker{svc: svc}
}

// Work finds all users with events at the decay boundary and recalculates.
func (w *NegativeDecayWorker) Work(ctx context.Context, _ *river.Job[NegativeDecayJobArgs]) error {
	slog.Info("reputation: decay job started")

	cutoff := time.Now().UTC().AddDate(0, 0, -decayWindowDays)
	userIDs, err := w.svc.repo.FindUsersWithNegativeEventsOlderThan(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("reputation: decay: find candidates: %w", err)
	}

	var errs int
	for _, userID := range userIDs {
		if _, err := w.svc.Recalculate(ctx, userID); err != nil {
			slog.Warn("reputation: decay: recalc failed", "userId", userID, "error", err)
			errs++
		}
	}

	slog.Info("reputation: decay job complete",
		"users_processed", len(userIDs), "errors", errs)
	return nil
}
