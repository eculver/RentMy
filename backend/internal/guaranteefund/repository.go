package guaranteefund

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles guarantee fund database operations.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given connection pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// InsertEntry inserts a guarantee fund ledger entry within the given transaction.
// balance_after is computed as the previous entry's balance_after plus amount.
func (r *Repository) InsertEntry(ctx context.Context, tx pgx.Tx, entry Entry) error {
	var prev float64
	err := tx.QueryRow(ctx,
		`SELECT COALESCE(balance_after, 0) FROM guarantee_fund_entries ORDER BY created_at DESC LIMIT 1`,
	).Scan(&prev)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("get previous balance: %w", err)
	}

	balanceAfter := prev + float64(entry.Amount)/100

	_, err = tx.Exec(ctx,
		`INSERT INTO guarantee_fund_entries (id, transaction_id, entry_type, amount, balance_after)
		 VALUES ($1, $2, $3, $4, $5)`,
		entry.ID,
		entry.TransactionID,
		string(entry.EntryType),
		float64(entry.Amount)/100,
		balanceAfter,
	)
	if err != nil {
		return fmt.Errorf("insert guarantee fund entry: %w", err)
	}
	return nil
}

// GetCurrentBalance returns the current guarantee fund balance in cents.
func (r *Repository) GetCurrentBalance(ctx context.Context) (int64, error) {
	var balance float64
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(balance_after, 0) FROM guarantee_fund_entries ORDER BY created_at DESC LIMIT 1`,
	).Scan(&balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get current balance: %w", err)
	}
	return int64(balance * 100), nil
}

// GetOutstandingGaps returns the sum of guarantee_gap for all active transactions, in cents.
func (r *Repository) GetOutstandingGaps(ctx context.Context) (int64, error) {
	var total float64
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(guarantee_gap), 0) FROM transactions WHERE status = 'ACTIVE'`,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("get outstanding gaps: %w", err)
	}
	return int64(total * 100), nil
}

// GetRolling90DayClaims returns the total claims amount (absolute value, in cents)
// for the last 90 days.
func (r *Repository) GetRolling90DayClaims(ctx context.Context) (int64, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -90)
	var total float64
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(ABS(amount)), 0) FROM guarantee_fund_entries
		 WHERE entry_type = 'CLAIM' AND created_at >= $1`,
		cutoff,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("get rolling 90-day claims: %w", err)
	}
	return int64(total * 100), nil
}

// GetRolling90DayContributions returns the total contributions (in cents)
// for the last 90 days.
func (r *Repository) GetRolling90DayContributions(ctx context.Context) (int64, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -90)
	var total float64
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM guarantee_fund_entries
		 WHERE entry_type = 'CONTRIBUTION' AND created_at >= $1`,
		cutoff,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("get rolling 90-day contributions: %w", err)
	}
	return int64(total * 100), nil
}

// GetEntries returns paginated guarantee fund ledger entries (newest first).
func (r *Repository) GetEntries(ctx context.Context, limit, offset int) ([]Entry, int, error) {
	if limit <= 0 {
		limit = 20
	}

	var total int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM guarantee_fund_entries`,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count entries: %w", err)
	}

	rows, err := r.pool.Query(ctx,
		`SELECT id, transaction_id, entry_type,
		        ROUND(amount * 100)::bigint,
		        ROUND(balance_after * 100)::bigint,
		        created_at
		 FROM guarantee_fund_entries
		 ORDER BY created_at DESC
		 LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("query entries: %w", err)
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var entryType string
		if err := rows.Scan(&e.ID, &e.TransactionID, &entryType, &e.Amount, &e.BalanceAfter, &e.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan entry: %w", err)
		}
		e.EntryType = EntryType(entryType)
		entries = append(entries, e)
	}
	return entries, total, rows.Err()
}

// BeginTx starts a new database transaction.
func (r *Repository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	return tx, nil
}
