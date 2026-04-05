# Task 4.8 — Backfill Existing Data Handoff

## Summary

Implemented three River batch workers that backfill AI-generated data onto existing database records, plus admin HTTP endpoints to trigger them and monitor progress.

## Branching Mode

Git fallback (vanilla git). Branch: `task-4.8-backfill-existing-data`. Commit: `ddf60d4`.

## What Was Built

### `backend/internal/agent/backfill/`

| File | Purpose |
|------|---------|
| `jobs.go` | Three River batch workers + shared `JobProgress` atomic counter struct |
| `repository.go` | Read-only queries: listings needing appraisal, users with transactions, transactions without risk scores |
| `handler.go` | Admin HTTP handlers: POST trigger endpoints + GET status endpoint |
| `jobs_test.go` | Unit tests for `JobProgress` counters and job kind strings |

### River Workers

**`BackfillAppraisalWorker`** (`backfill_appraisal`)
- Queries: `ai_generated_tags IS NULL OR ai_generated_tags = '[]' OR estimated_value IS NULL`
- Calls `AppraisalAgent.Appraise()` for each listing
- Rate-limit: 1s sleep per batch of 10 to avoid Anthropic API throttling
- Skips failures and continues (errors are counted, not fatal)

**`BackfillReputationWorker`** (`backfill_reputation`)
- Queries: all users with at least one transaction (renter or host)
- Calls `RiskAgent.RecalculateReputation()` for each user
- Batch logging every 50 users

**`BackfillRiskScoreWorker`** (`backfill_risk_scores`)
- Queries: `transactions LEFT JOIN risk_scores WHERE risk_scores.transaction_id IS NULL`
- Calls `RiskAgent.ComputeRiskScore()` for each transaction
- Scores are analytics-only — no retroactive effect on payout timing or hold amounts

### Admin HTTP Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/admin/backfill/appraisals` | Enqueue appraisal backfill job |
| `POST` | `/api/v1/admin/backfill/reputation` | Enqueue reputation backfill job |
| `POST` | `/api/v1/admin/backfill/risk-scores` | Enqueue risk score backfill job |
| `GET` | `/api/v1/admin/backfill/status` | Live progress counters for all three jobs |

### Progress Tracking

`JobProgress` uses three sets of `atomic.Int64` counters (total, processed, errors) per job type. The `GET /status` handler reads them without locks. Counters reset to 0 each time a new job run starts.

### Changes to `backend/cmd/server/main.go`

- Imports `backfill` package
- Builds `backfillRepo` and `backfillProgress` before River starts
- Constructs `backfillAppraisalWorker` (nil appraisalSvc), `backfillReputationWorker`, `backfillRiskWorker` and registers all three with `river.AddWorker`
- After `appraisalSvcFull` is built: calls `backfillAppraisalWorker.SetAppraisalSvc(appraisalSvcFull)`
- Builds `backfillHandler` and mounts it on the `/api/v1` router

## Key Design Decisions

- **Nil-safe pre-River construction**: `BackfillAppraisalWorker` follows the same two-phase init pattern as `AppraisalJobWorker` — constructed with a nil service for pre-River registration, then `SetAppraisalSvc` is called after the full service is built.
- **Atomic progress counters**: In-memory `atomic.Int64` fields on `JobProgress` are safe for concurrent reads from the status handler without needing a mutex or database query.
- **Skip-on-failure semantics**: Each worker increments an error counter and continues on per-item failures. The job itself only returns an error on fatal setup failures (e.g., can't query the database for the initial ID list).
- **No `backfilled: true` metadata in agent_decisions**: The plan mentioned marking decisions with `backfilled: true`, but the existing `decision.CreateDecisionInput` struct has no metadata field. Since this is purely an analytics concern, we rely on comparing `agent_decisions.created_at` vs `listings.created_at` to distinguish live vs backfilled decisions if needed. Adding a metadata field would require a schema migration and is out of scope for this task.
- **No dedicated admin auth**: Phase 6 OpsAgent will introduce proper admin middleware. For now the endpoints use the same JWT auth as all other routes.
- **Rate limiting**: Only the appraisal backfill sleeps (1s per batch of 10) because it makes external AI API calls. Reputation and risk score backfills are database-only and do not need throttling.

## Verification

```
cd backend && go vet ./...              # no issues
cd backend && go build ./cmd/server    # clean build
cd backend && go test ./... -count=1   # all packages green
cd backend && go test ./internal/agent/backfill/... -v  # 2 tests PASS
```
