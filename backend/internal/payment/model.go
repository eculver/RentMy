// Package payment implements RentMy payment processing: tiered holds, rental fees,
// host payouts, Stripe Express Connect onboarding, and guarantee fund accounting.
package payment

import (
	"context"
	"errors"
	"time"
)

// Sentinel errors returned by the payment service and repository.
var (
	ErrListingNotFound     = errors.New("listing not found")
	ErrTransactionNotFound = errors.New("transaction not found")
	ErrNoPaymentMethod     = errors.New("renter has no payment method set up")
	ErrNoStripeAccount     = errors.New("host has no Stripe account")
	ErrInsufficientHold    = errors.New("capture amount exceeds remaining hold")
	ErrInvalidCapture      = errors.New("invalid capture reason")
)

// PaymentAdapter defines the interface for payment processing operations.
// Stripe is the default implementation; the interface allows future swapping.
type PaymentAdapter interface {
	// AuthorizeHold places a manual-capture PaymentIntent (pre-authorization hold).
	// Returns the PaymentIntent ID (holdID).
	AuthorizeHold(ctx context.Context, amount int64, currency string, paymentMethodID string, customerID string) (holdID string, err error)

	// CaptureHold captures a portion of an existing hold.
	// Returns the Charge ID.
	CaptureHold(ctx context.Context, holdID string, amount int64) (chargeID string, err error)

	// ReleaseHold cancels/releases an existing hold.
	ReleaseHold(ctx context.Context, holdID string) error

	// ChargeRentalFee charges the rental fee immediately (automatic capture).
	// Returns the Charge ID.
	ChargeRentalFee(ctx context.Context, amount int64, currency string, paymentMethodID string, customerID string) (chargeID string, err error)

	// PayoutHost transfers funds to the host's connected account.
	// Returns the Transfer ID.
	PayoutHost(ctx context.Context, amount int64, currency string, hostAccountID string) (payoutID string, err error)

	// Refund issues a refund for a previous charge.
	// Returns the Refund ID.
	Refund(ctx context.Context, chargeID string, amount int64) (refundID string, err error)

	// CreateConnectedAccount creates a Stripe Express connected account for a host.
	// Returns the account ID and the onboarding URL.
	CreateConnectedAccount(ctx context.Context, email string) (accountID string, onboardingURL string, err error)

	// CreateCustomer creates a Stripe customer for a renter.
	// Returns the customer ID.
	CreateCustomer(ctx context.Context, email string, name string) (customerID string, err error)

	// CreateSetupIntent creates a SetupIntent for saving a payment method.
	// Returns the client secret for the mobile SDK.
	CreateSetupIntent(ctx context.Context, customerID string) (clientSecret string, err error)
}

// HoldAllocation tracks how the hold authorized at booking has been used.
// Stored as JSONB in the transactions.hold_allocation column.
type HoldAllocation struct {
	TotalAuthorized      int64 `json:"totalAuthorized"`
	CapturedForLateFees  int64 `json:"capturedForLateFees"`
	CapturedForDamage    int64 `json:"capturedForDamage"`
	DamageReserve        int64 `json:"damageReserve"`
	Released             int64 `json:"released"`
	Remaining            int64 `json:"remaining"`
}

// CaptureReason classifies why a hold capture is being requested.
type CaptureReason string

const (
	CaptureReasonLateFee CaptureReason = "late_fee"
	CaptureReasonDamage  CaptureReason = "damage"
)

// BookingInput is the input for creating a new booking.
type BookingInput struct {
	RenterID        string
	ListingID       string
	PaymentMethodID string
	ScheduledStart  time.Time
	ScheduledEnd    time.Time
}

// BookingResult is returned after a successful booking is created.
type BookingResult struct {
	TransactionID        string `json:"transactionId"`
	HoldAmount           int64  `json:"holdAmount"`
	RentalFee            int64  `json:"rentalFee"`
	PlatformFee          int64  `json:"platformFee"`
	HostPayout           int64  `json:"hostPayout"`
	GuaranteeGap         int64  `json:"guaranteeGap"`
	GuaranteeContribution int64  `json:"guaranteeContribution"`
	TotalImpact          int64  `json:"totalImpact"` // holdAmount + rentalFee
}

// GuaranteeFundEntry represents a ledger entry in the guarantee fund.
type GuaranteeFundEntry struct {
	ID            string    `json:"id"`
	TransactionID string    `json:"transactionId"`
	EntryType     string    `json:"entryType"` // CONTRIBUTION, CLAIM, CARD_RECOVERY, COLLECTIONS_REFERRAL
	Amount        int64     `json:"amount"`    // cents; positive = in, negative = out
	BalanceAfter  int64     `json:"balanceAfter"`
	CreatedAt     time.Time `json:"createdAt"`
}

// GuaranteeFundHealth summarizes the guarantee fund financial state.
type GuaranteeFundHealth struct {
	Balance              int64   `json:"balance"`
	OutstandingGaps      int64   `json:"outstandingGaps"`
	ReserveRatio         float64 `json:"reserveRatio"` // balance / outstandingGaps
}

// ListingSnapshot holds the listing fields needed to compute booking amounts.
type ListingSnapshot struct {
	ID                string
	HostID            string
	PricePerHour      *float64
	PricePerDay       *float64
	HostDeclaredValue *float64
	EstimatedValue    *float64
}

// Transaction is the domain representation of a RentMy booking transaction.
type Transaction struct {
	ID                      string         `json:"id"`
	RenterID                string         `json:"renterId"`
	HostID                  string         `json:"hostId"`
	ListingID               string         `json:"listingId"`
	RentalFee               int64          `json:"rentalFee"`
	HoldAmount              int64          `json:"holdAmount"`
	ItemValue               int64          `json:"itemValue"`
	GuaranteeGap            int64          `json:"guaranteeGap"`
	PlatformFee             int64          `json:"platformFee"`
	HostPayout              int64          `json:"hostPayout"`
	GuaranteeContribution   int64          `json:"guaranteeContribution"`
	EscrowStatus            string         `json:"escrowStatus"`
	HoldStatus              string         `json:"holdStatus"`
	HoldAllocation          HoldAllocation `json:"holdAllocation"`
	StripePaymentIntentID   string         `json:"stripePaymentIntentId,omitempty"`
	StripeChargeID          string         `json:"stripeChargeId,omitempty"`
	StripeTransferID        string         `json:"stripeTransferId,omitempty"`
	ScheduledStart          time.Time      `json:"scheduledStart"`
	ScheduledEnd            time.Time      `json:"scheduledEnd"`
	Status                  string         `json:"status"`
	CreatedAt               time.Time      `json:"createdAt"`
}

// OnboardHostResult is returned after a host's Stripe Express account is created.
type OnboardHostResult struct {
	AccountID     string `json:"accountId"`
	OnboardingURL string `json:"onboardingUrl"`
}

// SetupPaymentResult is returned after a renter's payment setup is created.
type SetupPaymentResult struct {
	CustomerID   string `json:"customerId"`
	ClientSecret string `json:"clientSecret"`
}

// HoldEstimateResult is the response for the hold-estimate endpoint.
type HoldEstimateResult struct {
	ItemValue    int64 `json:"itemValue"`
	HoldAmount   int64 `json:"holdAmount"`
	GuaranteeGap int64 `json:"guaranteeGap"`
}
