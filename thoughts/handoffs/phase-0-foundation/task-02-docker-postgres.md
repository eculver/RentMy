# Task 0.2 — Docker Compose + PostgreSQL + PostGIS + Migrations

## Status: Complete

## What Was Done

### 1. Docker Compose (`docker-compose.yml` at repo root)
- **postgres**: `postgis/postgis:16-3.4` on port 5432, with health check via `pg_isready`
- **redis**: `redis:7-alpine` on port 6379, with health check via `redis-cli ping`
- **minio**: `minio/minio:latest` on ports 9000 (API) + 9001 (console), with health check
- **soketi**: `quay.io/soketi/soketi:latest` on port 6001, with health check
- Named volumes: `postgres_data`, `minio_data`

### 2. Postgres Connection Pool (`backend/internal/platform/postgres/postgres.go`)
- pgx v5 connection pool with `MaxConns=25`, `MinConns=5`
- Pings on connect to verify liveness
- `HealthCheck()` function for use in health endpoint

### 3. Migration Runner (`backend/internal/platform/postgres/migrate.go`)
- Uses goose v3 with embedded FS
- Accepts `fs.FS` and subdirectory path for flexibility
- Uses pgx stdlib driver (`database/sql` interface for goose compatibility)

### 4. Migration Embedding (`backend/migrations/embed.go`)
- Embeds all `*.sql` files from the migrations directory
- Exports `migrations.FS` for use by main.go and any other caller

### 5. Initial Schema (`backend/migrations/001_initial_schema.sql`)
- Full goose-annotated migration with `-- +goose Up` and `-- +goose Down`
- All 9 tables from PRD section 6:
  - `users` (ULID PK, identity_status CHECK, reputation_score CHECK)
  - `listings` (PostGIS `GEOGRAPHY(POINT, 4326)`, GIST index on location)
  - `media` (FK to both listings and transactions)
  - `transactions` (hold_allocation JSONB, multiple CHECK constraints)
  - `proximity_proofs` (GPS/BLE/SMS_FALLBACK methods)
  - `messages` (per-transaction chat)
  - `ratings` (bubble-based, unique per user per transaction)
  - `agent_decisions` (AI audit trail with outcome tracking)
  - `guarantee_fund_entries` (fund accounting)
- PostGIS extension enabled
- Full-text search vector column + GIN index on listings
- Down migration drops everything in reverse dependency order

### 6. Updated `main.go`
- Imports postgres and migrations packages
- Runs migrations on startup (before opening pool)
- Creates pgx connection pool, defers Close()
- Health endpoint now checks Postgres connectivity
- Returns `{"status":"ok","postgres":"connected"}` or `{"status":"degraded","postgres":"error: ..."}`

### 7. Updated Makefile
- Added `docker-up` and `docker-down` targets (relative path to repo root docker-compose.yml)
- Added `migrate` target (informational — migrations run on startup)

### 8. Dependencies Added
- `github.com/jackc/pgx/v5` v5.9.1
- `github.com/pressly/goose/v3` v3.27.0
- Transitive: `github.com/jackc/pgpassfile`, `github.com/jackc/pgservicefile`, `github.com/mfridman/interpolate`, `github.com/sethvargo/go-retry`, `go.uber.org/multierr`

## Verification

- `go build ./...` passes
- `go vet ./...` passes
- Docker Compose starts Postgres with PostGIS (tested — container goes healthy)
- Server starts, runs migrations, connects pool, serves health endpoint

## Notes

- The `postgis/postgis:16-3.4` image only has `linux/amd64` builds. On Apple Silicon it runs under Rosetta emulation (Docker Desktop handles this transparently). A platform warning is expected but harmless.
- If port 5432 is already in use on the host (e.g., another Postgres), change the host port mapping in docker-compose.yml to `"5433:5432"` and update `DATABASE_URL` accordingly.

## Files Created/Modified

| File | Operation |
|------|-----------|
| `docker-compose.yml` | Created |
| `backend/internal/platform/postgres/postgres.go` | Created |
| `backend/internal/platform/postgres/migrate.go` | Created |
| `backend/migrations/embed.go` | Created |
| `backend/migrations/001_initial_schema.sql` | Created |
| `backend/cmd/server/main.go` | Modified |
| `backend/internal/platform/config/config.go` | Unchanged (already had DATABASE_URL) |
| `backend/Makefile` | Modified |
| `backend/go.mod` | Modified (new dependencies) |
| `backend/go.sum` | Modified (new dependencies) |

## Next Steps

- **Step 0.3**: Redis integration (`internal/platform/redis/redis.go`) + add Redis health check to `/health`
- **Step 0.4**: River job queue (`internal/platform/river/river.go`) — requires pgx pool from this step
