# Phase 0 — Foundation Implementation Plan

> **Scope:** Wk 1-2. Everything else depends on this being solid.
> **Exit criteria:** Server boots, DB migrated, S3/Redis/River/Pusher connected, RN app navigates, CI green.

## Resolved Decisions

| Question | Answer | Notes |
|----------|--------|-------|
| Monorepo vs separate repos | **Monorepo** | `/backend` + `/mobile` in one repo |
| ULID storage format | **text(26)** | Readable, sortable, optimize later if needed |
| Pusher hosting | **Pusher hosted** | Soketi for local dev only |
| CI/CD timing | **Now (Phase 0)** | GitHub Actions for Go tests + RN build check |

---

## Technology Decisions

### Go Backend

| Decision | Choice | Import / Module |
|----------|--------|-----------------|
| HTTP router | chi v5 | `github.com/go-chi/chi/v5` |
| Migrations | goose v3 | `github.com/pressly/goose/v3` |
| Config | caarlos0/env v11 | `github.com/caarlos0/env/v11` |
| Database driver | pgx v5 (required by River) | `github.com/jackc/pgx/v5` |
| Redis client | go-redis v9 | `github.com/redis/go-redis/v9` |
| Job queue | River | `github.com/riverqueue/river` |
| S3 client | aws-sdk-go-v2 | `github.com/aws/aws-sdk-go-v2/service/s3` |
| Pusher | pusher-http-go v5 | `github.com/pusher/pusher-http-go/v5` |
| ULID | oklog/ulid v2 | `github.com/oklog/ulid/v2` |
| Logging | slog (stdlib) | `log/slog` |
| Auth (JWT) | golang-jwt v5 | `github.com/golang-jwt/jwt/v5` |
| Validation | go-playground/validator v10 | `github.com/go-playground/validator/v10` |

**Rationale for non-obvious choices:**
- **pgx over database/sql:** River requires pgx v5. Commit fully — pgx is faster, supports LISTEN/NOTIFY, COPY, and Postgres-specific types natively.
- **goose over golang-migrate:** golang-migrate is in maintenance mode. goose v3 is actively maintained, embeddable as a library (run on app startup), and supports both SQL and Go migrations.
- **caarlos0/env over viper:** 12-factor — config from env vars only. Viper's file/remote config adds unnecessary complexity and a massive dependency tree.
- **slog over zerolog:** stdlib since Go 1.21. Zero external dependency. zerolog's zero-alloc advantage is nanoseconds — irrelevant for this app.
- **chi over stdlib mux:** Go 1.22 stdlib routing is sufficient for simple APIs, but chi's subrouter mounting (`r.Mount`) is essential for the modular monolith pattern where each module returns its own router.

### React Native

| Decision | Choice | Package |
|----------|--------|---------|
| Framework | Expo + Dev Client | `expo`, `expo-dev-client` |
| Navigation | Expo Router v4 | `expo-router` |
| State (client) | Zustand v5 | `zustand` |
| State (server) | TanStack Query v5 | `@tanstack/react-query` |
| API client | ky | `ky` |
| Secure storage | Expo Secure Store | `expo-secure-store` |
| Styling | NativeWind v4 | `nativewind` |
| Forms | React Hook Form + Zod | `react-hook-form`, `zod` |

**Deferred to Phase 1+ (when features need them):**
- `react-native-vision-camera` v4 — camera capture with frame processors (Phase 1)
- `react-native-maps` — map view (Phase 2)
- `@stripe/stripe-react-native` — payments (Phase 2)
- `pusher-js` — real-time events (Phase 3)
- `expo-sensors` — gyroscope for angle enforcement (Phase 1)
- `expo-notifications` — push notifications (Phase 3)

