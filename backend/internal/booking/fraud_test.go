package booking

import (
	"context"
	"testing"
	"time"
)

// mockFraudRepo satisfies the fraudChecker interface for tests.
type mockFraudRepo struct {
	createdAt      map[string]time.Time
	completedCount map[string]int
	damageClaimed  int64
}

func (m *mockFraudRepo) GetUserCreatedAt(_ context.Context, userID string) (time.Time, error) {
	if t, ok := m.createdAt[userID]; ok {
		return t, nil
	}
	return time.Now().Add(-60 * 24 * time.Hour), nil // default: established account
}

func (m *mockFraudRepo) CountCompletedByUser(_ context.Context, userID string) (int, error) {
	return m.completedCount[userID], nil
}

func (m *mockFraudRepo) SumHostDamageClaimed(_ context.Context, _ string, _ time.Time) (int64, error) {
	return m.damageClaimed, nil
}

func defaultCfg() FraudVelocityConfig {
	return FraudVelocityConfig{
		NewAccountThresholdDays:  30,
		FirstNTransactions:       3,
		PayoutDelay:              48 * time.Hour,
		DamageClaimCapCents:      50000, // $500
		DamageClaimWindowDays:    60,
	}
}

func TestFraudVelocity_NewToNew_Blocked(t *testing.T) {
	repo := &mockFraudRepo{
		createdAt: map[string]time.Time{
			"renter": time.Now().Add(-5 * 24 * time.Hour),  // 5 days old
			"host":   time.Now().Add(-10 * 24 * time.Hour), // 10 days old
		},
	}

	result, err := CheckFraudVelocity(context.Background(), repo, "renter", "host", defaultCfg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Blocked {
		t.Error("expected booking to be blocked (new-to-new), but it was not")
	}
}

func TestFraudVelocity_OneEstablished_Allowed(t *testing.T) {
	repo := &mockFraudRepo{
		createdAt: map[string]time.Time{
			"renter": time.Now().Add(-5 * 24 * time.Hour),  // new
			"host":   time.Now().Add(-60 * 24 * time.Hour), // established
		},
		completedCount: map[string]int{
			"renter": 5,
			"host":   10,
		},
	}

	result, err := CheckFraudVelocity(context.Background(), repo, "renter", "host", defaultCfg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Blocked {
		t.Errorf("expected booking to be allowed, blocked with reason: %s", result.BlockReason)
	}
}

func TestFraudVelocity_FirstN_PayoutDelayed(t *testing.T) {
	repo := &mockFraudRepo{
		completedCount: map[string]int{
			"renter": 1, // below first-3
			"host":   10,
		},
	}

	result, err := CheckFraudVelocity(context.Background(), repo, "renter", "host", defaultCfg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Blocked {
		t.Error("expected booking not to be blocked")
	}
	if !result.PayoutDelayed {
		t.Error("expected payout to be delayed for first-N rule")
	}
}

func TestFraudVelocity_DamageCap_Blocked(t *testing.T) {
	repo := &mockFraudRepo{
		createdAt: map[string]time.Time{
			// Host is new (within 60-day window)
			"host": time.Now().Add(-10 * 24 * time.Hour),
		},
		completedCount: map[string]int{
			"renter": 5,
			"host":   5,
		},
		damageClaimed: 55000, // $550 — over $500 cap
	}

	result, err := CheckFraudVelocity(context.Background(), repo, "renter", "host", defaultCfg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Blocked {
		t.Error("expected booking to be blocked (damage cap exceeded)")
	}
}

func TestFraudVelocity_EstablishedBothParties_NoDelay(t *testing.T) {
	repo := &mockFraudRepo{
		completedCount: map[string]int{
			"renter": 5,
			"host":   5,
		},
	}

	result, err := CheckFraudVelocity(context.Background(), repo, "renter", "host", defaultCfg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Blocked {
		t.Errorf("expected no block, got: %s", result.BlockReason)
	}
	if result.PayoutDelayed {
		t.Error("expected no payout delay for established parties")
	}
}
