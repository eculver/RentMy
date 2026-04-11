package user

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"golang.org/x/crypto/bcrypt"

	"github.com/giits/rentmy/backend/internal/platform/auth"
	"github.com/giits/rentmy/backend/internal/platform/ulid"
)

// ErrBadCredentials is returned when login email/password do not match.
var ErrBadCredentials = errors.New("invalid email or password")

// RepositoryInterface declares the persistence operations required by Service.
type RepositoryInterface interface {
	Insert(ctx context.Context, u *User, passwordHash string) (*User, error)
	FindByID(ctx context.Context, id string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, string, error)
	UpdateLastActive(ctx context.Context, id string) error
	Update(ctx context.Context, id string, in UpdateInput) (*User, error)
}

// RedisStore is the subset of redis operations the Service needs for refresh tokens.
type RedisStore interface {
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, key string) error
}

// Service implements user registration, authentication, and profile management.
type Service struct {
	repo     RepositoryInterface
	issuer   *auth.Issuer
	redis    RedisStore
	validate *validator.Validate
}

// NewService constructs a Service backed by the concrete Repository.
func NewService(repo *Repository, issuer *auth.Issuer, redis RedisStore) *Service {
	return NewServiceWithInterfaces(repo, issuer, redis)
}

// NewServiceWithInterfaces constructs a Service with interface-typed dependencies,
// useful for testing with fakes.
func NewServiceWithInterfaces(repo RepositoryInterface, issuer *auth.Issuer, redis RedisStore) *Service {
	return &Service{
		repo:     repo,
		issuer:   issuer,
		redis:    redis,
		validate: validator.New(),
	}
}

// Register creates a new user account and returns auth tokens.
// Returns ErrEmailTaken if the email is already registered.
func (s *Service) Register(ctx context.Context, in RegisterInput) (*AuthResponse, error) {
	if err := s.validate.Struct(in); err != nil {
		return nil, fmt.Errorf("validation: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), 12)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	now := time.Now().UTC()
	u := &User{
		ID:             ulid.New(),
		Email:          &in.Email,
		Phone:          in.Phone,
		Name:           in.Name,
		IdentityStatus: IdentityStatusPending,
		ReputationScore: 0,
		CreatedAt:      now,
		LastActiveAt:   now,
	}

	inserted, err := s.repo.Insert(ctx, u, string(hash))
	if err != nil {
		return nil, err // ErrEmailTaken propagates directly
	}

	tokens, err := s.issuer.Issue(inserted.ID)
	if err != nil {
		return nil, fmt.Errorf("issuing tokens: %w", err)
	}

	if err := s.storeRefresh(ctx, inserted.ID, tokens.RefreshToken); err != nil {
		return nil, err
	}

	return &AuthResponse{User: inserted, AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken}, nil
}

// Login authenticates a user and returns auth tokens.
func (s *Service) Login(ctx context.Context, in LoginInput) (*AuthResponse, error) {
	if err := s.validate.Struct(in); err != nil {
		return nil, fmt.Errorf("validation: %w", err)
	}

	u, hash, err := s.repo.FindByEmail(ctx, in.Email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrBadCredentials
		}
		return nil, fmt.Errorf("finding user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)); err != nil {
		return nil, ErrBadCredentials
	}

	tokens, err := s.issuer.Issue(u.ID)
	if err != nil {
		return nil, fmt.Errorf("issuing tokens: %w", err)
	}

	if err := s.storeRefresh(ctx, u.ID, tokens.RefreshToken); err != nil {
		return nil, err
	}

	_ = s.repo.UpdateLastActive(ctx, u.ID)

	return &AuthResponse{User: u, AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken}, nil
}

// Refresh validates a refresh token and issues a new token pair.
func (s *Service) Refresh(ctx context.Context, in RefreshInput) (*AuthResponse, error) {
	if err := s.validate.Struct(in); err != nil {
		return nil, fmt.Errorf("validation: %w", err)
	}

	userID, err := s.issuer.Validate(in.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	stored, err := s.redis.Get(ctx, refreshKey(userID))
	if err != nil || stored != in.RefreshToken {
		return nil, fmt.Errorf("refresh token not recognised")
	}

	u, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("finding user: %w", err)
	}

	// Rotate: delete old, issue new.
	_ = s.redis.Del(ctx, refreshKey(userID))

	tokens, err := s.issuer.Issue(userID)
	if err != nil {
		return nil, fmt.Errorf("issuing tokens: %w", err)
	}

	if err := s.storeRefresh(ctx, userID, tokens.RefreshToken); err != nil {
		return nil, err
	}

	return &AuthResponse{User: u, AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken}, nil
}

// GetProfile returns the user record for the given ID.
func (s *Service) GetProfile(ctx context.Context, userID string) (*User, error) {
	u, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting profile: %w", err)
	}
	return u, nil
}

// UpdateProfile applies non-nil fields from in to the user record.
func (s *Service) UpdateProfile(ctx context.Context, userID string, in UpdateInput) (*User, error) {
	if err := s.validate.Struct(in); err != nil {
		return nil, fmt.Errorf("validation: %w", err)
	}

	u, err := s.repo.Update(ctx, userID, in)
	if err != nil {
		return nil, fmt.Errorf("updating profile: %w", err)
	}
	return u, nil
}

// storeRefresh stores a refresh token in Redis keyed by userID.
func (s *Service) storeRefresh(ctx context.Context, userID, token string) error {
	// TTL comes from the token itself; use a generous bound here so Redis
	// doesn't outlive a valid token by much.
	if err := s.redis.Set(ctx, refreshKey(userID), token, 8*24*time.Hour); err != nil {
		return fmt.Errorf("storing refresh token: %w", err)
	}
	return nil
}

func refreshKey(userID string) string {
	return "refresh:" + userID
}
