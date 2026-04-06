package dispute

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/riverqueue/river"

	"github.com/giits/rentmy/backend/internal/platform/auth"
)

// timeNow is a seam for testing.
var timeNow = time.Now

// Handler serves the dispute HTTP API.
type Handler struct {
	svc *Service
}

// NewHandler creates a Handler backed by the given Service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers dispute routes onto the provided chi.Router.
func (h *Handler) Mount(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMW)

		// User-facing endpoints.
		r.Post("/transactions/{transactionId}/disputes", h.handleFileDispute)
		r.Get("/transactions/{transactionId}/disputes", h.handleGetByTransaction)
		r.Get("/disputes/{disputeId}", h.handleGetByID)

		// Admin endpoints.
		r.Get("/admin/disputes/review-queue", h.handleReviewQueue)
		r.Post("/admin/disputes/{disputeId}/resolve", h.handleResolve)
		r.Post("/admin/disputes/{disputeId}/request-evidence", h.handleRequestEvidence)
	})
}

type fileDisputeRequest struct {
	Reason      string `json:"reason"`
	Description string `json:"description"`
}

// POST /api/v1/transactions/:id/disputes
func (h *Handler) handleFileDispute(w http.ResponseWriter, r *http.Request) {
	transactionID := chi.URLParam(r, "transactionId")
	if transactionID == "" {
		http.Error(w, "transactionId is required", http.StatusBadRequest)
		return
	}

	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req fileDisputeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Reason == "" {
		http.Error(w, "reason is required", http.StatusBadRequest)
		return
	}

	d, err := h.svc.FileDispute(r.Context(), FileDisputeInput{
		TransactionID: transactionID,
		ReporterID:    userID,
		Reason:        req.Reason,
		Description:   req.Description,
	})
	if err != nil {
		if errors.Is(err, ErrAlreadyDisputed) {
			http.Error(w, "transaction already has an open dispute", http.StatusConflict)
			return
		}
		slog.Warn("dispute: file failed", "error", err)
		http.Error(w, "failed to file dispute", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(d)
}

// GET /api/v1/transactions/:id/disputes
func (h *Handler) handleGetByTransaction(w http.ResponseWriter, r *http.Request) {
	transactionID := chi.URLParam(r, "transactionId")
	if transactionID == "" {
		http.Error(w, "transactionId is required", http.StatusBadRequest)
		return
	}

	disputes, err := h.svc.GetDisputesByTransaction(r.Context(), transactionID)
	if err != nil {
		slog.Warn("dispute: get by transaction failed", "error", err)
		http.Error(w, "failed to get disputes", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(disputes)
}

// GET /api/v1/disputes/:id
func (h *Handler) handleGetByID(w http.ResponseWriter, r *http.Request) {
	disputeID := chi.URLParam(r, "disputeId")
	if disputeID == "" {
		http.Error(w, "disputeId is required", http.StatusBadRequest)
		return
	}

	d, err := h.svc.GetDispute(r.Context(), disputeID)
	if err != nil {
		if errors.Is(err, ErrDisputeNotFound) {
			http.Error(w, "dispute not found", http.StatusNotFound)
			return
		}
		slog.Warn("dispute: get by id failed", "error", err)
		http.Error(w, "failed to get dispute", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(d)
}

// GET /api/v1/admin/disputes/review-queue
func (h *Handler) handleReviewQueue(w http.ResponseWriter, r *http.Request) {
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

	disputes, err := h.svc.GetReviewQueue(r.Context(), limit, offset)
	if err != nil {
		slog.Warn("dispute: review queue failed", "error", err)
		http.Error(w, "failed to get review queue", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(disputes)
}

type resolveRequest struct {
	Action       string `json:"action"` // "APPROVE" or "OVERRIDE"
	ChargeAmount *int64 `json:"chargeAmount,omitempty"`
	Notes        string `json:"notes"`
}

// POST /api/v1/admin/disputes/:id/resolve
func (h *Handler) handleResolve(w http.ResponseWriter, r *http.Request) {
	disputeID := chi.URLParam(r, "disputeId")
	if disputeID == "" {
		http.Error(w, "disputeId is required", http.StatusBadRequest)
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
	if req.Action != "APPROVE" && req.Action != "OVERRIDE" {
		http.Error(w, "action must be APPROVE or OVERRIDE", http.StatusBadRequest)
		return
	}

	err := h.svc.ResolveByHuman(r.Context(), ResolveInput{
		DisputeID:    disputeID,
		ReviewerID:   userID,
		Action:       req.Action,
		ChargeAmount: req.ChargeAmount,
		Notes:        req.Notes,
	})
	if err != nil {
		if errors.Is(err, ErrDisputeNotFound) {
			http.Error(w, "dispute not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrInvalidStatus) {
			http.Error(w, "dispute is not in human review status", http.StatusConflict)
			return
		}
		slog.Warn("dispute: resolve failed", "error", err)
		http.Error(w, "failed to resolve dispute", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "resolved"})
}

// POST /api/v1/admin/disputes/:id/request-evidence
func (h *Handler) handleRequestEvidence(w http.ResponseWriter, r *http.Request) {
	disputeID := chi.URLParam(r, "disputeId")
	if disputeID == "" {
		http.Error(w, "disputeId is required", http.StatusBadRequest)
		return
	}

	d, err := h.svc.GetDispute(r.Context(), disputeID)
	if err != nil {
		if errors.Is(err, ErrDisputeNotFound) {
			http.Error(w, "dispute not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to get dispute", http.StatusInternalServerError)
		return
	}

	if err := h.svc.repo.UpdateStatus(r.Context(), disputeID, StatusInconclusive); err != nil {
		http.Error(w, "failed to update status", http.StatusInternalServerError)
		return
	}

	// Schedule re-prompt expiry job (2 hours).
	if h.svc.riverClient != nil {
		_, err := h.svc.riverClient.Insert(r.Context(), RePromptExpiryJobArgs{
			DisputeID:     disputeID,
			TransactionID: d.TransactionID,
		}, &river.InsertOpts{
			ScheduledAt: timeNow().Add(2 * time.Hour),
		})
		if err != nil {
			slog.Warn("dispute: failed to enqueue re-prompt expiry", "error", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "evidence_requested"})
}
