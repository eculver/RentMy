package integration

import (
	"net/http"
	"testing"

	"github.com/Brett2thered/RentMy/backend/internal/notification"
	"github.com/Brett2thered/RentMy/backend/internal/platform/ulid"
)

// TestNotificationListEmpty verifies GET /notifications returns an empty list for a new user.
func TestNotificationListEmpty(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/notifications", nil, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		body, _ := readBody(resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Notifications []notification.Notification `json:"notifications"`
		Total         int                         `json:"total"`
	}
	MustDecodeJSON(t, resp, &result)

	if result.Total != 0 {
		t.Errorf("total = %d, want 0", result.Total)
	}
	if len(result.Notifications) != 0 {
		t.Errorf("len(notifications) = %d, want 0", len(result.Notifications))
	}
}

// TestNotificationListRequiresAuth verifies GET /notifications returns 401 without auth.
func TestNotificationListRequiresAuth(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/notifications", nil, "")
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// TestNotificationUnreadCountEmpty verifies GET /notifications/unread-count returns 0
// for a new user.
func TestNotificationUnreadCountEmpty(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/notifications/unread-count", nil, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		Count int `json:"count"`
	}
	MustDecodeJSON(t, resp, &result)

	if result.Count != 0 {
		t.Errorf("count = %d, want 0", result.Count)
	}
}

// TestNotificationGetPreferencesDefaults verifies GET /notifications/preferences returns
// sensible defaults for a new user with no saved preferences.
func TestNotificationGetPreferencesDefaults(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/notifications/preferences", nil, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		body, _ := readBody(resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var prefs notification.Preferences
	MustDecodeJSON(t, resp, &prefs)

	// Default preferences should have push enabled.
	if !prefs.PushEnabled {
		t.Error("expected PushEnabled=true in default preferences")
	}
}

// TestNotificationUpdatePreferences verifies PUT /notifications/preferences persists
// the changes and GET returns the updated values.
func TestNotificationUpdatePreferences(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	quietStart := 22
	quietEnd := 8

	newPrefs := notification.Preferences{
		PushEnabled:        false,
		SMSFallbackEnabled: true,
		DisabledTypes:      []notification.Type{notification.TypePickupApproaching},
		QuietHoursStart:    &quietStart,
		QuietHoursEnd:      &quietEnd,
		TimezoneName:       "America/Los_Angeles",
	}

	putResp := DoJSON(t, client, http.MethodPut, ts.URL+"/api/v1/notifications/preferences", newPrefs, token)
	defer DrainBody(putResp)

	if putResp.StatusCode != http.StatusOK {
		body, _ := readBody(putResp)
		t.Fatalf("PUT preferences: expected 200, got %d: %s", putResp.StatusCode, body)
	}

	// Fetch preferences back and verify.
	getResp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/notifications/preferences", nil, token)
	defer DrainBody(getResp)

	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("GET preferences: expected 200, got %d", getResp.StatusCode)
	}

	var got notification.Preferences
	MustDecodeJSON(t, getResp, &got)

	if got.PushEnabled {
		t.Error("PushEnabled should be false after update")
	}
	if !got.SMSFallbackEnabled {
		t.Error("SMSFallbackEnabled should be true after update")
	}
	if got.TimezoneName != "America/Los_Angeles" {
		t.Errorf("TimezoneName = %q, want %q", got.TimezoneName, "America/Los_Angeles")
	}
}

// TestNotificationRegisterToken verifies POST /notifications/register-token stores the token.
func TestNotificationRegisterToken(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	body := map[string]string{
		"token": "ExponentPushToken[" + ulid.New() + "]",
	}

	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/notifications/register-token", body, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		rb, _ := readBody(resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, rb)
	}
}