**Rationale for non-obvious choices:**
- **Expo + Dev Client over bare RN:** Meta officially recommends Expo as the framework for new projects. Dev Client + Config Plugins allow any native module without ejecting. Gains: OTA updates, EAS Build CI/CD.
- **Expo Router over React Navigation standalone:** File-based routing with automatic deep linking. Every route is a deep link — critical for sharing listings and notification taps.
- **Zustand + TanStack Query over Redux:** Separates client state (auth, UI) from server state (listings, bookings). TanStack Query handles caching, refetching, pagination, optimistic updates. Zustand handles the rest with zero boilerplate.
- **ky over axios:** Fetch API-based (aligns with RN New Architecture), 3KB vs 13KB, cleaner hook system for auth token injection.
- **NativeWind over Tamagui:** Full creative control — Tailwind design tokens without pre-built component aesthetics. RentMy is mobile-only, so Tamagui's web cross-platform advantage doesn't apply.

---

## Project Structure

### Monorepo Layout

```
rentmy/                              # repo root
  backend/                           # Go modular monolith
    cmd/
      server/
        main.go                      # Wire everything, start server
    internal/
      user/                          # UserService module
        handler.go                   # HTTP handlers, returns chi.Router
        service.go                   # Business logic
        repository.go                # Postgres queries
        model.go                     # Domain types
      listing/                       # ListingService module (stubbed)
        handler.go
        service.go
        repository.go
        model.go
      booking/                       # BookingService module (stubbed)
        model.go
      payment/                       # PaymentService module (stubbed)
        model.go
      platform/                      # Shared infrastructure
        postgres/
          postgres.go                # Connection pool, health check
        redis/
          redis.go                   # Connection pool, health check
        s3/
          s3.go                      # Client, bucket ops, upload/download helpers
        pusher/
          pusher.go                  # Client, trigger helper
        river/
          river.go                   # Client setup, worker lifecycle
          testjob.go                 # Test job for verification
        auth/                        # JWT middleware (stubbed for Phase 1)
          middleware.go
        config/
          config.go                  # Env-based config struct
        httpserver/
          server.go                  # HTTP server with graceful shutdown
          middleware.go              # RequestID, Logger, Recoverer, CORS
        ulid/
          ulid.go                    # ULID generator with monotonic entropy
    migrations/
      001_initial_schema.sql         # All tables from PRD section 6
    go.mod                           # module github.com/giits/rentmy/backend
    go.sum
    Makefile                         # build, test, migrate, dev, lint
  mobile/                            # React Native (Expo)
    app/
      _layout.tsx                    # Root layout: providers (QueryClient, auth gate)
      (auth)/
        _layout.tsx                  # Auth stack layout
        login.tsx                    # Login screen
        register.tsx                 # Register screen
      (tabs)/
        _layout.tsx                  # Tab bar: feed, search, map, messages, profile
        (feed)/
          index.tsx                  # Feed (placeholder)
        (search)/
          index.tsx                  # Search (placeholder)
        (map)/
          index.tsx                  # Map (placeholder)
        (messages)/
          index.tsx                  # Messages (placeholder)
        (profile)/
          index.tsx                  # Profile (placeholder)
    components/
      ui/
        Button.tsx                   # Shared button primitive
        Input.tsx                    # Shared text input
        Card.tsx                     # Listing card skeleton
        Avatar.tsx                   # User avatar
        Badge.tsx                    # Status badge
    lib/
      api.ts                         # ky client with auth hooks
      auth.ts                        # Zustand auth store
      query.ts                       # TanStack Query client config
    app.json                         # Expo config
    tailwind.config.js               # NativeWind / Tailwind config
    tsconfig.json
    package.json
  docker-compose.yml                 # Postgres+PostGIS, Redis, MinIO, Soketi
  .env.example                       # Template for required env vars
  .github/
    workflows/
      ci.yml                         # Go test + lint, RN typecheck + lint
  .gitignore
```

---

## Implementation Steps

### Step 0.1 — Go Project Scaffold (Day 1)

