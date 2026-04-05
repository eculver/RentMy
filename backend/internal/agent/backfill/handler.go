package backfill

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// riverInserter is the minimal River interface needed to enqueue backfill jobs.
type riverInserter interface {
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// ensure *river.Client[any] satisfies riverInserter at compile time (checked in main.go).

// Handler serves the backfill admin HTTP API.
type Handler struct {
	riverClient riverInserter
	progress    *JobProgress
}

// NewHandler creates a Handler backed by the given River client and progress tracker.
func NewHandler(riverClient riverInserter, progress *JobProgress) *Handler {
	return &Handler{riverClient: riverClient, progress: progress}
}

// Mount registers the backfill admin routes onto the provided chi.Router.
// All routes are admin-only. The authMW parameter is the standard JWT middleware;
// no dedicated admin middleware exists yet (Phase 6) so we reuse the same JWT check.
func (h *Handler) Mount(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMW)
		r.Post("/admin/backfill/appraisals", h.handleBackfillAppraisals)
		r.Post("/admin/backfill/reputation", h.handleBackfillReputation)
		r.Post("/admin/backfill/risk-scores", h.handleBackfillRiskScores)
		r.Get("/admin/backfill/status", h.handleBackfillStatus)
	})
}

// handleBackfillAppraisals enqueues the appraisal backfill job.
// POST /api/v1/admin/backfill/appraisals
func (h *Handler) handleBackfillAppraisals(w http.ResponseWriter, r *http.Request) {
	res, err := h.riverClient.Insert(r.Context(), BackfillAppraisalJobArgs{}, nil)
	if err != nil {
		slog.Warn("backfill: failed to enqueue appraisal job", "error", err)
		http.Error(w, "failed to enqueue backfill job", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": res.Job.ID})
}

// handleBackfillReputation enqueues the reputation backfill job.
// POST /api/v1/admin/backfill/reputation
func (h *Handler) handleBackfillReputation(w http.ResponseWriter, r *http.Request) {
	res, err := h.riverClient.Insert(r.Context(), BackfillReputationJobArgs{}, nil)
	if err != nil {
		slog.Warn("backfill: failed to enqueue reputation job", "error", err)
		http.Error(w, "failed to enqueue backfill job", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": res.Job.ID})
}

// handleBackfillRiskScores enqueues the risk score backfill job.
// POST /api/v1/admin/backfill/risk-scores
func (h *Handler) handleBackfillRiskScores(w http.ResponseWriter, r *http.Request) {
	res, err := h.riverClient.Insert(r.Context(), BackfillRiskScoreJobArgs{}, nil)
	if err != nil {
		slog.Warn("backfill: failed to enqueue risk scores job", "error", err)
		http.Error(w, "failed to enqueue backfill job", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": res.Job.ID})
}

// handleBackfillStatus returns live progress counters for all three backfill jobs.
// GET /api/v1/admin/backfill/status
func (h *Handler) handleBackfillStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"jobs": h.progress.Status()})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
