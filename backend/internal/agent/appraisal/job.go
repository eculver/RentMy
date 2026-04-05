package appraisal

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/riverqueue/river"
)

// AppraisalJobArgs is the River job payload for async listing appraisal.
type AppraisalJobArgs struct {
	ListingID string `json:"listing_id"`
}

// Kind implements river.JobArgs.
func (AppraisalJobArgs) Kind() string { return "appraisal" }

// AppraisalJobWorker executes appraisals for newly created listings.
// It is registered in main.go before River starts.
type AppraisalJobWorker struct {
	river.WorkerDefaults[AppraisalJobArgs]
	svc *Service
}

// NewAppraisalJobWorker constructs the worker with the given service.
func NewAppraisalJobWorker(svc *Service) *AppraisalJobWorker {
	return &AppraisalJobWorker{svc: svc}
}

// Work performs the appraisal for the listing identified in the job args.
// River will retry on error (up to 3 times by default).
func (w *AppraisalJobWorker) Work(ctx context.Context, job *river.Job[AppraisalJobArgs]) error {
	listingID := job.Args.ListingID
	slog.Info("appraisal: job started", "listingId", listingID, "attempt", job.Attempt)

	if err := w.svc.Appraise(ctx, listingID); err != nil {
		slog.Warn("appraisal: job failed", "listingId", listingID, "attempt", job.Attempt, "error", err)
		return fmt.Errorf("appraisal job: %w", err)
	}

	slog.Info("appraisal: job complete", "listingId", listingID)
	return nil
}
