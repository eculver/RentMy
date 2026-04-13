package rating

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Brett2thered/RentMy/backend/internal/platform/ulid"
)

// Repository handles persistence for the rating domain.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// transactionRow is a lightweight projection of the transactions table used
// for rating validation.
type transactionRow struct {
	RenterID string
	HostID   string
	Status   string
}

// FindTransactionForRating fetches the renter_id, host_id, and status fields
// of a transaction. Returns ErrTransactionNotFound when the row is missing.
func (r *Repository) FindTransactionForRating(ctx context.Context, txnID string) (transactionRow, error) {
	const q = `SELECT renter_id, host_id, status FROM transactions WHERE id = $1`
	var row transactionRow
	err := r.pool.QueryRow(ctx, q, txnID).Scan(&row.RenterID, &row.HostID, &row.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return transactionRow{}, ErrTransactionNotFound
	}
	if err != nil {
		return transactionRow{}, fmt.Errorf("rating: find transaction: %w", err)
	}
	return row, nil
}

// HasUserRated returns true when (transaction_id, from_user_id) already exists
// in the ratings table.
func (r *Repository) HasUserRated(ctx context.Context, txnID, fromUserID string) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM ratings WHERE transaction_id = $1 AND from_user_id = $2)`
	var exists bool
	if err := r.pool.QueryRow(ctx, q, txnID, fromUserID).Scan(&exists); err != nil {
		return false, fmt.Errorf("rating: has user rated: %w", err)
	}
	return exists, nil
}

// Insert persists a new rating. Returns ErrAlreadyRated when the UNIQUE
// constraint (transaction_id, from_user_id) is violated.
func (r *Repository) Insert(ctx context.Context, rating *Rating) error {
	if rating.ID == "" {
		rating.ID = ulid.New()
	}
	const q = `
		INSERT INTO ratings (id, transaction_id, from_user_id, to_user_id, bubbles)
		VALUES ($1, $2, $3, $4, $5::jsonb)`

	bubblesJSON, err := marshalBubbles(rating.Bubbles)
	if err != nil {
		return fmt.Errorf("rating: marshal bubbles: %w", err)
	}

	_, err = r.pool.Exec(ctx, q,
		rating.ID, rating.TransactionID,
		rating.FromUserID, rating.ToUserID,
		bubblesJSON,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrAlreadyRated
		}
		return fmt.Errorf("rating: insert: %w", err)
	}
	return nil
}

// FindByTransactionID returns all ratings for a transaction.
func (r *Repository) FindByTransactionID(ctx context.Context, txnID string) ([]Rating, error) {
	const q = `
		SELECT id, transaction_id, from_user_id, to_user_id, bubbles, created_at
		FROM ratings
		WHERE transaction_id = $1
		ORDER BY created_at ASC`
	return r.scanRatings(ctx, q, txnID)
}

// FindByToUserID returns paginated ratings received by a user.
func (r *Repository) FindByToUserID(ctx context.Context, userID string, limit, offset int) ([]Rating, int, error) {
	if limit <= 0 {
		limit = 20
	}

	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM ratings WHERE to_user_id = $1`, userID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("rating: count ratings for user: %w", err)
	}

	const q = `
		SELECT id, transaction_id, from_user_id, to_user_id, bubbles, created_at
		FROM ratings
		WHERE to_user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("rating: find ratings for user: %w", err)
	}
	defer rows.Close()

	ratings, err := scanRowsToRatings(rows)
	if err != nil {
		return nil, 0, err
	}
	return ratings, total, nil
}

// BubbleSummary aggregates bubble counts for a user's received ratings.
func (r *Repository) BubbleSummary(ctx context.Context, userID string) ([]BubbleSummaryItem, error) {
	const q = `
		SELECT bubble, COUNT(*) AS cnt
		FROM ratings, jsonb_array_elements_text(bubbles) AS bubble
		WHERE to_user_id = $1
		GROUP BY bubble
		ORDER BY cnt DESC`

	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("rating: bubble summary: %w", err)
	}
	defer rows.Close()

	var items []BubbleSummaryItem
	for rows.Next() {
		var item BubbleSummaryItem
		var bubbleStr string
		if err := rows.Scan(&bubbleStr, &item.Count); err != nil {
			return nil, fmt.Errorf("rating: scan bubble summary row: %w", err)
		}
		item.Bubble = Bubble(bubbleStr)
		items = append(items, item)
	}
	return items, rows.Err()
}

// scanRatings is a helper that runs q with a single arg and scans all rows.
func (r *Repository) scanRatings(ctx context.Context, q, arg string) ([]Rating, error) {
	rows, err := r.pool.Query(ctx, q, arg)
	if err != nil {
		return nil, fmt.Errorf("rating: query: %w", err)
	}
	defer rows.Close()
	return scanRowsToRatings(rows)
}

func scanRowsToRatings(rows pgx.Rows) ([]Rating, error) {
	var ratings []Rating
	for rows.Next() {
		var rt Rating
		var bubblesJSON []byte
		if err := rows.Scan(
			&rt.ID, &rt.TransactionID,
			&rt.FromUserID, &rt.ToUserID,
			&bubblesJSON, &rt.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("rating: scan row: %w", err)
		}
		bubbles, err := unmarshalBubbles(bubblesJSON)
		if err != nil {
			return nil, fmt.Errorf("rating: unmarshal bubbles: %w", err)
		}
		rt.Bubbles = bubbles
		ratings = append(ratings, rt)
	}
	return ratings, rows.Err()
}
