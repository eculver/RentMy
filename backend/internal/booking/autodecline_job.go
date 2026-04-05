package booking

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/giits/rentmy/backend/internal/notification"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// AutoDeclineJobArgs is the River job payload for auto-declining a booking
// that has not been accepted within the configured timeout window.
type AutoDeclineJobArgs struct {
	TransactionID string `json:"transactionId"`
}

// Kind implements river.JobArgs.
func (AutoDeclineJobArgs) Kind() string { return "booking_auto_decline" }

// AutoDeclineJobWorker transitions a REQUESTED booking to AUTO_DECLINED
// if the host has not responded within the timeout window.
// If the booking is no longer REQUESTED (host accepted/declined), this is a no-op.
type AutoDeclineJobWorker struct {
	river.WorkerDefaults[AutoDeclineJobArgs]
	repo            *Repository
	notificationSvc notificationSvc
}

// NewAutoDeclineJobWorker creates a new AutoDeclineJobWorker.
// notificationSvc may be nil (notifications disabled for testing).
func NewAutoDeclineJobWorker(repo *Repository, notificationSvc notificationSvc) *AutoDeclineJobWorker {
	return &AutoDeclineJobWorker{repo: repo, notificationSvc: notificationSvc}
}

// Work executes the auto-decline transition.
func (w *AutoDeclineJobWorker) Work(ctx context.Context, job *river.Job[AutoDeclineJobArgs]) error {
	id := job.Args.TransactionID

	booking, err := w.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("auto decline: find booking: %w", err)
	}

	// If the booking is no longer REQUESTED, the host already acted — nothing to do.
	if booking.Status != StatusRequested {
		slog.Info("auto-decline no-op: booking already transitioned",
			"transactionId", id,
			"status", booking.Status,
		)
		return nil
	}

	tx, err := w.repo.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("auto decline: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := w.repo.UpdateStatus(ctx, tx, id, StatusAutoDeclined); err != nil {
		return fmt.Errorf("auto decline: update status: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("auto decline: commit: %w", err)
	}

	slog.Info("booking auto-declined", "transactionId", id)

	// Notify both parties (best-effort; failure does not roll back the decline).
	if w.notificationSvc != nil {
		data := map[string]string{"transactionId": id}
		for _, userID := range []string{booking.RenterID, booking.HostID} {
			if err := w.notificationSvc.Notify(ctx, userID, notification.TypeBookingAutoDeclined,
				"Booking auto-declined",
				"The booking was automatically declined because the host did not respond in time.",
				data,
			); err != nil {
				slog.Warn("auto-decline notification failed", "userID", userID, "error", err)
			}
		}
	}
	return nil
}

// scheduleAutoDecline enqueues an AutoDeclineJob with the configured delay.
func scheduleAutoDecline(ctx context.Context, riverClient riverInserter, id string, cfg Config) error {
	opts := &river.InsertOpts{
		ScheduledAt: cfg.autoDeclineAt(),
	}
	_, err := riverClient.Insert(ctx, AutoDeclineJobArgs{TransactionID: id}, opts)
	if err != nil {
		return fmt.Errorf("schedule auto decline: %w", err)
	}
	return nil
}

// riverInserter is the minimal River interface needed by the booking service.
type riverInserter interface {
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// ensure *river.Client[pgx.Tx] satisfies riverInserter at compile time.
var _ riverInserter = (*river.Client[pgx.Tx])(nil)
