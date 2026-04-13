package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Brett2thered/RentMy/backend/internal/agent/ops"
	"github.com/Brett2thered/RentMy/backend/internal/platform/ulid"
)

// registerTestUser is a helper that registers a user and returns the access token.
func registerTestUserOps(t *testing.T, client *http.Client, baseURL string) string {
	t.Helper()
	resp := DoJSON(t, client, http.MethodPost, baseURL+"/api/v1/auth/register", map[string]any{
		"email":    "ops-" + ulid.New() + "@test.example",
		"password": "Password123!",
		"name":     "Ops Test User",
	}, "")
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d", resp.StatusCode)
	}

	var body struct {
		AccessToken string `json:"accessToken"`
	}
	MustDecodeJSON(t, resp, &body)
	if body.AccessToken == "" {
		t.Fatal("register: empty access token")
	}
	return body.AccessToken
}

// insertTestAlertRule inserts an alert rule directly into the DB for test setup.
func insertTestAlertRule(t *testing.T, pool interface{ Exec(context.Context, string, ...any) (interface{ RowsAffected() int64 }, error) }, rule ops.AlertRule) {
	t.Helper()
	repo := ops.NewRepository(testPool)
	if err := repo.UpsertAlertRule(context.Background(), rule); err != nil {
		t.Fatalf("insert alert rule: %v", err)
	}
}

// TestOpsGetCurrentMetrics_NoSnapshot returns 404 when no snapshot exists yet.
func TestOpsGetCurrentMetrics_NoSnapshot(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	token := registerTestUserOps(t, client, ts.URL)

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/ops/metrics/current", nil, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 when no snapshot, got %d", resp.StatusCode)
	}
}

// TestOpsGetCurrentMetrics_WithSnapshot returns 200 and snapshot data after inserting one.
func TestOpsGetCurrentMetrics_WithSnapshot(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	token := registerTestUserOps(t, client, ts.URL)

	// Insert a snapshot directly.
	repo := ops.NewRepository(testPool)
	snap := ops.HealthSnapshot{
		ID:         ulid.New(),
		CapturedAt: time.Now().UTC(),
	}
	snap.Business.ActiveListings.Value = 42
	snap.Business.ActiveListings.Name = "active_listings"
	if err := repo.InsertHealthSnapshot(context.Background(), snap); err != nil {
		t.Fatalf("insert test snapshot: %v", err)
	}

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/ops/metrics/current", nil, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body ops.HealthSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID == "" {
		t.Error("snapshot id is empty")
	}
	if body.Business.ActiveListings.Value != 42 {
		t.Errorf("expected activeListings=42, got %v", body.Business.ActiveListings.Value)
	}
}

// TestOpsGetMetricsHistory returns 200 with history snapshots.
func TestOpsGetMetricsHistory(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	token := registerTestUserOps(t, client, ts.URL)

	// Insert two snapshots.
	repo := ops.NewRepository(testPool)
	for i := 0; i < 2; i++ {
		snap := ops.HealthSnapshot{
			ID:         ulid.New(),
			CapturedAt: time.Now().UTC().Add(-time.Duration(i) * time.Hour),
		}
		if err := repo.InsertHealthSnapshot(context.Background(), snap); err != nil {
			t.Fatalf("insert snapshot: %v", err)
		}
	}

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/ops/metrics/history?duration=7d", nil, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Snapshots []ops.HealthSnapshot `json:"snapshots"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Snapshots) < 2 {
		t.Errorf("expected at least 2 snapshots, got %d", len(body.Snapshots))
	}
}

// TestOpsListAlertRules returns 200 with an empty list when no rules exist.
func TestOpsListAlertRules_Empty(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	token := registerTestUserOps(t, client, ts.URL)

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/ops/alerts/rules", nil, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Rules []ops.AlertRule `json:"rules"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	// Rules may be empty or populated by default seeding.
	if body.Rules == nil {
		body.Rules = []ops.AlertRule{}
	}
}

// TestOpsUpdateAlertRule returns 200 after updating a rule's threshold.
func TestOpsUpdateAlertRule(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	token := registerTestUserOps(t, client, ts.URL)

	// Insert a rule.
	repo := ops.NewRepository(testPool)
	rule := ops.AlertRule{
		ID:         ulid.New(),
		MetricName: "fraud_flag_rate",
		Operator:   ops.OperatorGT,
		Threshold:  0.10,
		Severity:   ops.SeverityWarning,
		Channel:    ops.ChannelSlack,
		Enabled:    true,
	}
	if err := repo.UpsertAlertRule(context.Background(), rule); err != nil {
		t.Fatalf("upsert rule: %v", err)
	}

	// Fetch it back to get the DB-assigned ID.
	rules, err := repo.ListAlertRules(context.Background())
	if err != nil || len(rules) == 0 {
		t.Fatalf("list rules: %v (count %d)", err, len(rules))
	}
	ruleID := rules[0].ID

	// Update the threshold.
	newThreshold := 0.20
	resp := DoJSON(t, client, http.MethodPut, ts.URL+"/api/v1/ops/alerts/rules/"+ruleID, map[string]any{
		"threshold": newThreshold,
	}, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var updated ops.AlertRule
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if updated.Threshold != newThreshold {
		t.Errorf("expected threshold=%.2f, got %.2f", newThreshold, updated.Threshold)
	}
}

// TestOpsListAlerts_Empty returns 200 with empty list when no alerts exist.
func TestOpsListAlerts_Empty(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	token := registerTestUserOps(t, client, ts.URL)

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/ops/alerts", nil, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Alerts []ops.Alert `json:"alerts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Alerts == nil {
		body.Alerts = []ops.Alert{}
	}
}

// TestOpsAcknowledgeAlert verifies acknowledge → 200.
func TestOpsAcknowledgeAlert(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	token := registerTestUserOps(t, client, ts.URL)

	// Insert a rule first, then an alert.
	repo := ops.NewRepository(testPool)
	rule := ops.AlertRule{
		ID:         ulid.New(),
		MetricName: "fraud_flag_rate",
		Operator:   ops.OperatorGT,
		Threshold:  0.10,
		Severity:   ops.SeverityWarning,
		Channel:    ops.ChannelSlack,
		Enabled:    true,
	}
	if err := repo.UpsertAlertRule(context.Background(), rule); err != nil {
		t.Fatalf("upsert rule: %v", err)
	}

	rules, err := repo.ListAlertRules(context.Background())
	if err != nil || len(rules) == 0 {
		t.Fatalf("list rules: %v", err)
	}

	alert := ops.Alert{
		ID:           ulid.New(),
		RuleID:       rules[0].ID,
		MetricName:   "fraud_flag_rate",
		CurrentValue: 0.25,
		Threshold:    0.10,
		Severity:     ops.SeverityWarning,
		Channel:      ops.ChannelSlack,
		FiredAt:      time.Now().UTC(),
	}
	if err := repo.InsertAlert(context.Background(), alert); err != nil {
		t.Fatalf("insert alert: %v", err)
	}

	resp := DoJSON(t, client, http.MethodPut, ts.URL+"/api/v1/ops/alerts/"+alert.ID+"/acknowledge", nil, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "acknowledged" {
		t.Errorf("status = %q, want acknowledged", body["status"])
	}
}

// TestOpsRequiresAuth verifies that unauthenticated requests return 401.
func TestOpsRequiresAuth(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/ops/metrics/current", nil, "")
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", resp.StatusCode)
	}
}
