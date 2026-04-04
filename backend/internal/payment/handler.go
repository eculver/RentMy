package payment

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/giits/rentmy/backend/internal/platform/auth"
)

// Handler holds the HTTP handlers for the payment domain.
type Handler struct {
	svc *Service
}

// NewHandler creates a Handler with the given service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers payment and booking routes onto an existing chi.Router.
func (h *Handler) Mount(r chi.Router, authMW func(http.Handler) http.Handler) {
	// Public: hold estimate does not require authentication.
	r.Get("/listings/{id}/hold-estimate", h.holdEstimate)

	r.Group(func(r chi.Router) {
		r.Use(authMW)

		// Bookings
		r.Post("/bookings", h.createBooking)
		r.Get("/bookings/{id}", h.getBooking)
		r.Get("/users/me/bookings", h.listBookings)

		// Payment setup and onboarding
		r.Post("/payments/onboard", h.onboardHost)
		r.Post("/payments/setup", h.setupPayment)
	})
}

// holdEstimate returns the tiered hold amount for a listing.
// GET /api/v1/listings/:id/hold-estimate — no auth required.
func (h *Handler) holdEstimate(w http.ResponseWriter, r *http.Request) {
	listingID := chi.URLParam(r, "id")

	result, err := h.svc.GetHoldEstimate(r.Context(), listingID)
	if err != nil {
		if errors.Is(err, ErrListingNotFound) {
			writeError(w, http.StatusNotFound, "listing not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not compute hold estimate")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// createBookingRequest is the JSON body for POST /api/v1/bookings.
type createBookingRequest struct {
	ListingID       string `json:"listingId"`
	PaymentMethodID string `json:"paymentMethodId"`
	ScheduledStart  string `json:"scheduledStart"` // RFC3339
	ScheduledEnd    string `json:"scheduledEnd"`   // RFC3339
}

// createBooking handles POST /api/v1/bookings.
func (h *Handler) createBooking(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ListingID == "" {
		writeError(w, http.StatusBadRequest, "listingId is required")
		return
	}
	if req.PaymentMethodID == "" {
		writeError(w, http.StatusBadRequest, "paymentMethodId is required")
		return
	}

	start, err := time.Parse(time.RFC3339, req.ScheduledStart)
	if err != nil {
		writeError(w, http.StatusBadRequest, "scheduledStart must be RFC3339")
		return
	}
	end, err := time.Parse(time.RFC3339, req.ScheduledEnd)
	if err != nil {
		writeError(w, http.StatusBadRequest, "scheduledEnd must be RFC3339")
		return
	}
	if !end.After(start) {
		writeError(w, http.StatusBadRequest, "scheduledEnd must be after scheduledStart")
		return
	}

	result, err := h.svc.CreateBooking(r.Context(), BookingInput{
		RenterID:        userID,
		ListingID:       req.ListingID,
		PaymentMethodID: req.PaymentMethodID,
		ScheduledStart:  start,
		ScheduledEnd:    end,
	})
	if err != nil {
		if errors.Is(err, ErrListingNotFound) {
			writeError(w, http.StatusNotFound, "listing not found")
			return
		}
		if errors.Is(err, ErrNoPaymentMethod) {
			writeError(w, http.StatusBadRequest, "no payment method on file — call POST /api/v1/payments/setup first")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not create booking")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// getBooking handles GET /api/v1/bookings/:id.
func (h *Handler) getBooking(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	txn, err := h.svc.GetTransaction(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrTransactionNotFound) {
			writeError(w, http.StatusNotFound, "booking not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not retrieve booking")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"booking": txn})
}

// listBookings handles GET /api/v1/users/me/bookings.
func (h *Handler) listBookings(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 20
	}

	txns, total, err := h.svc.GetRenterBookings(r.Context(), userID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list bookings")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"bookings": txns,
		"total":    total,
	})
}

// onboardHost handles POST /api/v1/payments/onboard.
func (h *Handler) onboardHost(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	result, err := h.svc.OnboardHost(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create connected account")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// setupPayment handles POST /api/v1/payments/setup.
func (h *Handler) setupPayment(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	result, err := h.svc.SetupRenterPayment(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not set up payment")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// writeJSON encodes v as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
