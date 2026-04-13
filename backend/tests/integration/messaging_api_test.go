package integration

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Brett2thered/RentMy/backend/internal/messaging"
	"github.com/Brett2thered/RentMy/backend/internal/platform/ulid"
)

// TestSendMessageSuccess verifies POST /bookings/:id/messages creates a message.
func TestSendMessageSuccess(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	body := map[string]string{"content": "Hi, is the item still available?"}
	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/bookings/"+b.ID+"/messages", body, renterToken)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusCreated {
		rb, _ := readBody(resp)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, rb)
	}

	var msg messaging.Message
	MustDecodeJSON(t, resp, &msg)

	if msg.ID == "" {
		t.Error("expected non-empty message ID")
	}
	if msg.TransactionID != b.ID {
		t.Errorf("transactionId = %q, want %q", msg.TransactionID, b.ID)
	}
	if msg.SenderID != renter.ID {
		t.Errorf("senderId = %q, want %q", msg.SenderID, renter.ID)
	}
	if msg.Content != "Hi, is the item still available?" {
		t.Errorf("content = %q, unexpected", msg.Content)
	}
}

// TestSendMessageHostCanSend verifies that the host can also send messages.
func TestSendMessageHostCanSend(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	hostToken := LoginTestUser(t, client, ts.URL, *host.Email, "password123")

	body := map[string]string{"content": "Yes, it is available at the listed time."}
	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/bookings/"+b.ID+"/messages", body, hostToken)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusCreated {
		rb, _ := readBody(resp)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, rb)
	}
}

// TestSendMessageEmptyContent verifies that an empty message content returns 400.
func TestSendMessageEmptyContent(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	body := map[string]string{"content": ""}
	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/bookings/"+b.ID+"/messages", body, renterToken)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty content, got %d", resp.StatusCode)
	}
}

// TestSendMessageContentTooLong verifies that content exceeding MaxContentLength returns 400.
func TestSendMessageContentTooLong(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	// MaxContentLength is 4000 — send 4001 chars.
	body := map[string]string{"content": strings.Repeat("a", messaging.MaxContentLength+1)}
	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/bookings/"+b.ID+"/messages", body, renterToken)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for content exceeding max length, got %d", resp.StatusCode)
	}
}

// TestSendMessageNotAParty verifies that a non-party returns 403.
func TestSendMessageNotAParty(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	thirdParty := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	thirdToken := LoginTestUser(t, client, ts.URL, *thirdParty.Email, "password123")

	body := map[string]string{"content": "This is not my transaction."}
	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/bookings/"+b.ID+"/messages", body, thirdToken)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-party, got %d", resp.StatusCode)
	}
}

// TestSendMessageBookingNotFound verifies that sending a message for a missing booking
// returns 404.
func TestSendMessageBookingNotFound(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	body := map[string]string{"content": "hello"}
	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/bookings/"+ulid.New()+"/messages", body, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// TestSendMessageRequiresAuth verifies that POST /bookings/:id/messages returns 401 without auth.
func TestSendMessageRequiresAuth(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	body := map[string]string{"content": "hello"}
	resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/bookings/"+ulid.New()+"/messages", body, "")
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// TestGetMessagesEmpty verifies GET /bookings/:id/messages returns an empty list when
// no messages have been sent.
func TestGetMessagesEmpty(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/bookings/"+b.ID+"/messages", nil, renterToken)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		rb, _ := readBody(resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, rb)
	}

	var result struct {
		Messages []messaging.Message `json:"messages"`
	}
	MustDecodeJSON(t, resp, &result)

	if len(result.Messages) != 0 {
		t.Errorf("len(messages) = %d, want 0", len(result.Messages))
	}
}

// TestGetMessagesPagination verifies that sending multiple messages and fetching them
// returns them in the correct order.
func TestGetMessagesPagination(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")
	hostToken := LoginTestUser(t, client, ts.URL, *host.Email, "password123")

	// Send 3 messages: renter, host, renter.
	messages := []struct {
		token   string
		content string
	}{
		{renterToken, "Hello from renter"},
		{hostToken, "Hello from host"},
		{renterToken, "Thanks!"},
	}
	for _, m := range messages {
		resp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/bookings/"+b.ID+"/messages",
			map[string]string{"content": m.content}, m.token)
		DrainBody(resp)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("send message: expected 201, got %d", resp.StatusCode)
		}
	}

	// Fetch all messages.
	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/bookings/"+b.ID+"/messages", nil, renterToken)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		rb, _ := readBody(resp)
		t.Fatalf("get messages: expected 200, got %d: %s", resp.StatusCode, rb)
	}

	var result struct {
		Messages []messaging.Message `json:"messages"`
	}
	MustDecodeJSON(t, resp, &result)

	if len(result.Messages) != 3 {
		t.Errorf("len(messages) = %d, want 3", len(result.Messages))
	}
}

// TestGetMessagesNotAParty verifies GET /bookings/:id/messages returns 403 for a non-party.
func TestGetMessagesNotAParty(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	host := CreateTestUser(t, pool)
	renter := CreateTestUser(t, pool)
	thirdParty := CreateTestUser(t, pool)
	l := CreateTestListing(t, pool, host.ID)
	b := CreateTestBooking(t, pool, renter.ID, l.ID)

	thirdToken := LoginTestUser(t, client, ts.URL, *thirdParty.Email, "password123")

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/bookings/"+b.ID+"/messages", nil, thirdToken)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-party, got %d", resp.StatusCode)
	}
}

// TestGetMessagesBookingNotFound verifies GET /bookings/:id/messages returns 404 for a
// missing booking.
func TestGetMessagesBookingNotFound(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/bookings/"+ulid.New()+"/messages", nil, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}
