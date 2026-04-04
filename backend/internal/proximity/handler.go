package proximity

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/giits/rentmy/backend/internal/platform/auth"
)

// Handler holds HTTP handlers for the proximity domain.
type Handler struct {
	svc *Service
}

// NewHandler creates a Handler wrapping the given Service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers proximity routes on the given router.
// Routes follow the convention established by other services.
func (h *Handler) Mount(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMW)

		// GPS verification for any party.
		r.Post("/proximity/verify", h.verifyGPS)
		// PIN entry for renter at check-in.
		r.Post("/proximity/pin", h.verifyPIN)
		// Host triggers SMS delivery of the PIN.
		r.Post("/proximity/sms-fallback", h.smsFallback)
		// Aggregated proximity status for a booking.
		r.Get("/bookings/{id}/proximity", h.getStatus)
	})
}

// verifyGPSRequest is the JSON body for POST /api/v1/proximity/verify.
type verifyGPSRequest struct {
	TransactionID string  `json:"transactionId"`
	Lat           float64 `json:"lat"`
	Lng           float64 `json:"lng"`
	ProofType     string  `json:"proofType"` // "CHECK_IN" | "CHECK_OUT"
}

func (h *Handler) verifyGPS(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req verifyGPSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.TransactionID == "" {
		writeError(w, http.StatusBadRequest, "transactionId is required")
		return
	}
	if req.ProofType != "CHECK_IN" && req.ProofType != "CHECK_OUT" {
		writeError(w, http.StatusBadRequest, "proofType must be CHECK_IN or CHECK_OUT")
		return
	}

	verified, err := h.svc.VerifyGPS(r.Context(), VerifyGPSInput{
		TransactionID: req.TransactionID,
		UserID:        userID,
		Lat:           req.Lat,
		Lng:           req.Lng,
		ProofType:     ProofType(req.ProofType),
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrTooFarAway):
			writeError(w, http.StatusUnprocessableEntity, "location is not within required proximity threshold")
		default:
			writeError(w, http.StatusInternalServerError, "could not verify GPS location")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"verified": verified})
}

// verifyPINRequest is the JSON body for POST /api/v1/proximity/pin.
type verifyPINRequest struct {
	TransactionID string `json:"transactionId"`
	PIN           string `json:"pin"`
}

func (h *Handler) verifyPIN(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req verifyPINRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.TransactionID == "" {
		writeError(w, http.StatusBadRequest, "transactionId is required")
		return
	}
	if len(req.PIN) != 4 {
		writeError(w, http.StatusBadRequest, "pin must be exactly 4 digits")
		return
	}

	if err := h.svc.VerifyPIN(r.Context(), VerifyPINInput{
		TransactionID: req.TransactionID,
		RenterID:      userID,
		PIN:           req.PIN,
	}); err != nil {
		switch {
		case errors.Is(err, ErrPINExpired):
			writeError(w, http.StatusUnprocessableEntity, "PIN has expired")
		case errors.Is(err, ErrPINInvalid):
			writeError(w, http.StatusUnprocessableEntity, "PIN is incorrect")
		default:
			writeError(w, http.StatusInternalServerError, "could not verify PIN")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"verified": true})
}

// smsFallbackRequest is the JSON body for POST /api/v1/proximity/sms-fallback.
type smsFallbackRequest struct {
	TransactionID string `json:"transactionId"`
	ToPhone       string `json:"toPhone"` // E.164, e.g. "+12135551234"
}

func (h *Handler) smsFallback(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req smsFallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.TransactionID == "" {
		writeError(w, http.StatusBadRequest, "transactionId is required")
		return
	}
	if req.ToPhone == "" {
		writeError(w, http.StatusBadRequest, "toPhone is required")
		return
	}

	if err := h.svc.SMSFallback(r.Context(), SMSFallbackInput{
		TransactionID: req.TransactionID,
		HostID:        userID,
		ToPhone:       req.ToPhone,
	}); err != nil {
		switch {
		case errors.Is(err, ErrNotAuthorized):
			writeError(w, http.StatusForbidden, "only the host may trigger SMS fallback")
		case errors.Is(err, ErrPINExpired):
			writeError(w, http.StatusUnprocessableEntity, "PIN has expired — re-accept booking to regenerate")
		default:
			writeError(w, http.StatusInternalServerError, "could not send SMS")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getStatus(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	transactionID := chi.URLParam(r, "id")

	status, err := h.svc.GetStatus(r.Context(), transactionID, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotAuthorized):
			writeError(w, http.StatusForbidden, "not authorized to view proximity status for this booking")
		default:
			writeError(w, http.StatusInternalServerError, "could not retrieve proximity status")
		}
		return
	}

	writeJSON(w, http.StatusOK, status)
}

// writeJSON encodes v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
