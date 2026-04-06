package photodiff

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles persistence for photo diff results on the transactions table.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given connection pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// UpdateDiffResult stores the photo diff result and confidence on the transaction row.
func (r *Repository) UpdateDiffResult(ctx context.Context, transactionID string, result DiffResult, confidence float64) error {
	const q = `
		UPDATE transactions
		SET photo_diff_result = $2, photo_diff_confidence = $3
		WHERE id = $1`

	tag, err := r.pool.Exec(ctx, q, transactionID, string(result), confidence)
	if err != nil {
		return fmt.Errorf("update photo diff result: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("transaction %s not found", transactionID)
	}
	return nil
}

// GetDiffResult reads the photo diff result for a transaction.
func (r *Repository) GetDiffResult(ctx context.Context, transactionID string) (*PhotoDiff, error) {
	const q = `
		SELECT id, photo_diff_result, photo_diff_confidence, created_at
		FROM transactions
		WHERE id = $1`

	var (
		txnID      string
		result     *string
		confidence *float64
		createdAt  interface{}
	)
	err := r.pool.QueryRow(ctx, q, transactionID).Scan(&txnID, &result, &confidence, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("get photo diff result: %w", err)
	}

	if result == nil {
		return nil, nil // No diff has been run yet.
	}

	pd := &PhotoDiff{
		TransactionID: txnID,
		Result:        DiffResult(*result),
	}
	if confidence != nil {
		pd.Confidence = *confidence
	}
	return pd, nil
}
