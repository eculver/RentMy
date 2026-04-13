package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Brett2thered/RentMy/backend/internal/agent/decision"
	"github.com/Brett2thered/RentMy/backend/internal/outcome"
	"github.com/Brett2thered/RentMy/backend/internal/platform/ulid"
)

// TestOutcomeCalibrationEmpty verifies GET /admin/agents/calibration returns 200
// with an empty (or null) array when no decisions have outcome data.
func TestOutcomeCalibrationEmpty(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/admin/agents/calibration", nil, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		body, _ := readBody(resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
}

// TestOutcomeCalibrationRequiresAuth verifies calibration endpoint needs auth.
func TestOutcomeCalibrationRequiresAuth(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/admin/agents/calibration", nil, "")
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// TestOutcomeDecisionsEmpty verifies GET /admin/agents/decisions returns 200
// with an empty (or null) array when no decisions exist.
func TestOutcomeDecisionsEmpty(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/admin/agents/decisions", nil, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		body, _ := readBody(resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
}

// TestOutcomeDecisionsFilterByAgent verifies filtering decisions by agent type.
func TestOutcomeDecisionsFilterByAgent(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	host := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, u.ID, l.ID)

	ctx := context.Background()

	// Seed agent decisions of different types.
	for _, at := range []string{"DISPUTE", "RISK", "DISPUTE"} {
		_, err := pool.Exec(ctx, `
			INSERT INTO agent_decisions (id, agent_type, transaction_id, input, decision, escalated, created_at)
			VALUES ($1, $2, $3, '{}'::jsonb, '{}'::jsonb, false, $4)`,
			ulid.New(), at, b.ID, time.Now().UTC(),
		)
		if err != nil {
			t.Fatalf("seed decision: %v", err)
		}
	}

	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	resp := DoJSON(t, client, http.MethodGet,
		ts.URL+"/api/v1/admin/agents/decisions?agentType=DISPUTE", nil, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		body, _ := readBody(resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var decisions []outcome.DecisionWithOutcome
	MustDecodeJSON(t, resp, &decisions)

	if len(decisions) != 2 {
		t.Errorf("expected 2 DISPUTE decisions, got %d", len(decisions))
	}
	for _, d := range decisions {
		if d.AgentType != "DISPUTE" {
			t.Errorf("expected agentType DISPUTE, got %s", d.AgentType)
		}
	}
}

// TestOutcomeLinkingDisputeNotOverridden verifies that a dispute decision that
// was NOT overridden by a human is marked as correct when outcome linking runs.
func TestOutcomeLinkingDisputeNotOverridden(t *testing.T) {
	ts, _ := NewTestServer(t)
	_ = ts
	pool := NewTestDB(t)
	redis := NewTestRedis(t)
	CleanupDB(t, pool)

	ctx := context.Background()
	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	// Insert a DISPUTE agent decision (no human override).
	decisionID := ulid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO agent_decisions (id, agent_type, transaction_id, input, decision,
		                             confidence, escalated, created_at)
		VALUES ($1, 'DISPUTE', $2, '{}'::jsonb, '{"verdict":"NO_DAMAGE"}'::jsonb,
		        0.85, false, $3)`,
		decisionID, b.ID, time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("seed dispute decision: %v", err)
	}

	// Run outcome linking directly via the service.
	decisionRepo := decision.NewRepository(pool)
	decisionSvc := decision.NewService(decisionRepo)
	outcomeRepo := outcome.NewRepository(pool, redis)
	outcomeSvc := outcome.NewService(outcomeRepo, decisionSvc)

	if err := outcomeSvc.LinkOutcomes(ctx, b.ID); err != nil {
		t.Fatalf("LinkOutcomes: %v", err)
	}

	// Verify the decision now has outcome_correct = true.
	var outcomeCorrect *bool
	var outcomeIDVal *string
	err = pool.QueryRow(ctx,
		`SELECT outcome_correct, outcome_id FROM agent_decisions WHERE id = $1`, decisionID,
	).Scan(&outcomeCorrect, &outcomeIDVal)
	if err != nil {
		t.Fatalf("query decision: %v", err)
	}
	if outcomeCorrect == nil {
		t.Fatal("outcome_correct should not be nil")
	}
	if !*outcomeCorrect {
		t.Error("expected outcome_correct = true for non-overridden dispute")
	}
	if outcomeIDVal == nil || *outcomeIDVal == "" {
		t.Error("outcome_id should be set")
	}
}

// TestOutcomeLinkingDisputeOverridden verifies that a dispute decision that WAS
// overridden by a human is marked as incorrect.
func TestOutcomeLinkingDisputeOverridden(t *testing.T) {
	ts, _ := NewTestServer(t)
	_ = ts
	pool := NewTestDB(t)
	redis := NewTestRedis(t)
	CleanupDB(t, pool)

	ctx := context.Background()
	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	// Insert a DISPUTE agent decision.
	decisionID := ulid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO agent_decisions (id, agent_type, transaction_id, input, decision,
		                             confidence, escalated, created_at)
		VALUES ($1, 'DISPUTE', $2, '{}'::jsonb, '{"verdict":"MINOR_DAMAGE","chargeAmount":500}'::jsonb,
		        0.7, false, $3)`,
		decisionID, b.ID, time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("seed dispute decision: %v", err)
	}

	// Insert a HUMAN_OVERRIDE decision that overrides the above.
	_, err = pool.Exec(ctx, `
		INSERT INTO agent_decisions (id, agent_type, transaction_id, input, decision,
		                             override_of, escalated, created_at)
		VALUES ($1, 'HUMAN_OVERRIDE', $2, '{}'::jsonb, '{"action":"OVERRIDE","chargeAmount":0}'::jsonb,
		        $3, false, $4)`,
		ulid.New(), b.ID, decisionID, time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("seed human override: %v", err)
	}

	decisionRepo := decision.NewRepository(pool)
	decisionSvc := decision.NewService(decisionRepo)
	outcomeRepo := outcome.NewRepository(pool, redis)
	outcomeSvc := outcome.NewService(outcomeRepo, decisionSvc)

	if err := outcomeSvc.LinkOutcomes(ctx, b.ID); err != nil {
		t.Fatalf("LinkOutcomes: %v", err)
	}

	var outcomeCorrect *bool
	err = pool.QueryRow(ctx,
		`SELECT outcome_correct FROM agent_decisions WHERE id = $1`, decisionID,
	).Scan(&outcomeCorrect)
	if err != nil {
		t.Fatalf("query decision: %v", err)
	}
	if outcomeCorrect == nil {
		t.Fatal("outcome_correct should not be nil")
	}
	if *outcomeCorrect {
		t.Error("expected outcome_correct = false for overridden dispute")
	}
}

// TestOutcomeLinkingCalibrationUpdate verifies that calibration metrics are updated
// in Redis after outcome linking runs.
func TestOutcomeLinkingCalibrationUpdate(t *testing.T) {
	ts, _ := NewTestServer(t)
	_ = ts
	pool := NewTestDB(t)
	redis := NewTestRedis(t)
	CleanupDB(t, pool)

	ctx := context.Background()
	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	// Insert multiple DISPUTE decisions with outcomes already linked.
	for i := 0; i < 5; i++ {
		_, err := pool.Exec(ctx, `
			INSERT INTO agent_decisions (id, agent_type, transaction_id, input, decision,
			                             confidence, escalated, outcome_correct, outcome_id, created_at)
			VALUES ($1, 'DISPUTE', $2, '{}'::jsonb, '{}'::jsonb, 0.85, false, true, $3, $4)`,
			ulid.New(), b.ID, ulid.New(), time.Now().UTC(),
		)
		if err != nil {
			t.Fatalf("seed decision %d: %v", i, err)
		}
	}

	decisionRepo := decision.NewRepository(pool)
	decisionSvc := decision.NewService(decisionRepo)
	outcomeRepo := outcome.NewRepository(pool, redis)
	outcomeSvc := outcome.NewService(outcomeRepo, decisionSvc)

	if err := outcomeSvc.UpdateCalibrationMetrics(ctx, "DISPUTE"); err != nil {
		t.Fatalf("UpdateCalibrationMetrics: %v", err)
	}

	// Verify calibration data exists in Redis.
	buckets, err := outcomeRepo.GetCalibrationBuckets(ctx, "DISPUTE")
	if err != nil {
		t.Fatalf("GetCalibrationBuckets: %v", err)
	}

	// The 0.8-0.9 bucket should have data (5 decisions at conf 0.85).
	found := false
	for _, bucket := range buckets {
		if bucket.BucketLow == 0.8 && bucket.BucketHigh == 0.9 && bucket.TotalDecisions > 0 {
			found = true
			if bucket.ActualAccuracy != 1.0 {
				t.Errorf("expected 100%% accuracy in 0.8-0.9 bucket, got %.2f", bucket.ActualAccuracy)
			}
			if bucket.TotalDecisions != 5 {
				t.Errorf("expected 5 decisions in bucket, got %d", bucket.TotalDecisions)
			}
		}
	}
	if !found {
		t.Error("expected calibration data in 0.8-0.9 bucket")
	}
}

// TestOutcomeLinkingRiskLowPassNoDispute verifies that a low-risk RISK decision
// (not escalated) on a transaction with no dispute is marked correct.
func TestOutcomeLinkingRiskLowPassNoDispute(t *testing.T) {
	ts, _ := NewTestServer(t)
	_ = ts
	pool := NewTestDB(t)
	redis := NewTestRedis(t)
	CleanupDB(t, pool)

	ctx := context.Background()
	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	// Insert a RISK decision (not escalated = low risk pass).
	decisionID := ulid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO agent_decisions (id, agent_type, transaction_id, input, decision,
		                             confidence, escalated, created_at)
		VALUES ($1, 'RISK', $2, '{}'::jsonb, '{"control":"APPROVE"}'::jsonb,
		        0.9, false, $3)`,
		decisionID, b.ID, time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("seed risk decision: %v", err)
	}

	decisionRepo := decision.NewRepository(pool)
	decisionSvc := decision.NewService(decisionRepo)
	outcomeRepo := outcome.NewRepository(pool, redis)
	outcomeSvc := outcome.NewService(outcomeRepo, decisionSvc)

	if err := outcomeSvc.LinkOutcomes(ctx, b.ID); err != nil {
		t.Fatalf("LinkOutcomes: %v", err)
	}

	var outcomeCorrect *bool
	err = pool.QueryRow(ctx,
		`SELECT outcome_correct FROM agent_decisions WHERE id = $1`, decisionID,
	).Scan(&outcomeCorrect)
	if err != nil {
		t.Fatalf("query decision: %v", err)
	}
	if outcomeCorrect == nil {
		t.Fatal("outcome_correct should not be nil")
	}
	if !*outcomeCorrect {
		t.Error("expected outcome_correct = true: low-risk pass with no dispute")
	}
}

// TestOutcomeLinkingNoDecisions verifies that LinkOutcomes is a no-op when
// there are no agent decisions for the transaction.
func TestOutcomeLinkingNoDecisions(t *testing.T) {
	ts, _ := NewTestServer(t)
	_ = ts
	pool := NewTestDB(t)
	redis := NewTestRedis(t)
	CleanupDB(t, pool)

	ctx := context.Background()
	decisionRepo := decision.NewRepository(pool)
	decisionSvc := decision.NewService(decisionRepo)
	outcomeRepo := outcome.NewRepository(pool, redis)
	outcomeSvc := outcome.NewService(outcomeRepo, decisionSvc)

	// Should not error on a nonexistent transaction.
	if err := outcomeSvc.LinkOutcomes(ctx, "nonexistent-txn-id"); err != nil {
		t.Fatalf("LinkOutcomes on empty should not fail: %v", err)
	}
}
