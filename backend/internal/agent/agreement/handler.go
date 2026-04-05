package agreement

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/giits/rentmy/backend/internal/platform/auth"
)

// Handler serves the AgreementAgent HTTP API.
type Handler struct {
	svc *Service
}

// NewHandler creates a Handler backed by the given Service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers the agreement routes onto the provided chi.Router.
func (h *Handler) Mount(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMW)
		r.Get("/transactions/{transactionId}/agreement", h.handleGetAgreement)
		r.Post("/transactions/{transactionId}/agreement/accept", h.handleAccept)
		r.Get("/transactions/{transactionId}/agreement/status", h.handleGetStatus)
	})
}

// handleGetAgreement returns the full agreement for a transaction.
// GET /api/v1/transactions/:id/agreement
func (h *Handler) handleGetAgreement(w http.ResponseWriter, r *http.Request) {
	txID := chi.URLParam(r, "transactionId")
	if txID == "" {
		http.Error(w, "transactionId is required", http.StatusBadRequest)
		return
	}

	a, err := h.svc.GetAgreement(r.Context(), txID)
	if err != nil {
		if errors.Is(err, ErrAgreementNotFound) {
			http.Error(w, "agreement not found", http.StatusNotFound)
			return
		}
		slog.Warn("agreement: get failed", "transactionId", txID, "error", err)
		http.Error(w, "failed to fetch agreement", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, AgreementResponse{
		ID:            a.ID,
		TransactionID: a.TransactionID,
		Version:       a.Version,
		FullAgreement: a.FullAgreement,
		CustomClauses: a.CustomClauses,
		CreatedAt:     a.CreatedAt,
	})
}

// handleAccept records the authenticated user's acceptance of the agreement.
// POST /api/v1/transactions/:id/agreement/accept
func (h *Handler) handleAccept(w http.ResponseWriter, r *http.Request) {
	txID := chi.URLParam(r, "transactionId")
	if txID == "" {
		http.Error(w, "transactionId is required", http.StatusBadRequest)
		return
	}

	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req AcceptRequest
	_ = json.NewDecoder(r.Body).Decode(&req) // deviceId is optional

	ipAddress := r.RemoteAddr
	// X-Forwarded-For takes precedence behind a proxy.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ipAddress = xff
	}

	if err := h.svc.ValidateAcceptance(r.Context(), txID, userID, ipAddress, req.DeviceID); err != nil {
		switch {
		case errors.Is(err, ErrAgreementNotFound):
			http.Error(w, "agreement not found", http.StatusNotFound)
		case errors.Is(err, ErrAlreadyAccepted):
			http.Error(w, "agreement already accepted", http.StatusConflict)
		case errors.Is(err, ErrNotParty):
			http.Error(w, "not a party to this agreement", http.StatusForbidden)
		default:
			slog.Warn("agreement: accept failed", "transactionId", txID, "userId", userID, "error", err)
			http.Error(w, "failed to record acceptance", http.StatusInternalServerError)
		}
		return
	}

	// Return updated acceptance status.
	status, err := h.svc.GetAcceptanceStatus(r.Context(), txID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"accepted": "true"})
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// handleGetStatus returns which parties have accepted the agreement.
// GET /api/v1/transactions/:id/agreement/status
func (h *Handler) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	txID := chi.URLParam(r, "transactionId")
	if txID == "" {
		http.Error(w, "transactionId is required", http.StatusBadRequest)
		return
	}

	status, err := h.svc.GetAcceptanceStatus(r.Context(), txID)
	if err != nil {
		if errors.Is(err, ErrAgreementNotFound) {
			http.Error(w, "agreement not found", http.StatusNotFound)
			return
		}
		slog.Warn("agreement: get status failed", "transactionId", txID, "error", err)
		http.Error(w, "failed to fetch acceptance status", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, status)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
