package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Brett2thered/RentMy/backend/internal/rating"
)

func TestSubmitRating_RenterRatesHost(t *testing.T) {
	pool := NewTestDB(t)
	CleanupDB(t, pool)
	ts, client := NewTestServer(t)

	renter := CreateTestUser(t, pool)
	host := CreateTestUser(t, pool)
	listing := CreateTestListing(t, pool, host.ID)
	bk := CreateTestBooking(t, pool, renter.ID, listing.ID)

	// Force the transaction to COMPLETED so ratings are accepted.
	_, err := pool.Exec(context.Background(),
		`UPDATE transactions SET status = 'COMPLETED' WHERE id = $1`, bk.ID)
	require.NoError(t, err)

	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	// Renter submits a rating for the host.
	resp := DoJSON(t, client, http.MethodPost,
		ts.URL+"/api/v1/transactions/"+bk.ID+"/ratings",
		map[string]any{
			"bubbles": []string{"GOOD_COMMUNICATION", "ON_TIME", "ITEM_AS_DESCRIBED"},
		},
		renterToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var rt rating.Rating
	MustDecodeJSON(t, resp, &rt)
	assert.Equal(t, bk.ID, rt.TransactionID)
	assert.Equal(t, renter.ID, rt.FromUserID)
	assert.Equal(t, host.ID, rt.ToUserID)
	assert.Len(t, rt.Bubbles, 3)
}

func TestSubmitRating_HostRatesRenter(t *testing.T) {
	pool := NewTestDB(t)
	CleanupDB(t, pool)
	ts, client := NewTestServer(t)

	renter := CreateTestUser(t, pool)
	host := CreateTestUser(t, pool)
	listing := CreateTestListing(t, pool, host.ID)
	bk := CreateTestBooking(t, pool, renter.ID, listing.ID)

	_, err := pool.Exec(context.Background(),
		`UPDATE transactions SET status = 'COMPLETED' WHERE id = $1`, bk.ID)
	require.NoError(t, err)

	hostToken := LoginTestUser(t, client, ts.URL, *host.Email, "password123")

	resp := DoJSON(t, client, http.MethodPost,
		ts.URL+"/api/v1/transactions/"+bk.ID+"/ratings",
		map[string]any{
			"bubbles": []string{"ON_TIME_RETURN", "CAREFUL_WITH_ITEM", "RESPECTFUL"},
		},
		hostToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var rt rating.Rating
	MustDecodeJSON(t, resp, &rt)
	assert.Equal(t, renter.ID, rt.ToUserID)
}

func TestSubmitRating_DuplicateReturns409(t *testing.T) {
	pool := NewTestDB(t)
	CleanupDB(t, pool)
	ts, client := NewTestServer(t)

	renter := CreateTestUser(t, pool)
	host := CreateTestUser(t, pool)
	listing := CreateTestListing(t, pool, host.ID)
	bk := CreateTestBooking(t, pool, renter.ID, listing.ID)

	_, err := pool.Exec(context.Background(),
		`UPDATE transactions SET status = 'COMPLETED' WHERE id = $1`, bk.ID)
	require.NoError(t, err)

	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")
	body := map[string]any{"bubbles": []string{"GOOD_COMMUNICATION"}}

	resp := DoJSON(t, client, http.MethodPost,
		ts.URL+"/api/v1/transactions/"+bk.ID+"/ratings", body, renterToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	DrainBody(resp)

	resp = DoJSON(t, client, http.MethodPost,
		ts.URL+"/api/v1/transactions/"+bk.ID+"/ratings", body, renterToken)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	DrainBody(resp)
}

func TestSubmitRating_TransactionNotCompleted(t *testing.T) {
	pool := NewTestDB(t)
	CleanupDB(t, pool)
	ts, client := NewTestServer(t)

	renter := CreateTestUser(t, pool)
	host := CreateTestUser(t, pool)
	listing := CreateTestListing(t, pool, host.ID)
	bk := CreateTestBooking(t, pool, renter.ID, listing.ID)
	// Leave the transaction in REQUESTED state (default from CreateTestBooking).

	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	resp := DoJSON(t, client, http.MethodPost,
		ts.URL+"/api/v1/transactions/"+bk.ID+"/ratings",
		map[string]any{"bubbles": []string{"GOOD_COMMUNICATION"}},
		renterToken)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	DrainBody(resp)
}

func TestSubmitRating_WrongBubbleForRole(t *testing.T) {
	pool := NewTestDB(t)
	CleanupDB(t, pool)
	ts, client := NewTestServer(t)

	renter := CreateTestUser(t, pool)
	host := CreateTestUser(t, pool)
	listing := CreateTestListing(t, pool, host.ID)
	bk := CreateTestBooking(t, pool, renter.ID, listing.ID)

	_, err := pool.Exec(context.Background(),
		`UPDATE transactions SET status = 'COMPLETED' WHERE id = $1`, bk.ID)
	require.NoError(t, err)

	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	// ON_TIME_RETURN is a host-rates-renter bubble; renter should not use it.
	resp := DoJSON(t, client, http.MethodPost,
		ts.URL+"/api/v1/transactions/"+bk.ID+"/ratings",
		map[string]any{"bubbles": []string{"ON_TIME_RETURN"}},
		renterToken)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	DrainBody(resp)
}

func TestGetTransactionRatings(t *testing.T) {
	pool := NewTestDB(t)
	CleanupDB(t, pool)
	ts, client := NewTestServer(t)

	renter := CreateTestUser(t, pool)
	host := CreateTestUser(t, pool)
	listing := CreateTestListing(t, pool, host.ID)
	bk := CreateTestBooking(t, pool, renter.ID, listing.ID)

	_, err := pool.Exec(context.Background(),
		`UPDATE transactions SET status = 'COMPLETED' WHERE id = $1`, bk.ID)
	require.NoError(t, err)

	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")
	hostToken := LoginTestUser(t, client, ts.URL, *host.Email, "password123")

	// Both parties submit ratings.
	r1 := DoJSON(t, client, http.MethodPost,
		ts.URL+"/api/v1/transactions/"+bk.ID+"/ratings",
		map[string]any{"bubbles": []string{"GOOD_COMMUNICATION"}}, renterToken)
	require.Equal(t, http.StatusCreated, r1.StatusCode)
	DrainBody(r1)

	r2 := DoJSON(t, client, http.MethodPost,
		ts.URL+"/api/v1/transactions/"+bk.ID+"/ratings",
		map[string]any{"bubbles": []string{"ON_TIME_RETURN"}}, hostToken)
	require.Equal(t, http.StatusCreated, r2.StatusCode)
	DrainBody(r2)

	// Fetch all ratings for the transaction.
	resp := DoJSON(t, client, http.MethodGet,
		ts.URL+"/api/v1/transactions/"+bk.ID+"/ratings", nil, renterToken)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var ratings []rating.Rating
	MustDecodeJSON(t, resp, &ratings)
	assert.Len(t, ratings, 2)
}

func TestGetUserRatings(t *testing.T) {
	pool := NewTestDB(t)
	CleanupDB(t, pool)
	ts, client := NewTestServer(t)

	renter := CreateTestUser(t, pool)
	host := CreateTestUser(t, pool)
	listing := CreateTestListing(t, pool, host.ID)
	bk := CreateTestBooking(t, pool, renter.ID, listing.ID)

	_, err := pool.Exec(context.Background(),
		`UPDATE transactions SET status = 'COMPLETED' WHERE id = $1`, bk.ID)
	require.NoError(t, err)

	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	// Renter rates host.
	r := DoJSON(t, client, http.MethodPost,
		ts.URL+"/api/v1/transactions/"+bk.ID+"/ratings",
		map[string]any{"bubbles": []string{"GOOD_COMMUNICATION", "FRIENDLY"}}, renterToken)
	require.Equal(t, http.StatusCreated, r.StatusCode)
	DrainBody(r)

	// Fetch ratings received by the host.
	resp := DoJSON(t, client, http.MethodGet,
		ts.URL+"/api/v1/users/"+host.ID+"/ratings", nil, renterToken)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]any
	MustDecodeJSON(t, resp, &body)
	assert.EqualValues(t, 1, body["total"])
}

func TestGetUserRatingsSummary(t *testing.T) {
	pool := NewTestDB(t)
	CleanupDB(t, pool)
	ts, client := NewTestServer(t)

	renter := CreateTestUser(t, pool)
	host := CreateTestUser(t, pool)
	listing := CreateTestListing(t, pool, host.ID)
	bk := CreateTestBooking(t, pool, renter.ID, listing.ID)

	_, err := pool.Exec(context.Background(),
		`UPDATE transactions SET status = 'COMPLETED' WHERE id = $1`, bk.ID)
	require.NoError(t, err)

	renterToken := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	r := DoJSON(t, client, http.MethodPost,
		ts.URL+"/api/v1/transactions/"+bk.ID+"/ratings",
		map[string]any{"bubbles": []string{"GOOD_COMMUNICATION", "FRIENDLY"}}, renterToken)
	require.Equal(t, http.StatusCreated, r.StatusCode)
	DrainBody(r)

	resp := DoJSON(t, client, http.MethodGet,
		ts.URL+"/api/v1/users/"+host.ID+"/ratings/summary", nil, renterToken)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]any
	MustDecodeJSON(t, resp, &body)
	summary, ok := body["summary"].([]any)
	require.True(t, ok)
	assert.NotEmpty(t, summary)
}
