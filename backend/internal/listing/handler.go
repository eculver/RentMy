package listing

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"

	"github.com/giits/rentmy/backend/internal/platform/auth"
)

// Handler holds the HTTP handlers for the listing domain.
type Handler struct {
	svc *Service
}

// NewHandler creates a Handler with the given service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers listing routes onto an existing chi.Router.
// All routes require authentication.
func (h *Handler) Mount(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMW)
		r.Post("/listings", h.create)
		r.Get("/listings/{id}", h.getByID)
		r.Put("/listings/{id}", h.update)
		r.Post("/listings/{id}/media", h.attachMedia)
		r.Get("/users/me/listings", h.listMine)
	})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var in CreateListingInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	l, err := h.svc.Create(r.Context(), userID, in)
	if err != nil {
		if errors.Is(err, ErrDurationExceedsLimit) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if isValidationError(err) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "could not create listing")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"listing": l})
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	l, err := h.svc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "listing not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not retrieve listing")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"listing": l})
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")

	var in UpdateListingInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	l, err := h.svc.Update(r.Context(), id, userID, in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "listing not found")
			return
		}
		if errors.Is(err, ErrNotOwner) {
			writeError(w, http.StatusForbidden, "not the listing owner")
			return
		}
		if errors.Is(err, ErrDurationExceedsLimit) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if isValidationError(err) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "could not update listing")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"listing": l})
}

func (h *Handler) attachMedia(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")

	var in AttachMediaInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	l, err := h.svc.AttachMedia(r.Context(), id, userID, in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "listing not found")
			return
		}
		if errors.Is(err, ErrNotOwner) {
			writeError(w, http.StatusForbidden, "not the listing owner")
			return
		}
		if isValidationError(err) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "could not attach media")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"listing": l})
}

func (h *Handler) listMine(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	result, err := h.svc.ListByHost(r.Context(), userID, page, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list listings")
		return
	}

	writeJSON(w, http.StatusOK, result)
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

// isValidationError reports whether err wraps a validator.ValidationErrors.
func isValidationError(err error) bool {
	var ve validator.ValidationErrors
	return errors.As(err, &ve)
}
