package appraisal

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Handler serves the AppraisalAgent HTTP API.
type Handler struct {
	svc *Service
}

// NewHandler creates a Handler backed by the given Service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers the appraisal routes onto the provided chi.Router.
// All routes require the auth middleware (authMW) passed in from main.
func (h *Handler) Mount(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMW)
		r.Post("/listings/{listingId}/appraise", h.handleTriggerAppraisal)
		r.Post("/listings/{listingId}/override", h.handleReviewOverride)
		r.Get("/listings/{listingId}/appraisal", h.handleGetAppraisal)
	})
}

// handleTriggerAppraisal triggers (or re-triggers) an AI appraisal for a listing.
// POST /api/v1/listings/:id/appraise
func (h *Handler) handleTriggerAppraisal(w http.ResponseWriter, r *http.Request) {
	listingID := chi.URLParam(r, "listingId")
	if listingID == "" {
		http.Error(w, "listingId is required", http.StatusBadRequest)
		return
	}

	if err := h.svc.Appraise(r.Context(), listingID); err != nil {
		slog.Warn("appraisal: trigger failed", "listingId", listingID, "error", err)
		http.Error(w, "appraisal failed", http.StatusInternalServerError)
		return
	}

	a, err := h.svc.GetAppraisal(r.Context(), listingID)
	if err != nil {
		http.Error(w, "failed to fetch appraisal result", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, toResponse(a))
}

// handleReviewOverride evaluates a host's declared value override request.
// POST /api/v1/listings/:id/override
func (h *Handler) handleReviewOverride(w http.ResponseWriter, r *http.Request) {
	listingID := chi.URLParam(r, "listingId")
	if listingID == "" {
		http.Error(w, "listingId is required", http.StatusBadRequest)
		return
	}

	var req OverrideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.DeclaredValueCents <= 0 {
		http.Error(w, "declaredValueCents must be positive", http.StatusBadRequest)
		return
	}
	if req.Justification == "" {
		http.Error(w, "justification is required", http.StatusBadRequest)
		return
	}

	result, err := h.svc.ReviewOverride(r.Context(), listingID, req)
	if err != nil {
		if errors.Is(err, ErrAppraisalNotFound) {
			http.Error(w, "no appraisal found for this listing", http.StatusNotFound)
			return
		}
		slog.Warn("appraisal: override review failed", "listingId", listingID, "error", err)
		http.Error(w, "override review failed", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// handleGetAppraisal returns the current appraisal result for a listing.
// GET /api/v1/listings/:id/appraisal
func (h *Handler) handleGetAppraisal(w http.ResponseWriter, r *http.Request) {
	listingID := chi.URLParam(r, "listingId")
	if listingID == "" {
		http.Error(w, "listingId is required", http.StatusBadRequest)
		return
	}

	a, err := h.svc.GetAppraisal(r.Context(), listingID)
	if err != nil {
		if errors.Is(err, ErrAppraisalNotFound) {
			http.Error(w, "no appraisal found for this listing", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch appraisal", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, toResponse(a))
}

func toResponse(a *Appraisal) AppraisalResponse {
	return AppraisalResponse{
		ID:                         a.ID,
		ListingID:                  a.ListingID,
		Status:                     a.Status,
		ItemName:                   a.ItemName,
		Category:                   a.Category,
		Condition:                  a.Condition,
		EstimatedValueCents:        a.EstimatedValueCents,
		SuggestedPricePerHourCents: a.SuggestedPricePerHourCents,
		SuggestedPricePerDayCents:  a.SuggestedPricePerDayCents,
		Description:                a.Description,
		Tags:                       a.Tags,
		Confidence:                 a.Confidence,
		OverrideApproved:           a.OverrideApproved,
		OverrideReasoning:          a.OverrideReasoning,
		FailureReason:              a.FailureReason,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
