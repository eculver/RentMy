package media

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/giits/rentmy/backend/internal/platform/auth"
)

// Handler holds the HTTP handlers for the media domain.
type Handler struct {
	svc *Service
}

// NewHandler creates a Handler with the given service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers media routes onto an existing chi.Router.
// All routes require authentication.
func (h *Handler) Mount(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMW)
		r.Post("/media/upload", h.upload)
		r.Get("/media/{id}", h.getByID)
	})
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Enforce 10 MB body limit before parsing.
	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadBytes)
	if err := r.ParseMultipartForm(MaxUploadBytes); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
		return
	}
	defer r.MultipartForm.RemoveAll() //nolint:errcheck

	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing image field")
		return
	}
	defer file.Close()

	if header.Size > MaxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "image exceeds 10 MB limit")
		return
	}

	imageData, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read image")
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	var in UploadInput
	if raw := r.FormValue("orientation"); raw != "" {
		var o Orientation
		if err := json.Unmarshal([]byte(raw), &o); err != nil {
			writeError(w, http.StatusBadRequest, "invalid orientation JSON")
			return
		}
		in.Orientation = &o
	}
	if raw := r.FormValue("mediaType"); raw != "" {
		in.MediaType = MediaType(raw)
	}

	m, err := h.svc.Upload(r.Context(), imageData, contentType, in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upload failed")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"media": m})
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "media not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not retrieve media")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"media": m})
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
