# Task 2.1 — DiscoveryService (backend)

## Status: COMPLETE

**Commit:** 152cf31
**Branch:** task-2.1-discovery-service
**Branching mode:** git fallback (Graphite unavailable)

---

## What Was Done

### New files

**`backend/internal/discovery/model.go`**
- `FeedQuery`, `SearchQuery`, `MapQuery` — input types for the three discovery endpoints
- `HostStats` — aggregated transaction metrics used in ranking
- `RankedListing` — flat response struct with core listing fields, discovery-computed fields (driveTimeMin, rankScore, fuzzedLat/Lng), and thumbnail

**`backend/internal/discovery/repository.go`**
- `FindNearby` — PostGIS `ST_DWithin` radius query, ULID cursor pagination, LEFT JOIN LATERAL for first photo thumbnail
- `SearchFulltext` — `search_vector @@ plainto_tsquery(...)` with ILIKE fallback, price range filters
- `FindInBoundingBox` — `ST_Within(location, ST_MakeEnvelope(...))` for map bounding box
- `GetHostStats` — aggregate response_rate, on_time_rate, acceptance_rate from transactions table; returns 0.5 defaults when no rows exist
- `nowUTC` package variable for testable time injection

**`backend/internal/discovery/drivetime.go`**
- `DriveTimeClient` — hits `GET /route/v1/driving/{lng,lat};{lng,lat}?overview=false` on OSRM
- Redis cache with 60-min TTL, keyed on ~1 km grid (2 decimal places)
- Returns 0 on any error (OSRM unreachable in dev) — discovery never fails due to missing OSRM

**`backend/internal/discovery/service.go`**
- `Feed`, `Search`, `Map` — public methods delegating to repo + enrichment
- `enrich` — two-pass enrichment: first pass computes drive times to find maxDriveTime, second pass normalises proximity score and applies fuzz
- `computeRankScore` — PRD section 13 formula: `0.35*avail + 0.30*proximity + 0.20*reputation + 0.15*reliability`
- `fuzzLocation` — FNV64a hash of listing ID seeds math/rand for deterministic ~500m jitter
- `isAvailableNow` — checks `[]TimeSlot` availability windows; empty array = always available

**`backend/internal/discovery/handler.go`**
- `GET /api/v1/discovery/feed` — lat, lng, radius, cursor, limit
- `GET /api/v1/discovery/search` — q, lat, lng, radius, min_price, max_price, max_drive_min, cursor, limit
- `GET /api/v1/discovery/map` — sw_lat, sw_lng, ne_lat, ne_lng, limit
- All routes require authentication (authMW)

**`backend/internal/discovery/service_test.go`** (15 tests, all passing)
- `computeRankScore`: full score, zero score, partial, zero max drive
- `fuzzLocation`: deterministic, different seeds produce different results, within bounds
- `isAvailableNow`: empty slots, nil, matching, non-matching, end-boundary edge case
- `Feed`: filters hosts below 0.30 response_rate, sorts by rank score
- `Map`: verifies fuzzed coordinates are applied

**`backend/migrations/002_search_vector_trigger.sql`**
- `update_listing_search_vector()` PL/pgSQL function: sets search_vector = `setweight(title, 'A') || setweight(description, 'B')`
- Trigger fires `BEFORE INSERT OR UPDATE OF title, description ON listings`
- Backfills existing rows via `UPDATE listings SET title = title WHERE search_vector IS NULL`

**`scripts/osrm-prepare.sh`**
- Downloads a Geofabrik OSM PBF extract (default: California)
- Runs `osrm-extract`, `osrm-partition`, `osrm-customize` to produce `/data/region.osrm`
- Intended to be run once inside the OSRM container before starting the routing service

### Modified files

**`backend/internal/platform/config/config.go`**
- Added: `OSRMBaseURL`, `WeightAvailability`, `WeightProximity`, `WeightReputation`, `WeightReliability`, `DefaultFeedRadiusMeters`, `MaxFeedLimit`, `MaxMapLimit`

**`backend/cmd/server/main.go`**
- Added: `discovery.NewDriveTimeClient`, `discovery.NewRepository`, `discovery.NewService`, `discovery.NewHandler`
- `discoveryHandler.Mount(apiV1, authMW)` mounts at `/api/v1/discovery/...`

**`docker-compose.yml`**
- Added: `osrm` service (Docker profile `osrm` — opt-in, requires pre-processed data)
- Added: `osrm_data` volume

---

## Architecture Decisions

- **OSRM in optional Docker profile:** Pre-processing a regional OSM extract (California ~700MB) is a one-time setup. Making OSRM opt-in via `--profile osrm` means `docker compose up -d` works out of the box without OSRM. Feed still ranks correctly using 0 drive time (proximity weight = 0 when all drive times are 0).
- **Exact coords only in DB, fuzzed in API:** FuzzedLat/FuzzedLng are set to exact values by the repository scan, then overwritten by `fuzzLocation()` in the service before the response is serialised. The exact coordinates never appear in the JSON response.
- **Deterministic fuzzing:** Using FNV64a hash of listing ID as PRNG seed means the same pin appears at the same fuzzed location on every map load. If random per-request, pins would jump, which is confusing UX.
- **HostStats 0.5 defaults:** Phase 2 has no completed transactions, so 0.5 keeps unrated hosts at a neutral mid-ranking (not penalised, not boosted). Once Phase 3 transactions flow, real stats will replace the defaults.
- **Two-pass enrichment:** First pass collects all drive times to find maxDriveTime (needed for proximity normalisation). Second pass computes final rank scores. Single-pass would normalise by an incomplete max.

---

## Verification

```
cd backend && go vet ./...          → exit 0
cd backend && go build ./cmd/server → exit 0
cd backend && go test ./internal/discovery/... -v -count=1 → 15/15 PASS
cd backend && go test ./... -count=1 → all packages PASS
```

---

## Notes for Next Tasks

- **Task 2.2 (PaymentService):** No dependencies on DiscoveryService. Can start immediately.
- **Task 2.3 (Feed screen, RN):** Calls `GET /api/v1/discovery/feed`. Response shape: `{listings: RankedListing[], count: int}`. Key display fields: title, pricePerDay/pricePerHour, thumbnailUrl, distanceMeters, driveTimeMin.
- **Task 2.4 (Search screen, RN):** Uses `GET /api/v1/discovery/search?q=...&lat=...&lng=...`.
- **Task 2.5 (Map screen, RN):** Uses `GET /api/v1/discovery/map?sw_lat=...&sw_lng=...&ne_lat=...&ne_lng=...`. Fuzzed lat/lng are in the root of each `RankedListing` as `lat` and `lng` fields.
- **OSRM setup for staging/prod:** The `osrm-prepare.sh` script should be run as part of infrastructure provisioning. For staging, a smaller extract (e.g., Southern California) reduces memory footprint.
