// Package risk implements the RiskAgent: per-transaction risk scoring and
// user reputation management (dual-score system from PRD §8).
package risk

import (
	"errors"
	"time"
)

// Sentinel errors for the risk domain.
var (
	ErrUserNotFound        = errors.New("user not found")
	ErrTransactionNotFound = errors.New("transaction not found")
)

// RiskLevel categorises a score into a human-readable band.
type RiskLevel string

const (
	RiskLevelLow      RiskLevel = "LOW"      // 0–30
	RiskLevelMedium   RiskLevel = "MEDIUM"   // 31–50
	RiskLevelHigh     RiskLevel = "HIGH"     // 51–70
	RiskLevelCritical RiskLevel = "CRITICAL" // 71–100
)

// RiskControl is the booking control action derived from the risk score.
type RiskControl string

const (
	// ControlApprove is the normal path: fast payout, standard hold.
	ControlApprove RiskControl = "APPROVE"
	// ControlReview adds a 48-hour payout delay.
	ControlReview RiskControl = "REVIEW"
	// ControlBlock rejects the booking.
	ControlBlock RiskControl = "BLOCK"
)

// SignalType is a named reputation event.
type SignalType string

const (
	// Positive signals
	SignalCompletedNoDispute  SignalType = "completed_no_dispute"  // +15
	SignalPositiveRating      SignalType = "positive_rating"       // +5 per bubble
	SignalOnTimeReturn        SignalType = "on_time_return"        // +10
	SignalAccountAge30        SignalType = "account_age_30"        // +25 one-time
	SignalAccountAge90        SignalType = "account_age_90"        // +25 one-time
	SignalAccountAge365       SignalType = "account_age_365"       // +25 one-time
	SignalKYCVerified         SignalType = "kyc_verified"          // +50 one-time
	SignalMilestone5          SignalType = "milestone_5_rentals"   // +50 one-time
	SignalMilestone15         SignalType = "milestone_15_rentals"  // +50 one-time
	SignalMilestone50         SignalType = "milestone_50_rentals"  // +50 one-time

	// Negative signals
	SignalDisputeFiled  SignalType = "dispute_filed"  // -30
	SignalDisputeLost   SignalType = "dispute_lost"   // -50
	SignalCancellation  SignalType = "cancellation"   // -20
	SignalLateReturn    SignalType = "late_return"    // -15
	SignalFraudFlag     SignalType = "fraud_flag"     // -100

	// Host-specific (recalculated monthly)
	SignalHighResponseRate    SignalType = "high_response_rate"    // +25
	SignalHighAcceptanceRate  SignalType = "high_acceptance_rate"  // +25
	SignalZeroCancellations90 SignalType = "zero_cancellations_90d" // +25
	SignalLowResponseRate     SignalType = "low_response_rate"    // -40
	SignalLowAcceptanceRate   SignalType = "low_acceptance_rate"  // -40
)

// signalPoints maps each signal type to its default point value.
// Negative values subtract from the reputation score.
var signalPoints = map[SignalType]int{
	SignalCompletedNoDispute:  15,
	SignalPositiveRating:      5,
	SignalOnTimeReturn:        10,
	SignalAccountAge30:        25,
	SignalAccountAge90:        25,
	SignalAccountAge365:       25,
	SignalKYCVerified:         50,
	SignalMilestone5:          50,
	SignalMilestone15:         50,
	SignalMilestone50:         50,
	SignalDisputeFiled:        -30,
	SignalDisputeLost:         -50,
	SignalCancellation:        -20,
	SignalLateReturn:          -15,
	SignalFraudFlag:           -100,
	SignalHighResponseRate:    25,
	SignalHighAcceptanceRate:  25,
	SignalZeroCancellations90: 25,
	SignalLowResponseRate:     -40,
	SignalLowAcceptanceRate:   -40,
}

