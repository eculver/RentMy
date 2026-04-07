// Package referral implements the RentMy referral growth system.
// Users generate a unique code and share it; when a referee completes their
// first rental both parties receive a $20 platform credit via Stripe transfer.
package referral

import "time"

// ReferralCodeStatus constants.
const (
	ReferralPayoutAmount = 2000 // cents ($20.00)
)

// ReferralStatus tracks the lifecycle of a referral relationship.
type ReferralStatus string

const (
	ReferralStatusSignedUp              ReferralStatus = "SIGNED_UP"
	ReferralStatusFirstRentalCompleted  ReferralStatus = "FIRST_RENTAL_COMPLETED"
	ReferralStatusPaid                  ReferralStatus = "PAID"
	ReferralStatusFraudulent            ReferralStatus = "FRAUDULENT"
)

// PayoutStatus tracks the lifecycle of a single payout.
type PayoutStatus string

const (
	PayoutStatusPending    PayoutStatus = "PENDING"
	PayoutStatusProcessing PayoutStatus = "PROCESSING"
	PayoutStatusPaid       PayoutStatus = "PAID"
	PayoutStatusFailed     PayoutStatus = "FAILED"
	PayoutStatusBlocked    PayoutStatus = "BLOCKED"
)

// ReferralCode is the shareable code owned by a user.
type ReferralCode struct {
	ID        string     `json:"id"`
	Code      string     `json:"code"`
	UserID    string     `json:"userId"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	MaxUses   int        `json:"maxUses"`  // 0 = unlimited
	UseCount  int        `json:"useCount"`
	CreatedAt time.Time  `json:"createdAt"`
}

// Referral records the relationship between a referrer and a referee.
type Referral struct {
	ID             string         `json:"id"`
	ReferralCodeID string         `json:"referralCodeId"`
	ReferrerID     string         `json:"referrerId"`
	RefereeID      string         `json:"refereeId"`
	Status         ReferralStatus `json:"status"`
	ReferrerPayout int            `json:"referrerPayout"` // cents
	RefereePayout  int            `json:"refereePayout"`  // cents
	CompletedAt    *time.Time     `json:"completedAt,omitempty"`
	PaidAt         *time.Time     `json:"paidAt,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
}

// ReferralPayout is an individual payout record for one party of a referral.
type ReferralPayout struct {
	ID               string       `json:"id"`
	ReferralID       string       `json:"referralId"`
	UserID           string       `json:"userId"`
	Amount           int          `json:"amount"` // cents
	Status           PayoutStatus `json:"status"`
	StripeTransferID *string      `json:"stripeTransferId,omitempty"`
	CreatedAt        time.Time    `json:"createdAt"`
}

// ReferralStats is returned by the ops summary endpoint.
type ReferralStats struct {
	Total          int     `json:"total"`
	SignedUp       int     `json:"signedUp"`
	Converted      int     `json:"converted"` // FIRST_RENTAL_COMPLETED + PAID
	Paid           int     `json:"paid"`
	Fraudulent     int     `json:"fraudulent"`
	ConversionRate float64 `json:"conversionRate"` // converted / total
	TotalPayoutCents int   `json:"totalPayoutCents"`
}

// ListReferralsFilter is used by the ops paginated list endpoint.
type ListReferralsFilter struct {
	Page  int
	Limit int
}
