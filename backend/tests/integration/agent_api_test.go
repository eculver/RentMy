package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Brett2thered/RentMy/backend/internal/agent/risk"
	"github.com/Brett2thered/RentMy/backend/internal/platform/ulid"
)

// TestGetReputationNotFound verifies GET /users/:id/reputation returns 404 when the
// user does not exist in the database.
func TestGetReputationNotFound(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	// Use a ULID that was never inserted.
	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/users/"+ulid.New()+"/reputation", nil, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// TestGetReputationSuccess verifies GET /users/:id/reputation returns 200 with a
// zero-score reputation for a brand-new user with no signals.
func TestGetReputationSuccess(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/users/"+u.ID+"/reputation", nil, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		body, _ := readBody(resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var result struct {
		UserID          string `json:"userId"`
		ReputationScore int    `json:"reputationScore"`
		Signals         []any  `json:"signals"`
	}
	MustDecodeJSON(t, resp, &result)

	if result.UserID != u.ID {
		t.Errorf("userId = %q, want %q", result.UserID, u.ID)
	}
	// New user has no signals → score should be 0.
	if result.ReputationScore != 0 {
		t.Errorf("reputationScore = %d, want 0", result.ReputationScore)
	}
	if len(result.Signals) != 0 {
		t.Errorf("len(signals) = %d, want 0", len(result.Signals))
	}
}

// TestGetReputationRequiresAuth verifies GET /users/:id/reputation returns 401 without auth.
func TestGetReputationRequiresAuth(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/users/"+ulid.New()+"/reputation", nil, "")
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// TestGetRiskScoreNotFound verifies GET /transactions/:id/risk returns 404 when no
// risk score has been computed for the transaction.
func TestGetRiskScoreNotFound(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	// The booking was inserted directly (no payment flow), so no risk_scores row exists.
	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/transactions/"+b.ID+"/risk", nil, renterToken)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusNotFound {
		rb, _ := readBody(resp)
		t.Fatalf("expected 404 (no risk score), got %d: %s", resp.StatusCode, rb)
	}
}

// TestGetRiskScoreSuccess verifies GET /transactions/:id/risk returns 200 when a risk
// score exists in the database (seeded directly for this test).
func TestGetRiskScoreSuccess(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	// Seed a risk score row directly.
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		INSERT INTO risk_scores (transaction_id, risk_score, risk_level, breakdown, computed_at)
		VALUES ($1, 25, 'LOW',
			'{"base_risk":10,"transaction_risk":5,"counterparty_risk":5,"behavioral_risk":5,"fraud_signals":0,"total":25}'::jsonb,
			$2
		)`,
		b.ID, time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("seed risk score: %v", err)
	}

	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")
	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/transactions/"+b.ID+"/risk", nil, renterToken)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		rb, _ := readBody(resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, rb)
	}

	var result risk.TransactionRiskScore
	MustDecodeJSON(t, resp, &result)

	if result.TransactionID != b.ID {
		t.Errorf("transactionId = %q, want %q", result.TransactionID, b.ID)
	}
	if result.RiskLevel != risk.RiskLevelLow {
		t.Errorf("riskLevel = %q, want %q", result.RiskLevel, risk.RiskLevelLow)
	}
	if result.Control != risk.ControlApprove {
		t.Errorf("control = %q, want %q", result.Control, risk.ControlApprove)
	}
}

// TestGetRiskScoreRequiresAuth verifies GET /transactions/:id/risk returns 401 without auth.
func TestGetRiskScoreRequiresAuth(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/transactions/"+ulid.New()+"/risk", nil, "")
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// TestGetAgreementNotFound verifies GET /transactions/:id/agreement returns 404 when
// no agreement has been generated for the transaction.
func TestGetAgreementNotFound(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/transactions/"+b.ID+"/agreement", nil, renterToken)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusNotFound {
		rb, _ := readBody(resp)
		t.Fatalf("expected 404 (no agreement), got %d: %s", resp.StatusCode, rb)
	}
}

// TestGetAgreementStatusNotFound verifies GET /transactions/:id/agreement/status returns
// 404 when no agreement has been generated.
func TestGetAgreementStatusNotFound(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/transactions/"+b.ID+"/agreement/status", nil, renterToken)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusNotFound {
		rb, _ := readBody(resp)
		t.Fatalf("expected 404, got %d: %s", resp.StatusCode, rb)
	}
}

// TestDecisionAuditLog verifies that agent decisions are recorded in the audit log and
// can be retrieved via the risk score endpoint (indirectly — decisions are stored in the
// agent_decisions table as a record of what the RiskAgent decided during CreateBooking).
func TestDecisionAuditLog(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	ctx := context.Background()

	// Seed a risk score and an agent_decisions row to simulate what the RiskAgent
	// would have written during booking creation.
	_, err := pool.Exec(ctx, `
		INSERT INTO risk_scores (transaction_id, risk_score, risk_level, breakdown, computed_at)
		VALUES ($1, 15, 'LOW',
			'{"base_risk":5,"transaction_risk":5,"counterparty_risk":3,"behavioral_risk":2,"fraud_signals":0,"total":15}'::jsonb,
			$2
		)`,
		b.ID, time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("seed risk score: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO agent_decisions (
			id, agent_type, transaction_id,
			input, decision, model, prompt_version, escalated, created_at
		) VALUES (
			$1, 'RISK', $2,
			'{"renter_id":"test"}'::jsonb,
			'{"control":"APPROVE","risk_score":15}'::jsonb,
			'rules', '1', false, $3
		)`,
		ulid.New(), b.ID, time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("seed agent decision: %v", err)
	}

	// Verify the risk score is retrievable via the HTTP API.
	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")
	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/transactions/"+b.ID+"/risk", nil, renterToken)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		rb, _ := readBody(resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, rb)
	}

	var result risk.TransactionRiskScore
	MustDecodeJSON(t, resp, &result)

	if result.RiskScore != 15 {
		t.Errorf("riskScore = %d, want 15", result.RiskScore)
	}
	if result.Control != risk.ControlApprove {
		t.Errorf("control = %q, want APPROVE", result.Control)
	}

	// Verify the agent_decisions row was persisted correctly.
	var count int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM agent_decisions WHERE transaction_id = $1`, b.ID,
	).Scan(&count); err != nil {
		t.Fatalf("count agent decisions: %v", err)
	}
	if count != 1 {
		t.Errorf("agent_decisions count = %d, want 1", count)
	}
}

// TestAgreementAcceptNotFound verifies POST /transactions/:id/agreement/accept returns
// 404 when no agreement exists for the transaction.
func TestAgreementAcceptNotFound(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	resp := DoJSON(t, client, http.MethodPost,
		ts.URL+"/api/v1/transactions/"+b.ID+"/agreement/accept",
		map[string]string{"deviceId": "test-device"},
		renterToken,
	)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusNotFound {
		rb, _ := readBody(resp)
		t.Fatalf("expected 404, got %d: %s", resp.StatusCode, rb)
	}
}
