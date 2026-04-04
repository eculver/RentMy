package booking

import "time"

// CancellerRole identifies who initiated a cancellation.
type CancellerRole string

const (
	CancellerRenter CancellerRole = "RENTER"
	CancellerHost   CancellerRole = "HOST"
)

// HostCancelConfig holds tunable cancellation fees for host-initiated cancellations (PRD section 18).
type HostCancelConfig struct {
	// LateFeeBPS is the fee (in BPS of rental fee) when host cancels < 2h before pickup.
	LateFeeBPS int
	// VeryLateFeeBPS is the fee when host cancels after scheduled start.
	VeryLateFeeBPS int
}

// CalculateCancellationFee returns the cancellation fee in cents for a booking cancellation.
//
// Renter policy (PRD section 18):
//   - > 2h before scheduled pickup: no fee
//   - 1-2h before scheduled pickup: 25% of rental fee
//   - < 1h before scheduled pickup: 50% of rental fee
//   - After scheduled start (host accepted and waiting): 100% of rental fee
//
// Host policy (PRD section 18):
//   - > 2h before scheduled pickup: no fee (warning tracked separately)
//   - < 2h before scheduled pickup: LateFeeBPS of rental fee
//   - After scheduled start: VeryLateFeeBPS of rental fee
func CalculateCancellationFee(
	role CancellerRole,
	rentalFeeCents int64,
	scheduledStart time.Time,
	now time.Time,
	hostCfg HostCancelConfig,
) int64 {
	timeUntilPickup := scheduledStart.Sub(now)

	switch role {
	case CancellerRenter:
		switch {
		case timeUntilPickup < 0:
			// After scheduled start: 100% of rental fee.
			return rentalFeeCents
		case timeUntilPickup < time.Hour:
			// Less than 1h before pickup: 50%.
			return rentalFeeCents / 2
		case timeUntilPickup < 2*time.Hour:
			// 1-2h before pickup: 25%.
			return rentalFeeCents / 4
		default:
			// More than 2h before pickup: no fee.
			return 0
		}

	case CancellerHost:
		switch {
		case timeUntilPickup < 0:
			// After scheduled start: higher penalty.
			return rentalFeeCents * int64(hostCfg.VeryLateFeeBPS) / 10_000
		case timeUntilPickup < 2*time.Hour:
			// Less than 2h before pickup: late fee.
			return rentalFeeCents * int64(hostCfg.LateFeeBPS) / 10_000
		default:
			// More than 2h: no fee.
			return 0
		}
	}

	return 0
}