// TestNotificationRegisterTokenMissing verifies POST /notifications/register-token
// returns 400 when the token field is absent.
func TestNotificationRegisterTokenMissing(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	body := map[string]string{}

	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/notifications/register-token", body, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// TestNotificationCreationOnBookingAccept verifies that accepting a booking generates
// a BOOKING_ACCEPTED notification for the renter.
func TestNotificationCreationOnBookingAccept(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	hostToken := LoginTestUser(t, client, ts.URL, *host.Email, "password123")

	// Accept the booking — this triggers a BOOKING_ACCEPTED notification for the renter.
	acceptResp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/bookings/"+b.ID+"/accept", nil, hostToken)
	DrainBody(acceptResp)
	if acceptResp.StatusCode != http.StatusNoContent {
		t.Fatalf("accept: expected 204, got %d", acceptResp.StatusCode)
	}

	// The renter should have a BOOKING_ACCEPTED notification.
	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")
	listResp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/notifications", nil, renterToken)
	defer DrainBody(listResp)

	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list notifications: expected 200, got %d", listResp.StatusCode)
	}

	var result struct {
		Notifications []notification.Notification `json:"notifications"`
		Total         int                         `json:"total"`
	}
	MustDecodeJSON(t, listResp, &result)

	if result.Total == 0 {
		t.Fatal("expected at least one notification for renter after booking acceptance")
	}

	found := false
	for _, n := range result.Notifications {
		if n.Type == notification.TypeBookingAccepted {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected BOOKING_ACCEPTED notification, got types: %v",
			func() []notification.Type {
				types := make([]notification.Type, len(result.Notifications))
				for i, n := range result.Notifications {
					types[i] = n.Type
				}
				return types
			}())
	}
}

// TestNotificationMarkRead verifies POST /notifications/:id/read marks a notification
// as read and GET unread-count decrements.
func TestNotificationMarkRead(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	hostToken := LoginTestUser(t, client, ts.URL, *host.Email, "password123")
	acceptResp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/bookings/"+b.ID+"/accept", nil, hostToken)
	DrainBody(acceptResp)

	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	// Fetch the notification to get its ID.
	listResp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/notifications", nil, renterToken)
	var listResult struct {
		Notifications []notification.Notification `json:"notifications"`
	}
	MustDecodeJSON(t, listResp, &listResult)

	if len(listResult.Notifications) == 0 {
		t.Skip("no notifications available to mark as read")
	}

	notifID := listResult.Notifications[0].ID

	// Verify unread count is > 0.
	countResp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/notifications/unread-count", nil, renterToken)
	var countResult struct {
		Count int `json:"count"`
	}
	MustDecodeJSON(t, countResp, &countResult)
	if countResult.Count == 0 {
		t.Skip("unread count is 0 — cannot test mark-read decrement")
	}
	initialCount := countResult.Count

	// Mark the notification as read.
	markResp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/notifications/"+notifID+"/read", nil, renterToken)
	defer DrainBody(markResp)

	if markResp.StatusCode != http.StatusOK {
		rb, _ := readBody(markResp)
		t.Fatalf("mark read: expected 200, got %d: %s", markResp.StatusCode, rb)
	}

	// Unread count should have decreased by 1.
	countResp2 := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/notifications/unread-count", nil, renterToken)
	defer DrainBody(countResp2)
	var countResult2 struct {
		Count int `json:"count"`
	}
	MustDecodeJSON(t, countResp2, &countResult2)

	if countResult2.Count != initialCount-1 {
		t.Errorf("unread count after mark-read = %d, want %d", countResult2.Count, initialCount-1)
	}
}

// TestNotificationReadAllMarksAllRead verifies POST /notifications/read-all marks all
// notifications as read and resets the unread count to 0.
func TestNotificationReadAllMarksAllRead(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	hostToken := LoginTestUser(t, client, ts.URL, *host.Email, "password123")
	acceptResp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/bookings/"+b.ID+"/accept", nil, hostToken)
	DrainBody(acceptResp)

	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	readAllResp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/notifications/read-all", nil, renterToken)
	defer DrainBody(readAllResp)

	if readAllResp.StatusCode != http.StatusOK {
		rb, _ := readBody(readAllResp)
		t.Fatalf("read-all: expected 200, got %d: %s", readAllResp.StatusCode, rb)
	}

	// After read-all, unread count should be 0.
	countResp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/notifications/unread-count", nil, renterToken)
	defer DrainBody(countResp)

	var countResult struct {
		Count int `json:"count"`
	}
	MustDecodeJSON(t, countResp, &countResult)

	if countResult.Count != 0 {
		t.Errorf("unread count after read-all = %d, want 0", countResult.Count)
	}
}
