package booking

import (
	"context"
	"fmt"
	"time"
)

// FraudVelocityConfig holds tunable thresholds for fraud velocity rules (PRD section 9).
type FraudVelocityConfig struct {
	// NewAccountThresholdDays is the minimum account age (days) for at least one party.
	NewAccountThresholdDays int
	// FirstNTransactions is the number of early transactions subject to payout delay.
	FirstNTransactions int
	// PayoutDelay is the delay applied to payouts for first-N transactions.
	PayoutDelay time.Duration
	// DamageClaimCapCents is the maximum damage claimed by a host within the cap window.
	DamageClaimCapCents int64
	// DamageClaimWindowDays is the number of days from account creation to enforce the cap.
	DamageClaimWindowDays int
}

// FraudResult summarises the outcome of a fraud velocity check.
type FraudResult struct {
	// Blocked is true when the booking must be rejected outright.
	Blocked bool
	// BlockReason describes why the booking was blocked (only set when Blocked=true).
	BlockReason string
	// PayoutDelayed is true when the first-N rule requires a payout hold.
	PayoutDelayed bool
}

// fraudChecker provides the data access needed to evaluate velocity rules.
type fraudChecker interface {
	GetUserCreatedAt(ctx context.Context, userID string) (time.Time, error)
	CountCompletedByUser(ctx context.Context, userID string) (int, error)
	SumHostDamageClaimed(ctx context.Context, hostID string, since time.Time) (int64, error)
}

// CheckFraudVelocity runs all velocity rules and returns a FraudResult.
// It does not block the caller — callers must check FraudResult.Blocked themselves.
func CheckFraudVelocity(ctx context.Context, repo fraudChecker, renterID, hostID string, cfg FraudVelocityConfig) (FraudResult, error) {
	renterAge, err := repo.GetUserCreatedAt(ctx, renterID)
	if err != nil {
		return FraudResult{}, fmt.Errorf("get renter created_at: %w", err)
	}
	hostAge, err := repo.GetUserCreatedAt(ctx, hostID)
	if err != nil {
		return FraudResult{}, fmt.Errorf("get host created_at: %w", err)
	}

	threshold := time.Duration(cfg.NewAccountThresholdDays) * 24 * time.Hour
	renterNew := time.Since(renterAge) < threshold
	hostNew := time.Since(hostAge) < threshold

	// Rule 1: new-to-new lockout.
	if renterNew && hostNew {
		return FraudResult{
			Blocked:     true,
			BlockReason: "one party must have an established account (>" + fmt.Sprintf("%d", cfg.NewAccountThresholdDays) + " days)",
		}, nil
	}

	// Rule 2: damage claim cap — checked on the host account.
	if cfg.DamageClaimCapCents > 0 {
		since := hostAge.Add(time.Duration(cfg.DamageClaimWindowDays) * 24 * time.Hour)
		if time.Now().Before(since) {
			// Host is within their first DamageClaimWindowDays.
			since = hostAge // check from account creation
			claimed, err := repo.SumHostDamageClaimed(ctx, hostID, since)
			if err != nil {
				return FraudResult{}, fmt.Errorf("sum host damage claimed: %w", err)
			}
			if claimed >= cfg.DamageClaimCapCents {
				return FraudResult{
					Blocked:     true,
					BlockReason: "account under review — damage claim threshold exceeded",
				}, nil
			}
		}
	}

	// Rule 3: first-N payout delay — flagged when either party has fewer than N completed transactions.
	renterCount, err := repo.CountCompletedByUser(ctx, renterID)
	if err != nil {
		return FraudResult{}, fmt.Errorf("count renter transactions: %w", err)
	}
	hostCount, err := repo.CountCompletedByUser(ctx, hostID)
	if err != nil {
		return FraudResult{}, fmt.Errorf("count host transactions: %w", err)
	}

	payoutDelayed := renterCount < cfg.FirstNTransactions || hostCount < cfg.FirstNTransactions

	return FraudResult{PayoutDelayed: payoutDelayed}, nil
}
