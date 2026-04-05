// Package backfill provides River batch jobs for backfilling AI-generated data
// onto existing listings, users, and transactions.
package backfill

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/riverqueue/river"

	"github.com/giits/rentmy/backend/internal/agent/appraisal"
	"github.com/giits/rentmy/backend/internal/agent/risk"
)

// ----- BackfillAppraisalJob -----

// BackfillAppraisalJobArgs is the River job payload for the appraisal backfill.
type BackfillAppraisalJobArgs struct{}

// Kind implements river.JobArgs.
func (BackfillAppraisalJobArgs) Kind() string { return "backfill_appraisal" }

// BackfillAppraisalWorker processes all listings that are missing AI-generated
// tags or an estimated value. Listings are processed in batches of 10 with a
// 1-second delay between each to avoid API throttling.
type BackfillAppraisalWorker struct {
	river.WorkerDefaults[BackfillAppraisalJobArgs]
	repo        *Repository
	appraisalSvc *appraisal.Service
	progress    *JobProgress
}

// NewBackfillAppraisalWorker constructs the worker.
// appraisalSvc may be nil at construction time; use SetAppraisalSvc to inject it
// after the full service has been built (needed for pre-River worker registration).
func NewBackfillAppraisalWorker(repo *Repository, appraisalSvc *appraisal.Service, progress *JobProgress) *BackfillAppraisalWorker {
	return &BackfillAppraisalWorker{repo: repo, appraisalSvc: appraisalSvc, progress: progress}
}

// SetAppraisalSvc injects the AppraisalAgent service into the worker after it has
// been constructed. Must be called before the worker processes any jobs.
func (w *BackfillAppraisalWorker) SetAppraisalSvc(svc *appraisal.Service) {
	w.appraisalSvc = svc
}

// Work performs the appraisal backfill.
func (w *BackfillAppraisalWorker) Work(ctx context.Context, _ *river.Job[BackfillAppraisalJobArgs]) error {
	if w.appraisalSvc == nil {
		return fmt.Errorf("backfill: appraisal service not initialized")
	}
	listingIDs, err := w.repo.FindListingsNeedingAppraisal(ctx)
	if err != nil {
		return fmt.Errorf("backfill: fetch listings: %w", err)
	}

	total := int64(len(listingIDs))
	w.progress.AppraisalTotal.Store(total)
	w.progress.AppraisalDone.Store(0)
	w.progress.AppraisalErrors.Store(0)

	slog.Info("backfill: appraisal started", "total", total)

	for i, id := range listingIDs {
		if err := w.appraisalSvc.Appraise(ctx, id); err != nil {
			slog.Warn("backfill: appraisal failed for listing",
				"listingId", id, "error", err)
			w.progress.AppraisalErrors.Add(1)
		}
		w.progress.AppraisalDone.Add(1)

		if (i+1)%10 == 0 {
			slog.Info("backfill: appraisal progress",
				"processed", i+1, "total", total,
				"errors", w.progress.AppraisalErrors.Load())
			// Rate-limit: 1 appraisal/second within each batch of 10.
			time.Sleep(1 * time.Second)
		}
	}

	slog.Info("backfill: appraisal complete",
		"total", total,
		"errors", w.progress.AppraisalErrors.Load())
	return nil
}

// ----- BackfillReputationJob -----

// BackfillReputationJobArgs is the River job payload for the reputation backfill.
type BackfillReputationJobArgs struct{}

// Kind implements river.JobArgs.
func (BackfillReputationJobArgs) Kind() string { return "backfill_reputation" }

// BackfillReputationWorker recalculates the reputation score for every user
// that has at least one transaction. Processed in batches of 50.
type BackfillReputationWorker struct {
	river.WorkerDefaults[BackfillReputationJobArgs]
	repo     *Repository
	riskSvc  *risk.Service
	progress *JobProgress
}

// NewBackfillReputationWorker constructs the worker.
func NewBackfillReputationWorker(repo *Repository, riskSvc *risk.Service, progress *JobProgress) *BackfillReputationWorker {
	return &BackfillReputationWorker{repo: repo, riskSvc: riskSvc, progress: progress}
}

// Work performs the reputation backfill.
func (w *BackfillReputationWorker) Work(ctx context.Context, _ *river.Job[BackfillReputationJobArgs]) error {
	userIDs, err := w.repo.FindUsersWithTransactions(ctx)
	if err != nil {
		return fmt.Errorf("backfill: fetch users: %w", err)
	}

	total := int64(len(userIDs))
	w.progress.ReputationTotal.Store(total)
	w.progress.ReputationDone.Store(0)
	w.progress.ReputationErrors.Store(0)

	slog.Info("backfill: reputation started", "total", total)

	const batchSize = 50
	for i, userID := range userIDs {
		if _, err := w.riskSvc.RecalculateReputation(ctx, userID); err != nil {
			slog.Warn("backfill: reputation recalc failed",
				"userId", userID, "error", err)
			w.progress.ReputationErrors.Add(1)
		}
		w.progress.ReputationDone.Add(1)

		if (i+1)%batchSize == 0 {
			slog.Info("backfill: reputation progress",
				"processed", i+1, "total", total,
				"errors", w.progress.ReputationErrors.Load())
		}
	}

	slog.Info("backfill: reputation complete",
		"total", total,
		"errors", w.progress.ReputationErrors.Load())
	return nil
}

