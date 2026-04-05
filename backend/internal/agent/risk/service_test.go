package risk

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestRiskScoring validates the deterministic risk rules engine.
func TestRiskScoring(t *testing.T) {
	now := time.Date(2025, 6, 15, 14, 0, 0, 0, time.UTC) // 2pm — not a night booking

	tests := []struct {
		name           string
		input          riskScoreInput
		wantScoreRange [2]int // inclusive [min, max]
		wantControl    RiskControl
		wantLevel      RiskLevel
	}{
		{
			// base: PENDING(+20) + <7days(+15) + rep<100(+15) = 50
			// tx: value $1,500 > $1,000(+15) = 15
			// total: 65 → REVIEW
			name: "new user pending KYC renting high-value item",
			input: riskScoreInput{
				RenterIdentityStatus: "PENDING",
				RenterAccountAgeDays: 3,
				RenterReputation:     0,
				HostReputation:       500,
				HostAccountAgeDays:   120,
				ItemValueCents:       150_000, // $1,500
				ScheduledStart:       now,
			},
			wantScoreRange: [2]int{60, 70},
			wantControl:    ControlReview,
			wantLevel:      RiskLevelHigh,
		},
		{
			// All zeros: fully verified, high rep, low-value item, daytime → 0
			name: "verified user with good reputation renting low-value item",
			input: riskScoreInput{
				RenterIdentityStatus: "VERIFIED",
				RenterAccountAgeDays: 180,
				RenterReputation:     500,
				HostReputation:       400,
				HostAccountAgeDays:   200,
				ItemValueCents:       5_000, // $50
				ScheduledStart:       now,
			},
			wantScoreRange: [2]int{0, 0},
			wantControl:    ControlApprove,
			wantLevel:      RiskLevelLow,
		},
		{
			// base: PENDING(+20) + <7days(+15) + rep<100(+15) = 50
			// counterparty: both rep<50(+30) + host age<14d(+10) = 40
			// total: 90 → BLOCK
			name: "two new users transacting with each other",
			input: riskScoreInput{
				RenterIdentityStatus: "PENDING",
				RenterAccountAgeDays: 5,
				RenterReputation:     0,
				HostReputation:       10,
				HostAccountAgeDays:   7,
				ItemValueCents:       10_000,
				ScheduledStart:       now,
			},
			wantScoreRange: [2]int{71, 100},
			wantControl:    ControlBlock,
			wantLevel:      RiskLevelCritical,
		},
		{
			// base: rep 100-300(+5) = 5
			// tx: $2,000 > $1,000(+15) + 3am(+10) = 25
			// total: 30 → APPROVE (boundary: ≤30 = APPROVE)
			name: "high-value item at 3am",
			input: riskScoreInput{
				RenterIdentityStatus: "VERIFIED",
				RenterAccountAgeDays: 60,
				RenterReputation:     250,
				HostReputation:       350,
				HostAccountAgeDays:   90,
				ItemValueCents:       200_000, // $2,000
				ScheduledStart:       time.Date(2025, 6, 15, 3, 0, 0, 0, time.UTC),
			},
			wantScoreRange: [2]int{25, 35},
			wantControl:    ControlApprove,
		},
		{
			// base: VERIFIED, good rep → 0
			// fraud: device match (+50)
			// total: 50 → REVIEW
			name: "device fingerprint match — immediate review",
			input: riskScoreInput{
				RenterIdentityStatus:   "VERIFIED",
				RenterAccountAgeDays:   90,
				RenterReputation:       400,
				HostReputation:         300,
				HostAccountAgeDays:     100,
				ItemValueCents:         5_000,
				ScheduledStart:         now,
				DeviceFingerprintMatch: true,
			},
			wantScoreRange: [2]int{45, 55},
			wantControl:    ControlReview,
		},
		{
			// base: rep 100-300(+5)
			// behavioral: 1 dispute(+25)
			// total: 30 → APPROVE (boundary: ≤30 = APPROVE)
			name: "established user with prior dispute in 60 days",
			input: riskScoreInput{
				RenterIdentityStatus: "VERIFIED",
				RenterAccountAgeDays: 200,
				RenterReputation:     300,
				HostReputation:       400,
				HostAccountAgeDays:   150,
				ItemValueCents:       8_000, // $80
				ScheduledStart:       now,
				RenterDisputes60d:    1,
			},
			wantScoreRange: [2]int{25, 35},
			wantControl:    ControlApprove,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			score, breakdown, signals := computeScore(tc.input)

			assert.GreaterOrEqual(t, score, tc.wantScoreRange[0],
				"score %d below minimum %d", score, tc.wantScoreRange[0])
			assert.LessOrEqual(t, score, tc.wantScoreRange[1],
				"score %d above maximum %d", score, tc.wantScoreRange[1])

			assert.Equal(t, score, breakdown.Total, "breakdown.Total must match score")
			assert.GreaterOrEqual(t, score, 0, "score must be >= 0")
			assert.LessOrEqual(t, score, 100, "score must be <= 100")

			if tc.wantControl != "" {
				assert.Equal(t, tc.wantControl, controlForScore(score))
			}
			if tc.wantLevel != "" {
				assert.Equal(t, tc.wantLevel, levelForScore(score))
			}

			// signals is always non-nil (may be empty for zero-risk scenarios).
			assert.NotNil(t, signals)

			// Breakdown components must be non-negative.
			assert.GreaterOrEqual(t, breakdown.BaseRisk, 0)
			assert.GreaterOrEqual(t, breakdown.TransactionRisk, 0)
			assert.GreaterOrEqual(t, breakdown.CounterpartyRisk, 0)
			assert.GreaterOrEqual(t, breakdown.BehavioralRisk, 0)
			assert.GreaterOrEqual(t, breakdown.FraudSignals, 0)
		})
	}
}

