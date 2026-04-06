package integration

import (
	"net/http"
	"testing"
)

// TestCreateListing verifies POST /listings creates a listing with location.
func TestCreateListing(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	pricePerDay := 25.0
	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/listings", map[string]any{
		"title":       "Vintage Camera",
		"description": "A lovely 35mm film camera",
		"pricePerDay": pricePerDay,
		"location":    map[string]any{"lat": 37.7749, "lng": -122.4194},
	}, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusCreated {
		body, _ := readBody(resp)
		t.Fatalf("POST /listings: expected 201, got %d: %s", resp.StatusCode, body)
	}

	var body struct {
		Listing struct {
			ID          string  `json:"id"`
			HostID      string  `json:"hostId"`
			Title       string  `json:"title"`
			PricePerDay float64 `json:"pricePerDay"`
			Status      string  `json:"status"`
			Location    *struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			} `json:"location"`
		} `json:"listing"`
	}
	MustDecodeJSON(t, resp, &body)

	l := body.Listing
	if l.ID == "" {
		t.Error("listing.id is empty")
	}
	if l.HostID != u.ID {
		t.Errorf("listing.hostId = %q, want %q", l.HostID, u.ID)
	}
	if l.Title != "Vintage Camera" {
		t.Errorf("listing.title = %q, want %q", l.Title, "Vintage Camera")
	}
	// New listings start as PENDING (awaiting AI appraisal).
	if l.Status != "PENDING" {
		t.Errorf("listing.status = %q, want PENDING", l.Status)
	}
	if l.Location == nil {
		t.Error("listing.location is nil")
	}
}

// TestCreateListingRequiresTitle verifies that creating a listing without a title returns 400.
func TestCreateListingRequiresTitle(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/listings", map[string]any{
		"description": "no title here",
		"pricePerDay": 10.0,
	}, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// TestGetListing verifies GET /listings/:id returns the listing.
func TestGetListing(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")
	l := CreateTestListing(t, pool, u.ID)

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/listings/"+l.ID, nil, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		body, _ := readBody(resp)
		t.Fatalf("GET /listings/%s: expected 200, got %d: %s", l.ID, resp.StatusCode, body)
	}

	var body struct {
		Listing struct {
			ID     string `json:"id"`
			HostID string `json:"hostId"`
			Title  string `json:"title"`
		} `json:"listing"`
	}
	MustDecodeJSON(t, resp, &body)

	if body.Listing.ID != l.ID {
		t.Errorf("listing.id = %q, want %q", body.Listing.ID, l.ID)
	}
	if body.Listing.HostID != u.ID {
		t.Errorf("listing.hostId = %q, want %q", body.Listing.HostID, u.ID)
	}
}

// TestGetListingNotFound verifies that GET /listings/:id for a non-existent listing returns 404.
func TestGetListingNotFound(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/listings/01HNOTEXISTENT00000000000X", nil, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// TestUpdateListing verifies that PUT /listings/:id updates fields for the owner.
func TestUpdateListing(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")
	l := CreateTestListing(t, pool, u.ID)

	newTitle := "Updated Listing Title"
	newPrice := 35.0
	resp := DoJSON(t, client, http.MethodPut, ts.URL+"/api/v1/listings/"+l.ID, map[string]any{
		"title":       newTitle,
		"pricePerDay": newPrice,
	}, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		body, _ := readBody(resp)
		t.Fatalf("PUT /listings/%s: expected 200, got %d: %s", l.ID, resp.StatusCode, body)
	}

	var body struct {
		Listing struct {
			Title       string  `json:"title"`
			PricePerDay float64 `json:"pricePerDay"`
		} `json:"listing"`
	}
	MustDecodeJSON(t, resp, &body)

	if body.Listing.Title != newTitle {
		t.Errorf("listing.title = %q, want %q", body.Listing.Title, newTitle)
	}
	if body.Listing.PricePerDay != newPrice {
		t.Errorf("listing.pricePerDay = %v, want %v", body.Listing.PricePerDay, newPrice)
	}
}

// TestUpdateListingOwnerOnly verifies that a different user cannot update a listing.
func TestUpdateListingOwnerOnly(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	owner := CreateTestUser(t, pool)
	other := CreateTestUser(t, pool)
	otherToken := LoginTestUser(t, client, ts.URL, *other.Email, "password123")

	l := CreateTestListing(t, pool, owner.ID)

	resp := DoJSON(t, client, http.MethodPut, ts.URL+"/api/v1/listings/"+l.ID, map[string]any{
		"title": "Hijacked Title",
	}, otherToken)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

// TestListingSevenDayCeiling verifies that setting maxDuration > 7 days returns 400.
func TestListingSevenDayCeiling(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	// maxDuration of 8 days exceeds the 7-day ceiling.
	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/listings", map[string]any{
		"title":       "Long Rental Item",
		"pricePerDay": 10.0,
		"maxDuration": "192h", // 8 days > 7-day max
	}, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for duration > 7 days, got %d", resp.StatusCode)
	}
}

// TestListMyListings verifies that GET /users/me/listings returns only the caller's listings.
func TestListMyListings(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	other := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	// Create two listings for u and one for other.
	CreateTestListing(t, pool, u.ID)
	CreateTestListing(t, pool, u.ID)
	CreateTestListing(t, pool, other.ID)

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/users/me/listings", nil, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		body, _ := readBody(resp)
		t.Fatalf("GET /users/me/listings: expected 200, got %d: %s", resp.StatusCode, body)
	}

	var body struct {
		Listings []struct {
			ID string `json:"id"`
		} `json:"listings"`
		Total int `json:"total"`
	}
	MustDecodeJSON(t, resp, &body)

	if len(body.Listings) != 2 {
		t.Errorf("expected 2 listings, got %d", len(body.Listings))
	}
	if body.Total != 2 {
		t.Errorf("total = %d, want 2", body.Total)
	}
}
