# Task 6.3 — LateReturnAgent Handoff

## Summary
Implemented the LateReturnAgent that autonomously monitors overdue rentals, charges late fees, and escalates when warranted. The agent uses a two-stage architecture: deterministic hourly charging via River jobs, plus LLM-based escalation decisions via Claude Sonnet.

## What Was Built

### New Package: `backend/internal/latereturn/`
- **model.go** — Domain types: `LateReturn`, `LateReturnInput`, `EscalationDecisionOutput`, status/escalation enums
- **repository.go** — Full CRUD: Insert, FindByID, FindByTransactionID, FindActive, UpdateStatus, RecordCharge, RecordEscalation, Resolve, plus evidence-gathering queries (GetTransactionDetails, GetRenterReputationScore, CountRecentMessages, HasConflictingBooking)
- **service.go** — Core business logic:
  - `CheckAndCharge()` — calculates late duration, computes hourly fee (doubled for conflicts), captures from hold respecting damage reserve cap, re-enqueues hourly
  - `EvaluateEscalation()` — gathers context, calls LLM, records AgentDecision, executes escalation
  - `maxLateFeeCap()` — enforces `holdAmount * (1 - damageReserveRate)` cap
  - `computeHourlyRate()` — derives hourly rate from rental fee and scheduled duration
- **jobs.go** — Two River workers:
  - `LateReturnCheckWorker` (kind: `late_return_check`) — fires at scheduled_end
  - `LateReturnEscalationWorker` (kind: `late_return_escalation`) — fires when past threshold
  - `ScheduleLateReturnCheck()` — standalone scheduling function used by booking.Accept
- **prompt.go** — v1 system prompt for escalation decisions
- **handler.go** — Admin HTTP endpoint: `GET /api/v1/admin/late-returns`

### Migration: `backend/migrations/012_late_returns.sql`
- `late_returns` table with indexes on transaction_id, status, renter_id

### Modified Files
- `backend/app/server.go` — Registered LateReturnAgent workers and mounted handler
- `backend/internal/platform/config/config.go` — Added `LATE_RETURN_ESCALATION_THRESHOLD_HOURS` (default 4) and `LATE_RETURN_RECHECK_MINUTES` (default 60)
- `backend/internal/booking/service.go` — Added `latereturn.ScheduleLateReturnCheck()` call in `Accept()` method

### Tests
- `service_test.go` — Table-driven tests for:
  - `computeHourlyRate()` — standard, minimum floor, zero/sub-hour duration
  - `maxLateFeeCap()` — various reserve rates, edge cases (0%, >100%)
  - `TestMaxLateFeeCap_DamageReservePreserved` — invariant: damage reserve always >= 40%
  - `statusFromEscalation()` — all escalation level mappings

## Architecture Notes

### Hold Allocation Coordination
LateReturnAgent and DisputeAgent share the same hold. LateReturnAgent is capped at 60% of the hold (configurable). DisputeAgent has no cap and takes whatever remains. This prevents late fees from consuming the entire hold.

### Agent Decision Audit Trail
Every escalation decision is recorded via `decision.RecordDecision()` with the `LATE_RETURN` agent type. This feeds into the Agent Learning Framework (task 6.7).

### Existing Patterns Used
- Task types already defined in `agent/router/model.go` (`TaskEscalationDecision`, `TaskLateFeeCalculation`)
- Agent type already defined in `agent/decision/model.go` (`AgentTypeLateReturn`)
- Pre-river service pattern (construct with nil deps, inject real deps after river starts)

## Dependencies
- Task 5.1 (test infrastructure) — completed
- Uses: payment.Service, agent/decision.Service, agent/router.AnthropicRouter

## Verification
- `go vet ./...` — passed
- `go build ./cmd/server` — passed
- `go test ./internal/latereturn/... -v -count=1` — all tests pass
- `go test ./internal/... -count=1` — all existing tests still pass

## Mode
Graphite mode — branch `task-6.3-late-return-agent`
