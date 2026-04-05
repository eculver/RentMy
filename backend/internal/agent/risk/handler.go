package risk

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Handler serves the RiskAgent HTTP API.
type Handler struct {
	svc *Service
}

// NewHandler creates a Handler backed by the given Service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers risk routes onto the provided chi.Router.
// All routes require the auth middleware.
func (h *Handler) Mount(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMW)
		r.Get("/users/{userId}/reputation", h.handleGetReputation)
		r.Get("/transactions/{transactionId}/risk", h.handleGetRiskScore)
	})
}

// reputationResponse is the JSON response for GET /api/v1/users/:id/reputation.
type reputationResponse struct {
	UserID          string              `json:"userId"`
	ReputationScore int                 `json:"reputationScore"`
	Signals         []signalResponse    `json:"signals"`
}

type signalResponse struct {
	SignalType    string  `json:"signalType"`
	Points        int     `json:"points"`
	TransactionID *string `json:"transactionId,omitempty"`
	EmittedAt     string  `json:"emittedAt"`
}

// handleGetReputation returns a user's reputation score and signal history.
// GET /api/v1/users/:id/reputation
func (h *Handler) handleGetReputation(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	if userID == "" {
		http.Error(w, "userId is required", http.StatusBadRequest)
		return
	}

	signals, err := h.svc.GetReputationSignals(r.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch reputation", http.StatusInternalServerError)
		return
	}

	profile, err := h.svc.repo.FindUserProfile(r.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch reputation", http.StatusInternalServerError)
		return
	}

	resp := reputationResponse{
		UserID:          userID,
		ReputationScore: profile.ReputationScore,
		Signals:         make([]signalResponse, 0, len(signals)),
	}
	for _, sig := range signals {
		resp.Signals = append(resp.Signals, signalResponse{
			SignalType:    string(sig.SignalType),
			Points:        sig.Points,
			TransactionID: sig.TransactionID,
			EmittedAt:     sig.EmittedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleGetRiskScore returns the risk score for a transaction.
// GET /api/v1/transactions/:id/risk
func (h *Handler) handleGetRiskScore(w http.ResponseWriter, r *http.Request) {
	txID := chi.URLParam(r, "transactionId")
	if txID == "" {
		http.Error(w, "transactionId is required", http.StatusBadRequest)
		return
	}

	rs, err := h.svc.GetRiskScore(r.Context(), txID)
	if err != nil {
		if errors.Is(err, ErrTransactionNotFound) {
			http.Error(w, "risk score not found for this transaction", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch risk score", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, rs)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
