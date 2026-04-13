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
	// Status is the desired initial status of the booking.
	// Accepted values: "REQUESTED" (default), "ACCEPTED", "ACTIVE", "COMPLETED".
	// For "ACCEPTED": the host's CHECK_IN proximity proof is pre-inserted with
	// PIN="1234" and verified=true so the renter can immediately enter the PIN.
	// For "ACTIVE": all CHECK_IN proximity proofs are pre-verified; the host's
	// CHECK_OUT proof is also pre-verified so only the renter needs to complete
	// GPS verify during check-out.
	// For "COMPLETED": all four proximity proofs (CHECK_IN + CHECK_OUT for both
	// parties) are pre-inserted as verified.
	Status string `json:"status"`
}

// E2ECheckInPIN is the hardcoded PIN used for ACCEPTED-state test bookings.
// The renter inputs this PIN in the Maestro check-in flow.
const E2ECheckInPIN = "1234"

// createTestBookingResponse is the response body for POST /api/v1/test/booking.
type createTestBookingResponse struct {
	TransactionID string  `json:"transactionId"`
	// PIN is only populated when Status is "ACCEPTED".
	PIN           string  `json:"pin,omitempty"`
	ListingLat    float64 `json:"listingLat"`
	ListingLng    float64 `json:"listingLng"`
}

