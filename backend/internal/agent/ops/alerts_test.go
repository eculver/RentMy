package ops

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEvaluateRules_GTOperator(t *testing.T) {
	snap := HealthSnapshot{}
	snap.Trust.FraudFlagRate.Value = 0.15

	rules := []AlertRule{
		{
			ID:         "r1",
			MetricName: "fraud_flag_rate",
			Operator:   OperatorGT,
			Threshold:  0.10,
			Severity:   SeverityWarning,
			Channel:    ChannelSlack,
			Enabled:    true,
		},
	}

	triggered := EvaluateRules(snap, rules)
	assert.Len(t, triggered, 1)
	assert.Equal(t, "fraud_flag_rate", triggered[0].MetricName)
}

func TestEvaluateRules_LTOperator(t *testing.T) {
	snap := HealthSnapshot{}
	snap.Business.BookingConversionRate.Value = 0.30

	rules := []AlertRule{
		{
			ID:         "r2",
			MetricName: "booking_conversion_rate",
			Operator:   OperatorLT,
			Threshold:  0.50,
			Severity:   SeverityCritical,
			Channel:    ChannelBoth,
			Enabled:    true,
		},
	}

	triggered := EvaluateRules(snap, rules)
	assert.Len(t, triggered, 1)
}

func TestEvaluateRules_DisabledRuleIgnored(t *testing.T) {
	snap := HealthSnapshot{}
	snap.Trust.FraudFlagRate.Value = 0.99 // definitely above any threshold

	rules := []AlertRule{
		{
			ID:         "r3",
			MetricName: "fraud_flag_rate",
			Operator:   OperatorGT,
			Threshold:  0.01,
			Severity:   SeverityCritical,
			Channel:    ChannelBoth,
			Enabled:    false, // disabled
		},
	}

	triggered := EvaluateRules(snap, rules)
	assert.Empty(t, triggered)
}

func TestEvaluateRules_UnknownMetricSkipped(t *testing.T) {
	snap := HealthSnapshot{} // all zero values

	rules := []AlertRule{
		{
			ID:         "r4",
			MetricName: "nonexistent_metric",
			Operator:   OperatorGT,
			Threshold:  0.0,
			Severity:   SeverityInfo,
			Channel:    ChannelSlack,
			Enabled:    true,
		},
	}

	triggered := EvaluateRules(snap, rules)
	assert.Empty(t, triggered)
}

func TestEvaluateRules_BelowThresholdNotTriggered(t *testing.T) {
	snap := HealthSnapshot{}
	snap.Trust.FraudFlagRate.Value = 0.05 // below threshold

	rules := []AlertRule{
		{
			ID:         "r5",
			MetricName: "fraud_flag_rate",
			Operator:   OperatorGT,
			Threshold:  0.10,
			Severity:   SeverityWarning,
			Channel:    ChannelSlack,
			Enabled:    true,
		},
	}

	triggered := EvaluateRules(snap, rules)
	assert.Empty(t, triggered)
}

func TestEvaluateRules_MultipleRulesPartialTrigger(t *testing.T) {
	snap := HealthSnapshot{}
	snap.Trust.FraudFlagRate.Value = 0.20 // above threshold
	snap.Trust.DisputeRate.Value = 0.02   // below threshold

	rules := []AlertRule{
		{
			ID:         "r6",
			MetricName: "fraud_flag_rate",
			Operator:   OperatorGT,
			Threshold:  0.10,
			Severity:   SeverityWarning,
			Channel:    ChannelSlack,
			Enabled:    true,
		},
		{
			ID:         "r7",
			MetricName: "dispute_rate",
			Operator:   OperatorGT,
			Threshold:  0.05,
			Severity:   SeverityInfo,
			Channel:    ChannelSlack,
			Enabled:    true,
		},
	}

	triggered := EvaluateRules(snap, rules)
	assert.Len(t, triggered, 1)
	assert.Equal(t, "fraud_flag_rate", triggered[0].MetricName)
}
