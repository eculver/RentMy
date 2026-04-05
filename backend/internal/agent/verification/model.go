// Package verification implements the VerificationAgent, which orchestrates KYC
// using Stripe Identity and routes edge cases to Claude Haiku for interpretation.
package verification

import "time"

// VerificationStatus represents the outcome state of a KYC attempt.
type VerificationStatus string

const (
	VerificationStatusPending   VerificationStatus = "PENDING"
	VerificationStatusVerified  VerificationStatus = "VERIFIED"
	VerificationStatusRejected  VerificationStatus = "REJECTED"
	VerificationStatusEscalated VerificationStatus = "ESCALATED"
	VerificationStatusCanceled  VerificationStatus = "CANCELED"
)

// VerificationAttempt records a single KYC session for a user.
type VerificationAttempt struct {
	ID               string
	UserID           string
	StripeSessionID  string
	Status           VerificationStatus
	StripeStatus     string
	StripeReason     string
	DocumentType     string
	SelfieMatchScore *float64
	FraudIndicators  []string
	Decision         *string
	Confidence       *float64
	EscalationReason *string
	Model            *string
	PromptVersion    *string
	RetryCount       int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// StartVerificationResult is returned by StartVerification.
type StartVerificationResult struct {
	SessionID          string `json:"sessionId"`
	SessionURL         string `json:"sessionUrl"`
	EphemeralKeySecret string `json:"ephemeralKeySecret,omitempty"` // Stripe client_secret; empty for idempotent returns
}

// VerificationStatusResult is returned by GetStatus.
type VerificationStatusResult struct {
	Status           VerificationStatus `json:"status"`
	IdentityStatus   string             `json:"identityStatus"`
	EscalationReason *string            `json:"escalationReason,omitempty"`
}

// updateStatusInput carries the fields updated when a webhook is processed.
type updateStatusInput struct {
	Status           VerificationStatus
	StripeStatus     string
	StripeReason     string
	DocumentType     string
	SelfieMatchScore *float64
	FraudIndicators  []string
	Decision         *string
	Confidence       *float64
	EscalationReason *string
	Model            *string
	PromptVersion    *string
}

// interpretationInput is the template data for the verification/v1.txt prompt.
type interpretationInput struct {
	UserID           string
	SessionID        string
	StripeStatus     string
	StripeReason     string
	DocumentType     string
	SelfieMatchScore string
	FraudIndicators  string
}

// interpretationResult is the parsed JSON from the KYC interpretation prompt.
type interpretationResult struct {
	Decision         string  `json:"decision"`
	Confidence       float64 `json:"confidence"`
	Reasoning        string  `json:"reasoning"`
	EscalationReason string  `json:"escalation_reason"`
}
