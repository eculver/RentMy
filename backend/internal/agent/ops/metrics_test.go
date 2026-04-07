package ops

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectAnomalies_InsufficientHistory(t *testing.T) {
	current := HealthSnapshot{}
	current.Trust.FraudFlagRate.Value = 0.99 // extreme value, but not enough history

	history := []HealthSnapshot{{}, {}} // only 2 snapshots

	anomalies := DetectAnomalies(current, history)
	assert.Empty(t, anomalies, "need at least 3 snapshots for anomaly detection")
}

func TestDetectAnomalies_NoAnomalies(t *testing.T) {
	// Build history with stable values.
	history := make([]HealthSnapshot, 10)
	for i := range history {
		history[i].Trust.FraudFlagRate.Value = 0.05
		history[i].Trust.DisputeRate.Value = 0.03
		history[i].Business.BookingConversionRate.Value = 0.60
		history[i].Business.ActiveListings.Value = 100
		history[i].Trust.AvgAgentConfidence.Value = 0.85
		history[i].Supply.HostChurnRate.Value = 0.10
		history[i].Demand.FailedBookingRate.Value = 0.08
	}

	// Current is within normal range.
	current := HealthSnapshot{}
	current.Trust.FraudFlagRate.Value = 0.052
	current.Trust.DisputeRate.Value = 0.031
	current.Business.BookingConversionRate.Value = 0.61
	current.Business.ActiveListings.Value = 102
	current.Trust.AvgAgentConfidence.Value = 0.84
	current.Supply.HostChurnRate.Value = 0.101
	current.Demand.FailedBookingRate.Value = 0.079

	anomalies := DetectAnomalies(current, history)
	assert.Empty(t, anomalies)
}

func TestDetectAnomalies_FraudSpikeDetected(t *testing.T) {
	// Build history with varying fraud rate around 0.05 ± 0.01 so std > 0.
	baseVals := []float64{0.04, 0.05, 0.06, 0.05, 0.04, 0.06, 0.05, 0.05, 0.04, 0.06}
	history := make([]HealthSnapshot, len(baseVals))
	for i, v := range baseVals {
		history[i].Trust.FraudFlagRate.Value = v
	}

	// Current fraud rate spikes dramatically (>2 std devs above mean of ~0.05).
	current := HealthSnapshot{}
	current.Trust.FraudFlagRate.Value = 0.50 // far above normal

	anomalies := DetectAnomalies(current, history)
	assert.NotEmpty(t, anomalies, "fraud spike should be detected")
}

func TestDetectAnomalies_ConversionDropDetected(t *testing.T) {
	// Build history with varying conversion around 0.70 ± 0.02.
	baseVals := []float64{0.68, 0.70, 0.72, 0.70, 0.68, 0.72, 0.70, 0.70, 0.68, 0.72}
	history := make([]HealthSnapshot, len(baseVals))
	for i, v := range baseVals {
		history[i].Business.BookingConversionRate.Value = v
	}

	// Conversion collapses (>2 std devs below mean).
	current := HealthSnapshot{}
	current.Business.BookingConversionRate.Value = 0.10

	anomalies := DetectAnomalies(current, history)
	assert.NotEmpty(t, anomalies, "conversion drop should be detected")
}

func TestMeanStdDev_Empty(t *testing.T) {
	mean, std := meanStdDev(nil)
	assert.Equal(t, 0.0, mean)
	assert.Equal(t, 0.0, std)
}

func TestMeanStdDev_SingleValue(t *testing.T) {
	mean, std := meanStdDev([]float64{5.0})
	assert.Equal(t, 5.0, mean)
	assert.Equal(t, 0.0, std)
}

func TestMeanStdDev_KnownValues(t *testing.T) {
	vals := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	mean, std := meanStdDev(vals)
	assert.InDelta(t, 5.0, mean, 0.001)
	assert.InDelta(t, 2.0, std, 0.001)
}
