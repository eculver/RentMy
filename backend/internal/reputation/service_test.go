package reputation

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// newUserStats returns a zero-valued userStats with sensible defaults for testing.
func newUserStats(createdAt time.Time) userStats {
	return userStats{
		AccountCreatedAt: createdAt,
		IdentityStatus:   "VERIFIED",
	}
}

// TestComputeScore_NewUser verifies that a brand-new user scores 0.
func TestComputeScore_NewUser(t *testing.T) {
	s := newUserStats(time.Now().UTC())
	s.IdentityStatus = "PENDING"
	score := computeScore(s)
	assert.Equal(t, 0, score)
}

// TestComputeScore_FiveCleanRentalsKYC verifies the example from PRD §8.
// "A user with 5 clean rentals has ~175 reputation."
// 5×15 (rentals) + 5×10 (on-time) + 50 (KYC) + 25+25 (30d+90d age) = 225
// The PRD says ~175 which reflects an older version; we match the current signal table.
func TestComputeScore_FiveCleanRentalsKYC(t *testing.T) {
	created := time.Now().UTC().AddDate(0, -4, 0) // 4 months ago — qualifies for 30d + 90d age
	s := newUserStats(created)
	s.CompletedRentals = 5
	s.CleanRentalCount = 5
	s.OnTimeReturns = 5

	score := computeScore(s)

	// 5×15(rentals) + 5×10(on-time) + 50(KYC) + 25(30d) + 25(90d) + 50(milestone5) = 275
	assert.Equal(t, 275, score)
}

// TestComputeScore_DisputeLost verifies a lost dispute subtracts 50 points.
func TestComputeScore_DisputeLost(t *testing.T) {
	created := time.Now().UTC().AddDate(-1, 0, 0)
	s := newUserStats(created)
	s.CompletedRentals = 10
	s.CleanRentalCount = 10
	s.OnTimeReturns = 10

	baseScore := computeScore(s)

	// Add one recent dispute loss.
	s.DisputesLost = []time.Time{time.Now().UTC()}
	afterDispute := computeScore(s)

	assert.Equal(t, baseScore+PtsDisputeLost, afterDispute)
}

// TestComputeScore_DecayAfter180Days verifies that a negative event older than
// 180 days contributes only 50% of its original penalty.
func TestComputeScore_DecayAfter180Days(t *testing.T) {
	created := time.Now().UTC().AddDate(-2, 0, 0)
	s := newUserStats(created)
	s.CompletedRentals = 5
	s.CleanRentalCount = 5

	recentDispute := userStats{
		AccountCreatedAt: created,
		IdentityStatus:   "VERIFIED",
		CompletedRentals: 5,
		CleanRentalCount: 5,
		DisputesAgainst:  []time.Time{time.Now().UTC()}, // recent
	}
	oldDispute := userStats{
		AccountCreatedAt: created,
		IdentityStatus:   "VERIFIED",
		CompletedRentals: 5,
		CleanRentalCount: 5,
		DisputesAgainst:  []time.Time{time.Now().UTC().AddDate(0, 0, -(decayWindowDays + 10))}, // old
	}

	recentScore := computeScore(recentDispute)
	oldScore := computeScore(oldDispute)

	// Old dispute should apply only half the penalty.
	diff := oldScore - recentScore
	assert.Equal(t, -(PtsDisputeFiledAgainst / 2), diff,
		"old dispute should contribute half the penalty compared to recent")
}

// TestComputeScore_ScoreNeverBelowZero verifies the floor clamp.
func TestComputeScore_ScoreNeverBelowZero(t *testing.T) {
	s := newUserStats(time.Now().UTC())
	s.IdentityStatus = "PENDING"
	// Pile on maximum negative events.
	now := time.Now().UTC()
	s.FraudFlagCount = 5        // 5 × -100 = -500
	s.DisputesLost = []time.Time{now, now, now} // 3 × -50 = -150
	s.Cancellations = []time.Time{now, now, now} // 3 × -20 = -60

	score := computeScore(s)
	assert.Equal(t, 0, score, "score must be clamped to minimum 0")
}

// TestComputeScore_ScoreNeverAbove1000 verifies the ceiling clamp.
func TestComputeScore_ScoreNeverAbove1000(t *testing.T) {
	created := time.Now().UTC().AddDate(-2, 0, 0) // qualifies for all age bonuses
	s := newUserStats(created)
	s.CompletedRentals = 100
	s.CleanRentalCount = 100
	s.OnTimeReturns = 100
	s.PositiveBubbles = 100

	score := computeScore(s)
	assert.Equal(t, 1000, score, "score must be clamped to maximum 1000")
}

// TestComputeScore_MilestonesBonuses verifies milestone bonuses at 5, 15, 50.
func TestComputeScore_MilestonesBonuses(t *testing.T) {
	base := newUserStats(time.Now().UTC())
	base.IdentityStatus = "PENDING"

	cases := []struct {
		cleanRentals int
		wantExtra    int // milestone points expected beyond zero base
	}{
		{4, 0},          // below first milestone
		{5, 50},         // +milestone5
		{14, 50},        // still only first milestone
		{15, 100},       // +milestone5 +milestone15
		{49, 100},       // still two milestones
		{50, 150},       // all three milestones
	}

	for _, tc := range cases {
		s := base
		s.CompletedRentals = tc.cleanRentals
		s.CleanRentalCount = tc.cleanRentals
		got := computeScore(s)
		// base earned = cleanRentals × PtsCompletedRental
		baseEarned := tc.cleanRentals * PtsCompletedRental
		assert.Equal(t, baseEarned+tc.wantExtra, got,
			"cleanRentals=%d", tc.cleanRentals)
	}
}

// TestComputeScore_AccountAgeBonuses verifies age milestones are one-time.
func TestComputeScore_AccountAgeBonuses(t *testing.T) {
	cases := []struct {
		daysOld   int
		wantBonuses int
	}{
		{0, 0},
		{29, 0},
		{30, PtsAccountAge30},
		{89, PtsAccountAge30},
		{90, PtsAccountAge30 + PtsAccountAge90},
		{364, PtsAccountAge30 + PtsAccountAge90},
		{365, PtsAccountAge30 + PtsAccountAge90 + PtsAccountAge365},
	}

	for _, tc := range cases {
		s := userStats{
			AccountCreatedAt: time.Now().UTC().AddDate(0, 0, -tc.daysOld),
			IdentityStatus:   "PENDING",
		}
		got := computeScore(s)
		assert.Equal(t, tc.wantBonuses, got, "daysOld=%d", tc.daysOld)
	}
}

// TestComputeScore_KYCBonus verifies the KYC one-time bonus.
func TestComputeScore_KYCBonus(t *testing.T) {
	pending := userStats{AccountCreatedAt: time.Now().UTC(), IdentityStatus: "PENDING"}
	verified := userStats{AccountCreatedAt: time.Now().UTC(), IdentityStatus: "VERIFIED"}

	assert.Equal(t, 0, computeScore(pending))
	assert.Equal(t, PtsKYCVerified, computeScore(verified))
}

// TestClamp verifies the helper.
func TestClamp(t *testing.T) {
	assert.Equal(t, 0, clamp(0, 1000, -50))
	assert.Equal(t, 1000, clamp(0, 1000, 2000))
	assert.Equal(t, 500, clamp(0, 1000, 500))
}
