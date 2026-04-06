package integration

import (
	"fmt"
	"math"
	"net/http"
	"testing"
)

// TestFeedNearby verifies GET /discovery/feed returns nearby active listings.
func TestFeedNearby(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	// Insert two listings at San Francisco (factory default).
	CreateTestListing(t, pool, u.ID)
	CreateTestListing(t, pool, u.ID)

	// Query from San Francisco — both listings should appear.
	url := ts.URL + "/api/v1/discovery/feed?lat=37.7749&lng=-122.4194"
	resp := DoJSON(t, client, http.MethodGet, url, nil, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		body, _ := readBody(resp)
		t.Fatalf("GET /discovery/feed: expected 200, got %d: %s", resp.StatusCode, body)
	}

	var body struct {
		Listings []struct {
			ID    string `json:"id"`
			Lat   float64 `json:"lat"`
			Lng   float64 `json:"lng"`
			Title string `json:"title"`
		} `json:"listings"`
		Count int `json:"count"`
	}
	MustDecodeJSON(t, resp, &body)

	if body.Count < 2 {
		t.Errorf("expected at least 2 listings, got %d", body.Count)
	}
	if len(body.Listings) < 2 {
		t.Errorf("expected at least 2 listings in array, got %d", len(body.Listings))
	}
}

// TestFeedPagination verifies that the limit query parameter is respected.
func TestFeedPagination(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	// Insert 3 listings.
	for i := 0; i < 3; i++ {
		CreateTestListing(t, pool, u.ID)
	}

	// Request with limit=2 — should return at most 2.
	url := ts.URL + "/api/v1/discovery/feed?lat=37.7749&lng=-122.4194&limit=2"
	resp := DoJSON(t, client, http.MethodGet, url, nil, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		body, _ := readBody(resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var body struct {
		Listings []struct{ ID string } `json:"listings"`
	}
	MustDecodeJSON(t, resp, &body)

	if len(body.Listings) > 2 {
		t.Errorf("expected at most 2 listings with limit=2, got %d", len(body.Listings))
	}
}

// TestFeedRequiresLatLng verifies that the feed endpoint returns 400 when lat/lng are missing.
func TestFeedRequiresLatLng(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/discovery/feed", nil, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// TestSearchFulltext verifies GET /discovery/search returns listings matching the query.
func TestSearchFulltext(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	// The factory inserts listings with title "Test Listing <prefix>".
	CreateTestListing(t, pool, u.ID)

	url := ts.URL + "/api/v1/discovery/search?q=Test+Listing&lat=37.7749&lng=-122.4194"
	resp := DoJSON(t, client, http.MethodGet, url, nil, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		body, _ := readBody(resp)
		t.Fatalf("GET /discovery/search: expected 200, got %d: %s", resp.StatusCode, body)
	}

	var body struct {
		Listings []struct{ ID string } `json:"listings"`
		Count    int                   `json:"count"`
	}
	MustDecodeJSON(t, resp, &body)

	if body.Count < 1 {
		t.Errorf("expected at least 1 search result, got %d", body.Count)
	}
}

// TestSearchRequiresQ verifies that GET /discovery/search without q returns 400.
func TestSearchRequiresQ(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/discovery/search?lat=37.7749&lng=-122.4194", nil, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// TestMapBoundingBox verifies GET /discovery/map returns listings within the bbox.
func TestMapBoundingBox(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	// Factory inserts at lat=37.7749, lng=-122.4194 (San Francisco).
	CreateTestListing(t, pool, u.ID)

	// Bounding box that covers San Francisco.
	url := fmt.Sprintf("%s/api/v1/discovery/map?sw_lat=37.7&sw_lng=-122.5&ne_lat=37.85&ne_lng=-122.3", ts.URL)
	resp := DoJSON(t, client, http.MethodGet, url, nil, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		body, _ := readBody(resp)
		t.Fatalf("GET /discovery/map: expected 200, got %d: %s", resp.StatusCode, body)
	}

	var body struct {
		Listings []struct {
			ID  string  `json:"id"`
			Lat float64 `json:"lat"`
			Lng float64 `json:"lng"`
		} `json:"listings"`
		Count int `json:"count"`
	}
	MustDecodeJSON(t, resp, &body)

	if body.Count < 1 {
		t.Errorf("expected at least 1 listing in bbox, got %d", body.Count)
	}
}

// TestMapRequiresBbox verifies GET /discovery/map returns 400 when bbox params are missing.
func TestMapRequiresBbox(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/discovery/map", nil, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// TestLocationFuzzing verifies that returned feed coordinates differ from the stored location.
// The discovery service applies ~500m jitter to protect host privacy.
func TestLocationFuzzing(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	// Factory inserts at exact lat=37.7749, lng=-122.4194.
	CreateTestListing(t, pool, u.ID)

	url := ts.URL + "/api/v1/discovery/feed?lat=37.7749&lng=-122.4194"
	resp := DoJSON(t, client, http.MethodGet, url, nil, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		body, _ := readBody(resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var body struct {
		Listings []struct {
			Lat float64 `json:"lat"`
			Lng float64 `json:"lng"`
		} `json:"listings"`
	}
	MustDecodeJSON(t, resp, &body)

	if len(body.Listings) == 0 {
		t.Fatal("no listings returned, cannot check fuzz")
	}

	// At least one returned listing should have fuzzed coordinates that differ from
	// the stored exact location. We allow a small epsilon for floating point comparison.
	exactLat, exactLng := 37.7749, -122.4194
	const epsilon = 1e-6 // much smaller than ~500m jitter

	allExact := true
	for _, l := range body.Listings {
		if math.Abs(l.Lat-exactLat) > epsilon || math.Abs(l.Lng-exactLng) > epsilon {
			allExact = false
			break
		}
	}
	if allExact {
		t.Error("all returned coordinates match exact stored location — location fuzzing may not be applied")
	}
}
