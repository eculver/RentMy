package payment

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/riverqueue/river"
)

// PayoutJobArgs contains the arguments for a delayed host payout River job.
type PayoutJobArgs struct {
	TransactionID string `json:"transactionId"`
	HostAccountID string `json:"hostAccountId"`
	Amount        int64  `json:"amount"`   // cents
	Currency      string `json:"currency"` // e.g. "usd"
}

// Kind returns the job type identifier registered with River.
func (PayoutJobArgs) Kind() string { return "host_payout" }

// PayoutJobWorker executes a delayed host payout job by transferring funds
// to the host's Stripe Express connected account.
type PayoutJobWorker struct {
	river.WorkerDefaults[PayoutJobArgs]

	repo    *Repository
	adapter PaymentAdapter
}

// NewPayoutJobWorker creates a PayoutJobWorker with the given dependencies.
func NewPayoutJobWorker(repo *Repository, adapter PaymentAdapter) *PayoutJobWorker {
	return &PayoutJobWorker{repo: repo, adapter: adapter}
}

// Work executes the payout job: calls Stripe transfer and records the transfer ID.
func (w *PayoutJobWorker) Work(ctx context.Context, job *river.Job[PayoutJobArgs]) error {
	args := job.Args
	slog.Info("executing host payout job",
		"transactionId", args.TransactionID,
		"amount", args.Amount,
		"currency", args.Currency,
	)

	transferID, err := w.adapter.PayoutHost(ctx, args.Amount, args.Currency, args.HostAccountID)
	if err != nil {
		return fmt.Errorf("payout host via stripe: %w", err)
	}

	if err := w.repo.UpdateTransactionStripeTransferID(ctx, args.TransactionID, transferID); err != nil {
		// Log but don't fail — the transfer succeeded. A manual reconciliation can fix this.
		slog.Error("failed to record stripe transfer id",
			"transactionId", args.TransactionID,
			"transferId", transferID,
			"error", err,
		)
	}

	slog.Info("host payout completed",
		"transactionId", args.TransactionID,
		"transferId", transferID,
		"amount", args.Amount,
	)
	return nil
}
