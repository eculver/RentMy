package booking

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/giits/rentmy/backend/internal/payment"
	"github.com/giits/rentmy/backend/internal/platform/auth"
)

// Handler holds the HTTP handlers for the booking domain.
type Handler struct {
	svc        *Service
	paymentSvc *payment.Service
}

// NewHandler creates a Handler with the given services.
func NewHandler(svc *Service, paymentSvc *payment.Service) *Handler {
	return &Handler{svc: svc, paymentSvc: paymentSvc}
}

// Mount registers all booking routes onto an existing chi.Router.
func (h *Handler) Mount(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMW)

		// Booking CRUD
		r.Post("/bookings", h.createBooking)
		r.Get("/bookings/{id}", h.getBooking)
		r.Get("/users/me/bookings", h.listRenterBookings)
		r.Get("/users/me/hosted-bookings", h.listHostBookings)

		// State transitions
		r.Post("/bookings/{id}/accept", h.acceptBooking)
		r.Post("/bookings/{id}/decline", h.declineBooking)
		r.Post("/bookings/{id}/cancel", h.cancelBooking)
		r.Post("/bookings/{id}/check-in", h.checkIn)
		r.Post("/bookings/{id}/check-out", h.checkOut)
	})
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

	result, err := h.svc.CreateBooking(r.Context(), CreateInput{
		RenterID:        userID,
		ListingID:       req.ListingID,
		PaymentMethodID: req.PaymentMethodID,
		ScheduledStart:  start,
		ScheduledEnd:    end,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrDurationExceedsLimit):
			writeError(w, http.StatusBadRequest, "booking duration exceeds 7-day maximum")
		case errors.Is(err, ErrListingNotAvailable):
			writeError(w, http.StatusConflict, "listing is not available for the requested period")
		case errors.Is(err, ErrFraudBlocked):
			writeError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, payment.ErrListingNotFound):
			writeError(w, http.StatusNotFound, "listing not found")
		case errors.Is(err, payment.ErrNoPaymentMethod):
			writeError(w, http.StatusBadRequest, "no payment method on file — call POST /api/v1/payments/setup first")
		default:
			writeError(w, http.StatusInternalServerError, "could not create booking")
		}
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// getBooking handles GET /api/v1/bookings/:id.
func (h *Handler) getBooking(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")

	booking, err := h.svc.GetBooking(r.Context(), id, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrBookingNotFound):
			writeError(w, http.StatusNotFound, "booking not found")
		case errors.Is(err, ErrNotAuthorized):
			writeError(w, http.StatusForbidden, "not authorized to view this booking")
		default:
			writeError(w, http.StatusInternalServerError, "could not retrieve booking")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"booking": booking})
}

// listRenterBookings handles GET /api/v1/users/me/bookings.
func (h *Handler) listRenterBookings(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	limit, offset := parsePagination(r)

	bookings, total, err := h.svc.ListByRenter(r.Context(), userID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list bookings")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"bookings": bookings,
		"total":    total,
	})
}

// listHostBookings handles GET /api/v1/users/me/hosted-bookings.
func (h *Handler) listHostBookings(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	limit, offset := parsePagination(r)

	bookings, total, err := h.svc.ListByHost(r.Context(), userID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list hosted bookings")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"bookings": bookings,
		"total":    total,
	})
}

// acceptBooking handles POST /api/v1/bookings/:id/accept.
func (h *Handler) acceptBooking(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")

	if err := h.svc.Accept(r.Context(), AcceptInput{BookingID: id, HostID: userID}); err != nil {
		writeTransitionError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// declineBooking handles POST /api/v1/bookings/:id/decline.
func (h *Handler) declineBooking(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")

	if err := h.svc.Decline(r.Context(), DeclineInput{BookingID: id, HostID: userID}); err != nil {
		writeTransitionError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// cancelBooking handles POST /api/v1/bookings/:id/cancel.
func (h *Handler) cancelBooking(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")

	if err := h.svc.Cancel(r.Context(), CancelInput{BookingID: id, CancellerID: userID}); err != nil {
		writeTransitionError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// checkIn handles POST /api/v1/bookings/:id/check-in.
// Transitions the booking ACCEPTED → ACTIVE once both parties have verified proximity.
func (h *Handler) checkIn(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")

	if err := h.svc.CheckIn(r.Context(), id, userID); err != nil {
		switch {
		case errors.Is(err, ErrHandoffIncomplete):
			writeError(w, http.StatusUnprocessableEntity, err.Error())
		default:
			writeTransitionError(w, err)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// checkOut handles POST /api/v1/bookings/:id/check-out.
// Transitions the booking ACTIVE → COMPLETED once both parties have verified proximity.
func (h *Handler) checkOut(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")

	if err := h.svc.CheckOut(r.Context(), id, userID); err != nil {
		switch {
		case errors.Is(err, ErrHandoffIncomplete):
			writeError(w, http.StatusUnprocessableEntity, err.Error())
		default:
			writeTransitionError(w, err)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// writeTransitionError writes an appropriate HTTP error response for booking state errors.
func writeTransitionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrBookingNotFound):
		writeError(w, http.StatusNotFound, "booking not found")
	case errors.Is(err, ErrNotAuthorized):
		writeError(w, http.StatusForbidden, "not authorized for this action")
	case errors.Is(err, ErrInvalidTransition):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "could not update booking")
	}
}

// parsePagination extracts limit and offset from query parameters.
func parsePagination(r *http.Request) (limit, offset int) {
	limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ = strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 20
	}
	return limit, offset
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