**Create:**
- `backend/go.mod` with module path `github.com/giits/rentmy/backend`
- `backend/cmd/server/main.go` — minimal server with chi router, `/health` endpoint
- `backend/internal/platform/config/config.go` — env-based config struct
- `backend/internal/platform/httpserver/server.go` — HTTP server with graceful shutdown
- `backend/internal/platform/httpserver/middleware.go` — RequestID, structured logging, recoverer
- `backend/internal/platform/ulid/ulid.go` — ULID generator
- `backend/Makefile` with targets: `build`, `test`, `dev`, `lint`
- `.env.example` (repo root)
- `.gitignore` (repo root — Go binaries, node_modules, .env, .expo, etc.)

**Verify:** `cd backend && make dev` → server starts → `curl localhost:8080/health` returns `200 OK`

### Step 0.2 — Docker Compose + PostgreSQL + PostGIS (Day 1-2)

**Create:**
- `docker-compose.yml` with:
  - Postgres 16 + PostGIS 3.4 (`postgis/postgis:16-3.4`)
  - Redis 7 (`redis:7-alpine`)
  - MinIO (`minio/minio:latest`) with `media-originals` and `media-thumbnails` buckets
  - Soketi (`quay.io/soketi/soketi:latest`) for local Pusher-compatible server
- `internal/platform/postgres/postgres.go` — pgx connection pool, health check
- `migrations/001_initial_schema.sql` — Full schema from PRD section 6:
  - `users` table
  - `listings` table with PostGIS `geography(Point, 4326)` column
  - `transactions` table with `hold_allocations` JSONB
  - `messages` table
  - `ratings` table
  - `proximity_proofs` table
  - `agent_decisions` table
  - `guarantee_fund_entries` table
  - PostGIS indexes on location columns
  - ULID as primary key type (stored as `text` or `bytea`)

**Verify:**
- `docker compose up -d` → all services healthy
- `make migrate` → migrations run idempotently
- `SELECT PostGIS_Version()` returns version
- All tables exist with correct columns
- `make migrate` again → no changes (idempotent)

### Step 0.3 — Redis Integration (Day 2)

**Create:**
- `internal/platform/redis/redis.go` — go-redis v9 client, connection pool, health check

**Wire:**
- Add Redis health check to `/health` endpoint
- Config struct gains `REDIS_URL`

**Verify:** Server boots → `/health` checks Postgres AND Redis → both green. `SET/GET` works from Go test.

### Step 0.4 — River Job Queue (Day 2-3)

**Create:**
- `internal/platform/river/river.go` — River client setup with pgx, worker start/stop lifecycle
- `internal/platform/river/testjob.go` — Test job: `TestJobArgs{Message string}` → logs message on execution

**Wire:**
- River migrations run alongside goose migrations (River has its own migrator)
- Worker starts in `main.go`, graceful shutdown on `SIGTERM`/`SIGINT`

**Verify:**
- Enqueue test job via API endpoint (`POST /debug/enqueue-test`)
- Worker picks up job, logs completion
- Send `SIGTERM` → worker finishes in-flight jobs → exits cleanly
- No orphaned jobs in `river_job` table

### Step 0.5 — S3 Storage (Day 3)

**Create:**
- `internal/platform/s3/s3.go` — aws-sdk-go-v2 client with custom endpoint (MinIO), bucket creation helper, upload/download/delete functions

