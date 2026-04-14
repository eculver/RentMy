package messaging

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/Brett2thered/RentMy/backend/internal/platform/auth"
	"github.com/go-chi/chi/v5"
)

// Handler wires HTTP routes for the messaging domain.
type Handler struct {
	svc *Service
}

// NewHandler creates a Handler backed by svc.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers messaging routes on the provided router under the given
// auth middleware. Routes are nested under /bookings/:id/messages so they
// sit alongside the booking handler's routes.
func (h *Handler) Mount(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMW)
		r.Get("/users/me/conversations", h.listConversations)
		r.Post("/bookings/{id}/messages", h.sendMessage)
		r.Get("/bookings/{id}/messages", h.getMessages)
	})
}

// listConversations handles GET /api/v1/users/me/conversations.
func (h *Handler) listConversations(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	convs, err := h.svc.ListConversations(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list conversations")
		return
	}

	if convs == nil {
		convs = []Conversation{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"conversations": convs})
}

// sendMessage handles POST /api/v1/bookings/:id/messages.
func (h *Handler) sendMessage(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	transactionID := chi.URLParam(r, "id")

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	msg, err := h.svc.SendMessage(r.Context(), SendMessageInput{
		TransactionID: transactionID,
		SenderID:      userID,
		Content:       body.Content,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrEmptyContent):
			writeError(w, http.StatusBadRequest, "message content cannot be empty")
		case errors.Is(err, ErrContentTooLong):
			writeError(w, http.StatusBadRequest, "message content exceeds maximum length")
		case errors.Is(err, ErrNotAParty):
			writeError(w, http.StatusForbidden, "not a party to this transaction")
		case errors.Is(err, ErrTransactionNotFound):
			writeError(w, http.StatusNotFound, "booking not found")
		default:
			writeError(w, http.StatusInternalServerError, "could not send message")
		}
		return
	}

	writeJSON(w, http.StatusCreated, msg)
}

// getMessages handles GET /api/v1/bookings/:id/messages?cursor=<ulid>&limit=50.
func (h *Handler) getMessages(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	transactionID := chi.URLParam(r, "id")
	cursor := r.URL.Query().Get("cursor")
	limit := queryInt(r, "limit", 50)

	msgs, nextCursor, err := h.svc.GetMessages(r.Context(), transactionID, userID, cursor, limit)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotAParty):
			writeError(w, http.StatusForbidden, "not a party to this transaction")
		case errors.Is(err, ErrTransactionNotFound):
			writeError(w, http.StatusNotFound, "booking not found")
		default:
			writeError(w, http.StatusInternalServerError, "could not retrieve messages")
		}
		return
	}

	// Return empty array instead of null when there are no messages.
	if msgs == nil {
		msgs = []Message{}
	}

	resp := map[string]any{
		"messages": msgs,
	}
	if nextCursor != "" {
		resp["nextCursor"] = nextCursor
	}

	writeJSON(w, http.StatusOK, resp)
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

// queryInt reads an integer query parameter with a default fallback.
func queryInt(r *http.Request, key string, def int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return def
	}
	return v
}
