// Package booking implements the RentMy booking lifecycle: state machine transitions,
// fraud velocity checks, cancellation fee calculation, and auto-decline scheduling.
package booking

import (
	"errors"
	"time"
)

// Sentinel errors for the booking domain.
var (
	ErrBookingNotFound      = errors.New("booking not found")
	ErrInvalidTransition    = errors.New("invalid status transition")
	ErrNotAuthorized        = errors.New("not authorized for this booking action")
	ErrBookingNotRequested  = errors.New("booking is not in REQUESTED status")
	ErrListingNotAvailable  = errors.New("listing is not available for the requested period")
	ErrFraudBlocked         = errors.New("booking blocked by fraud velocity rules")
	ErrDurationExceedsLimit = errors.New("booking duration exceeds 7-day maximum")
	ErrHandoffIncomplete    = errors.New("handoff incomplete — both parties must complete proximity verification")
)

// Status represents the lifecycle state of a booking.
type Status string

const (
	StatusRequested    Status = "REQUESTED"
	StatusAccepted     Status = "ACCEPTED"
	StatusDeclined     Status = "DECLINED"
	StatusAutoDeclined Status = "AUTO_DECLINED"
	StatusActive       Status = "ACTIVE"
	StatusCompleted    Status = "COMPLETED"
	StatusDisputed     Status = "DISPUTED"
	StatusCancelled    Status = "CANCELLED"
)

// MaxDuration is the 7-day ceiling enforced by PRD section 7.
const MaxDuration = 7 * 24 * time.Hour

// Booking is the booking domain view of a transaction row.
// Financial fields are owned by the payment domain.
type Booking struct {
	ID              string     `json:"id"`
	RenterID        string     `json:"renterId"`
	HostID          string     `json:"hostId"`
	ListingID       string     `json:"listingId"`
	ScheduledStart  time.Time  `json:"scheduledStart"`
	ScheduledEnd    time.Time  `json:"scheduledEnd"`
	Status          Status     `json:"status"`
	CancelledBy     string     `json:"cancelledBy,omitempty"`
	CancellationFee int64      `json:"cancellationFee,omitempty"` // cents
	ActualStart     *time.Time `json:"actualStart,omitempty"`
	ActualEnd       *time.Time `json:"actualEnd,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
}

// CreateInput is the input for creating a new booking.
type CreateInput struct {
	RenterID        string
	ListingID       string
	PaymentMethodID string
	ScheduledStart  time.Time
	ScheduledEnd    time.Time
}

// AcceptInput is the input for accepting a booking.
type AcceptInput struct {
	BookingID string
	HostID    string
}

// DeclineInput is the input for declining a booking.
type DeclineInput struct {
	BookingID string
	HostID    string
}

// CancelInput is the input for cancelling a booking.
type CancelInput struct {
	BookingID   string
	CancellerID string
}
