# Task 6.7 — Outcome Linking (Agent Learning Framework)

## Summary
Implemented the Agent Learning Framework's outcome linking and calibration system (PRD §31). The system evaluates whether past agent decisions were correct by examining real-world outcomes 48h after transaction close, and maintains per-agent confidence calibration metrics.

## What Was Built

### New Package: `backend/internal/outcome/`
- **model.go** — Domain types: `EvalContext`, `CalibrationBucket`, `CalibrationReport`, `DecisionWithOutcome`
- **service.go** — Core business logic:
  - `LinkOutcomes(ctx, transactionID)` — Evaluates all agent decisions for a transaction using agent-specific correctness rules
  - `UpdateCalibrationMetrics(ctx, agentType)` — Computes rolling 90-day accuracy per confidence bucket
  - `GetCalibration(ctx)` / `GetCalibrationForAgent(ctx, agentType)` — Admin API data
  - Per-agent evaluation methods for 7 agent types (Dispute, Risk, Appraisal, LateReturn, Agreement, Verification, Fraud)
- **repository.go** — Postgres queries + Redis storage for calibration buckets
- **jobs.go** — River jobs:
  - `OutcomeLinkJob` — Fires 48h after transaction close, runs `LinkOutcomes` + `UpdateCalibrationMetrics`
  - `MonthlyCalibrationReportJob` — Aggregates calibration data across all agent types
  - `ScheduleOutcomeLink()` — Helper to enqueue delayed outcome link jobs
- **handler.go** — Admin HTTP endpoints:
  - `GET /api/v1/admin/agents/calibration` — All agents' calibration data
  - `GET /api/v1/admin/agents/calibration/{agentType}` — Single agent calibration
  - `GET /api/v1/admin/agents/decisions` — Paginated decisions with outcome data (filterable)

### Modified Files
- **`backend/app/server.go`** — Registered outcome workers and mounted handler
- **`backend/internal/dispute/service.go`** — Schedules `OutcomeLinkJob` after dispute auto-resolution and human resolution
- **`backend/internal/booking/service.go`** — Schedules `OutcomeLinkJob` after checkout (COMPLETED)

## Agent-Specific Correctness Rules
| Agent | Rule |
|-------|------|
| DisputeAgent | Correct if NOT overridden by human reviewer |
| RiskAgent | Low-risk pass correct if no dispute; high-risk block correct if dispute occurred |
| AppraisalAgent | Correct if NOT overridden by host/admin |
| LateReturnAgent | Correct if escalation was warranted (not premature) |
| AgreementAgent | Correct if no dispute arose from agreement gap |
| VerificationAgent | Correct if verified user NOT later fraud-flagged |
| FraudAgent | Correct if flagged user confirmed fraudulent |

## Calibration System
- 5 confidence buckets: [0.5-0.6), [0.6-0.7), [0.7-0.8), [0.8-0.9), [0.9-1.0)
- Stored in Redis with 90-day TTL: `calibration:{agentType}:{bucket}`
- CalibrationError = |expectedAccuracy - actualAccuracy|
- Target: calibration error < 0.10 for all agents

## Tests
- **Unit tests** (6 tests): `backend/internal/outcome/service_test.go`
  - Calibration bucket computation, aggregation, job args, model fields
- **Integration tests** (10 tests): `backend/tests/integration/outcome_api_test.go`
  - HTTP API: calibration endpoint, auth, decisions filtering
  - Outcome linking: dispute not-overridden (correct), dispute overridden (incorrect)
  - Calibration update in Redis, risk low-pass no-dispute (correct), no-decisions no-op

## Dependencies Used
- No new dependencies added

## Branch Mode
- Graphite mode (`gt create`)

## Verification
All passing:
- `go vet ./...` ✓
- `go build ./cmd/server` ✓
- `go test ./... -v -count=1` ✓
- `go test ./tests/integration/... -v -count=1 -timeout 180s` ✓
