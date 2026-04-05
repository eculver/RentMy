# Handoff: Task 5.1 — Backend Integration Test Infrastructure

**Status:** Completed  
**Commit:** c737f10  
**Branch:** task-5.1-backend-integration-infra  
**Date:** 2026-04-05  
**Graphite mode:** yes (gt create succeeded)

---

## What Was Built

### `backend/app/server.go` (new package)

Extracted the entire service-wiring and chi router-mounting logic from
`cmd/server/main.go` into a reusable `app.New(ctx, Deps)` function. Both
the production binary and the integration test helpers call `app.New` with
their respective infrastructure clients, guaranteeing the test server is
identical to production.

Key design decisions:
- `app.Deps.Pusher` and `app.Deps.S3` are optional (nil-safe) — real-time
  events and media uploads are disabled when nil, while all other routes work
- `app.Server.Stop(ctx)` gracefully drains the River job queue

### `backend/cmd/server/main.go` (refactored)

Simplified to: load config → create infrastructure clients → run migrations
→ call `app.New` → start HTTP server. All service wiring is now in `app/`.

### `backend/tests/integration/helpers_test.go`

- `TestMain` — starts `imresamu/postgis:16-3.5` and `redis:7-alpine` testcontainers,
  runs goose and River migrations, sets package-level `testPool`/`testRedis`/`testCfg`
- `NewTestDB` / `NewTestRedis` — return shared infrastructure clients
- `NewTestServer` — calls `app.New` with test deps, wraps handler in
  `httptest.Server`, registers `t.Cleanup` to stop River and the HTTP server
- `CleanupDB` — TRUNCATE CASCADE on all application tables for test isolation
- `DoJSON` / `MustDecodeJSON` / `DrainBody` — HTTP test utilities

### `backend/tests/integration/factories_test.go`

Factory functions for test data:
- `CreateTestUser(t, pool) *user.User` — inserts user with bcrypt hash of "password123"
- `CreateTestListing(t, pool, ownerID) *listing.Listing` — inserts ACTIVE listing at SF coordinates
- `CreateTestBooking(t, pool, renterID, listingID) *booking.Booking` — inserts REQUESTED transaction
- `LoginTestUser(t, client, baseURL, email, password) string` — calls POST /api/v1/auth/login, returns JWT
- `AuthHeader(token) http.Header` — builds Authorization header

### `backend/tests/integration/smoke_test.go`

`TestHealthSmoke` — starts a full test server, hits GET /health, asserts 200 with
`status: ok`, `postgres: connected`, `redis: connected`.

### Migration fixes (bugs discovered during testing)

- **`002_search_vector_trigger.sql`** — added `-- +goose StatementBegin/End` around the
  PL/pgSQL `CREATE FUNCTION` block. Without this, goose splits on `;` inside `$$...$$`
  and sends a malformed statement to Postgres. Previously undetected because the dev DB
  had the trigger from an earlier full-schema setup; fresh testcontainers DBs expose it.
- **`006_agent_decisions.sql`** — changed to `CREATE TABLE IF NOT EXISTS` and
  `CREATE INDEX IF NOT EXISTS`. The `agent_decisions` table is also created in
  `001_initial_schema.sql`; on a fresh DB both migrations run in order, causing a
  "relation already exists" error. The IF NOT EXISTS guard makes 006 idempotent.

---

## Verification Commands

```bash
# Passed:
go vet ./...
go build ./cmd/server
go test ./tests/integration/... -v -count=1 -timeout 120s
```

---

## Dependencies Added

| Package | Purpose |
|---------|---------|
| `github.com/testcontainers/testcontainers-go v0.41.0` | Container lifecycle management |
| `github.com/testcontainers/testcontainers-go/modules/postgres v0.41.0` | Postgres container |
| `github.com/testcontainers/testcontainers-go/modules/redis v0.41.0` | Redis container |

Docker image used for Postgres: `imresamu/postgis:16-3.5` (has arm64 support;
official `postgis/postgis:16-3.5-alpine` has no arm64 manifest).

---

## Notes for Next Tasks (5.2-5.6)

- Task 5.2 (mobile test infra) is independent — no deps on this task
- Tasks 5.3-5.5 (retroactive API tests) depend on this task
- `NewTestServer` starts a full River client per test server — this is intentional
  (jobs can be enqueued in tests). River doesn't process jobs immediately so tests
  that check async side-effects should use polling or River test mode in the future
- `CleanupDB` truncates application tables only — River system tables (`river_job`,
  `river_leader`, etc.) are NOT truncated; River handles its own cleanup
- S3 is nil in `NewTestServer` — tests that exercise media upload will need to spin
  up a MinIO container and pass it explicitly
