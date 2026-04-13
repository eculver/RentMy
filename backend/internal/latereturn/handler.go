package latereturn

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/Brett2thered/RentMy/backend/internal/platform/auth"
)

// Handler serves the late return HTTP API.
type Handler struct {
	svc *Service
}

// NewHandler creates a Handler backed by the given Service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers late return routes onto the provided chi.Router.
func (h *Handler) Mount(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMW)

		// Admin endpoints.
		r.Get("/admin/late-returns", h.handleListActive)
		r.Get("/admin/late-returns/{lateReturnId}", h.handleGetByID)
	})
}

// GET /api/v1/admin/late-returns
func (h *Handler) handleListActive(w http.ResponseWriter, r *http.Request) {
	_ = auth.UserIDFromContext(r.Context())

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

	lateReturns, err := h.svc.GetActiveLateReturns(r.Context(), limit, offset)
	if err != nil {
		slog.Warn("latereturn: list active failed", "error", err)
		http.Error(w, "failed to list late returns", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(lateReturns)
}

// GET /api/v1/admin/late-returns/:id
func (h *Handler) handleGetByID(w http.ResponseWriter, r *http.Request) {
	lateReturnID := chi.URLParam(r, "lateReturnId")
	if lateReturnID == "" {
		http.Error(w, "lateReturnId is required", http.StatusBadRequest)
		return
	}

	lr, err := h.svc.GetLateReturn(r.Context(), lateReturnID)
	if err != nil {
		if errors.Is(err, ErrLateReturnNotFound) {
			http.Error(w, "late return not found", http.StatusNotFound)
			return
		}
		slog.Warn("latereturn: get by id failed", "error", err)
		http.Error(w, "failed to get late return", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(lr)
}
