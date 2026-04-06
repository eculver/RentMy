package photodiff

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Handler serves the photo diff HTTP API.
type Handler struct {
	svc *Service
}

// NewHandler creates a Handler backed by the given Service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers the photo diff routes onto the provided chi.Router.
func (h *Handler) Mount(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMW)
		r.Post("/transactions/{transactionId}/photo-diff", h.handleRunDiff)
		r.Get("/transactions/{transactionId}/photo-diff", h.handleGetResult)
	})
}

// handleRunDiff triggers the photo diff pipeline for a transaction.
// POST /api/v1/transactions/:id/photo-diff
func (h *Handler) handleRunDiff(w http.ResponseWriter, r *http.Request) {
	transactionID := chi.URLParam(r, "transactionId")
	if transactionID == "" {
		http.Error(w, "transactionId is required", http.StatusBadRequest)
		return
	}

	result, err := h.svc.RunDiff(r.Context(), transactionID)
	if err != nil {
		slog.Warn("photodiff: run failed", "transactionId", transactionID, "error", err)
		http.Error(w, "photo diff failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

// handleGetResult returns the stored photo diff result for a transaction.
// GET /api/v1/transactions/:id/photo-diff
func (h *Handler) handleGetResult(w http.ResponseWriter, r *http.Request) {
	transactionID := chi.URLParam(r, "transactionId")
	if transactionID == "" {
		http.Error(w, "transactionId is required", http.StatusBadRequest)
		return
	}

	result, err := h.svc.GetResult(r.Context(), transactionID)
	if err != nil {
		slog.Warn("photodiff: get result failed", "transactionId", transactionID, "error", err)
		http.Error(w, "failed to get photo diff result", http.StatusInternalServerError)
		return
	}

	if result == nil {
		http.Error(w, "no photo diff result for this transaction", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}
