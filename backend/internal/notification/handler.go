package notification

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/Brett2thered/RentMy/backend/internal/platform/auth"
	"github.com/go-chi/chi/v5"
)

// Handler wires HTTP routes for the notification domain.
type Handler struct {
	svc *Service
}

// NewHandler creates a Handler backed by svc.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers notification routes on the provided router under the given
// auth middleware.
func (h *Handler) Mount(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Route("/notifications", func(r chi.Router) {
		r.Use(authMW)
		r.Get("/", h.list)
		r.Post("/read-all", h.readAll)
		r.Get("/unread-count", h.unreadCount)
		r.Get("/preferences", h.getPreferences)
		r.Put("/preferences", h.updatePreferences)
		r.Post("/register-token", h.registerToken)
		r.Post("/{id}/read", h.markRead)
	})
}

// list returns paginated notifications for the authenticated user.
// GET /api/v1/notifications?limit=20&offset=0
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	limit := queryInt(r, "limit", 20)
	offset := queryInt(r, "offset", 0)

	notifications, total, err := h.svc.GetNotifications(r.Context(), userID, limit, offset)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	respond(w, http.StatusOK, map[string]any{
		"notifications": notifications,
		"total":         total,
		"limit":         limit,
		"offset":        offset,
	})
}

// markRead marks a single notification as read.
// POST /api/v1/notifications/:id/read
func (h *Handler) markRead(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	if err := h.svc.MarkRead(r.Context(), id, userID); err != nil {
		if errors.Is(err, ErrNotificationNotFound) {
			http.Error(w, "notification not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	respond(w, http.StatusOK, map[string]string{"status": "ok"})
}

// readAll marks all notifications as read for the authenticated user.
// POST /api/v1/notifications/read-all
func (h *Handler) readAll(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if err := h.svc.MarkAllRead(r.Context(), userID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "ok"})
}

// unreadCount returns the number of unread notifications.
// GET /api/v1/notifications/unread-count
func (h *Handler) unreadCount(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	count, err := h.svc.CountUnread(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respond(w, http.StatusOK, map[string]int{"count": count})
}

// getPreferences returns the authenticated user's notification preferences.
// GET /api/v1/notifications/preferences
func (h *Handler) getPreferences(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	prefs, err := h.svc.GetPreferences(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respond(w, http.StatusOK, prefs)
}

// updatePreferences saves the authenticated user's notification preferences.
// PUT /api/v1/notifications/preferences
func (h *Handler) updatePreferences(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var prefs Preferences
	if err := json.NewDecoder(r.Body).Decode(&prefs); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.svc.UpdatePreferences(r.Context(), userID, prefs); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "ok"})
}

// registerToken saves an Expo push token for the authenticated user.
// POST /api/v1/notifications/register-token
func (h *Handler) registerToken(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Token == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}

	if err := h.svc.RegisterPushToken(r.Context(), userID, body.Token); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "ok"})
}

// respond writes a JSON response.
func respond(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// queryInt reads an integer query parameter with a default fallback.
func queryInt(r *http.Request, key string, def int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return def
	}
	return v
}
