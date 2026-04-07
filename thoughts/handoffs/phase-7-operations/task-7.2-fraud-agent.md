# Task 7.2 — FraudAgent

**Branch:** task-7.2-fraud-agent  
**Commit:** 7d3bb31b2066b30ee966e5f8f03f8c6c7f5fcb62  
**Branch mode:** Graphite (gt)

## What was implemented

### New package: `backend/internal/agent/fraud/`

**model.go**
- `SignalType` constants: `DEVICE_FINGERPRINT`, `PAYMENT_INSTRUMENT`, `CARRIER_BATCH`, `SIMULTANEOUS_CREATION`, `EXCLUSIVE_PAIR`, `WIFI_NETWORK`, `DAMAGE_PATTERN`, `VALUE_SPIKE`
- `FraudSignal` — per-user detected signal with score, evidence JSONB, `IsCompoundOnly` flag
- `SignalBundle` — aggregates signals; `TotalScore()` applies WiFi compound-only logic (WiFi contributes only if `HasNonCompoundSignal == true`)
- `FraudFlag` — flag record with action (`MONITOR` / `FLAG` / `SUSPEND`), linked `AgentDecision`
- `actionFromScore()`: score < 80 → MONITOR, 80–99 → FLAG, ≥100 → SUSPEND
- `ScoreThreshold = 80`

**signals.go**
Five per-user signal detectors:
- `DetectSharedDeviceFingerprint` — matches `device_fingerprint` across users; +40
- `DetectLinkedPaymentInstrument` — matches `signup_metadata->>'payment_fingerprint'`; +50
- `DetectCarrierBatchPhone` — same phone prefix + sequential suffix within 48h batch; +30
- `DetectSimultaneousCreation` — accounts created within 300s of each other; +35
- `DetectWiFiNetwork` — matches `signup_metadata->>'wifi_bssid'`; +30, `IsCompoundOnly=true`
- `RunAllSignals()` — orchestrates all detectors and builds a `SignalBundle`

**patterns.go**
Four cross-transaction pattern detectors (run on schedule):
- `DetectExclusivePairs` — >3 shared transactions where >80% of each user's activity is with the other; +45
- `DetectDamageAmountGaming` — dispute `charge_amount` within 5% of `hold_amount` on >50% of disputes; +50
- `DetectSerialDamage` — listing where >60% of completed rentals have a dispute; +40
- `DetectNewAccountValueSpike` — account <30 days with >3 listings valued >$500; +35
- `RunPatternAnalysis()` — orchestrates all pattern detectors

**agent.go**
- `EvaluateTransaction(ctx, transactionID)` — evaluates renter and host signals, writes `AgentDecision`, creates `FraudFlag` when score ≥ threshold
- `EvaluateUser(ctx, userID, transactionID)` — single-user signal evaluation
- `RunScheduledScan(ctx)` — runs pattern analysis across all users; creates flags for scored detections

**repository.go**
- `GetTransaction` — fetches renter_id, host_id for a transaction
- `InsertFraudFlag` — inserts flag + atomically appends flag ID to `users.risk_flags` JSONB
- `ListFraudFlags` — paginated, filterable by status/action
- `GetFraudFlag` — single flag lookup
- `ResolveFraudFlag` — marks resolved, updates `outcome_correct` on linked decision
- `GetUserFraudHistory` — all flags for a user

**handler.go**
HTTP routes mounted at `/api/v1/ops/fraud/`:
- `GET /flags` — paginated list with `?status=OPEN&action=FLAG`
- `GET /flags/:flagId` — single flag with embedded signals
- `PUT /flags/:flagId/resolve` — body `{outcome, notes}`
- `GET /users/:userId/signals` — all signals and flags for a user

**jobs.go**
- `PatternScanArgs` / `PatternScanWorker` — River job calling `RunScheduledScan`

### New migration: `backend/migrations/014_fraud_agent.sql`
- `ALTER TABLE users ADD COLUMN signup_metadata JSONB DEFAULT '{}'` — for WiFi BSSID and payment fingerprint at registration
- `CREATE TABLE fraud_flags` — with indexes on user_id, action, created_at

### Modified: `backend/app/server.go`
- Import `fraud` package
- Wire `fraudRepo`, `fraudAgent`, `fraudWorker`
- Register `PatternScanWorker` with River
- Add 6-hour periodic job `fraud_pattern_scan`
- Mount `fraudHandler` at `/api/v1/ops/fraud`
- Attach fraud agent to booking service via `.WithFraudAgent(fraudAgent)`

### Modified: `backend/internal/booking/service.go`
- Added `fraudSvc` interface and `fraudSvc` field on `Service`
- Added `WithFraudAgent()` method
- Hook after `scheduleAutoDecline`: calls `fraudSvc.EvaluateTransaction()` — logged, never fatal

## Verification
- `go vet ./...` — clean
- `go build ./cmd/server` — clean
- `go test ./internal/agent/fraud/... -v` — 6 tests pass (WiFi compound-only, actionFromScore, isSequentialPhone)
- `go test ./... -count=1` — all packages pass including integration tests

## WiFi compound-only logic (key design decision)
WiFi-only detections score 0 and don't create flags. WiFi unlocks its +30 only when at least one non-compound signal exists in the same bundle. This prevents false positives from shared public WiFi at cafes, airports, etc. Only suspicious when combined with device fingerprint, payment instrument, or simultaneous creation evidence.

## Notes for next session
- Task 7.3 (Ops Dashboard web) and 7.4 (Referral system) are the remaining tasks
- Both 7.3 and 7.4 depend only on 7.0, which is completed — they can be done in either order
- The `signup_metadata` column added in this task is intentionally minimal. The mobile app (or user registration endpoint) would need to populate `wifi_bssid` and `payment_fingerprint` fields for the WiFi and payment signal detectors to produce results. In a live system, this would be provided at signup time.
