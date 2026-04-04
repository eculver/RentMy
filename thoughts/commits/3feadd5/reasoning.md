# Commit 3feadd5 — MediaService

## Why this change

Task 1.2 requires an authenticated upload endpoint so that Phase 1.3 (ListingService) can attach photos to listings and Phase 3 can run photo-diff comparisons between check-in and check-out images.

## Key decisions

### disintegration/imaging over libvips
Pure Go, zero cgo, cross-compiles cleanly. Performance is sufficient for thumbnail generation at v1 scale. Aligned with the Phase 1 plan decision table.

### Mount pattern (mediaHandler.Mount vs mediaHandler.Router)
chi v5 does not support two separate `r.Mount("/api/v1", ...)` calls — the second overwrites the first in the radix tree. Rather than returning a standalone `chi.Router`, the media handler exposes `Mount(r chi.Router, authMW)` that registers directly on the existing v1 router returned by userHandler.

### storageURL passed into Service
The S3 client has no `GetURL` method. URLs are constructed as `{endpoint}/{bucket}/{key}`. In dev this is MinIO at `http://localhost:9002`; in prod it can be set to a CDN prefix via `S3_ENDPOINT`.

### 10 MB cap enforced in handler
`http.MaxBytesReader` limits the multipart body before `ParseMultipartForm` so an oversized upload never reaches the service.
