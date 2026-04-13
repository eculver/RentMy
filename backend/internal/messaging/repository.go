package messaging

import (
	"context"
	"errors"
	"fmt"

	"github.com/Brett2thered/RentMy/backend/internal/platform/ulid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles persistence for messages.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository backed by pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Insert persists a new message and returns it with a populated ID and CreatedAt.
func (r *Repository) Insert(ctx context.Context, m Message) (Message, error) {
	if m.ID == "" {
		m.ID = ulid.New()
	}
	err := r.pool.QueryRow(ctx,
		`INSERT INTO messages (id, transaction_id, sender_id, content)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, transaction_id, sender_id, content, created_at`,
		m.ID, m.TransactionID, m.SenderID, m.Content,
	).Scan(&m.ID, &m.TransactionID, &m.SenderID, &m.Content, &m.CreatedAt)
	if err != nil {
		return Message{}, fmt.Errorf("insert message: %w", err)
	}
	return m, nil
}

// FindByTransactionID returns messages for a transaction in chronological order
// (oldest first). cursor is a message ULID; if non-empty, only messages with
// id > cursor are returned (exclusive). nextCursor is the ULID of the last
// returned message, or "" if no more pages exist.
func (r *Repository) FindByTransactionID(ctx context.Context, transactionID, cursor string, limit int) ([]Message, string, error) {
	var (
		rows pgx.Rows
		err  error
	)

	if cursor == "" {
		rows, err = r.pool.Query(ctx,
			`SELECT id, transaction_id, sender_id, content, created_at
			 FROM messages
			 WHERE transaction_id = $1
			 ORDER BY id ASC
			 LIMIT $2`,
			transactionID, limit,
		)
	} else {
		rows, err = r.pool.Query(ctx,
			`SELECT id, transaction_id, sender_id, content, created_at
			 FROM messages
			 WHERE transaction_id = $1 AND id > $2
			 ORDER BY id ASC
			 LIMIT $3`,
			transactionID, cursor, limit,
		)
	}
	if err != nil {
		return nil, "", fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.TransactionID, &m.SenderID, &m.Content, &m.CreatedAt); err != nil {
			return nil, "", fmt.Errorf("scan message: %w", err)
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterate messages: %w", err)
	}

	var nextCursor string
	if len(msgs) == limit {
		nextCursor = msgs[len(msgs)-1].ID
	}

	return msgs, nextCursor, nil
}

// GetParties returns the renter_id and host_id for a transaction.
// Returns ErrTransactionNotFound if no row matches.
func (r *Repository) GetParties(ctx context.Context, transactionID string) (Parties, error) {
	var p Parties
	err := r.pool.QueryRow(ctx,
		`SELECT renter_id, host_id FROM transactions WHERE id = $1`,
		transactionID,
	).Scan(&p.RenterID, &p.HostID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Parties{}, ErrTransactionNotFound
		}
		return Parties{}, fmt.Errorf("get transaction parties: %w", err)
	}
	return p, nil
}
