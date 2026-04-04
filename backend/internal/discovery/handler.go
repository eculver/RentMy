package discovery

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// Handler holds HTTP handlers for the discovery domain.
type Handler struct {
	svc *Service
}

// NewHandler creates a Handler with the given service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers discovery routes onto an existing chi.Router.
// All routes require authentication (authMW must be applied by the caller).
func (h *Handler) Mount(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMW)
		r.Get("/discovery/feed", h.feed)
		r.Get("/discovery/search", h.search)
		r.Get("/discovery/map", h.mapView)
	})
}

// feed handles GET /api/v1/discovery/feed
// Query params: lat, lng, radius (optional), cursor (optional), limit (optional)
func (h *Handler) feed(w http.ResponseWriter, r *http.Request) {
	lat, ok := parseFloat(w, r, "lat")
	if !ok {
		return
	}
	lng, ok := parseFloat(w, r, "lng")
	if !ok {
		return
	}

	q := FeedQuery{
		Lat:    lat,
		Lng:    lng,
		Cursor: r.URL.Query().Get("cursor"),
	}
	if v := r.URL.Query().Get("radius"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			q.RadiusMeters = n
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			q.Limit = n
		}
	}

	listings, err := h.svc.Feed(r.Context(), q)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not fetch feed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"listings": listings, "count": len(listings)})
}

// search handles GET /api/v1/discovery/search
// Query params: q (required), lat, lng, radius, min_price, max_price,
//
//	max_drive_min, cursor, limit
func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "q is required")
		return
	}

	lat, ok := parseFloat(w, r, "lat")
	if !ok {
		return
	}
	lng, ok := parseFloat(w, r, "lng")
	if !ok {
		return
	}

	sq := SearchQuery{
		Query:  query,
		Lat:    lat,
		Lng:    lng,
		Cursor: r.URL.Query().Get("cursor"),
	}
	if v := r.URL.Query().Get("radius"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			sq.RadiusMeters = n
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			sq.Limit = n
		}
	}
	if v := r.URL.Query().Get("min_price"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			sq.MinPrice = &f
		}
	}
	if v := r.URL.Query().Get("max_price"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			sq.MaxPrice = &f
		}
	}
	if v := r.URL.Query().Get("max_drive_min"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			sq.MaxDriveMin = &n
		}
	}

	listings, err := h.svc.Search(r.Context(), sq)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not search listings")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"listings": listings, "count": len(listings)})
}

// mapView handles GET /api/v1/discovery/map
// Query params: sw_lat, sw_lng, ne_lat, ne_lng, limit (optional)
func (h *Handler) mapView(w http.ResponseWriter, r *http.Request) {
	swLat, ok := parseFloat(w, r, "sw_lat")
	if !ok {
		return
	}
	swLng, ok := parseFloat(w, r, "sw_lng")
	if !ok {
		return
	}
	neLat, ok := parseFloat(w, r, "ne_lat")
	if !ok {
		return
	}
	neLng, ok := parseFloat(w, r, "ne_lng")
	if !ok {
		return
	}

	mq := MapQuery{
		SWLat: swLat,
		SWLng: swLng,
		NELat: neLat,
		NELng: neLng,
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			mq.Limit = n
		}
	}

	listings, err := h.svc.Map(r.Context(), mq)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not fetch map listings")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"listings": listings, "count": len(listings)})
}

// parseFloat reads a required float query parameter. If missing or invalid,
// it writes a 400 response and returns (0, false).
func parseFloat(w http.ResponseWriter, r *http.Request, name string) (float64, bool) {
	v := r.URL.Query().Get(name)
	if v == "" {
		writeError(w, http.StatusBadRequest, name+" is required")
		return 0, false
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, name+" must be a number")
		return 0, false
	}
	return f, true
}

// writeJSON encodes v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
