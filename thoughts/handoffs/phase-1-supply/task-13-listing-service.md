# Task 1.3 — ListingService (backend)

## Status: COMPLETE

**Commit:** 4650d50  
**Branch:** task-1.3-listing-service  
**Branching mode:** git fallback (Graphite unavailable)

---

## What Was Done

### New files
- `backend/internal/listing/model.go` — `Listing`, `ListingStatus` constants, `Duration` (custom JSON type marshaling as Go duration string, e.g. "168h"), `Location`, `CreateListingInput`, `UpdateListingInput`, `AttachMediaInput`, `ListByHostResult`.
- `backend/internal/listing/repository.go` — `Repository` with `Insert`, `FindByID`, `FindByHostID` (paginated with COUNT + LIMIT/OFFSET), `Update`, `AttachMedia`. Handles PostGIS via WKT parameters and `ST_X`/`ST_Y` scan columns. Handles INTERVAL via `pgtype.Interval` with `durationToInterval`/`intervalToDuration` helpers.
- `backend/internal/listing/service.go` — `Service` with `Create` (validates 7-day ceiling, defaults status=PENDING), `Get`, `ListByHost`, `Update` (ownership check), `AttachMedia` (ownership check). `RepositoryInterface` enables unit tests without Postgres.
- `backend/internal/listing/handler.go` — `Handler` with `Mount(r chi.Router, authMW)`:
  - `POST /api/v1/listings`
  - `GET /api/v1/listings/{id}`
  - `PUT /api/v1/listings/{id}`
  - `POST /api/v1/listings/{id}/media`
  - `GET /api/v1/users/me/listings`
- `backend/internal/listing/service_test.go` — 15 unit tests: status default, host ID, validation, 7-day ceiling enforce/allow, location storage, not-found, pagination, owner-only update/attach, duration JSON round-trip.

### Modified files
- `backend/cmd/server/main.go` — Wires `listing.Repository`, `listing.Service`, `listing.Handler`. Mounts via `listingHandler.Mount(apiV1, authMW)`.

---

## API Endpoints

| Method | Path | Auth | Notes |
|--------|------|------|-------|
| POST | `/api/v1/listings` | Bearer | Creates listing; 400 if maxDuration > 168h |
| GET | `/api/v1/listings/{id}` | Bearer | Returns listing |
| PUT | `/api/v1/listings/{id}` | Bearer | Partial update; 403 if not owner |
| POST | `/api/v1/listings/{id}/media` | Bearer | Attaches media IDs; 403 if not owner |
| GET | `/api/v1/users/me/listings` | Bearer | Paginated; ?page=1&limit=20 |

---

## Verification Results

- `go vet ./...` — PASS
- `go build ./cmd/server` — PASS
- `go test ./... -v -count=1` — PASS (15 listing tests + 9 media tests + 6 user tests = 30 total)

---

## Key Decisions

### Duration type
Created a custom `Duration` type (alias of `time.Duration`) with `MarshalJSON`/`UnmarshalJSON` that serialize as Go duration strings (e.g. "168h0m0s" / "168h"). This matches the plan's curl examples and keeps the API human-readable.

### PostGIS location handling
- **INSERT**: Pass lat and lng as separate `*float64` parameters; SQL uses `ST_SetSRID(ST_MakePoint($lng, $lat), 4326)::geography` with a NULL guard.
- **SELECT**: `ST_Y(location::geometry) AS loc_lat, ST_X(location::geometry) AS loc_lng` — returns NULL when location is NULL, scanned into `*float64`.

### INTERVAL handling
Used `pgtype.Interval{Microseconds: dur.Microseconds(), Valid: true}` for both reads and writes. Days and Months fields in pgtype.Interval are accounted for in `intervalToDuration`.

### AttachMedia in listing repository
The `AttachMedia` method issues `UPDATE media SET listing_id = $1 WHERE id IN (...)`. Though it touches the `media` table, it belongs in the listing repository because it expresses a listing relationship. No new media package changes needed.

### Mount pattern (not Router)
Follows the same pattern established by MediaService — `Mount(r, authMW)` adds routes to the existing `/api/v1` sub-router to avoid chi radix-tree conflicts.

### `GET /users/me/listings` route placement
This route lives in the listing handler (not user handler) since it queries listing data. Mounted onto the shared apiV1 router alongside user routes. chi resolves it correctly.

---

## Notes for Next Task (1.4 — Auth screens RN)

- `POST /api/v1/auth/register`, `POST /api/v1/auth/login`, `POST /api/v1/auth/refresh` are all live.
- `GET /api/v1/users/me` and `PUT /api/v1/users/me` are live.
- Tokens: `accessToken` (15min), `refreshToken` (7d), stored in Redis.
- Auth header: `Authorization: Bearer <accessToken>`.
