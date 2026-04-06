# Task 5.3 — Phase 1-2 Backend Integration Tests

**Status:** completed  
**Branch:** task-5.3-phase12-integration-tests  
**Commit:** 30e5d1e  
**Graphite mode:** yes  

---

## What was built

Four new integration test files under `backend/tests/integration/`:

| File | Tests |
|------|-------|
| `user_api_test.go` | Register+login, duplicate email 409, GET /users/me, PUT /users/me, invalid/missing token 401 |
| `listing_api_test.go` | Create listing, missing title 400, GET by ID, GET 404, PUT update, 403 owner-only, 7-day ceiling, list mine |
| `discovery_api_test.go` | Feed nearby, pagination, feed missing lat/lng 400, fulltext search, search missing q 400, map bbox, map missing bbox 400, location fuzzing |
| `payment_api_test.go` | Hold estimate 404, tiered hold (5 sub-cases), hold estimate is public |

Total: 26 tests. All pass in ~5 seconds.

---

## Bugs fixed in task 5.1 infrastructure

Two bugs were discovered and fixed during this task:

**`helpers_test.go` — CleanupDB had wrong table names:**
- `bookings` → `transactions` (the actual table name)
- `proximity_verifications` → `proximity_proofs`
- `listing_media` → removed (not a real table; media is in `media`)
- `conversations` → removed (not a real table; messages in `messages`)
- Added missing tables: `agreement_acceptances`, `push_tokens`, `ratings`, `risk_scores`

**`factories_test.go` — CreateTestListing passed int64 nanoseconds for INTERVAL columns:**
- `min_duration` and `max_duration` are PostgreSQL `INTERVAL` columns
- Factory was passing `int64` nanoseconds which pgx cannot encode as interval
- Fixed by using `pgtype.Interval{Microseconds: ..., Valid: true}`
- Fixed scan to use `pgtype.Interval` instead of `*int64`

---

## Key design decisions

**No `t.Parallel()`:** All tests share a single database (testcontainers started once in `TestMain`). Running tests in parallel with `CleanupDB` causes races — parallel test A's user gets deleted by parallel test B's `CleanupDB`. Removing parallelism makes tests sequential and deterministic at the cost of ~5× longer runtime (still fast: <10s total).

**Payment tier subtests run sequentially:** The `TestHoldEstimateTiers` subtests were originally parallel (each inserting their own listing), but removed parallelism to match the rest of the suite.

**Hold-estimate uses SQL interval literals:** The payment test inserts listings with `'1 hour'::interval` and `'24 hours'::interval` directly in SQL to avoid pgx type encoding issues.

---

## Dependencies added

None — this task only adds test files and fixes bugs in existing test helpers.

---

## Verification

```bash
cd backend && go test ./tests/integration/... -v -count=1 -timeout 300s
# All 26 tests pass in ~5s

cd backend && go test ./... 
# All existing unit tests still pass
```
