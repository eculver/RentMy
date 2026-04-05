package risk

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/riverqueue/river"
)

// MonthlyReputationJobArgs is the River job payload for monthly host reputation recalculation.
type MonthlyReputationJobArgs struct{}

// Kind implements river.JobArgs.
func (MonthlyReputationJobArgs) Kind() string { return "risk_monthly_reputation" }

// MonthlyReputationWorker recalculates host-specific reputation signals for all hosts.
// Runs on the 1st of each month (scheduled via River's periodic job support or a cron trigger).
type MonthlyReputationWorker struct {
	river.WorkerDefaults[MonthlyReputationJobArgs]
	svc *Service
}

// NewMonthlyReputationWorker creates the worker.
func NewMonthlyReputationWorker(svc *Service) *MonthlyReputationWorker {
	return &MonthlyReputationWorker{svc: svc}
}

// Work recalculates host-specific reputation signals for all hosts.
func (w *MonthlyReputationWorker) Work(ctx context.Context, _ *river.Job[MonthlyReputationJobArgs]) error {
	slog.Info("risk: monthly reputation job started")

	hostIDs, err := w.svc.repo.FindAllHostIDs(ctx)
	if err != nil {
		return fmt.Errorf("risk: monthly reputation: fetch hosts: %w", err)
	}

	var errs []error
	for _, hostID := range hostIDs {
		if err := w.svc.RecalculateHostSignals(ctx, hostID); err != nil {
			slog.Warn("risk: monthly reputation: host recalc failed", "hostId", hostID, "error", err)
			errs = append(errs, err)
		}
	}

	slog.Info("risk: monthly reputation job complete", "hosts_processed", len(hostIDs), "errors", len(errs))
	return nil
}

// DecayCheckJobArgs is the River job payload for weekly negative-signal decay recalculation.
type DecayCheckJobArgs struct{}

// Kind implements river.JobArgs.
func (DecayCheckJobArgs) Kind() string { return "risk_decay_check" }

// DecayCheckWorker scans for users with negative signals that have crossed the 180-day
// decay boundary and recalculates their reputation scores.
type DecayCheckWorker struct {
	river.WorkerDefaults[DecayCheckJobArgs]
	svc *Service
}

// NewDecayCheckWorker creates the worker.
func NewDecayCheckWorker(svc *Service) *DecayCheckWorker {
	return &DecayCheckWorker{svc: svc}
}

// Work recalculates reputation for users whose negative signals have crossed the decay boundary.
func (w *DecayCheckWorker) Work(ctx context.Context, _ *river.Job[DecayCheckJobArgs]) error {
	slog.Info("risk: decay check job started")

	// Find users with negative signals older than 180 days.
	cutoff := time.Now().UTC().AddDate(0, 0, -decayWindowDays)
	userIDs, err := w.svc.repo.FindUsersWithNegativeSignalsOlderThan(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("risk: decay check: fetch users: %w", err)
	}

	var errs []error
	for _, userID := range userIDs {
		if _, err := w.svc.RecalculateReputation(ctx, userID); err != nil {
			slog.Warn("risk: decay check: recalc failed", "userId", userID, "error", err)
			errs = append(errs, err)
		}
	}

	slog.Info("risk: decay check job complete", "users_processed", len(userIDs), "errors", len(errs))
	return nil
}
