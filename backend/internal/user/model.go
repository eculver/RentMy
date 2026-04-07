// Package user implements the RentMy user domain: registration, login, and profile management.
package user

import (
	"encoding/json"
	"time"
)

// IdentityStatus represents the KYC verification state of a user.
type IdentityStatus string

const (
	IdentityStatusPending  IdentityStatus = "PENDING"
	IdentityStatusVerified IdentityStatus = "VERIFIED"
	IdentityStatusRejected IdentityStatus = "REJECTED"
)

// User is the domain representation of a RentMy account.
type User struct {
	ID                      string          `json:"id"`
	Email                   *string         `json:"email,omitempty"`
	Phone                   *string         `json:"phone,omitempty"`
	Name                    string          `json:"name"`
	AvatarURL               *string         `json:"avatarUrl,omitempty"`
	IdentityStatus          IdentityStatus  `json:"identityStatus"`
	ReputationScore         int             `json:"reputationScore"`
	NotificationPreferences json.RawMessage `json:"notificationPreferences"`
	CreatedAt               time.Time       `json:"createdAt"`
	LastActiveAt            time.Time       `json:"lastActiveAt"`
}

// RegisterInput is the request body for POST /api/v1/auth/register.
type RegisterInput struct {
	Email        string  `json:"email"        validate:"required,email"`
	Password     string  `json:"password"     validate:"required,min=8"`
	Name         string  `json:"name"         validate:"required,min=1,max=100"`
	Phone        *string `json:"phone"        validate:"omitempty,e164"`
	ReferralCode *string `json:"referralCode" validate:"omitempty"`
}

// LoginInput is the request body for POST /api/v1/auth/login.
type LoginInput struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// RefreshInput is the request body for POST /api/v1/auth/refresh.
type RefreshInput struct {
	RefreshToken string `json:"refreshToken" validate:"required"`
}

// UpdateInput is the request body for PUT /api/v1/users/me.
type UpdateInput struct {
	Name                    *string         `json:"name"                    validate:"omitempty,min=1,max=100"`
	AvatarURL               *string         `json:"avatarUrl"               validate:"omitempty,url"`
	NotificationPreferences json.RawMessage `json:"notificationPreferences" validate:"omitempty"`
}

// AuthResponse is returned on successful register/login/refresh.
type AuthResponse struct {
	User         *User  `json:"user"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}
