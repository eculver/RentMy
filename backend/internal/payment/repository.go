package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all payment-domain database operations.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given connection pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// GetListingForBooking fetches the fields from a listing needed to compute booking amounts.
func (r *Repository) GetListingForBooking(ctx context.Context, listingID string) (ListingSnapshot, error) {
	const q = `
		SELECT id, host_id, price_per_hour, price_per_day, host_declared_value, estimated_value
		FROM listings
		WHERE id = $1 AND status = 'ACTIVE'`

	var snap ListingSnapshot
	err := r.pool.QueryRow(ctx, q, listingID).Scan(
		&snap.ID,
		&snap.HostID,
		&snap.PricePerHour,
		&snap.PricePerDay,
		&snap.HostDeclaredValue,
		&snap.EstimatedValue,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ListingSnapshot{}, ErrListingNotFound
	}
	if err != nil {
		return ListingSnapshot{}, fmt.Errorf("get listing for booking: %w", err)
	}
	return snap, nil
}

// GetStripeCustomerID returns the Stripe customer ID for a user, or "" if not set.
func (r *Repository) GetStripeCustomerID(ctx context.Context, userID string) (string, error) {
	var id *string
	err := r.pool.QueryRow(ctx,
		`SELECT stripe_customer_id FROM users WHERE id = $1`, userID,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get stripe customer id: %w", err)
	}
	if id == nil {
		return "", nil
	}
	return *id, nil
}

// StoreStripeCustomerID stores the Stripe customer ID on a user row.
func (r *Repository) StoreStripeCustomerID(ctx context.Context, userID, customerID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET stripe_customer_id = $1 WHERE id = $2`, customerID, userID,
	)
	if err != nil {
		return fmt.Errorf("store stripe customer id: %w", err)
	}
	return nil
}

// GetStripeAccountID returns the Stripe connected account ID for a user, or "" if not set.
func (r *Repository) GetStripeAccountID(ctx context.Context, userID string) (string, error) {
	var id *string
	err := r.pool.QueryRow(ctx,
		`SELECT stripe_account_id FROM users WHERE id = $1`, userID,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get stripe account id: %w", err)
	}
	if id == nil {
		return "", nil
	}
	return *id, nil
}

// StoreStripeAccountID stores the Stripe connected account ID on a user row.
func (r *Repository) StoreStripeAccountID(ctx context.Context, userID, accountID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET stripe_account_id = $1 WHERE id = $2`, accountID, userID,
	)
	if err != nil {
		return fmt.Errorf("store stripe account id: %w", err)
	}
	return nil
}

// GetUserEmailAndName returns the email and name for a user (needed for Stripe customer/account creation).
func (r *Repository) GetUserEmailAndName(ctx context.Context, userID string) (email, name string, err error) {
	err = r.pool.QueryRow(ctx,
		`SELECT COALESCE(email, ''), name FROM users WHERE id = $1`, userID,
	).Scan(&email, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", fmt.Errorf("user not found: %s", userID)
	}
	if err != nil {
		return "", "", fmt.Errorf("get user email and name: %w", err)
	}
	return email, name, nil
}

// CreateTransaction inserts a new transaction row and returns the created transaction.
func (r *Repository) CreateTransaction(ctx context.Context, tx pgx.Tx, t Transaction) error {
	alloc, err := json.Marshal(t.HoldAllocation)
	if err != nil {
		return fmt.Errorf("marshal hold allocation: %w", err)
	}

	const q = `
		INSERT INTO transactions (
			id, renter_id, host_id, listing_id,
			rental_fee, hold_amount, item_value, guarantee_gap,
			platform_fee, host_payout, guarantee_contribution,
			escrow_status, hold_status, hold_allocation,
			stripe_payment_intent_id, stripe_charge_id,
			scheduled_start, scheduled_end, status
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			$9, $10, $11,
			$12, $13, $14,
			$15, $16,
			$17, $18, $19
		)`

	_, err = tx.Exec(ctx, q,
		t.ID, t.RenterID, t.HostID, t.ListingID,
		float64(t.RentalFee)/100, float64(t.HoldAmount)/100,
		float64(t.ItemValue)/100, float64(t.GuaranteeGap)/100,
		float64(t.PlatformFee)/100, float64(t.HostPayout)/100,
		float64(t.GuaranteeContribution)/100,
		t.EscrowStatus, t.HoldStatus, alloc,
		t.StripePaymentIntentID, t.StripeChargeID,
		t.ScheduledStart, t.ScheduledEnd, t.Status,
	)
	if err != nil {
		return fmt.Errorf("create transaction: %w", err)
	}
	return nil
}

// GetTransaction fetches a transaction by ID.
func (r *Repository) GetTransaction(ctx context.Context, id string) (Transaction, error) {
	const q = `
		SELECT
			id, renter_id, host_id, listing_id,
			ROUND(rental_fee * 100)::bigint,
			ROUND(hold_amount * 100)::bigint,
			ROUND(item_value * 100)::bigint,
			ROUND(guarantee_gap * 100)::bigint,
			ROUND(platform_fee * 100)::bigint,
			ROUND(host_payout * 100)::bigint,
			ROUND(guarantee_contribution * 100)::bigint,
			escrow_status, hold_status, hold_allocation,
			COALESCE(stripe_payment_intent_id, ''),
			COALESCE(stripe_charge_id, ''),
			COALESCE(stripe_transfer_id, ''),
			scheduled_start, scheduled_end, status, created_at
		FROM transactions
		WHERE id = $1`

	var t Transaction
	var allocJSON []byte
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&t.ID, &t.RenterID, &t.HostID, &t.ListingID,
		&t.RentalFee, &t.HoldAmount, &t.ItemValue, &t.GuaranteeGap,
		&t.PlatformFee, &t.HostPayout, &t.GuaranteeContribution,
		&t.EscrowStatus, &t.HoldStatus, &allocJSON,
		&t.StripePaymentIntentID, &t.StripeChargeID, &t.StripeTransferID,
		&t.ScheduledStart, &t.ScheduledEnd, &t.Status, &t.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Transaction{}, ErrTransactionNotFound
	}
	if err != nil {
		return Transaction{}, fmt.Errorf("get transaction: %w", err)
	}
	if err := json.Unmarshal(allocJSON, &t.HoldAllocation); err != nil {
		return Transaction{}, fmt.Errorf("unmarshal hold allocation: %w", err)
	}
	return t, nil
}