// oneTimeSignals are signals that can only be emitted once per user.
// The idempotency_key is set to the signal type string.
var oneTimeSignals = map[SignalType]bool{
	SignalAccountAge30:  true,
	SignalAccountAge90:  true,
	SignalAccountAge365: true,
	SignalKYCVerified:   true,
	SignalMilestone5:    true,
	SignalMilestone15:   true,
	SignalMilestone50:   true,
}

// negativeSignals are subject to 180-day decay.
var negativeSignals = map[SignalType]bool{
	SignalDisputeFiled: true,
	SignalDisputeLost:  true,
	SignalCancellation: true,
	SignalLateReturn:   true,
	SignalFraudFlag:    true,
}

const (
	// decayWindowDays is the threshold after which negative signals decay to 50%.
	decayWindowDays = 180
)

// ReputationSignal is a single event that modifies a user's reputation.
type ReputationSignal struct {
	ID             string
	UserID         string
	SignalType     SignalType
	Points         int
	IdempotencyKey *string
	TransactionID  *string
	EmittedAt      time.Time
}

// EmitSignalInput is the input for emitting a new reputation signal.
type EmitSignalInput struct {
	UserID        string
	SignalType    SignalType
	TransactionID *string
	// Points overrides the default point value for this signal type.
	// If zero, the default from signalPoints is used.
	Points int
}

// RiskBreakdown details each component of the per-transaction risk score.
type RiskBreakdown struct {
	BaseRisk        int `json:"base_risk"`
	TransactionRisk int `json:"transaction_risk"`
	CounterpartyRisk int `json:"counterparty_risk"`
	BehavioralRisk  int `json:"behavioral_risk"`
	FraudSignals    int `json:"fraud_signals"`
	Total           int `json:"total"`
}

// RiskSignalDetail is a single risk signal for transparency in the API response.
type RiskSignalDetail struct {
	Signal string  `json:"signal"`
	Weight int     `json:"weight"`
	Flag   bool    `json:"flag"`
}

// TransactionRiskScore is the stored + returned risk score for a transaction.
type TransactionRiskScore struct {
	TransactionID string             `json:"transactionId"`
	RiskScore     int                `json:"riskScore"`
	RiskLevel     RiskLevel          `json:"riskLevel"`
	Control       RiskControl        `json:"control"`
	Breakdown     RiskBreakdown      `json:"breakdown"`
	Signals       []RiskSignalDetail `json:"signals"`
	ComputedAt    time.Time          `json:"computedAt"`
}

// riskScoreInput contains all the data needed to compute the risk score.
// Assembled by the service before calling computeScore.
type riskScoreInput struct {
	// Renter profile
	RenterID             string
	RenterIdentityStatus string // "PENDING", "VERIFIED", "REJECTED"
	RenterAccountAgeDays int
	RenterReputation     int
	RenterFraudFlags     int

	// Host profile
	HostID           string
	HostReputation   int
	HostAccountAgeDays int

	// Transaction details
	ItemValueCents  int64
	ScheduledStart  time.Time // local time for night booking detection

	// Behavioral history (last 60 days)
	RenterCancellations60d int
	RenterDisputes60d      int
	GeoMismatch            bool // transaction outside user's usual geo radius

	// Fraud signals
	DeviceFingerprintMatch bool // same device used by another account
	CompoundNetworkSignal  bool // WiFi + one other fraud indicator
	VelocityFlag           bool // exceeds frequency threshold
}

// levelForScore returns the RiskLevel band for the given score.
func levelForScore(score int) RiskLevel {
	switch {
	case score <= 30:
		return RiskLevelLow
	case score <= 50:
		return RiskLevelMedium
	case score <= 70:
		return RiskLevelHigh
	default:
		return RiskLevelCritical
	}
}

// controlForScore returns the booking control action for the given score.
func controlForScore(score int) RiskControl {
	switch {
	case score <= 30:
		return ControlApprove
	case score <= 70:
		return ControlReview
	default:
		return ControlBlock
	}
}

// clamp returns v clamped to [lo, hi].
func clamp(lo, hi, v int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
