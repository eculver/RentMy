package dispute

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRouteDecision(t *testing.T) {
	tests := []struct {
		name            string
		confidence      float64
		chargeAmount    int64
		photoDiffResult string
		hasFraudFlags   bool
		expected        EscalationRoute
	}{
		// Rule 1: high confidence, low amount, no issues → AUTO_RESOLVE
		{
			name:            "high confidence, low amount",
			confidence:      0.90,
			chargeAmount:    15000, // $150
			photoDiffResult: "COSMETIC_DAMAGE",
			hasFraudFlags:   false,
			expected:        RouteAutoResolve,
		},
		// Rule 2: high confidence, mid amount → AUTO_RESOLVE_AUDIT
		{
			name:            "high confidence, mid amount",
			confidence:      0.90,
			chargeAmount:    50000, // $500
			photoDiffResult: "FUNCTIONAL_DAMAGE",
			hasFraudFlags:   false,
			expected:        RouteAutoResolveAudit,
		},
		// Rule 3: high confidence, high amount → HUMAN_REVIEW
		{
			name:            "high confidence, high amount",
			confidence:      0.90,
			chargeAmount:    150000, // $1,500
			photoDiffResult: "MISSING_ITEM",
			hasFraudFlags:   false,
			expected:        RouteHumanReview,
		},
		// Rule 4: low confidence → HUMAN_REVIEW regardless of amount
		{
			name:            "low confidence, low amount",
			confidence:      0.70,
			chargeAmount:    5000, // $50
			photoDiffResult: "COSMETIC_DAMAGE",
			hasFraudFlags:   false,
			expected:        RouteHumanReview,
		},
		// Rule 5: INCONCLUSIVE photo diff → HUMAN_REVIEW
		{
			name:            "inconclusive photo diff",
			confidence:      0.95,
			chargeAmount:    5000,
			photoDiffResult: "INCONCLUSIVE",
			hasFraudFlags:   false,
			expected:        RouteHumanReview,
		},
		// Rule 6: fraud flags → HUMAN_REVIEW (overrides all)
		{
			name:            "fraud flags override high confidence",
			confidence:      0.99,
			chargeAmount:    5000,
			photoDiffResult: "COSMETIC_DAMAGE",
			hasFraudFlags:   true,
			expected:        RouteHumanReview,
		},
		// Edge cases
		{
			name:            "exactly at confidence threshold",
			confidence:      0.85,
			chargeAmount:    15000,
			photoDiffResult: "COSMETIC_DAMAGE",
			hasFraudFlags:   false,
			expected:        RouteAutoResolve,
		},
		{
			name:            "just below confidence threshold",
			confidence:      0.849,
			chargeAmount:    15000,
			photoDiffResult: "COSMETIC_DAMAGE",
			hasFraudFlags:   false,
			expected:        RouteHumanReview,
		},
		{
			name:            "exactly at $200 boundary",
			confidence:      0.90,
			chargeAmount:    20000, // $200
			photoDiffResult: "COSMETIC_DAMAGE",
			hasFraudFlags:   false,
			expected:        RouteAutoResolve,
		},
		{
			name:            "just above $200 boundary",
			confidence:      0.90,
			chargeAmount:    20001, // $200.01
			photoDiffResult: "COSMETIC_DAMAGE",
			hasFraudFlags:   false,
			expected:        RouteAutoResolveAudit,
		},
		{
			name:            "exactly at $1000 boundary",
			confidence:      0.90,
			chargeAmount:    100000, // $1000
			photoDiffResult: "FUNCTIONAL_DAMAGE",
			hasFraudFlags:   false,
			expected:        RouteAutoResolveAudit,
		},
		{
			name:            "just above $1000 boundary",
			confidence:      0.90,
			chargeAmount:    100001, // $1000.01
			photoDiffResult: "FUNCTIONAL_DAMAGE",
			hasFraudFlags:   false,
			expected:        RouteHumanReview,
		},
		{
			name:            "zero charge amount",
			confidence:      0.95,
			chargeAmount:    0,
			photoDiffResult: "NO_CHANGE",
			hasFraudFlags:   false,
			expected:        RouteAutoResolve,
		},
		{
			name:            "no photo diff result",
			confidence:      0.90,
			chargeAmount:    15000,
			photoDiffResult: "",
			hasFraudFlags:   false,
			expected:        RouteAutoResolve,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RouteDecision(tt.confidence, tt.chargeAmount, tt.photoDiffResult, tt.hasFraudFlags)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRouteDecision_FraudFlagsOverrideAll(t *testing.T) {
	// Fraud flags should always result in HUMAN_REVIEW regardless of other factors.
	routes := []struct {
		confidence      float64
		chargeAmount    int64
		photoDiffResult string
	}{
		{0.99, 100, "NO_CHANGE"},
		{0.50, 200000, "INCONCLUSIVE"},
		{0.85, 50000, "FUNCTIONAL_DAMAGE"},
	}

	for _, r := range routes {
		result := RouteDecision(r.confidence, r.chargeAmount, r.photoDiffResult, true)
		assert.Equal(t, RouteHumanReview, result, "fraud flags should always route to human review")
	}
}
