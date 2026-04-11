package latereturn

import (
	"testing"
	"time"
)

func TestComputeHourlyRate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		rentalFee      int64 // cents
		scheduledStart time.Time
		scheduledEnd   time.Time
		wantRate       int64 // cents per hour
	}{
		{
			name:           "standard 4-hour rental at $40",
			rentalFee:      4000,
			scheduledStart: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
			scheduledEnd:   time.Date(2026, 1, 1, 14, 0, 0, 0, time.UTC),
			wantRate:       1000,
		},
		{
			name:           "1-hour rental at $15",
			rentalFee:      1500,
			scheduledStart: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
			scheduledEnd:   time.Date(2026, 1, 1, 11, 0, 0, 0, time.UTC),
			wantRate:       1500,
		},
		{
			name:           "24-hour rental at $120",
			rentalFee:      12000,
			scheduledStart: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
			scheduledEnd:   time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC),
			wantRate:       500,
		},
		{
			name:           "very cheap rental floors to $1/hour",
			rentalFee:      50, // 50 cents for 2 hours
			scheduledStart: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
			scheduledEnd:   time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
			wantRate:       100, // minimum $1/hour
		},
		{
			name:           "zero duration defaults to 1 hour",
			rentalFee:      2000,
			scheduledStart: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
			scheduledEnd:   time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
			wantRate:       2000,
		},
		{
			name:           "sub-hour duration treated as 1 hour",
			rentalFee:      500,
			scheduledStart: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
			scheduledEnd:   time.Date(2026, 1, 1, 10, 30, 0, 0, time.UTC),
			wantRate:       500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := computeHourlyRate(tt.rentalFee, tt.scheduledStart, tt.scheduledEnd)
			if got != tt.wantRate {
				t.Errorf("computeHourlyRate(%d, ...) = %d, want %d", tt.rentalFee, got, tt.wantRate)
			}
		})
	}
}

func TestMaxLateFeeCap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		holdAmount         int64
		damageReserveRateBPS int
		wantCap            int64
	}{
		{
			name:               "default 40% reserve on $100 hold",
			holdAmount:         10000,
			damageReserveRateBPS: 4000,
			wantCap:            6000, // 60% of $100
		},
		{
			name:               "30% reserve on $200 hold",
			holdAmount:         20000,
			damageReserveRateBPS: 3000,
			wantCap:            14000, // 70% of $200
		},
		{
			name:               "50% reserve on $50 hold",
			holdAmount:         5000,
			damageReserveRateBPS: 5000,
			wantCap:            2500, // 50% of $50
		},
		{
			name:               "zero reserve rate defaults to 40%",
			holdAmount:         10000,
			damageReserveRateBPS: 0,
			wantCap:            6000,
		},
		{
			name:               "invalid reserve rate (>100%) defaults to 40%",
			holdAmount:         10000,
			damageReserveRateBPS: 10001,
			wantCap:            6000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := &Service{
				cfg: Config{
					DamageReserveRateBPS: tt.damageReserveRateBPS,
				},
			}
			got := svc.maxLateFeeCap(tt.holdAmount)
			if got != tt.wantCap {
				t.Errorf("maxLateFeeCap(%d) with reserve %d BPS = %d, want %d",
					tt.holdAmount, tt.damageReserveRateBPS, got, tt.wantCap)
			}
		})
	}
}

func TestMaxLateFeeCap_DamageReservePreserved(t *testing.T) {
	t.Parallel()

	// Verify the core invariant: after late fees, at least damageReserveRate of
	// the hold remains for the DisputeAgent.
	svc := &Service{cfg: Config{DamageReserveRateBPS: 4000}} // 40% reserve

	holdAmounts := []int64{5000, 10000, 25000, 50000, 100000}
	for _, hold := range holdAmounts {
		cap := svc.maxLateFeeCap(hold)
		remaining := hold - cap
		reservePercent := float64(remaining) / float64(hold)

		if reservePercent < 0.39 { // allow tiny float rounding
			t.Errorf("hold=%d: after max late fee capture (%d), remaining=%d (%.1f%%) < 40%% damage reserve",
				hold, cap, remaining, reservePercent*100)
		}
	}
}

func TestStatusFromEscalation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		level EscalationLevel
		want  Status
	}{
		{EscalationCharging, StatusCharging},
		{EscalationWarning, StatusCharging},
		{EscalationEscalateToDispute, StatusEscalatedToDispute},
		{EscalationFlagForReview, StatusFlaggedForReview},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			t.Parallel()
			got := statusFromEscalation(tt.level)
			if got != tt.want {
				t.Errorf("statusFromEscalation(%s) = %s, want %s", tt.level, got, tt.want)
			}
		})
	}
}
