package payment

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTieredHold(t *testing.T) {
	tests := []struct {
		name          string
		itemValueCents int64
		want          int64
	}{
		{"$0 item", 0, 0},
		{"$100 item (tier 1)", 10_000, 10_000},
		{"$500 item (tier 1 ceiling)", 50_000, 50_000},
		{"$501 item (just into tier 2)", 50_100, 50_025},
		{"$1000 item (tier 2)", 100_000, 50_000 + 50_000*25/100},
		{"$2000 item (tier 2 ceiling)", 200_000, 87_500},
		{"$2001 item (just into tier 3)", 200_100, 87_500 + 100*15/100},
		{"$3500 item (tier 3)", 350_000, 87_500 + 150_000*15/100},
		{"$5000 item (tier 3 ceiling)", 500_000, 87_500 + 300_000*15/100},
		{"$5001 item (hard ceiling)", 500_100, 132_500},
		{"$100000 item (hard ceiling)", 10_000_000, 132_500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TieredHold(tt.itemValueCents)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGuaranteeGap(t *testing.T) {
	assert.Equal(t, int64(0), GuaranteeGap(100_000, 100_000))
	assert.Equal(t, int64(0), GuaranteeGap(100_000, 150_000)) // hold > value → no gap
	assert.Equal(t, int64(50_000), GuaranteeGap(100_000, 50_000))
	// $10k item → hold = $1325 → gap = $8675
	assert.Equal(t, int64(867_500), GuaranteeGap(1_000_000, 132_500))
}

func TestRentalFee(t *testing.T) {
	tests := []struct {
		name          string
		perHour       int64
		perDay        int64
		duration      time.Duration
		want          int64
	}{
		{"1 hour hourly rate", 500, 0, time.Hour, 500},
		{"3 hours hourly rate", 500, 0, 3 * time.Hour, 1_500},
		{"30 min rounds up to 1 hour", 500, 0, 30 * time.Minute, 500},
		{"24 hours uses daily rate", 3_000, 20_000, 24 * time.Hour, 20_000},
		{"48 hours daily = 2 days", 3_000, 20_000, 48 * time.Hour, 40_000},
		{"no price set", 0, 0, 24 * time.Hour, 0},
		{"only daily set, sub-day duration", 0, 20_000, 6 * time.Hour, 0},
		{"only hourly set, multi-day", 500, 0, 48 * time.Hour, 24_000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RentalFee(tt.perHour, tt.perDay, tt.duration)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPlatformFee(t *testing.T) {
	// 20% of $100 = $20
	assert.Equal(t, int64(2_000), PlatformFee(10_000, 2000))
	// 20% of $50 = $10
	assert.Equal(t, int64(1_000), PlatformFee(5_000, 2000))
	// 0 rental fee
	assert.Equal(t, int64(0), PlatformFee(0, 2000))
}

func TestHostPayout(t *testing.T) {
	// $100 rental - $20 platform fee = $80 payout
	assert.Equal(t, int64(8_000), HostPayout(10_000, 2_000))
}

func TestGuaranteeFundContribution(t *testing.T) {
	// 10% of $20 platform fee = $2 contribution
	assert.Equal(t, int64(200), GuaranteeFundContribution(2_000, 1000))
}
