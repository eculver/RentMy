package dispute

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/riverqueue/river"
)

// DisputeResolutionJobArgs is the payload for the async dispute processing job.
type DisputeResolutionJobArgs struct {
	DisputeID     string `json:"disputeId"`
	TransactionID string `json:"transactionId"`
}

func (DisputeResolutionJobArgs) Kind() string { return "dispute_resolution" }

// DisputeResolutionWorker processes dispute resolution asynchronously.
type DisputeResolutionWorker struct {
	river.WorkerDefaults[DisputeResolutionJobArgs]
	svc *Service
}

// NewDisputeResolutionWorker creates a new worker.
func NewDisputeResolutionWorker(svc *Service) *DisputeResolutionWorker {
	return &DisputeResolutionWorker{svc: svc}
}

func (w *DisputeResolutionWorker) Work(ctx context.Context, job *river.Job[DisputeResolutionJobArgs]) error {
	slog.Info("dispute: processing resolution job",
		"disputeId", job.Args.DisputeID,
		"transactionId", job.Args.TransactionID,
	)
	if err := w.svc.ProcessDispute(ctx, job.Args.DisputeID, job.Args.TransactionID); err != nil {
		return fmt.Errorf("process dispute: %w", err)
	}
	return nil
}

// RePromptExpiryJobArgs fires 2h after an INCONCLUSIVE re-prompt.
type RePromptExpiryJobArgs struct {
	DisputeID     string `json:"disputeId"`
	TransactionID string `json:"transactionId"`
}

func (RePromptExpiryJobArgs) Kind() string { return "dispute_reprompt_expiry" }

// RePromptExpiryWorker handles re-prompt expiry.
type RePromptExpiryWorker struct {
	river.WorkerDefaults[RePromptExpiryJobArgs]
	svc *Service
}

// NewRePromptExpiryWorker creates a new worker.
func NewRePromptExpiryWorker(svc *Service) *RePromptExpiryWorker {
	return &RePromptExpiryWorker{svc: svc}
}

func (w *RePromptExpiryWorker) Work(ctx context.Context, job *river.Job[RePromptExpiryJobArgs]) error {
	d, err := w.svc.repo.FindByID(ctx, job.Args.DisputeID)
	if err != nil {
		return fmt.Errorf("find dispute: %w", err)
	}

	if d.Status != StatusInconclusive {
		slog.Info("dispute: re-prompt expiry skipped, status changed",
			"disputeId", d.ID,
			"status", d.Status,
		)
		return nil
	}

	// Check if new photos were submitted since INCONCLUSIVE status.
	checkIn, checkOut, err := w.svc.repo.GatherMediaEvidence(ctx, job.Args.TransactionID)
	if err != nil {
		return fmt.Errorf("gather media: %w", err)
	}

	if len(checkIn) > 0 || len(checkOut) > 0 {
		slog.Info("dispute: new photos found, re-running pipeline",
			"disputeId", d.ID,
		)
		return w.svc.ProcessDispute(ctx, d.ID, job.Args.TransactionID)
	}

	slog.Info("dispute: re-prompt unanswered, escalating to human review",
		"disputeId", d.ID,
	)
	return w.svc.repo.UpdateStatus(ctx, d.ID, StatusHumanReview)
}

// SLAMonitorJobArgs is the periodic job that checks SLA compliance.
type SLAMonitorJobArgs struct{}

func (SLAMonitorJobArgs) Kind() string { return "dispute_sla_monitor" }

// SLAMonitorWorker runs every 15 minutes to check dispute SLA deadlines.
type SLAMonitorWorker struct {
	river.WorkerDefaults[SLAMonitorJobArgs]
	svc *Service
}

// NewSLAMonitorWorker creates a new worker.
func NewSLAMonitorWorker(svc *Service) *SLAMonitorWorker {
	return &SLAMonitorWorker{svc: svc}
}

func (w *SLAMonitorWorker) Work(ctx context.Context, _ *river.Job[SLAMonitorJobArgs]) error {
	// Find disputes at or past 80% of their SLA window.
	disputes, err := w.svc.repo.FindSLABreaching(ctx, 0.80)
	if err != nil {
		return fmt.Errorf("find sla breaching: %w", err)
	}

	now := time.Now()
	for _, d := range disputes {
		if d.SLADeadline == nil {
			continue
		}
		if now.After(*d.SLADeadline) {
			slog.Error("dispute: SLA BREACHED, auto-escalating",
				"disputeId", d.ID,
				"slaDeadline", d.SLADeadline,
			)
		} else {
			slog.Warn("dispute: SLA warning, approaching deadline",
				"disputeId", d.ID,
				"slaDeadline", d.SLADeadline,
				"remaining", time.Until(*d.SLADeadline),
			)
		}
	}

	return nil
}
