package integration

import (
	"net/http"
	"testing"

	"github.com/Brett2thered/RentMy/backend/internal/proximity"
	"github.com/Brett2thered/RentMy/backend/internal/platform/ulid"
)

// TestProximityVerifyGPSRequiresAuth verifies POST /proximity/verify returns 401 without auth.
func TestProximityVerifyGPSRequiresAuth(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	body := map[string]any{
		"transactionId": ulid.New(),
		"lat":           37.7749,
		"lng":           -122.4194,
		"proofType":     "CHECK_IN",
	}

	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/proximity/verify", body, "")
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// TestProximityVerifyGPSMissingTransactionID verifies that omitting transactionId returns 400.
func TestProximityVerifyGPSMissingTransactionID(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	body := map[string]any{
		"lat":       37.7749,
		"lng":       -122.4194,
		"proofType": "CHECK_IN",
	}

	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/proximity/verify", body, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// TestProximityVerifyGPSInvalidProofType verifies that an unknown proofType returns 400.
func TestProximityVerifyGPSInvalidProofType(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	body := map[string]any{
		"transactionId": ulid.New(),
		"lat":           37.7749,
		"lng":           -122.4194,
		"proofType":     "INVALID_TYPE",
	}

	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/proximity/verify", body, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// TestProximityVerifyGPSTooFarAway verifies that coordinates far from the listing
// location return 422 (ErrTooFarAway).
func TestProximityVerifyGPSTooFarAway(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID) // listing is at (37.7749, -122.4194) — San Francisco
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	// New York City is ~4000 km away — clearly outside the 100-meter threshold.
	body := map[string]any{
		"transactionId": b.ID,
		"lat":           40.7128,
		"lng":           -74.0060,
		"proofType":     "CHECK_IN",
	}

	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/proximity/verify", body, renterToken)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusUnprocessableEntity {
		rb, _ := readBody(resp)
		t.Fatalf("expected 422 (too far away), got %d: %s", resp.StatusCode, rb)
	}
}

// TestProximityVerifyGPSWithinRange verifies that coordinates within the threshold return
// 200 with verified=true.
func TestProximityVerifyGPSWithinRange(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID) // listing at (37.7749, -122.4194)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	// Use the exact listing coordinates — distance is 0, well within 100 m threshold.
	body := map[string]any{
		"transactionId": b.ID,
		"lat":           37.7749,
		"lng":           -122.4194,
		"proofType":     "CHECK_IN",
	}

	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/proximity/verify", body, renterToken)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		rb, _ := readBody(resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, rb)
	}

	var result struct {
		Verified bool `json:"verified"`
	}
	MustDecodeJSON(t, resp, &result)

	if !result.Verified {
		t.Error("expected verified=true for coordinates at listing location")
	}
}

// TestProximityPINVerifyInvalidLength verifies that a PIN that is not 4 digits returns 400.
func TestProximityPINVerifyInvalidLength(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	cases := []struct {
		name string
		pin  string
	}{
		{name: "too short", pin: "123"},
		{name: "too long", pin: "12345"},
		{name: "empty", pin: ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]any{
				"transactionId": ulid.New(),
				"pin":           tc.pin,
			}
			resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/proximity/pin", body, token)
			defer DrainBody(resp)

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("%s: expected 400, got %d", tc.name, resp.StatusCode)
			}
		})
	}
}

// TestProximityPINVerifyMissingTransactionID verifies that omitting transactionId returns 400.
func TestProximityPINVerifyMissingTransactionID(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	body := map[string]any{
		"pin": "1234",
	}

	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/proximity/pin", body, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// TestProximityPINGenerationAfterAccept verifies that after the host accepts a booking,
// a host-side CHECK_IN proximity proof is created (the PIN scaffold).
func TestProximityPINGenerationAfterAccept(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	hostToken := LoginTestUser(t, client, ts.URL, *host.Email, "password123")

	// Accept the booking — this triggers GenerateCheckInPIN inside the service.
	acceptResp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/bookings/"+b.ID+"/accept", nil, hostToken)
	DrainBody(acceptResp)
	if acceptResp.StatusCode != http.StatusNoContent {
		t.Fatalf("accept: expected 204, got %d", acceptResp.StatusCode)
	}

	// GET /bookings/:id/proximity should now show a host CHECK_IN proof was created.
	statusResp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/bookings/"+b.ID+"/proximity", nil, hostToken)
	defer DrainBody(statusResp)

	if statusResp.StatusCode != http.StatusOK {
		rb, _ := readBody(statusResp)
		t.Fatalf("expected 200 for proximity status, got %d: %s", statusResp.StatusCode, rb)
	}

	var status proximity.ProximityStatus
	MustDecodeJSON(t, statusResp, &status)

	if len(status.CheckInProofs) == 0 {
		t.Error("expected at least one CHECK_IN proof after booking acceptance")
	}
}

// TestProximityGetStatusUnauthorized verifies GET /bookings/:id/proximity returns 401 without auth.
func TestProximityGetStatusUnauthorized(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/bookings/"+ulid.New()+"/proximity", nil, "")
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// TestProximityGetStatusForbiddenForNonParty verifies that a non-party cannot view
// proximity status.
func TestProximityGetStatusForbiddenForNonParty(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	thirdParty := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	thirdToken := LoginTestUser(t, client, ts.URL, *thirdParty.Email, "password123")

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/bookings/"+b.ID+"/proximity", nil, thirdToken)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-party, got %d", resp.StatusCode)
	}
}

// TestProximitySMSFallbackMissingFields verifies POST /proximity/sms-fallback validates
// required fields.
func TestProximitySMSFallbackMissingFields(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	cases := []struct {
		name string
		body map[string]any
	}{
		{
			name: "missing transactionId",
			body: map[string]any{"toPhone": "+12135551234"},
		},
		{
			name: "missing toPhone",
			body: map[string]any{"transactionId": ulid.New()},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/proximity/sms-fallback", tc.body, token)
			defer DrainBody(resp)

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("%s: expected 400, got %d", tc.name, resp.StatusCode)
			}
		})
	}
}