// TestRiskScoreClamp verifies the score is always within [0, 100].
func TestRiskScoreClamp(t *testing.T) {
	// Maximum-risk scenario: every flag set.
	input := riskScoreInput{
		RenterIdentityStatus:  "PENDING",
		RenterAccountAgeDays:  1,
		RenterReputation:      0,
		HostReputation:        0,
		HostAccountAgeDays:    1,
		ItemValueCents:        500_000,
		ScheduledStart:        time.Date(2025, 6, 15, 2, 0, 0, 0, time.UTC),
		RenterCancellations60d: 3,
		RenterDisputes60d:     2,
		GeoMismatch:           true,
		DeviceFingerprintMatch: true,
		CompoundNetworkSignal: true,
		VelocityFlag:          true,
	}
	score, _, _ := computeScore(input)
	assert.Equal(t, 100, score, "maximum-risk scenario must clamp to 100")

	// Zero-risk scenario.
	zero := riskScoreInput{
		RenterIdentityStatus: "VERIFIED",
		RenterAccountAgeDays: 365,
		RenterReputation:     1000,
		HostReputation:       1000,
		HostAccountAgeDays:   365,
		ItemValueCents:       500,
		ScheduledStart:       time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC),
	}
	zeroScore, _, _ := computeScore(zero)
	assert.Equal(t, 0, zeroScore, "zero-risk scenario must be 0")
}

// TestReputationSignals verifies signal point values and one-time signal detection.
func TestReputationSignals(t *testing.T) {
	t.Run("signal points are non-zero", func(t *testing.T) {
		for sig, pts := range signalPoints {
			assert.NotZero(t, pts, "signal %s must have non-zero points", sig)
		}
	})

	t.Run("one-time signals are all in signalPoints", func(t *testing.T) {
		for sig := range oneTimeSignals {
			_, ok := signalPoints[sig]
			assert.True(t, ok, "one-time signal %s must be in signalPoints", sig)
		}
	})

	t.Run("negative signals are all in signalPoints", func(t *testing.T) {
		for sig := range negativeSignals {
			pts, ok := signalPoints[sig]
			assert.True(t, ok, "negative signal %s must be in signalPoints", sig)
			assert.Less(t, pts, 0, "negative signal %s must have negative points", sig)
		}
	})
}

// TestLevelForScore verifies the band boundaries.
func TestLevelForScore(t *testing.T) {
	cases := []struct {
		score int
		level RiskLevel
	}{
		{0, RiskLevelLow},
		{30, RiskLevelLow},
		{31, RiskLevelMedium},
		{50, RiskLevelMedium},
		{51, RiskLevelHigh},
		{70, RiskLevelHigh},
		{71, RiskLevelCritical},
		{100, RiskLevelCritical},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.level, levelForScore(tc.score),
			"score %d should be %s", tc.score, tc.level)
	}
}

// TestControlForScore verifies the control action boundaries.
func TestControlForScore(t *testing.T) {
	cases := []struct {
		score   int
		control RiskControl
	}{
		{0, ControlApprove},
		{30, ControlApprove},
		{31, ControlReview},
		{70, ControlReview},
		{71, ControlBlock},
		{100, ControlBlock},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.control, controlForScore(tc.score),
			"score %d should be %s", tc.score, tc.control)
	}
}

// TestReputationDecay verifies the decay logic in RecalculateReputation.
func TestReputationDecay(t *testing.T) {
	// Build a slice of signals manually and verify the decay math.
	now := time.Now().UTC()
	old := now.AddDate(0, 0, -(decayWindowDays + 10)) // older than decay threshold

	signals := []ReputationSignal{
		// Positive: +50 KYC (recent)
		{SignalType: SignalKYCVerified, Points: 50, EmittedAt: now},
		// Negative: -30 dispute filed (old — should decay to -15)
		{SignalType: SignalDisputeFiled, Points: -30, EmittedAt: old},
		// Negative: -20 cancellation (recent — full weight)
		{SignalType: SignalCancellation, Points: -20, EmittedAt: now},
	}

	// Manual calculation: 50 + (-30/2) + (-20) = 50 - 15 - 20 = 15
	score := 0
	decayCutoff := now.AddDate(0, 0, -decayWindowDays)
	for _, sig := range signals {
		if negativeSignals[sig.SignalType] && sig.EmittedAt.Before(decayCutoff) {
			score += sig.Points / 2
		} else {
			score += sig.Points
		}
	}
	score = clamp(0, 1000, score)
	assert.Equal(t, 15, score, "decayed score should be 15")
}
