package verification

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/giits/rentmy/backend/internal/platform/auth"
)

// Handler holds HTTP handlers for the verification domain.
type Handler struct {
	svc *Service
}

// NewHandler creates a Handler with the given service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers verification routes onto an existing chi.Router.
// Authenticated endpoints require authMW; the Stripe webhook endpoint is public
// (signature validation is performed inside the service).
func (h *Handler) Mount(r chi.Router, authMW func(http.Handler) http.Handler) {
	// Public: Stripe calls this endpoint — no JWT auth required.
	r.Post("/webhooks/stripe-identity", h.webhook)

	r.Group(func(r chi.Router) {
		r.Use(authMW)
		r.Post("/verification/start", h.start)
		r.Get("/verification/status", h.status)
	})
}

// start handles POST /api/v1/verification/start.
// Initiates a new Stripe Identity KYC session for the authenticated user.
func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	result, err := h.svc.StartVerification(r.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrAlreadyVerified) {
			writeError(w, http.StatusConflict, "user is already verified")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not start verification")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// status handles GET /api/v1/verification/status.
// Returns the current KYC status for the authenticated user.
func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	result, err := h.svc.GetStatus(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not get verification status")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// webhook handles POST /api/v1/webhooks/stripe-identity.
// Signature validation is performed inside the service using the Stripe-Signature header.
func (h *Handler) webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MiB limit
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read request body")
		return
	}
	defer r.Body.Close()

	sig := r.Header.Get("Stripe-Signature")
	if sig == "" {
		writeError(w, http.StatusBadRequest, "missing Stripe-Signature header")
		return
	}

	if err := h.svc.HandleWebhook(r.Context(), body, sig); err != nil {
		writeError(w, http.StatusBadRequest, "webhook processing failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
