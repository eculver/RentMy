package rating

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/Brett2thered/RentMy/backend/internal/platform/auth"
)

// Handler wires rating HTTP endpoints.
type Handler struct {
	svc *Service
}

// NewHandler creates a new Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers rating routes on r.
//
//	POST   /transactions/:id/ratings          — submit a rating
//	GET    /transactions/:id/ratings          — get ratings for a transaction
//	GET    /users/:id/ratings                 — get ratings received by a user
//	GET    /users/:id/ratings/summary         — bubble count summary
func (h *Handler) Mount(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMW)
		r.Post("/transactions/{id}/ratings", h.submitRating)
		r.Get("/transactions/{id}/ratings", h.getTransactionRatings)
		r.Get("/users/{id}/ratings", h.getUserRatings)
		r.Get("/users/{id}/ratings/summary", h.getUserRatingsSummary)
	})
}

// submitRatingRequest is the JSON body for POST /transactions/:id/ratings.
type submitRatingRequest struct {
	Bubbles []Bubble `json:"bubbles"`
}

// submitRating handles POST /api/v1/transactions/:id/ratings.
func (h *Handler) submitRating(w http.ResponseWriter, r *http.Request) {
	txnID := chi.URLParam(r, "id")
	fromUserID := auth.UserIDFromContext(r.Context())

	var req submitRatingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Bubbles) == 0 {
		writeError(w, http.StatusBadRequest, "at least one bubble is required")
		return
	}

	rt, err := h.svc.SubmitRating(r.Context(), CreateRatingInput{
		TransactionID: txnID,
		FromUserID:    fromUserID,
		Bubbles:       req.Bubbles,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrTransactionNotFound):
			writeError(w, http.StatusNotFound, "transaction not found")
		case errors.Is(err, ErrTransactionNotCompleted):
			writeError(w, http.StatusUnprocessableEntity, "transaction is not completed")
		case errors.Is(err, ErrAlreadyRated):
			writeError(w, http.StatusConflict, "already rated this transaction")
		case errors.Is(err, ErrNotParticipant):
			writeError(w, http.StatusForbidden, "not a participant in this transaction")
		case errors.Is(err, ErrInvalidBubble):
			writeError(w, http.StatusBadRequest, "invalid bubble for your role")
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeJSON(w, http.StatusCreated, rt)
}

// getTransactionRatings handles GET /api/v1/transactions/:id/ratings.
func (h *Handler) getTransactionRatings(w http.ResponseWriter, r *http.Request) {
	txnID := chi.URLParam(r, "id")
	ratings, err := h.svc.GetRatingsForTransaction(r.Context(), txnID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, ratings)
}

// getUserRatings handles GET /api/v1/users/:id/ratings.
func (h *Handler) getUserRatings(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	ratings, total, err := h.svc.GetRatingsForUser(r.Context(), userID, page)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ratings": ratings,
		"total":   total,
		"page":    page,
	})
}

// getUserRatingsSummary handles GET /api/v1/users/:id/ratings/summary.
func (h *Handler) getUserRatingsSummary(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	summary, err := h.svc.GetRatingBubbleSummary(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"summary": summary})
}

// writeJSON encodes v as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
