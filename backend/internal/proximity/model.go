// Package proximity implements GPS verification, PIN-based handoff, and proximity
// proof management for the RentMy check-in/check-out flow (PRD section 12).
package proximity

import (
	"errors"
	"time"
)

// Sentinel errors for the proximity domain.
var (
	ErrProofNotFound     = errors.New("proximity proof not found")
	ErrPINExpired        = errors.New("PIN has expired")
	ErrPINInvalid        = errors.New("PIN is invalid")
	ErrTooFarAway        = errors.New("GPS location is not within required proximity threshold")
	ErrHandoffIncomplete = errors.New("handoff is not complete — both parties must verify proximity")
	ErrNotAuthorized     = errors.New("not authorized for this proximity action")
)

// ProofType distinguishes check-in from check-out handoff events.
type ProofType string

const (
	ProofTypeCheckIn  ProofType = "CHECK_IN"
	ProofTypeCheckOut ProofType = "CHECK_OUT"
)

// Method is the verification mechanism used for a proof record.
type Method string

const (
	MethodGPS         Method = "GPS"
	MethodBLE         Method = "BLE"
	MethodSMSFallback Method = "SMS_FALLBACK"
)

// ProximityProof is a single party's verification record for a handoff event.
// Each (transaction_id, proof_type, user_id) triple has at most one record.
type ProximityProof struct {
	ID            string     `json:"id"`
	TransactionID string     `json:"transactionId"`
	UserID        string     `json:"userId"`
	ProofType     ProofType  `json:"proofType"`
	GPSDistance   *float64   `json:"gpsDistance,omitempty"` // meters
	PIN           string     `json:"-"`                     // never serialised to clients
	PINExpiresAt  *time.Time `json:"-"`
	Verified      bool       `json:"verified"`
	Method        Method     `json:"method"`
	DeviceID      string     `json:"deviceId,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
}

// ProximityStatus is the aggregated check-in or check-out state for a booking.
type ProximityStatus struct {
	TransactionID   string          `json:"transactionId"`
	CheckInProofs   []ProximityProof `json:"checkInProofs"`
	CheckOutProofs  []ProximityProof `json:"checkOutProofs"`
	CheckInComplete bool            `json:"checkInComplete"`
	CheckOutComplete bool           `json:"checkOutComplete"`
}

// VerifyGPSInput is the input for a GPS proximity verification request.
type VerifyGPSInput struct {
	TransactionID string
	UserID        string
	Lat           float64
	Lng           float64
	ProofType     ProofType
}

// VerifyPINInput is the input for a renter's PIN entry.
type VerifyPINInput struct {
	TransactionID string
	RenterID      string
	PIN           string
}

// SMSFallbackInput is the input for triggering an SMS PIN delivery.
type SMSFallbackInput struct {
	TransactionID string
	HostID        string // host must be the requester
	ToPhone       string
}
