package booking

import (
	"testing"
	"time"
)

func TestCalculateCancellationFee_Renter(t *testing.T) {
	rentalFee := int64(10000) // $100.00
	start := time.Now().Add(3 * time.Hour)
	hostCfg := HostCancelConfig{LateFeeBPS: 2500, VeryLateFeeBPS: 5000}

	tests := []struct {
		name    string
		now     time.Time
		wantFee int64
	}{
		{
			name:    "more than 2h before — no fee",
			now:     start.Add(-3 * time.Hour),
			wantFee: 0,
		},
		{
			name:    "1.5h before — 25%",
			now:     start.Add(-90 * time.Minute),
			wantFee: 2500,
		},
		{
			name:    "30min before — 50%",
			now:     start.Add(-30 * time.Minute),
			wantFee: 5000,
		},
		{
			name:    "after scheduled start — 100%",
			now:     start.Add(30 * time.Minute),
			wantFee: 10000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateCancellationFee(CancellerRenter, rentalFee, start, tt.now, hostCfg)
			if got != tt.wantFee {
				t.Errorf("got %d, want %d", got, tt.wantFee)
			}
		})
	}
}

func TestCalculateCancellationFee_Host(t *testing.T) {
	rentalFee := int64(10000) // $100.00
	start := time.Now().Add(3 * time.Hour)
	hostCfg := HostCancelConfig{LateFeeBPS: 2500, VeryLateFeeBPS: 5000}

	tests := []struct {
		name    string
		now     time.Time
		wantFee int64
	}{
		{
			name:    "more than 2h before — no fee",
			now:     start.Add(-3 * time.Hour),
			wantFee: 0,
		},
		{
			name:    "1h before — late fee (25%)",
			now:     start.Add(-1 * time.Hour),
			wantFee: 2500,
		},
		{
			name:    "after scheduled start — very late fee (50%)",
			now:     start.Add(15 * time.Minute),
			wantFee: 5000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateCancellationFee(CancellerHost, rentalFee, start, tt.now, hostCfg)
			if got != tt.wantFee {
				t.Errorf("got %d, want %d", got, tt.wantFee)
			}
		})
	}
}

func TestCalculateCancellationFee_ExactBoundaries(t *testing.T) {
	rentalFee := int64(20000) // $200.00
	hostCfg := HostCancelConfig{LateFeeBPS: 2500, VeryLateFeeBPS: 5000}

	// Use a fixed base time to avoid sub-millisecond drift between two time.Now() calls.
	base := time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)
	start := base.Add(2 * time.Hour)
	now := base // exactly 2h before start

	// timeUntilPickup == 2h → not < 2h → falls to default (no fee).
	got := CalculateCancellationFee(CancellerRenter, rentalFee, start, now, hostCfg)
	if got != 0 {
		t.Errorf("at exactly 2h boundary: got %d, want 0", got)
	}
}
