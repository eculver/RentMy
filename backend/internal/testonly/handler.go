// Package testonly provides HTTP handlers for end-to-end test support.
// These endpoints are ONLY mounted when E2E_MODE=true (never in production)
// and bypass normal payment processing to let Maestro flows seed test data.
package testonly

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/giits/rentmy/backend/internal/platform/ulid"
)

// Handler provides test-only HTTP endpoints.
type Handler struct {
	pool *pgxpool.Pool
}

// New creates a new Handler backed by the given pool.
func New(pool *pgxpool.Pool) *Handler {
	return &Handler{pool: pool}
}

// Mount registers all test-only routes onto r under /test.
// Only call this when E2E_MODE=true.
func (h *Handler) Mount(r chi.Router) {
	r.Post("/test/booking", h.createTestBooking)
}

// createTestBookingRequest is the request body for POST /api/v1/test/booking.
type createTestBookingRequest struct {
	// RenterEmail is the email of the renter account. Defaults to bob@test.com.
	RenterEmail string `json:"renterEmail"`
}

// createTestBookingResponse is the response body for POST /api/v1/test/booking.
type createTestBookingResponse struct {
	TransactionID string `json:"transactionId"`
}

// createTestBooking creates a booking in REQUESTED state, bypassing payment
// processing. Used by Maestro seed flows to set up test fixtures.
func (h *Handler) createTestBooking(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req createTestBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.RenterEmail == "" {
		req.RenterEmail = "bob@test.com"
	}

	renterID, err := h.getUserIDByEmail(ctx, req.RenterEmail)
	if err != nil {
		http.Error(w, "renter not found: "+err.Error(), http.StatusBadRequest)
		return
	}

	listingID, hostID, err := h.getTestListing(ctx)
	if err != nil {
		http.Error(w, "no active test listing: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Schedule the booking for tomorrow noon → +4 hours.
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), now.Day()+1, 12, 0, 0, 0, time.UTC)
	end := start.Add(4 * time.Hour)

	txID := ulid.New()

	const q = `
		INSERT INTO transactions (
			id, renter_id, host_id, listing_id,
			rental_fee, hold_amount, item_value, guarantee_gap,
			platform_fee, host_payout, guarantee_contribution,
			escrow_status, hold_status, hold_allocation,
			stripe_payment_intent_id, stripe_charge_id,
			scheduled_start, scheduled_end, status
		) VALUES (
			$1, $2, $3, $4,
			10.00, 50.00, 100.00, 0.00,
			2.00, 8.00, 0.20,
			'HELD', 'HELD', '{}',
			'pi_e2e_test', '',
			$5, $6, 'REQUESTED'
		)`

	if _, err := h.pool.Exec(ctx, q,
		txID, renterID, hostID, listingID, start, end,
	); err != nil {
		http.Error(w, "insert transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(createTestBookingResponse{TransactionID: txID})
}

func (h *Handler) getUserIDByEmail(ctx context.Context, email string) (string, error) {
	var id string
	err := h.pool.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, email).Scan(&id)
	return id, err
}

func (h *Handler) getTestListing(ctx context.Context) (listingID, hostID string, err error) {
	err = h.pool.QueryRow(ctx, `
		SELECT l.id, l.host_id
		FROM listings l
		JOIN users u ON u.id = l.host_id
		WHERE u.email = 'alice@test.com'
		  AND l.status = 'ACTIVE'
		LIMIT 1
	`).Scan(&listingID, &hostID)
	return listingID, hostID, err
}
