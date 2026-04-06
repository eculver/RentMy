package reputation

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

// Service computes authoritative reputation scores from source data.
// It is the canonical place to trigger score recalculation; other packages
// should call EnqueueRecalc rather than computing scores inline.
type Service struct {
	repo        *Repository
	riverClient *river.Client[pgx.Tx]
}

// NewService creates a Service.
func NewService(repo *Repository, riverClient *river.Client[pgx.Tx]) *Service {
	return &Service{repo: repo, riverClient: riverClient}
}

// Recalculate computes a user's reputation score from source tables and persists
// the result.  It is safe to call repeatedly; results are idempotent.
//
// Algorithm (PRD §8):
//  1. Positive signals from transactions, ratings, account metadata
//  2. Negative signals from disputes, cancellations, late returns, fraud flags
//  3. 180-day decay: negative events older than 180 days contribute only 50%
//  4. One-time bonuses for account age milestones, KYC, and rental milestones
//  5. Clamp final sum to [0, 1000]
func (s *Service) Recalculate(ctx context.Context, userID string) (int, error) {
	stats, err := s.repo.FetchUserStats(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("reputation: recalculate: %w", err)
	}

	score := computeScore(stats)

	if err := s.repo.SetScore(ctx, userID, score); err != nil {
		return score, fmt.Errorf("reputation: persist score: %w", err)
	}

	slog.Info("reputation: score recalculated", "userId", userID, "score", score)
	return score, nil
}

// RecalculateHostSignals applies the monthly host-specific bonuses/penalties for a
// single host and re-persists their score.
func (s *Service) RecalculateHostSignals(ctx context.Context, hostID string) error {
	// First do a full recalculation to get the base score.
	score, err := s.Recalculate(ctx, hostID)
	if err != nil {
		return err
	}

	// Fetch host-specific rolling metrics.
	hs, err := s.repo.FetchHostStats(ctx, hostID)
	if err != nil {
		return fmt.Errorf("reputation: host signals: %w", err)
	}

	if hs.TotalBookings90d == 0 {
		return nil // no data yet — nothing to adjust
	}

	acceptRate := float64(hs.AcceptedBookings90d) / float64(hs.TotalBookings90d)

	switch {
	case acceptRate >= 0.80:
		score += PtsHighAcceptanceRate
	case acceptRate < 0.30:
		score += PtsLowAcceptanceRate
	}

	if hs.HostCancels90d == 0 {
		score += PtsZeroCancellations
	}

	score = clamp(scoreMin, scoreMax, score)
	if err := s.repo.SetScore(ctx, hostID, score); err != nil {
		return fmt.Errorf("reputation: persist host score: %w", err)
	}

	slog.Info("reputation: host signals applied", "hostId", hostID, "score", score)
	return nil
}

// EnqueueRecalc schedules an async reputation recalculation for userID.
// Non-blocking: the job runs in the River worker pool.
func (s *Service) EnqueueRecalc(ctx context.Context, userID string) error {
	if s.riverClient == nil {
		return nil // no-op when River is not configured (tests)
	}
	if _, err := s.riverClient.Insert(ctx, ReputationRecalcJobArgs{UserID: userID}, nil); err != nil {
		return fmt.Errorf("reputation: enqueue recalc: %w", err)
	}
	return nil
}

// computeScore applies the PRD §8 rules engine to a userStats snapshot.
// Exported as a package-level function for testability (no DB required).
func computeScore(s userStats) int {
	score := 0
	now := time.Now().UTC()
	decayCutoff := now.AddDate(0, 0, -decayWindowDays)

	// --- Positive: completed rentals ---
	score += s.CompletedRentals * PtsCompletedRental

	// --- Positive: rating bubbles received ---
	score += s.PositiveBubbles * PtsPositiveBubble

	// --- Positive: on-time returns ---
	score += s.OnTimeReturns * PtsOnTimeReturn

	// --- One-time bonuses: account age ---
	ageDays := int(now.Sub(s.AccountCreatedAt).Hours() / 24)
	if ageDays >= 30 {
		score += PtsAccountAge30
	}
	if ageDays >= 90 {
		score += PtsAccountAge90
	}
	if ageDays >= 365 {
		score += PtsAccountAge365
	}

	// --- One-time bonus: KYC ---
	if s.IdentityStatus == "VERIFIED" {
		score += PtsKYCVerified
	}

	// --- One-time bonuses: rental milestones ---
	if s.CleanRentalCount >= 5 {
		score += PtsMilestone5
	}
	if s.CleanRentalCount >= 15 {
		score += PtsMilestone15
	}
	if s.CleanRentalCount >= 50 {
		score += PtsMilestone50
	}

	// --- Negative: disputes filed against ---
	for _, ts := range s.DisputesAgainst {
		pts := PtsDisputeFiledAgainst
		if ts.Before(decayCutoff) {
			pts /= 2
		}
		score += pts
	}

	// --- Negative: disputes lost ---
	for _, ts := range s.DisputesLost {
		pts := PtsDisputeLost
		if ts.Before(decayCutoff) {
			pts /= 2
		}
		score += pts
	}

	// --- Negative: cancellations ---
	for _, ts := range s.Cancellations {
		pts := PtsCancellation
		if ts.Before(decayCutoff) {
			pts /= 2
		}
		score += pts
	}

	// --- Negative: late returns ---
	for _, ts := range s.LateReturns {
		pts := PtsLateReturn
		if ts.Before(decayCutoff) {
			pts /= 2
		}
		score += pts
	}

	// --- Negative: fraud flags ---
	// Fraud flags do not decay (they persist until cleared by ops).
	score += s.FraudFlagCount * PtsFraudFlag

	return clamp(scoreMin, scoreMax, score)
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