// createTestBooking creates a booking in the requested state, bypassing payment
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
	if req.Status == "" {
		req.Status = "REQUESTED"
	}

	switch req.Status {
	case "REQUESTED", "ACCEPTED", "ACTIVE", "COMPLETED":
	default:
		http.Error(w, "status must be REQUESTED, ACCEPTED, ACTIVE, or COMPLETED", http.StatusBadRequest)
		return
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

	listingLat, listingLng, err := h.getListingLocation(ctx, listingID)
	if err != nil {
		// Non-fatal: listing location may not be set in all test environments.
		listingLat = 0
		listingLng = 0
	}

	// Schedule the booking for tomorrow noon → +4 hours.
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), now.Day()+1, 12, 0, 0, 0, time.UTC)
	end := start.Add(4 * time.Hour)

	txID := ulid.New()

	switch req.Status {
	case "REQUESTED":
		if err := h.insertTransaction(ctx, txID, renterID, hostID, listingID, start, end, "REQUESTED", nil, nil); err != nil {
			http.Error(w, "insert transaction: "+err.Error(), http.StatusInternalServerError)
			return
		}

	case "ACCEPTED":
		if err := h.insertTransaction(ctx, txID, renterID, hostID, listingID, start, end, "ACCEPTED", nil, nil); err != nil {
			http.Error(w, "insert transaction: "+err.Error(), http.StatusInternalServerError)
			return
		}
		// Pre-insert host's CHECK_IN proof with a known PIN and verified=true
		// so the renter can enter the PIN during the E2E check-in flow.
		if err := h.insertProximityProof(ctx, txID, hostID, "CHECK_IN", E2ECheckInPIN, true); err != nil {
			http.Error(w, "insert host check-in proof: "+err.Error(), http.StatusInternalServerError)
			return
		}

	case "ACTIVE":
		actualStart := now
		if err := h.insertTransaction(ctx, txID, renterID, hostID, listingID, start, end, "ACTIVE", &actualStart, nil); err != nil {
			http.Error(w, "insert transaction: "+err.Error(), http.StatusInternalServerError)
			return
		}
		// Pre-insert verified CHECK_IN proofs for both parties (handoff already done).
		if err := h.insertProximityProof(ctx, txID, hostID, "CHECK_IN", "", true); err != nil {
			http.Error(w, "insert host check-in proof: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := h.insertProximityProof(ctx, txID, renterID, "CHECK_IN", "", true); err != nil {
			http.Error(w, "insert renter check-in proof: "+err.Error(), http.StatusInternalServerError)
			return
		}
		// Pre-insert verified host CHECK_OUT proof so only the renter needs to
		// complete GPS verify during the E2E check-out flow.
		if err := h.insertProximityProof(ctx, txID, hostID, "CHECK_OUT", "", true); err != nil {
			http.Error(w, "insert host check-out proof: "+err.Error(), http.StatusInternalServerError)
			return
		}

	case "COMPLETED":
		actualStart := now.Add(-4 * time.Hour)
		actualEnd := now.Add(-1 * time.Hour)
		if err := h.insertTransaction(ctx, txID, renterID, hostID, listingID, start, end, "COMPLETED", &actualStart, &actualEnd); err != nil {
			http.Error(w, "insert transaction: "+err.Error(), http.StatusInternalServerError)
			return
		}
		// Pre-insert all four verified proximity proofs (check-in and check-out
		// for both host and renter).
		for _, userID := range []string{hostID, renterID} {
			for _, proofType := range []string{"CHECK_IN", "CHECK_OUT"} {
				if err := h.insertProximityProof(ctx, txID, userID, proofType, "", true); err != nil {
					http.Error(w, "insert proximity proof: "+err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}
	}

	resp := createTestBookingResponse{
		TransactionID: txID,
		ListingLat:    listingLat,
		ListingLng:    listingLng,
	}
	if req.Status == "ACCEPTED" {
		resp.PIN = E2ECheckInPIN
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// insertTransaction inserts a transaction row with the given parameters.
func (h *Handler) insertTransaction(
	ctx context.Context,
	txID, renterID, hostID, listingID string,
	start, end time.Time,
	status string,
	actualStart, actualEnd *time.Time,
) error {
	const q = `
		INSERT INTO transactions (
			id, renter_id, host_id, listing_id,
			rental_fee, hold_amount, item_value, guarantee_gap,
			platform_fee, host_payout, guarantee_contribution,
			escrow_status, hold_status, hold_allocation,
			stripe_payment_intent_id, stripe_charge_id,
			scheduled_start, scheduled_end, status,
			actual_start, actual_end
		) VALUES (
			$1, $2, $3, $4,
			10.00, 50.00, 100.00, 0.00,
			2.00, 8.00, 0.20,
			'HELD', 'HELD', '{}',
			'pi_e2e_test', '',
			$5, $6, $7,
			$8, $9
		)`
	_, err := h.pool.Exec(ctx, q,
		txID, renterID, hostID, listingID, start, end, status,
		actualStart, actualEnd,
	)
	return err
}

// insertProximityProof inserts a proximity_proofs row for the given user.
// When verified=true the record is pre-marked as GPS-verified (dist=0).
// For ACCEPTED-state bookings, pin is the E2E test PIN ("1234"); otherwise empty.
func (h *Handler) insertProximityProof(
	ctx context.Context,
	transactionID, userID, proofType, pin string,
	verified bool,
) error {
	proofID := ulid.New()
	expiresAt := time.Now().Add(30 * time.Minute)
	const q = `
		INSERT INTO proximity_proofs
		    (id, transaction_id, user_id, proof_type,
		     gps_distance, pin, pin_expires_at,
		     verified, method, device_id, created_at)
		VALUES ($1, $2, $3, $4,
		        $5, $6, $7,
		        $8, 'GPS', '', $9)`
	_, err := h.pool.Exec(ctx, q,
		proofID, transactionID, userID, proofType,
		0.0, pin, expiresAt,
		verified, time.Now(),
	)
	return err
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

// getListingLocation returns the lat/lng of a listing, extracted from PostGIS.
func (h *Handler) getListingLocation(ctx context.Context, listingID string) (lat, lng float64, err error) {
	err = h.pool.QueryRow(ctx, `
		SELECT ST_Y(location::geometry), ST_X(location::geometry)
		FROM listings
		WHERE id = $1 AND location IS NOT NULL
	`, listingID).Scan(&lat, &lng)
	return lat, lng, err
}
