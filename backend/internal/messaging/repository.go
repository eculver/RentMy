package messaging

import (
	"context"
	"errors"
	"fmt"

	"github.com/giits/rentmy/backend/internal/platform/ulid"
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

// FindByTransactionID returns messages for a transaction in reverse-chronological
// page order (newest page first, oldest page last) with messages within each page
// sorted oldest-first. This supports "load older messages on scroll-to-top" UX.
//
// When cursor is empty the most-recent `limit` messages are returned.
// When cursor is a message ULID, the `limit` messages older than that cursor are
// returned. nextCursor is the ULID of the oldest message in the page, used as
// the cursor for the next (older) page. It is "" when no further pages exist.
func (r *Repository) FindByTransactionID(ctx context.Context, transactionID, cursor string, limit int) ([]Message, string, error) {
	var (
		rows pgx.Rows
		err  error
	)

	// Fetch in DESC order so we get the correct page boundary, then reverse
	// in Go to return messages oldest-first within the page.
	if cursor == "" {
		rows, err = r.pool.Query(ctx,
			`SELECT id, transaction_id, sender_id, content, created_at
			 FROM messages
			 WHERE transaction_id = $1
			 ORDER BY id DESC
			 LIMIT $2`,
			transactionID, limit,
		)
	} else {
		rows, err = r.pool.Query(ctx,
			`SELECT id, transaction_id, sender_id, content, created_at
			 FROM messages
			 WHERE transaction_id = $1 AND id < $2
			 ORDER BY id DESC
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

	// nextCursor is the ID of the oldest message returned (last in DESC order),
	// which becomes the cursor for the next older page.
	var nextCursor string
	if len(msgs) == limit {
		nextCursor = msgs[len(msgs)-1].ID
	}

	// Reverse to chronological order (oldest first) for display.
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}

	return msgs, nextCursor, nil
}

// GetConversations returns all booking threads involving the given user,
// ordered by most-recently-active first. Each row includes the last message
// snippet and the count of unread NEW_MESSAGE notifications for that thread.
func (r *Repository) GetConversations(ctx context.Context, userID string) ([]Conversation, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT
			t.id,
			CASE WHEN t.renter_id = $1 THEN t.host_id ELSE t.renter_id END AS other_party_id,
			other_user.name AS other_party_name,
			l.title AS listing_title,
			last_msg.content AS last_message,
			last_msg.created_at AS last_message_at,
			COALESCE(unread.cnt, 0) AS unread_count,
			t.status AS booking_status
		FROM transactions t
		JOIN listings l ON l.id = t.listing_id
		JOIN users other_user
			ON other_user.id = CASE WHEN t.renter_id = $1 THEN t.host_id ELSE t.renter_id END
		LEFT JOIN LATERAL (
			SELECT content, created_at
			FROM messages
			WHERE transaction_id = t.id
			ORDER BY id DESC
			LIMIT 1
		) last_msg ON true
		LEFT JOIN LATERAL (
			SELECT COUNT(*)::int AS cnt
			FROM notifications
			WHERE user_id = $1
			  AND type = 'NEW_MESSAGE'
			  AND read = false
			  AND data->>'transactionId' = t.id
		) unread ON true
		WHERE t.renter_id = $1 OR t.host_id = $1
		ORDER BY last_msg.created_at DESC NULLS LAST, t.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query conversations: %w", err)
	}
	defer rows.Close()

	var convs []Conversation
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(
			&c.TransactionID,
			&c.OtherPartyID,
			&c.OtherPartyName,
			&c.ListingTitle,
			&c.LastMessage,
			&c.LastMessageAt,
			&c.UnreadCount,
			&c.BookingStatus,
		); err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}
		convs = append(convs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate conversations: %w", err)
	}

	if convs == nil {
		convs = []Conversation{}
	}
	return convs, nil
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
