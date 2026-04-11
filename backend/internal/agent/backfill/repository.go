package backfill

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles the read-only queries needed by the backfill workers.
// All writes are delegated to the domain services (AppraisalAgent, RiskAgent).
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given connection pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// FindListingsNeedingAppraisal returns IDs of all listings where AI-generated
// tags are absent (NULL or empty array) OR estimated_value is NULL.
// These are candidates for the appraisal backfill job.
func (r *Repository) FindListingsNeedingAppraisal(ctx context.Context) ([]string, error) {
	const q = `
		SELECT id
		FROM listings
		WHERE ai_generated_tags IS NULL
		   OR ai_generated_tags::text = '[]'
		   OR estimated_value IS NULL
		ORDER BY created_at ASC`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("backfill: query listings needing appraisal: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("backfill: scan listing id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// FindUsersWithTransactions returns IDs of all users who have appeared as
// either a renter or host in at least one transaction. These users need
// their reputation scores backfilled.
func (r *Repository) FindUsersWithTransactions(ctx context.Context) ([]string, error) {
	const q = `
		SELECT DISTINCT u.id
		FROM users u
		WHERE EXISTS (
			SELECT 1 FROM transactions t
			WHERE t.renter_id = u.id OR t.host_id = u.id
		)
		ORDER BY u.id`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("backfill: query users with transactions: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("backfill: scan user id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// FindTransactionsWithoutRiskScore returns stubs for all transactions that do
// not yet have a row in the risk_scores table. These are scored for analytics.
func (r *Repository) FindTransactionsWithoutRiskScore(ctx context.Context) ([]transactionStub, error) {
	const q = `
		SELECT t.id, t.renter_id, t.host_id
		FROM transactions t
		LEFT JOIN risk_scores rs ON rs.transaction_id = t.id
		WHERE rs.transaction_id IS NULL
		ORDER BY t.created_at ASC`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("backfill: query transactions without risk score: %w", err)
	}
	defer rows.Close()

	var txns []transactionStub
	for rows.Next() {
		var tx transactionStub
		if err := rows.Scan(&tx.ID, &tx.RenterID, &tx.HostID); err != nil {
			return nil, fmt.Errorf("backfill: scan transaction: %w", err)
		}
		txns = append(txns, tx)
	}
	return txns, rows.Err()
}
