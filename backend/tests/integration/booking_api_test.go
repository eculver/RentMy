package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/Brett2thered/RentMy/backend/internal/booking"
	"github.com/Brett2thered/RentMy/backend/internal/platform/ulid"
)

// TestBookingGetNotFound verifies GET /bookings/:id returns 404 for a missing booking.
func TestBookingGetNotFound(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/bookings/"+ulid.New(), nil, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// TestBookingGetUnauthorized verifies GET /bookings/:id returns 401 without auth.
func TestBookingGetUnauthorized(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/bookings/"+ulid.New(), nil, "")
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// TestBookingGetForbidden verifies GET /bookings/:id returns 403 when a non-party tries to view.
func TestBookingGetForbidden(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	thirdParty := CreateTestUser(t, pool)

	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	thirdToken := LoginTestUser(t, client, ts.URL, *thirdParty.Email, "password123")

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/bookings/"+b.ID, nil, thirdToken)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

// TestBookingGetSuccess verifies that a renter can GET their booking and the response is correct.
func TestBookingGetSuccess(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/bookings/"+b.ID, nil, renterToken)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		body, _ := readBody(resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Booking *booking.Booking `json:"booking"`
	}
	MustDecodeJSON(t, resp, &result)

	if result.Booking == nil {
		t.Fatal("expected booking in response, got nil")
	}
	if result.Booking.ID != b.ID {
		t.Errorf("booking.ID = %q, want %q", result.Booking.ID, b.ID)
	}
	if result.Booking.Status != booking.StatusRequested {
		t.Errorf("booking.Status = %q, want %q", result.Booking.Status, booking.StatusRequested)
	}
}

// TestBookingGetAsHost verifies that the host can also view the booking.
func TestBookingGetAsHost(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	hostToken := LoginTestUser(t, client, ts.URL, *host.Email, "password123")

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/bookings/"+b.ID, nil, hostToken)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		body, _ := readBody(resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
}

// TestBookingListRenter verifies GET /users/me/bookings returns the renter's bookings.
func TestBookingListRenter(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	CreateTestBooking(t, pool, renter.ID, l.ID)
	CreateTestBooking(t, pool, renter.ID, l.ID)

	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/users/me/bookings", nil, renterToken)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		body, _ := readBody(resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Bookings []booking.Booking `json:"bookings"`
		Total    int               `json:"total"`
	}
	MustDecodeJSON(t, resp, &result)

	if result.Total != 2 {
		t.Errorf("total = %d, want 2", result.Total)
	}
	if len(result.Bookings) != 2 {
		t.Errorf("len(bookings) = %d, want 2", len(result.Bookings))
	}
}

// TestBookingListRenterEmpty verifies GET /users/me/bookings returns empty when there are none.
func TestBookingListRenterEmpty(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/users/me/bookings", nil, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		Bookings []booking.Booking `json:"bookings"`
		Total    int               `json:"total"`
	}
	MustDecodeJSON(t, resp, &result)

	if result.Total != 0 {
		t.Errorf("total = %d, want 0", result.Total)
	}
}

// TestBookingListHost verifies GET /users/me/hosted-bookings returns the host's bookings.
func TestBookingListHost(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	CreateTestBooking(t, pool, renter.ID, l.ID)

	hostToken := LoginTestUser(t, client, ts.URL, *host.Email, "password123")

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/users/me/hosted-bookings", nil, hostToken)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		body, _ := readBody(resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Bookings []booking.Booking `json:"bookings"`
		Total    int               `json:"total"`
	}
	MustDecodeJSON(t, resp, &result)

	if result.Total != 1 {
		t.Errorf("total = %d, want 1", result.Total)
	}
}

// TestBookingAccept verifies that the host can accept a REQUESTED booking → ACCEPTED.
func TestBookingAccept(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	hostToken := LoginTestUser(t, client, ts.URL, *host.Email, "password123")

	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/bookings/"+b.ID+"/accept", nil, hostToken)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusNoContent {
		body, _ := readBody(resp)
		t.Fatalf("expected 204, got %d: %s", resp.StatusCode, body)
	}

	// Verify status changed to ACCEPTED.
	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")
	getResp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/bookings/"+b.ID, nil, renterToken)
	defer DrainBody(getResp)

	var result struct {
		Booking *booking.Booking `json:"booking"`
	}
	MustDecodeJSON(t, getResp, &result)

	if result.Booking.Status != booking.StatusAccepted {
		t.Errorf("after accept: status = %q, want %q", result.Booking.Status, booking.StatusAccepted)
	}
}

// TestBookingAcceptOnlyHost verifies that a renter cannot accept their own booking.
func TestBookingAcceptOnlyHost(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/bookings/"+b.ID+"/accept", nil, renterToken)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 when renter tries to accept, got %d", resp.StatusCode)
	}
}

// TestBookingDecline verifies that the host can decline a REQUESTED booking → DECLINED.
func TestBookingDecline(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	hostToken := LoginTestUser(t, client, ts.URL, *host.Email, "password123")

	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/bookings/"+b.ID+"/decline", nil, hostToken)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusNoContent {
		body, _ := readBody(resp)
		t.Fatalf("expected 204, got %d: %s", resp.StatusCode, body)
	}
}

