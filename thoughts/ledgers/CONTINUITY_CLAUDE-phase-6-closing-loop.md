# Phase 6 — Returns, Disputes, Trust — Progress Ledger

## Status: Complete

## Tasks

| Task | Name | Status | Branch | Commit |
|------|------|--------|--------|--------|
| 6.1 | Photo diff pipeline | Completed | task-6.1-photo-diff-pipeline | 0c5d226 |
| 6.2 | DisputeAgent with escalation gate | Completed | task-6.2-dispute-agent | 35a58c6 |
| 6.3 | LateReturnAgent | Completed | task-6.3-late-return-agent | 6d867cd |
| 6.4 | Rating system (backend + RN) | Completed | task-6.4-rating-system | 356ec39 |
| 6.5 | Reputation score recalculation | Completed | task-6.5-reputation-score-recalculation | 30a9259 |
| 6.6 | Guarantee fund accounting | Completed | task-6.6-guarantee-fund-accounting | d9589b0 |
| 6.7 | Outcome linking (Agent Learning Framework) | Completed | task-6.7-outcome-linking | 9bc1807 |
| 6.8 | Post-rental flow (RN) | Completed | task-6.8-post-rental-flow | 31e3fbb |
| 6.9 | Guarantee fund refinement | Completed | task-6.9-guarantee-fund-refinement | 93c629b |

## Notes

### Task 6.1 — Photo Diff Pipeline
- Two-stage pipeline: Python CV sidecar (Stage 1) + LLM via model router (Stage 2)
- cv-service runs in SAM 2 passthrough mode until model weights are provisioned
- Graceful degradation built in: cv-service failure → raw images, model router unavailable → INCONCLUSIVE
- All existing tests continue to pass

### Task 6.2 — DisputeAgent with Escalation Gate
- Full dispute service with escalation gate routing (PRD §20 routing table)
- Evidence gathering from existing tables (media, messages, proximity, photo diff, agreements)
- LLM-based damage assessment via model router (TaskEvidenceAnalysis)
- Hold capture via payment service, overflow handling (card + guarantee fund)
- Human review workflow with SLA monitoring (River periodic job)
- Added `UpdateTransactionStatus`, `ChargeForDamageOverflow`, `ClaimGuaranteeFund` to payment service
- 14 unit tests for escalation gate, 6 integration tests for API endpoints
- All existing tests continue to pass

### Task 6.3 — LateReturnAgent
- Two-stage architecture: deterministic hourly charging + LLM-based escalation
- Damage reserve cap enforced: late fees capped at 60% of hold (configurable), 40% reserved for DisputeAgent
- River jobs: LateReturnCheckJob (fires at scheduled_end, re-enqueues hourly), LateReturnEscalationJob (fires at 4h+ threshold)
- Double rate for conflicting bookings per PRD §19
- ScheduleLateReturnCheck() integrated into booking.Accept()
- Conservative escalation: LLM defaults to WARNING on failure, FLAGGED_FOR_REVIEW is rare
- 4 test groups covering hourly rate calculation, fee cap enforcement, damage reserve invariant, status mapping
- All existing tests continue to pass

### Task 6.4 — Rating System (backend + RN)
- Full vertical slice: `backend/internal/rating/` package + mobile components
- Bubble constants for renter-rates-host (5 tags) and host-rates-renter (5 tags); GOOD_COMMUNICATION shared
- Service validates COMPLETED status, role-correct bubble set, UNIQUE constraint
- Reputation integrated via `riskSvc.EmitSignal(positive_rating)` — one signal per bubble, synchronous, non-fatal on failure
- Four HTTP endpoints: POST/GET ratings per transaction, GET ratings per user (paginated), GET bubble summary
- Mobile: `useRatings.ts` hooks, `RatingBubbles` (tappable pills + read-only), `RatingPrompt` (post-rental modal)
- Profile screen shows received bubble summary in read-only pill format
- 8 backend unit tests, 7 integration tests, 7 mobile component tests (66 total mobile)
- All existing tests continue to pass

