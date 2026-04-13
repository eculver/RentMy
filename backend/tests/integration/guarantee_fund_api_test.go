package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Brett2thered/RentMy/backend/internal/guaranteefund"
	"github.com/Brett2thered/RentMy/backend/internal/platform/ulid" // used by insertGuaranteeFundEntry
)

// insertGuaranteeFundEntry directly inserts a ledger entry for testing.
func insertGuaranteeFundEntry(t *testing.T, pool *pgxpool.Pool, transactionID string, entryType string, amountDollars float64) {
	t.Helper()
	ctx := context.Background()

	id := ulid.New()
	// Compute running balance from the last entry.
	var prevBalance float64
	err := pool.QueryRow(ctx,
		`SELECT COALESCE(balance_after, 0) FROM guarantee_fund_entries ORDER BY created_at DESC LIMIT 1`,
	).Scan(&prevBalance)
	if err != nil {
		prevBalance = 0
	}
	balanceAfter := prevBalance + amountDollars

	_, err = pool.Exec(ctx,
		`INSERT INTO guarantee_fund_entries (id, transaction_id, entry_type, amount, balance_after)
		 VALUES ($1, $2, $3, $4, $5)`,
		id, transactionID, entryType, amountDollars, balanceAfter,
	)
	require.NoError(t, err, "insert guarantee fund entry")
}

func TestGuaranteeFundAdminEndpoints(t *testing.T) {
	pool := NewTestDB(t)
	CleanupDB(t, pool)
	ts, client := NewTestServer(t)

	// Create a user and log in.
	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	// --- GET /admin/guarantee-fund/health ---
	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/admin/guarantee-fund/health", nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var health guaranteefund.FundHealth
	MustDecodeJSON(t, resp, &health)
	assert.Equal(t, int64(0), health.Balance)
	assert.Equal(t, int64(0), health.OutstandingGaps)
	assert.Equal(t, ReserveActionNormal, string(health.Action))

	// --- GET /admin/guarantee-fund/entries (empty) ---
	resp = DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/admin/guarantee-fund/entries", nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var entriesResp struct {
		Entries []guaranteefund.Entry `json:"entries"`
		Total   int                   `json:"total"`
	}
	MustDecodeJSON(t, resp, &entriesResp)
	assert.Equal(t, 0, entriesResp.Total)
}

// ReserveActionNormal is a test constant to avoid importing guaranteefund just for string comparison.
const ReserveActionNormal = "NORMAL"

func TestGuaranteeFundLedgerIntegrity(t *testing.T) {
	pool := NewTestDB(t)
	CleanupDB(t, pool)
	ts, client := NewTestServer(t)

	renter := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *renter.Email, "password123")

	host := CreateTestUser(t, pool)
	listing := CreateTestListing(t, pool, host.ID)

	// Create real bookings to get valid transaction IDs.
	booking1 := CreateTestBooking(t, pool, renter.ID, listing.ID)
	booking2 := CreateTestBooking(t, pool, renter.ID, listing.ID)

	// Contribution of $50.00
	insertGuaranteeFundEntry(t, pool, booking1.ID, "CONTRIBUTION", 50.0)
	// Contribution of $30.00
	insertGuaranteeFundEntry(t, pool, booking2.ID, "CONTRIBUTION", 30.0)
	// Claim of -$20.00
	insertGuaranteeFundEntry(t, pool, booking1.ID, "CLAIM", -20.0)

	// Verify health endpoint reflects the ledger state.
	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/admin/guarantee-fund/health", nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var health guaranteefund.FundHealth
	MustDecodeJSON(t, resp, &health)
	// Balance should be (50 + 30 - 20) * 100 = 6000 cents.
	assert.Equal(t, int64(6_000), health.Balance)

	// Verify entries endpoint returns all 3 entries.
	resp = DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/admin/guarantee-fund/entries?limit=10", nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var entriesResp struct {
		Entries []json.RawMessage `json:"entries"`
		Total   int               `json:"total"`
	}
	MustDecodeJSON(t, resp, &entriesResp)
	assert.Equal(t, 3, entriesResp.Total)
	assert.Len(t, entriesResp.Entries, 3)
}
