package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/Brett2thered/RentMy/backend/internal/dispute"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileDispute(t *testing.T) {
	pool := NewTestDB(t)
	CleanupDB(t, pool)
	ts, client := NewTestServer(t)

	renter := CreateTestUser(t, pool)
	host := CreateTestUser(t, pool)
	listing := CreateTestListing(t, pool, host.ID)
	booking := CreateTestBooking(t, pool, renter.ID, listing.ID)

	// Transition booking to ACTIVE state so it can be disputed.
	_, err := pool.Exec(context.Background(),
		`UPDATE transactions SET status = 'ACTIVE' WHERE id = $1`, booking.ID)
	require.NoError(t, err)

	token := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	// File a dispute.
	resp := DoJSON(t, client, http.MethodPost,
		ts.URL+"/api/v1/transactions/"+booking.ID+"/disputes",
		map[string]string{
			"reason":      "Item returned damaged",
			"description": "Screen has a crack on the left side",
		}, token)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var d dispute.Dispute
	MustDecodeJSON(t, resp, &d)
	assert.Equal(t, booking.ID, d.TransactionID)
	assert.Equal(t, renter.ID, d.ReporterID)
	assert.Equal(t, "Item returned damaged", d.Reason)
	assert.Equal(t, dispute.StatusPending, d.Status)
	assert.NotEmpty(t, d.ID)

	// Verify transaction status was updated to DISPUTED.
	var txnStatus string
	err = pool.QueryRow(context.Background(),
		`SELECT status FROM transactions WHERE id = $1`, booking.ID).Scan(&txnStatus)
	require.NoError(t, err)
	assert.Equal(t, "DISPUTED", txnStatus)
}

func TestFileDispute_DuplicateReject(t *testing.T) {
	pool := NewTestDB(t)
	CleanupDB(t, pool)
	ts, client := NewTestServer(t)

	renter := CreateTestUser(t, pool)
	host := CreateTestUser(t, pool)
	listing := CreateTestListing(t, pool, host.ID)
	booking := CreateTestBooking(t, pool, renter.ID, listing.ID)

	_, err := pool.Exec(context.Background(),
		`UPDATE transactions SET status = 'ACTIVE' WHERE id = $1`, booking.ID)
	require.NoError(t, err)

	token := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	body := map[string]string{
		"reason":      "Damaged",
		"description": "Broken",
	}

	// First dispute should succeed.
	resp := DoJSON(t, client, http.MethodPost,
		ts.URL+"/api/v1/transactions/"+booking.ID+"/disputes", body, token)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	DrainBody(resp)

	// Second dispute should fail with 409 Conflict.
	resp = DoJSON(t, client, http.MethodPost,
		ts.URL+"/api/v1/transactions/"+booking.ID+"/disputes", body, token)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	DrainBody(resp)
}

func TestGetDisputesByTransaction(t *testing.T) {
	pool := NewTestDB(t)
	CleanupDB(t, pool)
	ts, client := NewTestServer(t)

	renter := CreateTestUser(t, pool)
	host := CreateTestUser(t, pool)
	listing := CreateTestListing(t, pool, host.ID)
	booking := CreateTestBooking(t, pool, renter.ID, listing.ID)

	_, err := pool.Exec(context.Background(),
		`UPDATE transactions SET status = 'ACTIVE' WHERE id = $1`, booking.ID)
	require.NoError(t, err)

	token := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	// File a dispute.
	resp := DoJSON(t, client, http.MethodPost,
		ts.URL+"/api/v1/transactions/"+booking.ID+"/disputes",
		map[string]string{"reason": "Damaged", "description": "Test"}, token)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	DrainBody(resp)

	// Get disputes for transaction.
	resp = DoJSON(t, client, http.MethodGet,
		ts.URL+"/api/v1/transactions/"+booking.ID+"/disputes", nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var disputes []dispute.Dispute
	MustDecodeJSON(t, resp, &disputes)
	assert.Len(t, disputes, 1)
	assert.Equal(t, booking.ID, disputes[0].TransactionID)
}

func TestGetDisputeByID(t *testing.T) {
	pool := NewTestDB(t)
	CleanupDB(t, pool)
	ts, client := NewTestServer(t)

	renter := CreateTestUser(t, pool)
	host := CreateTestUser(t, pool)
	listing := CreateTestListing(t, pool, host.ID)
	booking := CreateTestBooking(t, pool, renter.ID, listing.ID)

	_, err := pool.Exec(context.Background(),
		`UPDATE transactions SET status = 'ACTIVE' WHERE id = $1`, booking.ID)
	require.NoError(t, err)

	token := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	// File a dispute.
	resp := DoJSON(t, client, http.MethodPost,
		ts.URL+"/api/v1/transactions/"+booking.ID+"/disputes",
		map[string]string{"reason": "Missing", "description": "Item not returned"}, token)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var created dispute.Dispute
	MustDecodeJSON(t, resp, &created)

	// Get by ID.
	resp = DoJSON(t, client, http.MethodGet,
		ts.URL+"/api/v1/disputes/"+created.ID, nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var fetched dispute.Dispute
	MustDecodeJSON(t, resp, &fetched)
	assert.Equal(t, created.ID, fetched.ID)
	assert.Equal(t, "Missing", fetched.Reason)
}

func TestGetDisputeByID_NotFound(t *testing.T) {
	pool := NewTestDB(t)
	CleanupDB(t, pool)
	ts, client := NewTestServer(t)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	resp := DoJSON(t, client, http.MethodGet,
		ts.URL+"/api/v1/disputes/nonexistent", nil, token)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	DrainBody(resp)
}

func TestFileDispute_MissingReason(t *testing.T) {
	pool := NewTestDB(t)
	CleanupDB(t, pool)
	ts, client := NewTestServer(t)

	renter := CreateTestUser(t, pool)
	host := CreateTestUser(t, pool)
	listing := CreateTestListing(t, pool, host.ID)
	booking := CreateTestBooking(t, pool, renter.ID, listing.ID)

	_, err := pool.Exec(context.Background(),
		`UPDATE transactions SET status = 'ACTIVE' WHERE id = $1`, booking.ID)
	require.NoError(t, err)

	token := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	resp := DoJSON(t, client, http.MethodPost,
		ts.URL+"/api/v1/transactions/"+booking.ID+"/disputes",
		map[string]string{"description": "No reason given"}, token)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	DrainBody(resp)
}

func TestReviewQueue(t *testing.T) {
	pool := NewTestDB(t)
	CleanupDB(t, pool)
	ts, client := NewTestServer(t)

	admin := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *admin.Email, "password123")

	// Review queue should be empty initially.
	resp := DoJSON(t, client, http.MethodGet,
		ts.URL+"/api/v1/admin/disputes/review-queue", nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var disputes []dispute.Dispute
	MustDecodeJSON(t, resp, &disputes)
	assert.Empty(t, disputes)
}