// UpdateHoldAllocation atomically updates the hold_allocation JSONB and hold_status
// on a transaction. Must be called within a pgx transaction.
// Uses SELECT FOR UPDATE to prevent concurrent modification.
func (r *Repository) UpdateHoldAllocation(ctx context.Context, tx pgx.Tx, transactionID string, alloc HoldAllocation, holdStatus string) error {
	allocJSON, err := json.Marshal(alloc)
	if err != nil {
		return fmt.Errorf("marshal hold allocation: %w", err)
	}

	// Lock the row first.
	var id string
	err = tx.QueryRow(ctx,
		`SELECT id FROM transactions WHERE id = $1 FOR UPDATE`, transactionID,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrTransactionNotFound
	}
	if err != nil {
		return fmt.Errorf("lock transaction row: %w", err)
	}

	_, err = tx.Exec(ctx,
		`UPDATE transactions SET hold_allocation = $1, hold_status = $2 WHERE id = $3`,
		allocJSON, holdStatus, transactionID,
	)
	if err != nil {
		return fmt.Errorf("update hold allocation: %w", err)
	}
	return nil
}

// UpdateTransactionStatus sets the status field on a transaction.
func (r *Repository) UpdateTransactionStatus(ctx context.Context, transactionID, status string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE transactions SET status = $1 WHERE id = $2`, status, transactionID,
	)
	if err != nil {
		return fmt.Errorf("update transaction status: %w", err)
	}
	return nil
}

// UpdateTransactionStripeTransferID stores the transfer ID after host payout.
func (r *Repository) UpdateTransactionStripeTransferID(ctx context.Context, transactionID, transferID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE transactions SET stripe_transfer_id = $1 WHERE id = $2`, transferID, transactionID,
	)
	if err != nil {
		return fmt.Errorf("update stripe transfer id: %w", err)
	}
	return nil
}

// GetHostTransactionCount returns the number of completed transactions for a host.
func (r *Repository) GetHostTransactionCount(ctx context.Context, hostID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM transactions WHERE host_id = $1 AND status = 'COMPLETED'`, hostID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("get host transaction count: %w", err)
	}
	return count, nil
}

// GetHostReputationScore returns the reputation_score for a user (host).
func (r *Repository) GetHostReputationScore(ctx context.Context, hostID string) (int, error) {
	var score int
	err := r.pool.QueryRow(ctx,
		`SELECT reputation_score FROM users WHERE id = $1`, hostID,
	).Scan(&score)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get host reputation score: %w", err)
	}
	return score, nil
}

// GetRenterTransactions returns paginated transactions for a renter.
func (r *Repository) GetRenterTransactions(ctx context.Context, renterID string, limit, offset int) ([]Transaction, int, error) {
	if limit <= 0 {
		limit = 20
	}

	var total int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM transactions WHERE renter_id = $1`, renterID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count renter transactions: %w", err)
	}

	const q = `
		SELECT
			id, renter_id, host_id, listing_id,
			ROUND(rental_fee * 100)::bigint,
			ROUND(hold_amount * 100)::bigint,
			ROUND(item_value * 100)::bigint,
			ROUND(guarantee_gap * 100)::bigint,
			ROUND(platform_fee * 100)::bigint,
			ROUND(host_payout * 100)::bigint,
			ROUND(guarantee_contribution * 100)::bigint,
			escrow_status, hold_status, hold_allocation,
			COALESCE(stripe_payment_intent_id, ''),
			COALESCE(stripe_charge_id, ''),
			COALESCE(stripe_transfer_id, ''),
			scheduled_start, scheduled_end, status, created_at
		FROM transactions
		WHERE renter_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, q, renterID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query renter transactions: %w", err)
	}
	defer rows.Close()

	var txns []Transaction
	for rows.Next() {
		var t Transaction
		var allocJSON []byte
		if err := rows.Scan(
			&t.ID, &t.RenterID, &t.HostID, &t.ListingID,
			&t.RentalFee, &t.HoldAmount, &t.ItemValue, &t.GuaranteeGap,
			&t.PlatformFee, &t.HostPayout, &t.GuaranteeContribution,
			&t.EscrowStatus, &t.HoldStatus, &allocJSON,
			&t.StripePaymentIntentID, &t.StripeChargeID, &t.StripeTransferID,
			&t.ScheduledStart, &t.ScheduledEnd, &t.Status, &t.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan transaction: %w", err)
		}
		if err := json.Unmarshal(allocJSON, &t.HoldAllocation); err != nil {
			return nil, 0, fmt.Errorf("unmarshal hold allocation: %w", err)
		}
		txns = append(txns, t)
	}
	return txns, total, rows.Err()
}

// BeginTx starts a new database transaction.
func (r *Repository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	return tx, nil
}
