// Package reputation implements source-based reputation score recalculation (PRD §8).
// It provides an authoritative, idempotent Recalculate function that derives a user's
// reputation score by querying source tables (transactions, ratings, disputes, users)
// directly rather than replaying stored signals.  This makes it safe to run at any
// time without risk of double-counting.
//
// The risk package (internal/agent/risk) manages the incremental signal log and
// per-transaction risk scores.  This package provides the periodic reconciliation
// layer and the event-driven async recalculation job.
package reputation

import "time"

// Signal point values — mirrors PRD §8 exactly.
const (
	PtsCompletedRental    = 15  // per completed rental with no dispute
	PtsPositiveBubble     = 5   // per bubble received in a rating
	PtsOnTimeReturn       = 10  // renter returned within 15 min of scheduled end
	PtsAccountAge30       = 25  // one-time: account older than 30 days
	PtsAccountAge90       = 25  // one-time: account older than 90 days
	PtsAccountAge365      = 25  // one-time: account older than 365 days
	PtsKYCVerified        = 50  // one-time: identity verified
	PtsMilestone5         = 50  // one-time: 5+ completed rentals with no disputes
	PtsMilestone15        = 50  // one-time: 15+ completed rentals with no disputes
	PtsMilestone50        = 50  // one-time: 50+ completed rentals with no disputes
	PtsHighResponseRate   = 25  // host: response rate > 90% over last 30 days
	PtsHighAcceptanceRate = 25  // host: acceptance rate > 80% over last 30 days
	PtsZeroCancellations  = 25  // host: zero host-initiated cancellations in 90 days

	PtsDisputeFiledAgainst = -30  // dispute filed against this user
	PtsDisputeLost         = -50  // dispute decided against this user
	PtsCancellation        = -20  // per cancellation as the cancelling party
	PtsLateReturn          = -15  // renter returned more than 15 min late
	PtsFraudFlag           = -100 // per active fraud flag on the account
	PtsLowResponseRate     = -40  // host: response rate < 50% over last 30 days
	PtsLowAcceptanceRate   = -40  // host: acceptance rate < 30% over last 30 days

	// onTimeWindowMinutes is the grace period for classifying a return as on-time.
	onTimeWindowMinutes = 15

	// decayWindowDays is the age after which negative events are decayed by 50%.
	decayWindowDays = 180

	// scoreMin / scoreMax are the reputation score clamp boundaries.
	scoreMin = 0
	scoreMax = 1000
)

// userStats holds all the raw data fetched from source tables for one user.
// All slices contain event timestamps; count-only fields omit timestamps where
// decay cannot apply (positive events are never decayed).
type userStats struct {
	// User metadata
	AccountCreatedAt time.Time
	IdentityStatus   string // "VERIFIED", "PENDING", "REJECTED"

	// Positive events — timestamps used only for milestones (not decay)
	CompletedRentals int // clean rentals (no filed dispute) as renter or host
	PositiveBubbles  int // bubbles received across all ratings
	OnTimeReturns    int // as renter: actual_end <= scheduled_end + 15 min

	// Milestone counter — only clean rentals (no disputes)
	CleanRentalCount int

	// Negative events — timestamps carry decay eligibility
	DisputesAgainst []time.Time // disputes filed against this user (non-reporter)
	DisputesLost    []time.Time // disputes decided against this user (renter charged)
	Cancellations   []time.Time // as the cancelling party (renter cancelled or host cancelled)
	LateReturns     []time.Time // as renter: overdue returns
	FraudFlagCount  int         // active fraud flags from users.risk_flags

	// Host-specific (refreshed monthly)
	TotalBookings90d    int
	AcceptedBookings90d int
	HostCancels90d      int
}
