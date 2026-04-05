# Commit 152cf31 — feat: add DiscoveryService with ranked feed, fulltext search, and map

## Why

Phase 2 opens with DiscoveryService because renters need a way to find listings
before they can book. All three discovery surfaces (feed, search, map) share the
same PostGIS + ranking infrastructure, so they ship together.

## Architecture choices

**Location fuzzing via deterministic PRNG (hash/fnv + math/rand):**
Exact host coordinates are never returned to clients. The same listing ID
always produces the same jitter (~500m) so map pins don't jump on refresh.
Using FNV64a hash of listing ID as the PRNG seed makes the result
deterministic without storing a fuzz offset in the database.

**OSRM via self-hosted Docker (optional profile):**
Drive time improves ranking quality but isn't required for the service to
function. The DriveTimeClient returns 0 on any error, so development and
CI work without the OSRM container. Added docker-compose profile "osrm"
so it's opt-in, and scripts/osrm-prepare.sh documents the data prep step.

**search_vector trigger in migration 002:**
The schema already had the GIN index on search_vector but no mechanism
to populate it. The trigger fires on INSERT/UPDATE of title/description,
keeping the vector current without application-level bookkeeping. A
fallback ILIKE clause in SearchFulltext handles rows that predate the
trigger.

**HostStats defaulting to 0.5:**
No transactions exist in Phase 2, so GetHostStats returns 0.5 defaults
for all three rates. This puts unrated hosts at a neutral ranking position
rather than at the bottom (0) or top (1). Phase 3's BookingService will
start populating actual transaction data.

**repo interface in service.go:**
The Service depends on the repo interface, not the concrete Repository.
This allowed writing comprehensive unit tests (15 passing) using fake
implementations without a database connection.

## Files

| File | Purpose |
|------|---------|
| `internal/discovery/model.go` | FeedQuery, SearchQuery, MapQuery, RankedListing types |
| `internal/discovery/repository.go` | FindNearby, SearchFulltext, FindInBoundingBox, GetHostStats |
| `internal/discovery/drivetime.go` | OSRM HTTP client with Redis cache |
| `internal/discovery/service.go` | Ranking, fuzzing, availability check, enrich pipeline |
| `internal/discovery/handler.go` | GET /discovery/feed, /search, /map |
| `internal/discovery/service_test.go` | 15 unit tests: ranking, fuzzing, availability, filtering |
| `migrations/002_search_vector_trigger.sql` | Auto-update search_vector on listing changes |
| `scripts/osrm-prepare.sh` | Download + process OSM extract for OSRM |