**Wire:**
- Config struct gains `S3_ENDPOINT`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_REGION`
- Bucket creation on startup (idempotent: create if not exists)
- Health check: list buckets

**Verify:** Upload test file → retrieve → content matches → delete → gone

### Step 0.6 — Pusher Integration (Day 3)

**Create:**
- `internal/platform/pusher/pusher.go` — Pusher HTTP client for triggering events, configured for Soketi in dev

**Wire:**
- Config struct gains `PUSHER_APP_ID`, `PUSHER_KEY`, `PUSHER_SECRET`, `PUSHER_HOST`
- Test endpoint: `POST /debug/trigger-event`

**Verify:** Trigger event from Go → Soketi receives it → browser JS client (quick HTML test page) receives the event

### Step 0.7 — React Native Scaffold (Day 3-5)

**Create:**
- Expo project in `mobile/` with `npx create-expo-app mobile --template tabs`
- Restructure to Expo Router file-based layout (see project structure above)
- Install and configure: `zustand`, `@tanstack/react-query`, `ky`, `nativewind`, `expo-secure-store`, `react-hook-form`, `zod`
- `mobile/lib/api.ts` — ky client with `prefixUrl` and auth header hook
- `mobile/lib/auth.ts` — Zustand auth store (token, user, login/logout actions)
- `mobile/lib/query.ts` — TanStack Query client with defaults (stale time, retry)
- `mobile/app/_layout.tsx` — Root layout wrapping QueryClientProvider + auth gate
- `mobile/app/(auth)/_layout.tsx` + `login.tsx` + `register.tsx` — Auth screens (placeholder UI)
- `mobile/app/(tabs)/_layout.tsx` — Tab bar with 5 tabs (placeholder screens)
- `mobile/components/ui/Button.tsx` — Primary/secondary/ghost variants with NativeWind
- `mobile/components/ui/Input.tsx` — Text input with label, error state
- `mobile/components/ui/Card.tsx` — Card container
- `mobile/components/ui/Avatar.tsx` — Image circle with fallback initials
- `mobile/components/ui/Badge.tsx` — Status indicator

**Verify:** `cd mobile && npx expo start` → app boots on iOS simulator → all 5 tabs navigate → auth screens render → UI primitives render correctly

### Step 0.8 — CI/CD with GitHub Actions (Day 5)

**Create:**
- `.github/workflows/ci.yml` — single workflow with two jobs:

**Job 1: `backend`**
```yaml
- Trigger: push/PR to main, paths: backend/**
- Services: postgres:16 (with postgis), redis:7
- Steps: checkout → setup Go → go mod download → go vet → golangci-lint → go test ./...
```

**Job 2: `mobile`**
```yaml
- Trigger: push/PR to main, paths: mobile/**
- Steps: checkout → setup Node 20 → npm ci → npx tsc --noEmit → npx eslint .
```

**Verify:** Push to a branch → both jobs run → both pass green

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `backend/cmd/server/main.go` | Create | Entry point: wire deps, start server + River worker |
| `backend/internal/platform/config/config.go` | Create | Env-based config with `caarlos0/env` |
| `backend/internal/platform/postgres/postgres.go` | Create | pgx pool, health check |
| `backend/internal/platform/redis/redis.go` | Create | go-redis client, health check |
| `backend/internal/platform/s3/s3.go` | Create | S3 client, bucket ops |
| `backend/internal/platform/pusher/pusher.go` | Create | Pusher trigger client |
| `backend/internal/platform/river/river.go` | Create | River client, worker lifecycle |
| `backend/internal/platform/river/testjob.go` | Create | Verification test job |
| `backend/internal/platform/httpserver/server.go` | Create | HTTP server, graceful shutdown |
| `backend/internal/platform/httpserver/middleware.go` | Create | RequestID, logging, recovery |
| `backend/internal/platform/ulid/ulid.go` | Create | ULID generator with monotonic entropy |
| `backend/migrations/001_initial_schema.sql` | Create | Full PRD section 6 schema |
| `backend/Makefile` | Create | build, test, migrate, dev, lint |
| `docker-compose.yml` | Create | Postgres+PostGIS, Redis, MinIO, Soketi |
| `.env.example` | Create | Required env var template |
| `.github/workflows/ci.yml` | Create | Go test + lint, RN typecheck + lint |
| `mobile/app/_layout.tsx` | Create | Root layout with providers |
| `mobile/app/(tabs)/_layout.tsx` | Create | Tab bar navigation |
| `mobile/lib/api.ts` | Create | ky API client |
| `mobile/lib/auth.ts` | Create | Zustand auth store |
| `mobile/lib/query.ts` | Create | TanStack Query config |
| `mobile/components/ui/*.tsx` | Create | 5 shared UI primitives |

---

## Database Schema (Migration 001)

All tables from PRD section 6 with these implementation details:

```sql
-- ULIDs stored as text(26) for readability and portability
-- PostGIS geography type for location columns (meters, WGS84)
-- JSONB for flexible fields: hold_allocation, agreement_snapshot, notification_preferences
-- Timestamps as timestamptz (always UTC)
-- Enums as text with CHECK constraints (not Postgres ENUM — easier to evolve)

CREATE EXTENSION IF NOT EXISTS postgis;

-- users
CREATE TABLE users (
    id                      TEXT PRIMARY KEY,            -- ULID
    email                   TEXT UNIQUE,
    phone                   TEXT UNIQUE,
    password_hash           TEXT,
    name                    TEXT NOT NULL DEFAULT '',
    avatar_url              TEXT,
    identity_status         TEXT NOT NULL DEFAULT 'PENDING'
                            CHECK (identity_status IN ('VERIFIED', 'PENDING', 'REJECTED')),
    reputation_score        INTEGER NOT NULL DEFAULT 0
                            CHECK (reputation_score >= 0 AND reputation_score <= 1000),
    device_fingerprint      TEXT,
    risk_flags              JSONB NOT NULL DEFAULT '[]',
    notification_preferences JSONB NOT NULL DEFAULT '{}',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_active_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- listings (with PostGIS)
CREATE TABLE listings (
    id                      TEXT PRIMARY KEY,
    host_id                 TEXT NOT NULL REFERENCES users(id),
    title                   TEXT NOT NULL DEFAULT '',
    description             TEXT NOT NULL DEFAULT '',
    ai_generated_tags       JSONB NOT NULL DEFAULT '[]',
    estimated_value         NUMERIC(10,2),
    host_declared_value     NUMERIC(10,2),
    value_justification     TEXT,
    price_per_hour          NUMERIC(10,2),
    price_per_day           NUMERIC(10,2),
    min_duration            INTERVAL,
    max_duration            INTERVAL,
    location                GEOGRAPHY(POINT, 4326),
    availability            JSONB NOT NULL DEFAULT '[]',
    has_video               BOOLEAN NOT NULL DEFAULT FALSE,
    status                  TEXT NOT NULL DEFAULT 'PENDING'
                            CHECK (status IN ('ACTIVE', 'PENDING', 'FLAGGED', 'SUSPENDED')),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_listings_host_id ON listings(host_id);
CREATE INDEX idx_listings_location ON listings USING GIST(location);
CREATE INDEX idx_listings_status ON listings(status);

-- media
CREATE TABLE media (
    id                      TEXT PRIMARY KEY,
    listing_id              TEXT REFERENCES listings(id),
    transaction_id          TEXT,                         -- FK added after transactions table
    media_type              TEXT NOT NULL DEFAULT 'LISTING_PHOTO'
                            CHECK (media_type IN ('LISTING_PHOTO', 'CHECK_IN', 'CHECK_OUT', 'KYC_ID', 'KYC_SELFIE')),
    original_url            TEXT NOT NULL,
    thumbnail_url           TEXT,
    orientation_roll        REAL,                         -- degrees
    orientation_pitch       REAL,
    orientation_yaw         REAL,
    gps_lat                 REAL,
    gps_lng                 REAL,
    device_id               TEXT,
    captured_at             TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_media_listing_id ON media(listing_id);

-- transactions
CREATE TABLE transactions (
    id                      TEXT PRIMARY KEY,
    renter_id               TEXT NOT NULL REFERENCES users(id),
    host_id                 TEXT NOT NULL REFERENCES users(id),
    listing_id              TEXT NOT NULL REFERENCES listings(id),
    rental_fee              NUMERIC(10,2),
    hold_amount             NUMERIC(10,2),
    item_value              NUMERIC(10,2),
    guarantee_gap           NUMERIC(10,2),
    risk_score              INTEGER CHECK (risk_score >= 0 AND risk_score <= 100),
    escrow_status           TEXT DEFAULT 'HELD'
                            CHECK (escrow_status IN ('HELD', 'RELEASED', 'CHARGED')),
    hold_status             TEXT DEFAULT 'AUTHORIZED'
                            CHECK (hold_status IN ('AUTHORIZED', 'RELEASED', 'CAPTURED', 'PARTIALLY_CAPTURED')),
    hold_allocation         JSONB NOT NULL DEFAULT '{"totalAuthorized":0,"capturedForLateFees":0,"capturedForDamage":0,"damageReserve":0,"released":0,"remaining":0}',
    guarantee_fund_charged  NUMERIC(10,2) NOT NULL DEFAULT 0,
    agreement_snapshot      JSONB,
    photo_diff_result       TEXT CHECK (photo_diff_result IN ('NO_CHANGE', 'COSMETIC_DAMAGE', 'FUNCTIONAL_DAMAGE', 'MISSING_ITEM', 'INCONCLUSIVE')),
    photo_diff_confidence   REAL CHECK (photo_diff_confidence >= 0 AND photo_diff_confidence <= 1),
    scheduled_start         TIMESTAMPTZ,
    scheduled_end           TIMESTAMPTZ,
    actual_start            TIMESTAMPTZ,
    actual_end              TIMESTAMPTZ,
    status                  TEXT NOT NULL DEFAULT 'REQUESTED'
                            CHECK (status IN ('REQUESTED', 'ACCEPTED', 'DECLINED', 'AUTO_DECLINED', 'ACTIVE', 'COMPLETED', 'DISPUTED', 'CANCELLED')),
    cancelled_by            TEXT CHECK (cancelled_by IN ('RENTER', 'HOST')),
    cancellation_fee        NUMERIC(10,2),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_transactions_renter_id ON transactions(renter_id);
CREATE INDEX idx_transactions_host_id ON transactions(host_id);
CREATE INDEX idx_transactions_listing_id ON transactions(listing_id);
CREATE INDEX idx_transactions_status ON transactions(status);

-- Add FK from media to transactions
ALTER TABLE media ADD CONSTRAINT fk_media_transaction
    FOREIGN KEY (transaction_id) REFERENCES transactions(id);
CREATE INDEX idx_media_transaction_id ON media(transaction_id);

-- proximity_proofs
CREATE TABLE proximity_proofs (
    id                      TEXT PRIMARY KEY,
    transaction_id          TEXT NOT NULL REFERENCES transactions(id),
    proof_type              TEXT NOT NULL CHECK (proof_type IN ('CHECK_IN', 'CHECK_OUT')),
    gps_distance            REAL,
    pin                     TEXT,
    verified                BOOLEAN NOT NULL DEFAULT FALSE,
    method                  TEXT NOT NULL DEFAULT 'GPS'
                            CHECK (method IN ('GPS', 'BLE', 'SMS_FALLBACK')),
    device_id               TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_proximity_proofs_transaction_id ON proximity_proofs(transaction_id);

-- messages
CREATE TABLE messages (
    id                      TEXT PRIMARY KEY,
    transaction_id          TEXT NOT NULL REFERENCES transactions(id),
    sender_id               TEXT NOT NULL REFERENCES users(id),
    content                 TEXT NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_messages_transaction_id ON messages(transaction_id);

-- ratings
CREATE TABLE ratings (
    id                      TEXT PRIMARY KEY,
    transaction_id          TEXT NOT NULL REFERENCES transactions(id),
    from_user_id            TEXT NOT NULL REFERENCES users(id),
    to_user_id              TEXT NOT NULL REFERENCES users(id),
    bubbles                 JSONB NOT NULL DEFAULT '[]',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(transaction_id, from_user_id)   -- one rating per user per transaction
);
CREATE INDEX idx_ratings_to_user_id ON ratings(to_user_id);

-- agent_decisions
CREATE TABLE agent_decisions (
    id                      TEXT PRIMARY KEY,
    agent_type              TEXT NOT NULL
                            CHECK (agent_type IN ('RISK', 'VERIFICATION', 'APPRAISAL', 'DISPUTE', 'AGREEMENT', 'LATE_RETURN', 'FRAUD', 'OPS', 'HUMAN_OVERRIDE')),
    transaction_id          TEXT REFERENCES transactions(id),
    user_id                 TEXT REFERENCES users(id),
    input                   JSONB NOT NULL,
    decision                JSONB NOT NULL,
    model                   TEXT,
    prompt_version          TEXT,
    confidence              REAL,
    escalated               BOOLEAN NOT NULL DEFAULT FALSE,
    escalation_reason       TEXT,
    reviewed_by             TEXT REFERENCES users(id),
    override_of             TEXT REFERENCES agent_decisions(id),
    outcome_id              TEXT,
    outcome_correct         BOOLEAN,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_agent_decisions_transaction_id ON agent_decisions(transaction_id);
CREATE INDEX idx_agent_decisions_agent_type ON agent_decisions(agent_type);
CREATE INDEX idx_agent_decisions_outcome ON agent_decisions(outcome_correct) WHERE outcome_correct IS NOT NULL;

-- guarantee_fund_entries
CREATE TABLE guarantee_fund_entries (
    id                      TEXT PRIMARY KEY,
    transaction_id          TEXT REFERENCES transactions(id),
    entry_type              TEXT NOT NULL
                            CHECK (entry_type IN ('CONTRIBUTION', 'CLAIM', 'CARD_RECOVERY', 'COLLECTIONS_REFERRAL')),
    amount                  NUMERIC(10,2) NOT NULL,
    balance_after           NUMERIC(10,2) NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_guarantee_fund_entries_transaction_id ON guarantee_fund_entries(transaction_id);

-- fulltext search index for listings (Phase 2 will use this)
ALTER TABLE listings ADD COLUMN search_vector TSVECTOR;
CREATE INDEX idx_listings_search ON listings USING GIN(search_vector);
```

---

## Risks and Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| pgx v5 learning curve (different from database/sql) | Slower initial development | pgx is the standard for Postgres-heavy Go apps. River requires it anyway. Invest upfront |
| River is relatively young (~2 years) | Potential edge cases in production | River is by the pgx ecosystem author, Postgres-backed (reliable primitives), and used in production at scale. Alternative (Temporal) is overkill for v1 |
| NativeWind v4 stability | Potential build issues | NativeWind v4 has been stable since late 2024. Fallback: StyleSheet.create is always available for individual components |
| Docker Compose complexity (4 services) | Developer setup friction | Single `docker compose up -d` command. Health checks on all services. `.env.example` documents all vars |
| Schema migration for ULID-as-text | Larger index footprint vs UUID | Text ULIDs are 26 bytes vs UUID's 16 bytes. Acceptable tradeoff for human-readability and sortability. Can migrate to bytea later if needed |
| Expo Router v4 breaking changes | Navigation refactoring | Expo Router is on a stable release cadence now. Pin exact version in package.json |

---

## Testing Strategy (Phase 0 Scope)

- **Go:** `go test ./...` passes. Integration tests for Postgres (testcontainers-go), Redis, S3 connections
- **RN:** App builds and boots on iOS simulator. Tab navigation works. UI primitives render
- **Docker:** All services start, health checks pass
- **Migrations:** Idempotent — running twice produces no errors
- **CI:** Both GitHub Actions jobs pass green

---

## Implementation Order (8 Steps)

| Step | What | Day | Depends On |
|------|------|-----|------------|
| 0.1 | Go scaffold + health endpoint | 1 | — |
| 0.2 | Docker Compose + Postgres/PostGIS + migrations | 1-2 | 0.1 |
| 0.3 | Redis integration | 2 | 0.1 |
| 0.4 | River job queue | 2-3 | 0.2 |
| 0.5 | S3 storage | 3 | 0.1 |
| 0.6 | Pusher integration | 3 | 0.1 |
| 0.7 | React Native scaffold | 3-5 | — (independent) |
| 0.8 | GitHub Actions CI | 5 | 0.1, 0.7 |

Steps 0.3, 0.5, 0.6 are independent of each other — can be parallelized.
Step 0.7 is fully independent of backend work — can start on Day 1.
