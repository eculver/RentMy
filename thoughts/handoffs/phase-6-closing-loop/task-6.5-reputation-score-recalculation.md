# Task 6.5 Handoff — Reputation Score Recalculation

**Status:** Completed  
**Commit:** 30a9259  
**Branch:** task-6.5-reputation-score-recalculation  
**Date:** 2026-04-06

---

## What Was Built

Created `backend/internal/reputation/` — a new package that provides authoritative,
source-based reputation score recalculation (PRD §8).

### New Package: `backend/internal/reputation/`

| File | Purpose |
|------|---------|
| `model.go` | Signal point constants, `userStats` internal type |
| `repository.go` | Source-table queries (transactions, ratings, disputes, users) |
| `service.go` | `Recalculate`, `RecalculateHostSignals`, `EnqueueRecalc`; pure `computeScore` |
| `jobs.go` | `ReputationRecalcWorker`, `MonthlyHostReputationWorker`, `NegativeDecayWorker` |
| `service_test.go` | 10 unit tests covering scoring math, decay, clamping, milestones |

### Key Design Decision: Source-Based vs Signal-Based

The existing `risk` package maintains an incremental signal log (`reputation_signals` table)
and recomputes from stored signals. Task 6.5 adds a **source-based** computation path that
queries raw source tables (transactions, ratings, disputes, users) directly. This is:

- **Idempotent**: safe to run at any time without double-counting
- **Authoritative**: reflects actual DB state, not dependent on signals being emitted
- **Complementary**: the risk package's incremental approach remains for real-time updates;
  the reputation package corrects via async jobs

The two paths write to the same `users.reputation_score` field. The source-based recalc
is considered more authoritative since it can catch events that signals missed.

### Algorithm (PRD §8 faithfully implemented)

Positive (never decay):
- +15 per completed rental with no dispute filed against the user
- +5 per bubble received across all ratings  
- +10 per on-time return (actual_end ≤ scheduled_end + 15 min)
- +25/+25/+25 account age milestones at 30/90/365 days (one-time each)
- +50 KYC verified (one-time)
- +50/+50/+50 rental count milestones at 5/15/50 clean rentals (one-time each)

Negative (decay 50% after 180 days):
- -30 per dispute filed against user (user is non-reporter)
- -50 per dispute lost (user is renter, charge_amount > 0)
- -20 per cancellation as cancelling party
- -15 per late return (as renter)
- -100 per fraud flag (no decay — cleared by ops)

Host-specific (monthly):
- +25 acceptance rate ≥ 80%
- -40 acceptance rate < 30%
- +25 zero host-initiated cancellations in 90 days

Score clamped to [0, 1000].

### River Jobs

| Kind | Trigger | Action |
|------|---------|--------|
| `reputation_recalc` | Event (rating, dispute resolution) | `Recalculate(ctx, userID)` |
| `reputation_monthly_host` | Monthly schedule | Iterates all hosts → `RecalculateHostSignals` |
| `reputation_negative_decay` | Daily schedule | Finds users with 180-day-old events → `Recalculate` |

### Service Modifications

**`rating/service.go`**:
- Added `reputationEnqueuer` interface field (optional dependency)
- `WithReputation(r)` functional setter
- After `repo.Insert` succeeds: enqueues `ReputationRecalcJob` for the rated user
- Existing `riskAgent.EmitSignal` calls retained (maintain incremental signal log)

**`dispute/service.go`**:
- Added `reputationEnqueuer` interface field (optional dependency)
- `WithReputation(r)` functional setter
- `executeDecision`: enqueues `ReputationRecalcJob` for both renter and host after resolution
- Helper `enqueueReputationRecalc(ctx, transactionID)` for charge=0 path (fetches parties)
- Helper `enqueueReputationRecalcForParties(ctx, renterID, hostID)` shared by both paths

**`app/server.go`**:
- Added `reputation` import
- Reputation service created pre-river (no riverClient needed for workers themselves)
- Three new workers registered
- After river starts: full service rebuilt with riverClient, pointer-updated into workers
- `disputeSvc.WithReputation(reputationSvc)` and `ratingSvc.WithReputation(reputationSvc)`

---

## Verification Results

```
go build ./...        ✅ no errors
go vet ./...          ✅ no issues
go test ./internal/reputation/... -v   ✅ 10/10 tests pass
go test ./... -count=1                 ✅ all packages pass (including integration)
```

---

## Branching Mode

Used Graphite (`gt create`) — Graphite mode for this session.

---

## Dependencies

- Depends on task 6.4 (rating system) — rating service existed to inject into
- Depends on task 5.1 (test infrastructure) — integration tests continued to pass

---

## What Next Sessions Should Know

1. The `reputation_signals` table (risk package) and `users.reputation_score` (reputation package) 
   coexist. Both write `reputation_score`. The source-based path is authoritative.

2. The `MonthlyHostReputationJobArgs` kind is `"reputation_monthly_host"` (NEW).
   The old risk package's `"risk_monthly_reputation"` job still exists and is registered.
   Both will run; the source-based job's result overwrites. If deduplication is desired
   in a future task, remove the risk package's monthly worker registration from `app/server.go`.

3. Same situation with decay: `"reputation_negative_decay"` (new) and `"risk_decay_check"` (old).
   Both registered, both harmless to have running.

4. Fraud flags are read from `users.risk_flags` (JSONB array). Count of array elements
   × -100. No decay. Cleared by ops via direct DB update (no admin endpoint yet).
