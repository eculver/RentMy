package latereturn

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// riverInserter is the minimal River interface needed for scheduling jobs.
type riverInserter interface {
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// ensure *river.Client[pgx.Tx] satisfies riverInserter at compile time.
var _ riverInserter = (*river.Client[pgx.Tx])(nil)

// LateReturnCheckJobArgs is the payload for the scheduled late return check.
// Enqueued at scheduled_end for every ACTIVE rental.
type LateReturnCheckJobArgs struct {
	TransactionID string `json:"transactionId"`
}

func (LateReturnCheckJobArgs) Kind() string { return "late_return_check" }

// LateReturnCheckWorker processes the initial and recurring late return checks.
type LateReturnCheckWorker struct {
	river.WorkerDefaults[LateReturnCheckJobArgs]
	svc *Service
}

// NewLateReturnCheckWorker creates a new worker.
func NewLateReturnCheckWorker(svc *Service) *LateReturnCheckWorker {
	return &LateReturnCheckWorker{svc: svc}
}

func (w *LateReturnCheckWorker) Work(ctx context.Context, job *river.Job[LateReturnCheckJobArgs]) error {
	slog.Info("latereturn: processing check job",
		"transactionId", job.Args.TransactionID,
	)
	if err := w.svc.CheckAndCharge(ctx, job.Args.TransactionID); err != nil {
		return fmt.Errorf("check and charge: %w", err)
	}
	return nil
}

// LateReturnEscalationJobArgs is the payload for escalation evaluation.
// Fired after the configurable escalation threshold (default 4 hours overdue).
type LateReturnEscalationJobArgs struct {
	TransactionID string `json:"transactionId"`
}

func (LateReturnEscalationJobArgs) Kind() string { return "late_return_escalation" }

// LateReturnEscalationWorker evaluates whether to escalate a late return.
type LateReturnEscalationWorker struct {
	river.WorkerDefaults[LateReturnEscalationJobArgs]
	svc *Service
}

// NewLateReturnEscalationWorker creates a new worker.
func NewLateReturnEscalationWorker(svc *Service) *LateReturnEscalationWorker {
	return &LateReturnEscalationWorker{svc: svc}
}

func (w *LateReturnEscalationWorker) Work(ctx context.Context, job *river.Job[LateReturnEscalationJobArgs]) error {
	slog.Info("latereturn: processing escalation job",
		"transactionId", job.Args.TransactionID,
	)
	if err := w.svc.EvaluateEscalation(ctx, job.Args.TransactionID); err != nil {
		return fmt.Errorf("evaluate escalation: %w", err)
	}
	return nil
}

// ScheduleLateReturnCheck enqueues a late return check job at the scheduled end time.
// Called from booking.Accept when a booking is accepted.
func ScheduleLateReturnCheck(ctx context.Context, rc riverInserter, transactionID string, scheduledEnd time.Time) error {
	if scheduledEnd.IsZero() {
		return nil
	}
	// Fire exactly at scheduled_end — if the rental is still active, the check will charge.
	_, err := rc.Insert(ctx, LateReturnCheckJobArgs{
		TransactionID: transactionID,
	}, &river.InsertOpts{
		ScheduledAt: scheduledEnd,
	})
	if err != nil {
		return fmt.Errorf("schedule late return check: %w", err)
	}
	return nil
}
