package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/giits/rentmy/backend/internal/platform/ulid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles persistence for notifications and push tokens.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository backed by pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Insert persists a new notification.
func (r *Repository) Insert(ctx context.Context, n Notification) error {
	if n.ID == "" {
		n.ID = ulid.New()
	}
	dataJSON := n.Data
	if dataJSON == nil {
		dataJSON = json.RawMessage("{}")
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO notifications (id, user_id, type, title, body, data, read)
		 VALUES ($1, $2, $3, $4, $5, $6, FALSE)`,
		n.ID, n.UserID, string(n.Type), n.Title, n.Body, dataJSON,
	)
	if err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}
	return nil
}

// FindByUserID returns paginated notifications for a user, newest first.
// total is the count of all notifications for that user.
func (r *Repository) FindByUserID(ctx context.Context, userID string, limit, offset int) ([]Notification, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications WHERE user_id = $1`, userID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count notifications: %w", err)
	}

	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, type, title, body, data, read, created_at
		 FROM notifications
		 WHERE user_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("query notifications: %w", err)
	}
	defer rows.Close()

	var out []Notification
	for rows.Next() {
		var n Notification
		var t string
		if err := rows.Scan(&n.ID, &n.UserID, &t, &n.Title, &n.Body, &n.Data, &n.Read, &n.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan notification: %w", err)
		}
		n.Type = Type(t)
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate notifications: %w", err)
	}
	return out, total, nil
}

// MarkRead marks a single notification as read.
// Returns ErrNotificationNotFound if the notification does not belong to userID.
func (r *Repository) MarkRead(ctx context.Context, notificationID, userID string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE notifications SET read = TRUE WHERE id = $1 AND user_id = $2`,
		notificationID, userID,
	)
	if err != nil {
		return fmt.Errorf("mark notification read: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotificationNotFound
	}
	return nil
}

// MarkAllRead marks all of a user's notifications as read.
func (r *Repository) MarkAllRead(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE notifications SET read = TRUE WHERE user_id = $1 AND read = FALSE`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("mark all notifications read: %w", err)
	}
	return nil
}

// CountUnread returns the number of unread notifications for a user.
func (r *Repository) CountUnread(ctx context.Context, userID string) (int, error) {
	var count int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read = FALSE`, userID,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}
	return count, nil
}

// InsertPushToken saves an Expo push token for a user, ignoring conflicts on
// the token's unique constraint (idempotent registration).
func (r *Repository) InsertPushToken(ctx context.Context, userID, token string) error {
	id := ulid.New()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO push_tokens (id, user_id, token)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (token) DO NOTHING`,
		id, userID, token,
	)
	if err != nil {
		return fmt.Errorf("insert push token: %w", err)
	}
	return nil
}

// GetPushTokens returns all Expo push token strings registered for a user.
func (r *Repository) GetPushTokens(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT token FROM push_tokens WHERE user_id = $1`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query push tokens: %w", err)
	}
	defer rows.Close()

	var tokens []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, fmt.Errorf("scan push token: %w", err)
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// DeletePushToken removes a specific token (called when Expo returns DeviceNotRegistered).
func (r *Repository) DeletePushToken(ctx context.Context, token string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM push_tokens WHERE token = $1`, token,
	)
	if err != nil {
		return fmt.Errorf("delete push token: %w", err)
	}
	return nil
}

// GetUserPreferences reads notification_preferences JSONB from the users table.
// Returns DefaultPreferences() when no preferences have been saved yet.
func (r *Repository) GetUserPreferences(ctx context.Context, userID string) (Preferences, error) {
	var raw []byte
	err := r.pool.QueryRow(ctx,
		`SELECT notification_preferences FROM users WHERE id = $1`, userID,
	).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return DefaultPreferences(), nil
	}
	if err != nil {
		return Preferences{}, fmt.Errorf("get user preferences: %w", err)
	}
	if len(raw) == 0 || string(raw) == "{}" {
		return DefaultPreferences(), nil
	}
	var prefs Preferences
	if err := json.Unmarshal(raw, &prefs); err != nil {
		return DefaultPreferences(), nil
	}
	return prefs, nil
}

// UpdateUserPreferences writes notification preferences back to the users table.
func (r *Repository) UpdateUserPreferences(ctx context.Context, userID string, prefs Preferences) error {
	raw, err := json.Marshal(prefs)
	if err != nil {
		return fmt.Errorf("marshal preferences: %w", err)
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET notification_preferences = $1 WHERE id = $2`,
		raw, userID,
	)
	if err != nil {
		return fmt.Errorf("update user preferences: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("user not found: %s", userID)
	}
	return nil
}