// ----- BackfillRiskScoreJob -----

// BackfillRiskScoreJobArgs is the River job payload for the risk score backfill.
type BackfillRiskScoreJobArgs struct{}

// Kind implements river.JobArgs.
func (BackfillRiskScoreJobArgs) Kind() string { return "backfill_risk_scores" }

// BackfillRiskScoreWorker computes and persists risk scores for all transactions
// that do not yet have one. Scores are for analytics only — they do not
// retroactively affect payout timing or hold amounts. Processed in batches of 50.
type BackfillRiskScoreWorker struct {
	river.WorkerDefaults[BackfillRiskScoreJobArgs]
	repo     *Repository
	riskSvc  *risk.Service
	progress *JobProgress
}

// NewBackfillRiskScoreWorker constructs the worker.
func NewBackfillRiskScoreWorker(repo *Repository, riskSvc *risk.Service, progress *JobProgress) *BackfillRiskScoreWorker {
	return &BackfillRiskScoreWorker{repo: repo, riskSvc: riskSvc, progress: progress}
}

// Work performs the risk score backfill.
func (w *BackfillRiskScoreWorker) Work(ctx context.Context, _ *river.Job[BackfillRiskScoreJobArgs]) error {
	txns, err := w.repo.FindTransactionsWithoutRiskScore(ctx)
	if err != nil {
		return fmt.Errorf("backfill: fetch transactions: %w", err)
	}

	total := int64(len(txns))
	w.progress.RiskTotal.Store(total)
	w.progress.RiskDone.Store(0)
	w.progress.RiskErrors.Store(0)

	slog.Info("backfill: risk scores started", "total", total)

	const batchSize = 50
	for i, tx := range txns {
		in := risk.ComputeRiskInput{
			TransactionID: tx.ID,
			RenterID:      tx.RenterID,
			HostID:        tx.HostID,
		}
		if _, err := w.riskSvc.ComputeRiskScore(ctx, in); err != nil {
			slog.Warn("backfill: risk score failed",
				"transactionId", tx.ID, "error", err)
			w.progress.RiskErrors.Add(1)
		}
		w.progress.RiskDone.Add(1)

		if (i+1)%batchSize == 0 {
			slog.Info("backfill: risk scores progress",
				"processed", i+1, "total", total,
				"errors", w.progress.RiskErrors.Load())
		}
	}

	slog.Info("backfill: risk scores complete",
		"total", total,
		"errors", w.progress.RiskErrors.Load())
	return nil
}

// ----- Shared progress tracking -----

// JobProgress holds atomic counters for all three backfill jobs so the
// status handler can report live progress without a database round-trip.
type JobProgress struct {
	AppraisalTotal  atomic.Int64
	AppraisalDone   atomic.Int64
	AppraisalErrors atomic.Int64

	ReputationTotal  atomic.Int64
	ReputationDone   atomic.Int64
	ReputationErrors atomic.Int64

	RiskTotal  atomic.Int64
	RiskDone   atomic.Int64
	RiskErrors atomic.Int64
}

// NewJobProgress creates a zeroed JobProgress.
func NewJobProgress() *JobProgress {
	return &JobProgress{}
}

// jobStatusEntry is the per-job summary returned by the status endpoint.
type jobStatusEntry struct {
	Total     int64 `json:"total"`
	Processed int64 `json:"processed"`
	Errors    int64 `json:"errors"`
}

// Status returns a snapshot of all three jobs' progress.
func (p *JobProgress) Status() map[string]jobStatusEntry {
	return map[string]jobStatusEntry{
		"appraisals": {
			Total:     p.AppraisalTotal.Load(),
			Processed: p.AppraisalDone.Load(),
			Errors:    p.AppraisalErrors.Load(),
		},
		"reputation": {
			Total:     p.ReputationTotal.Load(),
			Processed: p.ReputationDone.Load(),
			Errors:    p.ReputationErrors.Load(),
		},
		"risk_scores": {
			Total:     p.RiskTotal.Load(),
			Processed: p.RiskDone.Load(),
			Errors:    p.RiskErrors.Load(),
		},
	}
}

// transactionStub holds the minimal fields needed to compute a risk score.
type transactionStub struct {
	ID       string
	RenterID string
	HostID   string
}

// Ensure river.JobArgs is satisfied at compile time.
var _ river.JobArgs = BackfillAppraisalJobArgs{}
var _ river.JobArgs = BackfillReputationJobArgs{}
var _ river.JobArgs = BackfillRiskScoreJobArgs{}

