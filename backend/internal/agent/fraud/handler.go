package fraud

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/Brett2thered/RentMy/backend/internal/platform/auth"
)

// Handler serves the FraudAgent HTTP API.
type Handler struct {
	repo  *Repository
	agent *Agent
}

// NewHandler creates a Handler.
func NewHandler(repo *Repository, agent *Agent) *Handler {
	return &Handler{repo: repo, agent: agent}
}

// Mount registers fraud routes onto the provided chi.Router.
// All routes require the auth middleware and are mounted under /ops/fraud.
func (h *Handler) Mount(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMW)

		r.Get("/ops/fraud/flags", h.handleListFlags)
		r.Get("/ops/fraud/flags/{flagId}", h.handleGetFlag)
		r.Put("/ops/fraud/flags/{flagId}/resolve", h.handleResolveFlag)
		r.Get("/ops/fraud/users/{userId}/signals", h.handleGetUserSignals)
	})
}

// handleListFlags returns paginated fraud flags.
// GET /api/v1/ops/fraud/flags?status=OPEN&action=FLAG&page=1&limit=50
func (h *Handler) handleListFlags(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	f := FraudFlagFilters{
		Status: q.Get("status"),
		Action: Action(q.Get("action")),
		Page:   1,
		Limit:  50,
	}
	if p := q.Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			f.Page = v
		}
	}
	if l := q.Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 200 {
			f.Limit = v
		}
	}

	flags, total, err := h.repo.ListFraudFlags(r.Context(), f)
	if err != nil {
		http.Error(w, "failed to fetch fraud flags", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"flags": flags,
		"total": total,
		"page":  f.Page,
	})
}

// handleGetFlag returns a single fraud flag with its signals.
// GET /api/v1/ops/fraud/flags/:flagId
func (h *Handler) handleGetFlag(w http.ResponseWriter, r *http.Request) {
	flagID := chi.URLParam(r, "flagId")
	if flagID == "" {
		http.Error(w, "flagId is required", http.StatusBadRequest)
		return
	}

	flag, err := h.repo.GetFraudFlag(r.Context(), flagID)
	if err != nil {
		if errors.Is(err, ErrFlagNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "fraud flag not found"})
			return
		}
		http.Error(w, "failed to fetch fraud flag", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, flag)
}

// handleResolveFlag resolves a fraud flag.
// PUT /api/v1/ops/fraud/flags/:flagId/resolve
// Body: {"outcome": "CORRECT"|"INCORRECT"|"INCONCLUSIVE", "notes": "..."}
func (h *Handler) handleResolveFlag(w http.ResponseWriter, r *http.Request) {
	flagID := chi.URLParam(r, "flagId")
	if flagID == "" {
		http.Error(w, "flagId is required", http.StatusBadRequest)
		return
	}

	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req resolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Outcome == "" {
		http.Error(w, "outcome is required", http.StatusBadRequest)
		return
	}

	if err := h.repo.ResolveFraudFlag(r.Context(), flagID, userID, req.Outcome, req.Notes); err != nil {
		if errors.Is(err, ErrFlagNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "fraud flag not found or already resolved"})
			return
		}
		http.Error(w, "failed to resolve fraud flag", http.StatusInternalServerError)
		return
	}

	flag, err := h.repo.GetFraudFlag(r.Context(), flagID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
		return
	}
	writeJSON(w, http.StatusOK, flag)
}

// handleGetUserSignals returns all fraud flags (and their embedded signals)
// for a given user.
// GET /api/v1/ops/fraud/users/:userId/signals
func (h *Handler) handleGetUserSignals(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	if userID == "" {
		http.Error(w, "userId is required", http.StatusBadRequest)
		return
	}

	flags, err := h.repo.GetUserFraudHistory(r.Context(), userID)
	if err != nil {
		http.Error(w, "failed to fetch user signals", http.StatusInternalServerError)
		return
	}

	// Flatten signals across all flags for the response.
	var signals []FraudSignal
	for _, flag := range flags {
		signals = append(signals, flag.Signals...)
	}

	writeJSON(w, http.StatusOK, map[string]any{"signals": signals, "flags": flags})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
