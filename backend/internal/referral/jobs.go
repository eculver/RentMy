package referral

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/riverqueue/river"
)

// ReferralPayoutJobArgs triggers EvaluateAndPayout for a referral after the
// referee completes their first rental.
type ReferralPayoutJobArgs struct {
	ReferralID string `json:"referralId"`
}

// Kind returns the River job type identifier.
func (ReferralPayoutJobArgs) Kind() string { return "referral_payout_evaluate" }

// ReferralPayoutJobWorker executes EvaluateAndPayout for a referral.
type ReferralPayoutJobWorker struct {
	river.WorkerDefaults[ReferralPayoutJobArgs]
	svc *Service
}

// NewReferralPayoutJobWorker creates a worker for referral payout evaluation.
func NewReferralPayoutJobWorker(svc *Service) *ReferralPayoutJobWorker {
	return &ReferralPayoutJobWorker{svc: svc}
}

// Work calls EvaluateAndPayout.
func (w *ReferralPayoutJobWorker) Work(ctx context.Context, job *river.Job[ReferralPayoutJobArgs]) error {
	slog.Info("referral: evaluating payout", "referralId", job.Args.ReferralID)
	if err := w.svc.EvaluateAndPayout(ctx, job.Args.ReferralID); err != nil {
		return fmt.Errorf("evaluate and payout: %w", err)
	}
	return nil
}

// ReferralStripeTransferJobArgs triggers ProcessPayout for a single payout record.
type ReferralStripeTransferJobArgs struct {
	PayoutID string `json:"payoutId"`
}

// Kind returns the River job type identifier.
func (ReferralStripeTransferJobArgs) Kind() string { return "referral_stripe_transfer" }

// ReferralStripeTransferWorker executes ProcessPayout for a single payout.
type ReferralStripeTransferWorker struct {
	river.WorkerDefaults[ReferralStripeTransferJobArgs]
	svc *Service
}

// NewReferralStripeTransferWorker creates a worker for Stripe referral transfers.
func NewReferralStripeTransferWorker(svc *Service) *ReferralStripeTransferWorker {
	return &ReferralStripeTransferWorker{svc: svc}
}

// Work calls ProcessPayout.
func (w *ReferralStripeTransferWorker) Work(ctx context.Context, job *river.Job[ReferralStripeTransferJobArgs]) error {
	slog.Info("referral: processing stripe transfer", "payoutId", job.Args.PayoutID)
	if err := w.svc.ProcessPayout(ctx, job.Args.PayoutID); err != nil {
		return fmt.Errorf("process payout: %w", err)
	}
	return nil
}
