package guaranteefund

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// Handler serves the guarantee fund admin HTTP API.
type Handler struct {
	svc *Service
}

// NewHandler creates a Handler backed by the given Service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers guarantee fund admin routes onto the provided chi.Router.
func (h *Handler) Mount(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMW)
		r.Get("/admin/guarantee-fund/health", h.handleGetHealth)
		r.Get("/admin/guarantee-fund/entries", h.handleGetEntries)
	})
}

// GET /api/v1/admin/guarantee-fund/health
func (h *Handler) handleGetHealth(w http.ResponseWriter, r *http.Request) {
	health, err := h.svc.GetFundHealth(r.Context())
	if err != nil {
		slog.Warn("guaranteefund: get health failed", "error", err)
		http.Error(w, "failed to get fund health", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(health)
}

// GET /api/v1/admin/guarantee-fund/entries
func (h *Handler) handleGetEntries(w http.ResponseWriter, r *http.Request) {
	limit := 20
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	entries, total, err := h.svc.GetEntries(r.Context(), limit, offset)
	if err != nil {
		slog.Warn("guaranteefund: get entries failed", "error", err)
		http.Error(w, "failed to get fund entries", http.StatusInternalServerError)
		return
	}

	type response struct {
		Entries []Entry `json:"entries"`
		Total   int     `json:"total"`
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response{
		Entries: entries,
		Total:   total,
	})
}
