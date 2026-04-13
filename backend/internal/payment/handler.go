package payment

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Brett2thered/RentMy/backend/internal/platform/auth"
)

// Handler holds the HTTP handlers for the payment domain.
type Handler struct {
	svc *Service
}

// NewHandler creates a Handler with the given service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers payment routes onto an existing chi.Router.
// Note: booking creation and retrieval routes are handled by the booking.Handler.
func (h *Handler) Mount(r chi.Router, authMW func(http.Handler) http.Handler) {
	// Public: hold estimate does not require authentication.
	r.Get("/listings/{id}/hold-estimate", h.holdEstimate)

	r.Group(func(r chi.Router) {
		r.Use(authMW)

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
