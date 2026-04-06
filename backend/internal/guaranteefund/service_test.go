package guaranteefund

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func defaultConfig() Config {
	return Config{
		ReserveRatioNormal:       0.15,
		ReserveRatioAlert:        0.10,
		ReserveRatioRestrictHigh: 0.05,
		LossRatioTarget:          0.6,
	}
}

func TestCheckReserveRatio_Normal(t *testing.T) {
	svc := NewService(nil, nil, defaultConfig())
	action := svc.CheckReserveRatio(0.20, 100_000)
	assert.Equal(t, ReserveActionNormal, action)
}

func TestCheckReserveRatio_ExactNormal(t *testing.T) {
	svc := NewService(nil, nil, defaultConfig())
	action := svc.CheckReserveRatio(0.15, 100_000)
	assert.Equal(t, ReserveActionNormal, action)
}

func TestCheckReserveRatio_Alert(t *testing.T) {
	svc := NewService(nil, nil, defaultConfig())
	action := svc.CheckReserveRatio(0.12, 100_000)
	assert.Equal(t, ReserveActionAlert, action)
}

func TestCheckReserveRatio_ExactAlert(t *testing.T) {
	svc := NewService(nil, nil, defaultConfig())
	action := svc.CheckReserveRatio(0.10, 100_000)
	assert.Equal(t, ReserveActionAlert, action)
}

func TestCheckReserveRatio_RestrictHigh(t *testing.T) {
	svc := NewService(nil, nil, defaultConfig())
	action := svc.CheckReserveRatio(0.07, 100_000)
	assert.Equal(t, ReserveActionRestrictHigh, action)
}

func TestCheckReserveRatio_ExactRestrictHigh(t *testing.T) {
	svc := NewService(nil, nil, defaultConfig())
	action := svc.CheckReserveRatio(0.05, 100_000)
	assert.Equal(t, ReserveActionRestrictHigh, action)
}

func TestCheckReserveRatio_RestrictAllGap(t *testing.T) {
	svc := NewService(nil, nil, defaultConfig())
	action := svc.CheckReserveRatio(0.03, 100_000)
	assert.Equal(t, ReserveActionRestrictAllGap, action)
}

func TestCheckReserveRatio_ZeroRatio(t *testing.T) {
	svc := NewService(nil, nil, defaultConfig())
	action := svc.CheckReserveRatio(0.0, 100_000)
	assert.Equal(t, ReserveActionRestrictAllGap, action)
}

func TestCheckReserveRatio_NoOutstandingGaps(t *testing.T) {
	svc := NewService(nil, nil, defaultConfig())
	// When there are no outstanding gaps, fund is always normal regardless of ratio.
	action := svc.CheckReserveRatio(0.0, 0)
	assert.Equal(t, ReserveActionNormal, action)
}

func TestEntryType_Values(t *testing.T) {
	assert.Equal(t, EntryType("CONTRIBUTION"), EntryTypeContribution)
	assert.Equal(t, EntryType("CLAIM"), EntryTypeClaim)
	assert.Equal(t, EntryType("CARD_RECOVERY"), EntryTypeCardRecovery)
	assert.Equal(t, EntryType("COLLECTIONS_REFERRAL"), EntryTypeCollectionsRef)
}

func TestReserveAction_Values(t *testing.T) {
	assert.Equal(t, ReserveAction("NORMAL"), ReserveActionNormal)
	assert.Equal(t, ReserveAction("ALERT"), ReserveActionAlert)
	assert.Equal(t, ReserveAction("RESTRICT_HIGH_VALUE"), ReserveActionRestrictHigh)
	assert.Equal(t, ReserveAction("RESTRICT_ALL_GAP"), ReserveActionRestrictAllGap)
}

func TestCheckReserveRatio_TableDriven(t *testing.T) {
	svc := NewService(nil, nil, defaultConfig())

	tests := []struct {
		name     string
		ratio    float64
		gaps     int64
		expected ReserveAction
	}{
		{"healthy fund", 0.25, 500_000, ReserveActionNormal},
		{"borderline normal", 0.15, 500_000, ReserveActionNormal},
		{"just below normal", 0.149, 500_000, ReserveActionAlert},
		{"borderline alert", 0.10, 500_000, ReserveActionAlert},
		{"just below alert", 0.099, 500_000, ReserveActionRestrictHigh},
		{"borderline restrict", 0.05, 500_000, ReserveActionRestrictHigh},
		{"just below restrict", 0.049, 500_000, ReserveActionRestrictAllGap},
		{"empty fund with gaps", 0.0, 500_000, ReserveActionRestrictAllGap},
		{"no gaps at all", 0.0, 0, ReserveActionNormal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := svc.CheckReserveRatio(tt.ratio, tt.gaps)
			assert.Equal(t, tt.expected, action)
		})
	}
}
