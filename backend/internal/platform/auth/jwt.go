// Package auth provides JWT issuance, validation, and HTTP middleware.
package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/oklog/ulid/v2"
)

// contextKey is an unexported type for context keys in this package.
type contextKey string

// UserIDKey is the context key under which the authenticated user ID is stored.
const UserIDKey contextKey = "userID"

// TokenPair holds an access token and a refresh token.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

// claims is the JWT payload for RentMy tokens.
type claims struct {
	UserID string `json:"uid"`
	jwt.RegisteredClaims
}

// Issuer creates and validates JWTs for a given secret.
type Issuer struct {
	secret        []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
}

// NewIssuer returns an Issuer configured with the provided secret and TTLs.
func NewIssuer(secret string, accessTTL, refreshTTL time.Duration) *Issuer {
	return &Issuer{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// Issue generates a new access/refresh token pair for the given user ID.
func (i *Issuer) Issue(userID string) (TokenPair, error) {
	now := time.Now()

	access, err := i.sign(userID, now, now.Add(i.accessTTL))
	if err != nil {
		return TokenPair{}, fmt.Errorf("signing access token: %w", err)
	}

	refresh, err := i.sign(userID, now, now.Add(i.refreshTTL))
	if err != nil {
		return TokenPair{}, fmt.Errorf("signing refresh token: %w", err)
	}

	return TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}

// Validate parses and validates a JWT string, returning the user ID on success.
func (i *Issuer) Validate(tokenStr string) (string, error) {
	tok, err := jwt.ParseWithClaims(tokenStr, &claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return i.secret, nil
	})
	if err != nil {
		return "", fmt.Errorf("parsing token: %w", err)
	}

	c, ok := tok.Claims.(*claims)
	if !ok || !tok.Valid {
		return "", fmt.Errorf("invalid token claims")
	}

	return c.UserID, nil
}

// sign creates a signed JWT string. A random ULID jti is embedded so that
// tokens issued within the same second are distinct (NumericDate is second-precision).
func (i *Issuer) sign(userID string, issuedAt, expiresAt time.Time) (string, error) {
	c := &claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        ulid.Make().String(),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return tok.SignedString(i.secret)
}

// UserIDFromContext extracts the authenticated user ID from a context.
// Returns empty string if not present.
func UserIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(UserIDKey).(string)
	return id
}
