package notification

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// riverInserter is the minimal River interface needed to enqueue notification jobs.
type riverInserter interface {
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// ensure *river.Client[pgx.Tx] satisfies riverInserter at compile time.
var _ riverInserter = (*river.Client[pgx.Tx])(nil)

// PickupApproachingArgs is the River job payload that fires ~30 minutes before
// a booking's scheduled start time to remind both parties.
type PickupApproachingArgs struct {
	TransactionID string `json:"transactionId"`
	RenterID      string `json:"renterId"`
	HostID        string `json:"hostId"`
}

// Kind implements river.JobArgs.
func (PickupApproachingArgs) Kind() string { return "notification_pickup_approaching" }

// PickupApproachingWorker sends pickup reminder notifications to both parties.
type PickupApproachingWorker struct {
	river.WorkerDefaults[PickupApproachingArgs]
	svc *Service
}

// NewPickupApproachingWorker creates a PickupApproachingWorker.
func NewPickupApproachingWorker(svc *Service) *PickupApproachingWorker {
	return &PickupApproachingWorker{svc: svc}
}

// Work sends pickup approaching notifications.
func (w *PickupApproachingWorker) Work(ctx context.Context, job *river.Job[PickupApproachingArgs]) error {
	args := job.Args
	data := map[string]string{"transactionId": args.TransactionID}

	for _, userID := range []string{args.RenterID, args.HostID} {
		if err := w.svc.Notify(ctx, userID, TypePickupApproaching,
			"Pickup in 30 minutes",
			"Your rental handoff is approaching. Meet at the agreed location.",
			data,
		); err != nil {
			slog.Warn("pickup approaching notification failed", "userID", userID, "error", err)
		}
	}
	return nil
}

// ReturnApproachingArgs is the River job payload that fires ~30 minutes before
// a booking's scheduled end time to remind the renter to return the item.
type ReturnApproachingArgs struct {
	TransactionID string `json:"transactionId"`
	RenterID      string `json:"renterId"`
}

// Kind implements river.JobArgs.
func (ReturnApproachingArgs) Kind() string { return "notification_return_approaching" }

// ReturnApproachingWorker sends return reminder notifications.
type ReturnApproachingWorker struct {
	river.WorkerDefaults[ReturnApproachingArgs]
	svc *Service
}

// NewReturnApproachingWorker creates a ReturnApproachingWorker.
func NewReturnApproachingWorker(svc *Service) *ReturnApproachingWorker {
	return &ReturnApproachingWorker{svc: svc}
}

// Work sends return approaching notification to the renter.
func (w *ReturnApproachingWorker) Work(ctx context.Context, job *river.Job[ReturnApproachingArgs]) error {
	args := job.Args
	if err := w.svc.Notify(ctx, args.RenterID, TypeReturnApproaching,
		"Return in 30 minutes",
		"Your rental period ends soon. Please arrange the return with the host.",
		map[string]string{"transactionId": args.TransactionID},
	); err != nil {
		slog.Warn("return approaching notification failed", "userID", args.RenterID, "error", err)
	}
	return nil
}

// QuietHoursDeferredArgs is the River job payload used to deliver a notification
// that was deferred because it arrived during the user's quiet hours.
type QuietHoursDeferredArgs struct {
	UserID        string            `json:"userId"`
	NotifType     string            `json:"type"`
	Title         string            `json:"title"`
	Body          string            `json:"body"`
	Data          map[string]string `json:"data"`
}

// Kind implements river.JobArgs.
func (QuietHoursDeferredArgs) Kind() string { return "notification_quiet_hours_deferred" }

// QuietHoursDeferredWorker delivers a deferred notification when quiet hours end.
type QuietHoursDeferredWorker struct {
	river.WorkerDefaults[QuietHoursDeferredArgs]
	svc *Service
}

// NewQuietHoursDeferredWorker creates a QuietHoursDeferredWorker.
func NewQuietHoursDeferredWorker(svc *Service) *QuietHoursDeferredWorker {
	return &QuietHoursDeferredWorker{svc: svc}
}

// Work delivers the deferred notification, skipping the quiet-hours check (we
// already waited).
func (w *QuietHoursDeferredWorker) Work(ctx context.Context, job *river.Job[QuietHoursDeferredArgs]) error {
	args := job.Args
	return w.svc.notifyDirect(ctx, args.UserID, Type(args.NotifType), args.Title, args.Body, args.Data)
}

// SchedulePickupApproaching enqueues a PickupApproachingJob to fire at
// scheduledStart minus the reminder window (e.g. 30 minutes before).
func SchedulePickupApproaching(ctx context.Context, rc riverInserter, txID, renterID, hostID string, scheduledStart time.Time, reminderBefore time.Duration) error {
	fireAt := scheduledStart.Add(-reminderBefore)
	if !fireAt.After(time.Now()) {
		// Already past the reminder window — skip.
		return nil
	}
	_, err := rc.Insert(ctx, PickupApproachingArgs{
		TransactionID: txID,
		RenterID:      renterID,
		HostID:        hostID,
	}, &river.InsertOpts{ScheduledAt: fireAt})
	if err != nil {
		return fmt.Errorf("schedule pickup approaching: %w", err)
	}
	return nil
}

// ScheduleReturnApproaching enqueues a ReturnApproachingJob to fire at
// scheduledEnd minus the reminder window.
func ScheduleReturnApproaching(ctx context.Context, rc riverInserter, txID, renterID string, scheduledEnd time.Time, reminderBefore time.Duration) error {
	fireAt := scheduledEnd.Add(-reminderBefore)
	if !fireAt.After(time.Now()) {
		return nil
	}
	_, err := rc.Insert(ctx, ReturnApproachingArgs{
		TransactionID: txID,
		RenterID:      renterID,
	}, &river.InsertOpts{ScheduledAt: fireAt})
	if err != nil {
		return fmt.Errorf("schedule return approaching: %w", err)
	}
	return nil
}
