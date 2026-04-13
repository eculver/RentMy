package ops

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Brett2thered/RentMy/backend/internal/platform/auth"
)

// Handler serves the OpsAgent HTTP API.
type Handler struct {
	repo   *Repository
	agent  *Agent
}

// NewHandler creates a Handler.
func NewHandler(repo *Repository, agent *Agent) *Handler {
	return &Handler{repo: repo, agent: agent}
}

// Mount registers ops routes onto the provided chi.Router.
// All routes require the auth middleware.
func (h *Handler) Mount(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMW)

		// Metrics.
		r.Get("/ops/metrics/current", h.handleGetCurrentMetrics)
		r.Get("/ops/metrics/history", h.handleGetMetricsHistory)

		// Alerts.
		r.Get("/ops/alerts", h.handleListAlerts)
		r.Put("/ops/alerts/{alertId}/acknowledge", h.handleAcknowledgeAlert)
		r.Get("/ops/alerts/rules", h.handleListAlertRules)
		r.Put("/ops/alerts/rules/{ruleId}", h.handleUpdateAlertRule)
	})
}

// handleGetCurrentMetrics returns the most recent health snapshot.
// GET /api/v1/ops/metrics/current
func (h *Handler) handleGetCurrentMetrics(w http.ResponseWriter, r *http.Request) {
	snap, err := h.repo.GetLatestSnapshot(r.Context())
	if err != nil {
		if errors.Is(err, ErrNoSnapshot) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no snapshot available"})
			return
		}
		http.Error(w, "failed to fetch metrics", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// handleGetMetricsHistory returns snapshot history for the requested duration.
// GET /api/v1/ops/metrics/history?duration=7d|30d|90d
func (h *Handler) handleGetMetricsHistory(w http.ResponseWriter, r *http.Request) {
	durationStr := r.URL.Query().Get("duration")
	since, err := parseDuration(durationStr)
	if err != nil {
		http.Error(w, "invalid duration: use 7d, 30d, or 90d", http.StatusBadRequest)
		return
	}

	snaps, err := h.repo.GetSnapshotHistory(r.Context(), since)
	if err != nil {
		http.Error(w, "failed to fetch history", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"snapshots": snaps})
}

// handleListAlerts returns paginated alerts with optional filters.
// GET /api/v1/ops/alerts?severity=CRITICAL&acknowledged=false&page=1&limit=50
func (h *Handler) handleListAlerts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	f := AlertFilters{
		Severity: Severity(q.Get("severity")),
		Page:     1,
		Limit:    50,
	}

	if ack := q.Get("acknowledged"); ack != "" {
		b := ack == "true"
		f.Acknowledged = &b
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

	alerts, err := h.repo.ListAlerts(r.Context(), f)
	if err != nil {
		http.Error(w, "failed to fetch alerts", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"alerts": alerts})
}

// handleAcknowledgeAlert marks an alert as acknowledged.
// PUT /api/v1/ops/alerts/:alertId/acknowledge
func (h *Handler) handleAcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	alertID := chi.URLParam(r, "alertId")
	if alertID == "" {
		http.Error(w, "alertId is required", http.StatusBadRequest)
		return
	}

	// Extract user ID from JWT claims embedded in context by auth middleware.
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.repo.AcknowledgeAlert(r.Context(), alertID, userID); err != nil {
		if errors.Is(err, ErrAlertNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "alert not found or already acknowledged"})
			return
		}
		http.Error(w, "failed to acknowledge alert", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "acknowledged"})
}

// handleListAlertRules returns all configured alert rules.
// GET /api/v1/ops/alerts/rules
func (h *Handler) handleListAlertRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.repo.ListAlertRules(r.Context())
	if err != nil {
		http.Error(w, "failed to fetch alert rules", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": rules})
}

// updateAlertRuleRequest is the JSON body for PUT /api/v1/ops/alerts/rules/:id.
type updateAlertRuleRequest struct {
	Threshold *float64 `json:"threshold"`
	Severity  Severity `json:"severity"`
	Channel   Channel  `json:"channel"`
	Enabled   *bool    `json:"enabled"`
}

// handleUpdateAlertRule updates an existing alert rule's threshold, severity, or enabled state.
// PUT /api/v1/ops/alerts/rules/:ruleId
func (h *Handler) handleUpdateAlertRule(w http.ResponseWriter, r *http.Request) {
	ruleID := chi.URLParam(r, "ruleId")
	if ruleID == "" {
		http.Error(w, "ruleId is required", http.StatusBadRequest)
		return
	}

	var req updateAlertRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Fetch existing rule to apply partial updates.
	rules, err := h.repo.ListAlertRules(r.Context())
	if err != nil {
		http.Error(w, "failed to fetch alert rules", http.StatusInternalServerError)
		return
	}

	var existing *AlertRule
	for i := range rules {
		if rules[i].ID == ruleID {
			existing = &rules[i]
			break
		}
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "alert rule not found"})
		return
	}

	if req.Threshold != nil {
		existing.Threshold = *req.Threshold
	}
	if req.Severity != "" {
		existing.Severity = req.Severity
	}
	if req.Channel != "" {
		existing.Channel = req.Channel
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}

	if err := h.repo.UpdateAlertRule(r.Context(), *existing); err != nil {
		if errors.Is(err, ErrRuleNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "alert rule not found"})
			return
		}
		http.Error(w, "failed to update alert rule", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, existing)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func parseDuration(s string) (time.Duration, error) {
	switch s {
	case "", "7d":
		return 7 * 24 * time.Hour, nil
	case "30d":
		return 30 * 24 * time.Hour, nil
	case "90d":
		return 90 * 24 * time.Hour, nil
	}
	return 0, errors.New("unsupported duration")
}

