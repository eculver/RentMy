package outcome

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	rivertype "github.com/riverqueue/river/rivertype"

	"github.com/giits/rentmy/backend/internal/agent/decision"
)

// OutcomeLinkJobArgs are the arguments for the outcome linking River job.
// Fires 48h after transaction close or dispute resolution.
type OutcomeLinkJobArgs struct {
	TransactionID string `json:"transactionId"`
}

func (OutcomeLinkJobArgs) Kind() string { return "outcome_link" }

// OutcomeLinkWorker processes outcome linking jobs.
type OutcomeLinkWorker struct {
	river.WorkerDefaults[OutcomeLinkJobArgs]
	svc *Service
}

// NewOutcomeLinkWorker creates a new worker for outcome linking jobs.
func NewOutcomeLinkWorker(svc *Service) *OutcomeLinkWorker {
	return &OutcomeLinkWorker{svc: svc}
}

func (w *OutcomeLinkWorker) Work(ctx context.Context, job *river.Job[OutcomeLinkJobArgs]) error {
	slog.Info("outcome: linking job started",
		"transactionId", job.Args.TransactionID,
		"attempt", job.Attempt,
	)
	if err := w.svc.LinkOutcomes(ctx, job.Args.TransactionID); err != nil {
		return fmt.Errorf("outcome link job: %w", err)
	}
	return nil
}

// MonthlyCalibrationReportJobArgs are the arguments for the monthly calibration
// report River job. Runs on the 1st of each month.
type MonthlyCalibrationReportJobArgs struct{}

func (MonthlyCalibrationReportJobArgs) Kind() string { return "monthly_calibration_report" }

// MonthlyCalibrationReportWorker processes the monthly calibration report.
type MonthlyCalibrationReportWorker struct {
	river.WorkerDefaults[MonthlyCalibrationReportJobArgs]
	svc *Service
}

// NewMonthlyCalibrationReportWorker creates a new worker for monthly calibration reports.
func NewMonthlyCalibrationReportWorker(svc *Service) *MonthlyCalibrationReportWorker {
	return &MonthlyCalibrationReportWorker{svc: svc}
}

func (w *MonthlyCalibrationReportWorker) Work(ctx context.Context, job *river.Job[MonthlyCalibrationReportJobArgs]) error {
	slog.Info("outcome: monthly calibration report started", "attempt", job.Attempt)

	agentTypes := []string{
		string(decision.AgentTypeDispute),
		string(decision.AgentTypeRisk),
		string(decision.AgentTypeAppraisal),
		string(decision.AgentTypeLateReturn),
		string(decision.AgentTypeAgreement),
		string(decision.AgentTypeVerification),
		string(decision.AgentTypeFraud),
	}

	for _, at := range agentTypes {
		if err := w.svc.UpdateCalibrationMetrics(ctx, at); err != nil {
			slog.Warn("outcome: failed to update calibration for agent",
				"agentType", at,
				"error", err,
			)
		}
	}

	slog.Info("outcome: monthly calibration report completed")
	return nil
}

// riverInserter is the minimal River interface needed to schedule outcome jobs.
type riverInserter interface {
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// ensure *river.Client[pgx.Tx] satisfies riverInserter at compile time.
var _ riverInserter = (*river.Client[pgx.Tx])(nil)

// ScheduleOutcomeLink enqueues an outcome linking job 48h in the future.
func ScheduleOutcomeLink(ctx context.Context, client riverInserter, transactionID string) error {
	fireAt := time.Now().Add(48 * time.Hour)
	_, err := client.Insert(ctx, OutcomeLinkJobArgs{
		TransactionID: transactionID,
	}, &river.InsertOpts{
		ScheduledAt: fireAt,
	})
	if err != nil {
		return fmt.Errorf("schedule outcome link: %w", err)
	}
	slog.Info("outcome: link job scheduled",
		"transactionId", transactionID,
		"scheduledAt", fireAt,
	)
	return nil
}
