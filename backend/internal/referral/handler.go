package referral

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/Brett2thered/RentMy/backend/internal/platform/auth"
)

// Handler exposes HTTP endpoints for the referral domain.
type Handler struct {
	svc *Service
}

// NewHandler creates a Handler backed by svc.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers user-facing and ops referral routes on r.
//
//	POST   /referrals/code              — generate (or fetch) the caller's referral code
//	GET    /referrals/code              — get the caller's existing referral code
//	POST   /referrals/apply             — apply a referral code (body: {code})
//	GET    /referrals/mine              — list referrals where caller is referrer
//	GET    /ops/referrals               — (ops) paginated list of all referrals
//	GET    /ops/referrals/stats         — (ops) aggregate stats
func (h *Handler) Mount(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMW)
		r.Post("/referrals/code", h.generateCode)
		r.Get("/referrals/code", h.getCode)
		r.Post("/referrals/apply", h.applyCode)
		r.Get("/referrals/mine", h.myReferrals)

		// Ops endpoints — in production these should be behind an ops-role middleware.
		r.Get("/ops/referrals", h.opsListReferrals)
		r.Get("/ops/referrals/stats", h.opsStats)
	})
}

// generateCode handles POST /api/v1/referrals/code.
func (h *Handler) generateCode(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	rc, err := h.svc.GenerateCode(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate referral code")
		return
	}
	writeJSON(w, http.StatusOK, rc)
}

// getCode handles GET /api/v1/referrals/code.
func (h *Handler) getCode(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	rc, err := h.svc.GetCodeByUser(r.Context(), userID)
	if errors.Is(err, ErrCodeNotFound) {
		writeError(w, http.StatusNotFound, "no referral code found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch referral code")
		return
	}
	writeJSON(w, http.StatusOK, rc)
}

// applyCodeRequest is the body for POST /api/v1/referrals/apply.
type applyCodeRequest struct {
	Code string `json:"code"`
}

// applyCode handles POST /api/v1/referrals/apply.
func (h *Handler) applyCode(w http.ResponseWriter, r *http.Request) {
	var req applyCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}

	userID := auth.UserIDFromContext(r.Context())
	ref, err := h.svc.ApplyReferralCode(r.Context(), userID, req.Code)
	if err != nil {
		switch {
		case errors.Is(err, ErrCodeNotFound):
			writeError(w, http.StatusNotFound, "referral code not found")
		case errors.Is(err, ErrSelfReferral):
			writeError(w, http.StatusBadRequest, "cannot apply your own referral code")
		case errors.Is(err, ErrCodeExpired):
			writeError(w, http.StatusBadRequest, "referral code has expired")
		case errors.Is(err, ErrCodeExhausted):
			writeError(w, http.StatusBadRequest, "referral code is no longer valid")
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, ref)
}

// myReferrals handles GET /api/v1/referrals/mine.
func (h *Handler) myReferrals(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	page, limit := parsePagination(r)

	refs, err := h.svc.ListReferralsByReferrer(r.Context(), userID, page, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch referrals")
		return
	}
	if refs == nil {
		refs = []*Referral{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"referrals": refs, "page": page, "limit": limit})
}

// opsListReferrals handles GET /api/v1/ops/referrals.
func (h *Handler) opsListReferrals(w http.ResponseWriter, r *http.Request) {
	page, limit := parsePagination(r)
	refs, err := h.svc.ListAllReferralsPaginated(r.Context(), ListReferralsFilter{Page: page, Limit: limit})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list referrals")
		return
	}
	if refs == nil {
		refs = []*Referral{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"referrals": refs, "page": page, "limit": limit})
}

// opsStats handles GET /api/v1/ops/referrals/stats.
func (h *Handler) opsStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.GetStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch referral stats")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func parsePagination(r *http.Request) (page, limit int) {
	page = 1
	limit = 20
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	return
}
