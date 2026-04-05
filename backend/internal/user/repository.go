package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a user record does not exist.
var ErrNotFound = errors.New("user not found")

// ErrEmailTaken is returned when a registration email is already in use.
var ErrEmailTaken = errors.New("email already registered")

// Repository handles all Postgres operations for the user domain.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given connection pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Insert persists a new user row and returns the inserted record.
// Returns ErrEmailTaken if a row with the same email already exists.
func (r *Repository) Insert(ctx context.Context, u *User, passwordHash string) (*User, error) {
	const q = `
		INSERT INTO users (id, email, phone, password_hash, name, identity_status, reputation_score,
		                   notification_preferences, created_at, last_active_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, email, phone, name, avatar_url, identity_status, reputation_score,
		          notification_preferences, created_at, last_active_at`

	notifPrefs := u.NotificationPreferences
	if notifPrefs == nil {
		notifPrefs = json.RawMessage("{}")
	}

	row := r.pool.QueryRow(ctx, q,
		u.ID, u.Email, u.Phone, passwordHash, u.Name,
		string(u.IdentityStatus), u.ReputationScore, notifPrefs,
		u.CreatedAt, u.LastActiveAt,
	)

	out, err := scanUser(row)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrEmailTaken
		}
		return nil, fmt.Errorf("inserting user: %w", err)
	}
	return out, nil
}

// FindByID retrieves a user by primary key. Returns ErrNotFound if absent.
func (r *Repository) FindByID(ctx context.Context, id string) (*User, error) {
	const q = `
		SELECT id, email, phone, name, avatar_url, identity_status, reputation_score,
		       notification_preferences, created_at, last_active_at
		FROM users WHERE id = $1`

	u, err := scanUser(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("finding user by id: %w", err)
	}
	return u, nil
}

// FindByEmail retrieves a user and password hash by email.
// Returns ErrNotFound if absent.
func (r *Repository) FindByEmail(ctx context.Context, email string) (*User, string, error) {
	const q = `
		SELECT id, email, phone, name, avatar_url, identity_status, reputation_score,
		       notification_preferences, created_at, last_active_at, COALESCE(password_hash, '')
		FROM users WHERE email = $1`

	row := r.pool.QueryRow(ctx, q, email)

	var (
		u    User
		em   *string
		ph   *string
		av   *string
		hash string
		np   []byte
	)
	err := row.Scan(
		&u.ID, &em, &ph, &u.Name, &av,
		&u.IdentityStatus, &u.ReputationScore,
		&np, &u.CreatedAt, &u.LastActiveAt, &hash,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", ErrNotFound
		}
		return nil, "", fmt.Errorf("finding user by email: %w", err)
	}
	u.Email = em
	u.Phone = ph
	u.AvatarURL = av
	u.NotificationPreferences = json.RawMessage(np)
	return &u, hash, nil
}

// UpdateLastActive sets last_active_at to now for the given user ID.
func (r *Repository) UpdateLastActive(ctx context.Context, id string) error {
	const q = `UPDATE users SET last_active_at = NOW() WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("updating last_active_at: %w", err)
	}
	return nil
}

// Update applies non-nil fields from UpdateInput to the user record.
func (r *Repository) Update(ctx context.Context, id string, in UpdateInput) (*User, error) {
	const q = `
		UPDATE users
		SET name                     = COALESCE($2, name),
		    avatar_url               = COALESCE($3, avatar_url),
		    notification_preferences = COALESCE($4, notification_preferences)
		WHERE id = $1
		RETURNING id, email, phone, name, avatar_url, identity_status, reputation_score,
		          notification_preferences, created_at, last_active_at`

	row := r.pool.QueryRow(ctx, q, id, in.Name, in.AvatarURL, in.NotificationPreferences)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("updating user: %w", err)
	}
	return u, nil
}

// UpdateIdentityStatus sets the identity_status for the given user ID.
func (r *Repository) UpdateIdentityStatus(ctx context.Context, id string, status IdentityStatus) error {
	const q = `UPDATE users SET identity_status = $2 WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id, string(status))
	if err != nil {
		return fmt.Errorf("updating identity status: %w", err)
	}
	return nil
}

// AddReputationScore adds delta (positive or negative) to the user's reputation_score,
// clamping the result to the [0, 1000] range.
func (r *Repository) AddReputationScore(ctx context.Context, id string, delta int) error {
	const q = `
		UPDATE users
		SET reputation_score = GREATEST(0, LEAST(1000, reputation_score + $2))
		WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id, delta)
	if err != nil {
		return fmt.Errorf("adding reputation score: %w", err)
	}
	return nil
}

// scanUser reads a user row from a pgx.Row or pgx.Rows scanner.
func scanUser(row pgx.Row) (*User, error) {
	var (
		u  User
		em *string
		ph *string
		av *string
		np []byte
	)
	err := row.Scan(
		&u.ID, &em, &ph, &u.Name, &av,
		&u.IdentityStatus, &u.ReputationScore,
		&np, &u.CreatedAt, &u.LastActiveAt,
	)
	if err != nil {
		return nil, err
	}
	u.Email = em
	u.Phone = ph
	u.AvatarURL = av
	u.NotificationPreferences = json.RawMessage(np)
	return &u, nil
}

// isUniqueViolation reports whether the error is a Postgres unique-constraint violation (23505).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "duplicate key")
}
