package outcome

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// Handler provides HTTP endpoints for the outcome/calibration admin API.
type Handler struct {
	svc *Service
}

// NewHandler creates a new outcome Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers outcome routes on the given router. All routes are admin-only.
func (h *Handler) Mount(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMW)
		r.Get("/admin/agents/calibration", h.handleGetCalibration)
		r.Get("/admin/agents/calibration/{agentType}", h.handleGetCalibrationByAgent)
		r.Get("/admin/agents/decisions", h.handleGetDecisions)
	})
}

func (h *Handler) handleGetCalibration(w http.ResponseWriter, r *http.Request) {
	reports, err := h.svc.GetCalibration(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(reports)
}

func (h *Handler) handleGetCalibrationByAgent(w http.ResponseWriter, r *http.Request) {
	agentType := chi.URLParam(r, "agentType")
	if agentType == "" {
		http.Error(w, "agentType is required", http.StatusBadRequest)
		return
	}

	report, err := h.svc.GetCalibrationForAgent(r.Context(), agentType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(report)
}

func (h *Handler) handleGetDecisions(w http.ResponseWriter, r *http.Request) {
	agentType := r.URL.Query().Get("agentType")

	var outcomeFilter *bool
	if v := r.URL.Query().Get("outcomeCorrect"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			http.Error(w, "invalid outcomeCorrect value", http.StatusBadRequest)
			return
		}
		outcomeFilter = &b
	}

	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if l, err := strconv.Atoi(v); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if o, err := strconv.Atoi(v); err == nil && o >= 0 {
			offset = o
		}
	}

	decisions, err := h.svc.GetDecisions(r.Context(), agentType, outcomeFilter, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(decisions)
}
