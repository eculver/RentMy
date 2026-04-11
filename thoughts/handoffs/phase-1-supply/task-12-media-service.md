# Task 1.2 — MediaService (backend)

## Status: COMPLETE

**Commit:** 3feadd5  
**Branch:** task-1.2-media-service  
**Branching mode:** git fallback (Graphite unavailable)

---

## What Was Done

### New files
- `backend/internal/media/model.go` — `Media`, `MediaType` constants, `UploadInput`, `Orientation`.
- `backend/internal/media/repository.go` — `Repository` with `Insert`, `FindByID`, `FindByListingID`, `FindByTransactionID`. Raw SQL via pgx.
- `backend/internal/media/service.go` — `Service` with `Upload` (validates, uploads original + thumbnail to S3, persists DB row) and `GetByID`. `RepositoryInterface` and `StorageClient` interfaces enable unit tests without Postgres/S3.
- `backend/internal/media/handler.go` — `Handler` with `Mount(r chi.Router, authMW)`:
  - `POST /api/v1/media/upload` (multipart: `image` file + optional `orientation` JSON + optional `mediaType` string)
  - `GET /api/v1/media/{id}`
- `backend/internal/media/service_test.go` — 9 unit tests covering: default media type, explicit type, orientation fields, two S3 calls per upload, empty image error, thumbnail downscaling, small image passthrough, not-found.

### Modified files
- `backend/cmd/server/main.go` — Wires `media.Repository`, `media.Service`, `media.Handler`. Mounts via `mediaHandler.Mount(apiV1, authMW)` on the same v1 chi.Router returned by `userHandler.Router(authMW)`.
- `backend/go.mod` / `backend/go.sum` — Added `github.com/disintegration/imaging v1.6.2`.

---

## API Endpoints

| Method | Path | Auth | Notes |
|--------|------|------|-------|
| POST | `/api/v1/media/upload` | Bearer | Multipart: `image` + `orientation` JSON + optional `mediaType` |
| GET | `/api/v1/media/{id}` | Bearer | Returns media record |

---

## Verification Results

- `go vet ./...` — PASS
- `go build ./cmd/server` — PASS
- `go test ./... -v -count=1` — PASS (9 media tests + 6 user tests = 15 total)

---

## Key Decisions

- **`Mount` not `Router`**: chi v5 radix tree overwrites duplicate path mounts. The media handler exposes `Mount(r, authMW)` so it adds routes to the existing `/api/v1` sub-router.
- **`storageURL` in Service**: URLs are `{endpoint}/{bucket}/{key}`. In dev this resolves to MinIO. Set `S3_ENDPOINT` to a CDN prefix in production.
- **10 MB limit**: `http.MaxBytesReader` in the handler, checked before any parsing.
- **Thumbnail target**: 800px longest side via `imaging.Fit` + Lanczos resampling + JPEG quality 85.

---

## Notes for Next Task (1.3 — ListingService)

- `media.Repository.FindByListingID(ctx, listingID)` is ready for attaching media to listings.
- When attaching media to a listing, `UPDATE media SET listing_id = $1 WHERE id = $2` is sufficient — no new repository method needed beyond what's here, or add `AttachToListing` if preferred.
- The `media` table's `listing_id` FK references `listings(id)`, which will be populated by task 1.3.
