# Task 0.4: River Durable Job Queue

**Status:** Complete
**Date:** 2026-03-31

## What Was Done

Integrated the River durable job queue (v0.32.0) into the RentMy Go backend. River is a Postgres-backed job queue that uses `FOR UPDATE SKIP LOCKED` for reliable, exactly-once job processing -- no Redis or external broker needed.

### Files Created

- `backend/internal/platform/river/river.go` -- River client factory (`New`) and migration runner (`RunMigrations`)
- `backend/internal/platform/river/testjob.go` -- Test job worker (`TestJobArgs` / `TestJobWorker`) for verifying queue functionality

### Files Modified

- `backend/cmd/server/main.go` -- Wired River migrations, worker registration, client lifecycle, and debug endpoint
- `backend/go.mod` / `backend/go.sum` -- Added River v0.32.0 and transitive dependencies

### Dependencies Added

- `github.com/riverqueue/river` v0.32.0
- `github.com/riverqueue/river/riverdriver/riverpgxv5` v0.32.0
- `github.com/riverqueue/river/rivermigrate` v0.32.0
- `github.com/riverqueue/river/riverdriver` v0.32.0 (transitive)
- `github.com/riverqueue/river/rivershared` v0.32.0 (transitive)
- `github.com/riverqueue/river/rivertype` v0.32.0 (transitive)

## Architecture Decisions

### Package Naming

The local package is `internal/platform/river` (same name as the library). In `main.go` the local package is aliased as `riverpkg` while the library is imported as `river`:

```go
"github.com/riverqueue/river"
riverpkg "github.com/giits/rentmy/backend/internal/platform/river"
```

### Generic Type Parameter

River's `Client` is generic over the transaction type: `river.Client[pgx.Tx]`. The riverpgxv5 driver binds to `pgx.Tx` from `github.com/jackc/pgx/v5`.

### Shutdown Ordering

Defers in `run()` execute LIFO, giving this shutdown sequence:
1. Redis close (doesn't affect River)
2. River `Stop()` with 30-second timeout (allows in-flight jobs to complete)
3. Postgres pool close (safe -- River is already stopped)

### Migration Strategy

River's internal schema migrations (`river_job`, `river_leader`, etc.) run via `rivermigrate.Migrator` against the pgxpool, separate from the app's goose migrations. They run after the pool connects but before the River client starts.

## Debug Endpoint

`POST /debug/enqueue-test` -- enqueues a test job with a timestamped message. The `TestJobWorker` logs the message on execution. Use this to verify end-to-end queue processing:

```bash
curl -X POST http://localhost:8080/debug/enqueue-test
# {"status":"enqueued"}
# Check server logs for: "test job executed" message
```

## Queue Configuration

- Queue: `default`
- Max workers: 10
- Future queues (e.g., `media_processing`, `notifications`) can be added to the `Queues` map in `river.New()`.

## Verification

- `go vet ./...` -- passes clean
- `go build -o bin/server ./cmd/server` -- builds successfully
