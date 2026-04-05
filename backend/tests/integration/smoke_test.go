package integration

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestHealthSmoke verifies that the full application server starts, connects to
// Postgres and Redis via testcontainers, and returns 200 on the /health endpoint.
// This is the gating test for the integration test infrastructure: if this fails,
// all subsequent integration tests will also fail.
func TestHealthSmoke(t *testing.T) {
	ts, client := NewTestServer(t)

	resp, err := client.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Status   string `json:"status"`
		Postgres string `json:"postgres"`
		Redis    string `json:"redis"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}

	if body.Status != "ok" {
		t.Errorf("health status = %q, want %q", body.Status, "ok")
	}
	if body.Postgres != "connected" {
		t.Errorf("postgres = %q, want %q", body.Postgres, "connected")
	}
	if body.Redis != "connected" {
		t.Errorf("redis = %q, want %q", body.Redis, "connected")
	}
}