// TestBookingCancel verifies that a renter can cancel a REQUESTED booking → CANCELLED.
func TestBookingCancel(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/bookings/"+b.ID+"/cancel", nil, renterToken)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusNoContent {
		body, _ := readBody(resp)
		t.Fatalf("expected 204, got %d: %s", resp.StatusCode, body)
	}
}

// TestBookingStateMachineRejections verifies that invalid state transitions return 400.
// Accepting an already-ACCEPTED booking is an invalid transition.
func TestBookingStateMachineRejections(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	hostToken := LoginTestUser(t, client, ts.URL, *host.Email, "password123")

	// First accept succeeds.
	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/bookings/"+b.ID+"/accept", nil, hostToken)
	DrainBody(resp)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("initial accept: expected 204, got %d", resp.StatusCode)
	}

	// Second accept on an ACCEPTED booking must fail.
	resp2 := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/bookings/"+b.ID+"/accept", nil, hostToken)
	defer DrainBody(resp2)

	if resp2.StatusCode != http.StatusBadRequest {
		t.Fatalf("double accept: expected 400, got %d", resp2.StatusCode)
	}
}

// TestBookingCheckInHandoffIncomplete verifies check-in returns 422 when neither party
// has completed proximity verification.
func TestBookingCheckInHandoffIncomplete(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	hostToken := LoginTestUser(t, client, ts.URL, *host.Email, "password123")

	// Accept first — booking must be ACCEPTED before check-in.
	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/bookings/"+b.ID+"/accept", nil, hostToken)
	DrainBody(resp)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("accept: expected 204, got %d", resp.StatusCode)
	}

	// Attempt check-in without proximity verification → 422.
	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")
	checkInResp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/bookings/"+b.ID+"/check-in", nil, renterToken)
	defer DrainBody(checkInResp)

	if checkInResp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("check-in without proximity: expected 422, got %d", checkInResp.StatusCode)
	}
}

// TestBookingCreateDurationExceeds7Days verifies that bookings longer than 7 days
// are rejected with 400.
func TestBookingCreateDurationExceeds7Days(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	start := time.Now().Add(24 * time.Hour)
	end := start.Add(8 * 24 * time.Hour) // 8 days > 7-day limit

	body := map[string]string{
		"listingId":       l.ID,
		"paymentMethodId": "pm_test_placeholder",
		"scheduledStart":  start.Format(time.RFC3339),
		"scheduledEnd":    end.Format(time.RFC3339),
	}

	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/bookings", body, renterToken)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusBadRequest {
		rb, _ := readBody(resp)
		t.Fatalf("expected 400 for >7-day duration, got %d: %s", resp.StatusCode, rb)
	}
}

// TestBookingCreateMissingListingID verifies that booking creation rejects a missing listingId.
func TestBookingCreateMissingListingID(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	body := map[string]string{
		"paymentMethodId": "pm_test",
		"scheduledStart":  time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"scheduledEnd":    time.Now().Add(28 * time.Hour).Format(time.RFC3339),
	}

	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/bookings", body, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// TestBookingCreateMissingPaymentMethod verifies that booking creation rejects a missing
// paymentMethodId.
func TestBookingCreateMissingPaymentMethod(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	token := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	body := map[string]string{
		"listingId":      l.ID,
		"scheduledStart": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"scheduledEnd":   time.Now().Add(28 * time.Hour).Format(time.RFC3339),
	}

	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/bookings", body, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// TestBookingCreateRequiresAuth verifies that booking creation returns 401 without auth.
func TestBookingCreateRequiresAuth(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)

	body := map[string]string{
		"listingId":       l.ID,
		"paymentMethodId": "pm_test",
		"scheduledStart":  time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"scheduledEnd":    time.Now().Add(28 * time.Hour).Format(time.RFC3339),
	}

	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/bookings", body, "")
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// TestBookingCancelNotFound verifies cancelling a non-existent booking returns 404.
func TestBookingCancelNotFound(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/bookings/"+ulid.New()+"/cancel", nil, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// TestBookingFraudVelocityCheck verifies that bookings between two brand-new accounts
// are blocked by the fraud velocity rules (both-new-account rule).
func TestBookingFraudVelocityCheck(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	// Both users are brand-new (0 days old), satisfying the both-new-account rule.
	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	start := time.Now().Add(24 * time.Hour)
	end := start.Add(4 * time.Hour)

	body := map[string]string{
		"listingId":       l.ID,
		"paymentMethodId": "pm_test_placeholder",
		"scheduledStart":  start.Format(time.RFC3339),
		"scheduledEnd":    end.Format(time.RFC3339),
	}

	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/bookings", body, renterToken)
	defer DrainBody(resp)

	// Both accounts are < FraudNewAccountDays (30 days), so the booking should be blocked.
	if resp.StatusCode != http.StatusForbidden {
		rb, _ := readBody(resp)
		t.Fatalf("expected 403 (fraud blocked for both-new-account), got %d: %s", resp.StatusCode, rb)
	}
}