### Task 6.5 — Reputation Score Recalculation
- New `backend/internal/reputation/` package — source-based, idempotent score computation
- Three River jobs: `reputation_recalc` (event-driven), `reputation_monthly_host`, `reputation_negative_decay`
- `computeScore` reads raw tables (transactions, ratings, disputes, users) — never double-counts
- 180-day decay applied per-event for disputes, cancellations, late returns; fraud flags never decay
- `rating.Service` and `dispute.Service` both enqueue `ReputationRecalcJob` after relevant events
- Two parallel scoring paths exist: risk package (incremental signal log) + reputation package (source truth)
- 10 unit tests for scoring math, decay, milestones, clamping; all integration tests continue to pass

### Task 6.6 — Guarantee Fund Accounting
- New `backend/internal/guaranteefund/` package — dedicated fund ledger, monitoring, and admin API
- Service methods: `Contribute`, `Claim` (caps at available balance), `RecordCardRecovery`, `RecordCollectionsReferral`, `GetFundHealth`, `CheckReserveRatio`
- Four-tier reserve ratio thresholds from PRD §7: NORMAL (≥15%), ALERT (10-15%), RESTRICT_HIGH_VALUE (5-10%), RESTRICT_ALL_GAP (<5%)
- Rolling 90-day loss ratio tracking (claims / contributions), target < 0.6
- River jobs: `FundHealthCheckWorker` (hourly), `LossRatioCheckWorker` (daily)
- Admin endpoints: `GET /api/v1/admin/guarantee-fund/health`, `GET /api/v1/admin/guarantee-fund/entries`
- Config fields added: `ReserveRatioNormal`, `ReserveRatioAlert`, `ReserveRatioRestrictHigh`, `LossRatioTarget`
- 20 unit tests for threshold logic; all existing tests continue to pass
- No migration needed — uses existing `guarantee_fund_entries` table from 001_initial_schema.sql

### Task 6.7 — Outcome Linking (Agent Learning Framework)
- New `backend/internal/outcome/` package — PRD §31 decision→outcome→calibration pipeline
- `LinkOutcomes` evaluates all agent decisions per transaction using agent-specific correctness rules
- 7 agent-specific rules: DisputeAgent (not overridden), RiskAgent (assessment matched outcome), AppraisalAgent (not overridden), LateReturnAgent (escalation warranted), AgreementAgent (no gap dispute), VerificationAgent (not fraud-flagged), FraudAgent (confirmed fraudulent)
- Calibration: 5 confidence buckets, rolling 90-day window, stored in Redis with TTL
- River jobs: `OutcomeLinkJob` (48h delay), `MonthlyCalibrationReportJob`
- Dispute and booking services schedule outcome linking after resolution/checkout
- Admin endpoints: calibration overview, per-agent calibration, paginated decisions with outcome data
- 6 unit tests + 10 integration tests; all existing tests continue to pass
- No migration needed — uses existing `outcome_id`/`outcome_correct` columns from migration 006

### Task 6.9 — Guarantee Fund Refinement (refines 6.6)
- Eliminated duplicate guarantee fund types/methods from `payment` package — now delegates to `guaranteefund.Service`
- `payment.Service` accepts `*guaranteefund.Service` as constructor dependency
- Wired River periodic jobs: `FundHealthCheckJob` (hourly), `LossRatioCheckJob` (daily) via `river.Config.PeriodicJobs`
- Enhanced `Claim()` to return `ClaimResult{Requested, Claimed, Shortfall}` struct
- `dispute/hold.go:CaptureAndEscalate` now records `CollectionsReferral` for partial claims
- Added `RepositoryInterface` to enable mock-based testing
- 11 new unit tests (contribute, claim full/partial/empty, fund health, double-entry integrity)
- 2 new integration tests (admin endpoints, ledger integrity)
- All existing tests continue to pass
