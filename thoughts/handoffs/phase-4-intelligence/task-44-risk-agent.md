# Task 4.4 — RiskAgent Handoff

## Summary

Implemented the dual-score system from PRD §8: per-transaction risk scoring (0–100) and user reputation management (0–1000). The risk engine is fully deterministic (rules-based, no LLM calls for v1). Risk scores gate `CreateBooking` — CRITICAL scores (≥71) block the booking before payment is attempted.

## Branching Mode

Git fallback (vanilla git). Branch: `task-4.4-risk-agent`. Commit: `e613b60`.

## What Was Built

### `backend/migrations/009_reputation_signals.sql`

Two new tables:
- `reputation_signals` — event log for reputation changes. `idempotency_key` unique index prevents double-counting one-time signals (KYC, milestones, account age).
- `risk_scores` — stores the computed per-transaction risk score and breakdown JSONB.

### `backend/internal/agent/risk/`

| File | Purpose |
|------|---------|
| `model.go` | `SignalType`, `RiskLevel`, `RiskControl`, `ReputationSignal`, `RiskBreakdown`, `TransactionRiskScore`, `riskScoreInput`, constants and tables for points/one-time/negative signal sets |
| `repository.go` | `InsertSignal`, `FindSignalsByUserID`, `FindUserProfile`, `SetReputationScore`, `FindTransactionDetails`, `FindBehavioralHistory`, `UpsertRiskScore`, `FindRiskScore`, `FindAllHostIDs`, `FindHostMetrics`, `FindUsersWithNegativeSignalsOlderThan` |
| `service.go` | `ComputeRiskScore`, `GetRiskScore`, `EmitSignal`, `RecalculateReputation`, `GetReputationSignals`, `RecalculateHostSignals`, `computeScore` (deterministic rules engine) |
| `handler.go` | `GET /api/v1/users/:id/reputation`, `GET /api/v1/transactions/:id/risk` |
| `jobs.go` | `MonthlyReputationWorker` (host signals recalc), `DecayCheckWorker` (180-day decay) |
| `service_test.go` | 10 tests covering all rule bands, clamp boundaries, signal tables, decay math |

### Risk Rules (from PRD §8)

```
Base risk:     PENDING identity(+20), <7d account(+15), rep<100(+15), rep 100-300(+5)
Transaction:   value >$3k(+25), value >$1k(+15), night 00:00-05:00(+10)
Counterparty:  both rep<50(+30), host age <14d(+10)
Behavioral:    2+ cancels 60d(+20), 1+ disputes 60d(+25), geo mismatch(+10)
Fraud:         device fingerprint match(+50), compound network(+30), velocity(+20)

Controls: 0-30 → APPROVE, 31-70 → REVIEW, 71+ → BLOCK
```

### Changes to Existing Files

**`backend/internal/booking/service.go`**
- Added `riskSvc *risk.Service` field (nil-safe)
- Added `WithRiskAgent(r *risk.Service)` builder method
- `CreateBooking` now calls `ComputeRiskScore` after fraud velocity check; returns `ErrFraudBlocked` if control is BLOCK

**`backend/cmd/server/main.go`**
- Imports `backend/internal/agent/risk`
- Builds `riskRepo`, `riskSvc`
- Registers `monthlyReputationWorker` and `decayCheckWorker` with River
- `bookingSvc.WithPusher(...).WithRiskAgent(riskSvc)`
- Mounts `riskHandler` on `/api/v1`

## Design Decisions

- **No LLM for v1 scoring** — fully deterministic rules engine, 100% auditable. The `risk/v1.txt` prompt (from 4.1) is available for future Haiku-assisted scoring when needed.
- **Nil-safe riskSvc in booking** — when risk service is unavailable (e.g. missing pool), booking continues with a warning log. This prevents CI failures in dev environments.
- **`SetReputationScore` vs `AddReputationScore`** — risk service computes the full score from all signals and sets it directly (avoids drift). The existing `AddReputationScore` (user package) is preserved for the VerificationAgent's one-time KYC bonus (pre-signal-system award).
- **No admin recalculation endpoint** — the plan mentioned `POST /api/v1/admin/reputation/:userId/recalculate` but there is no admin auth middleware yet (Phase 6). Skipped to avoid dead code; can be added in Phase 6 when OpsAgent and the ops dashboard are built.
- **River cron jobs are registered but not scheduled** — River's periodic job feature requires the `PeriodicJobs` config field. Monthly/weekly scheduling will be wired in Phase 6 when the full ops scheduling framework is set up. Workers are registered and functional for manual enqueue.

## Reputation Signal Idempotency

One-time signals (KYC, milestones, account age) use an `idempotency_key = signal_type` with a UNIQUE index. `InsertSignal` uses `ON CONFLICT DO NOTHING` so re-emitting is safe.

## Verification

All passed:
```
cd backend && go vet ./...              # no issues
cd backend && go build ./cmd/server    # clean build
cd backend && go test ./...            # all packages green
cd backend && go test ./internal/agent/risk/ -v -count=1  # 10 tests PASS
```
