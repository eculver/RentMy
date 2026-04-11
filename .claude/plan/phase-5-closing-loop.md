# Phase 5 — Returns, Disputes, Trust / Closing the Loop Implementation Plan

> **Scope:** Wk 11-14. Returns verified via photo diff, damage detected, disputes resolved through escalation gate, reputation updated, payouts land. The platform can run autonomously.
> **Exit criteria:** CV+LLM photo diff detects damage, disputes resolve via hold allocation with damage reserve, reputation scores update, guarantee fund tracks reserve ratio, outcome linking feeds learning framework. Phase 6 is unblocked.
> **Blockers:** Phases 1-4 complete (Users, Listings, Bookings, Payments, Handoff, Messaging, AI Agents)

## Resolved Decisions

| Question | Answer | Notes |
|----------|--------|-------|
| CV preprocessing runtime | Python FastAPI sidecar (containerized) | Go calls via HTTP. Not gocv — avoids cgo/OpenCV build pain in Go |
| Segmentation model | SAM 2 (Segment Anything Model 2) | Best-in-class zero-shot segmentation. Runs in Python sidecar |
| Python base image | `python:3.11-slim` | Minimal footprint. Adds `opencv-python-headless`, `segment-anything-2`, `fastapi`, `uvicorn` |
| CV sidecar port | 8090 | Added to docker-compose as `cv-service` |
| Go CV client location | `internal/platform/cv/client.go` | Simple HTTP client, no SDK |
| Dispute evidence assembly | In-memory from existing records | No duplication — query agreement snapshot, photos, messages, proximity proofs at resolution time |
| Reputation storage | Materialized on `users.reputation_score` | Recalculated via River job on each transaction close |
| Rating bubble storage | JSONB array on `ratings.bubbles` | Already in schema from migration 001 |
| Photo diff result storage | On `transactions` table | `photo_diff_result` and `photo_diff_confidence` columns already exist |
| Hold allocation coordination | `SELECT ... FOR UPDATE` row-level locking | Postgres `READ COMMITTED` + explicit lock prevents LateReturnAgent/DisputeAgent race |
| Guarantee fund balance tracking | Double-entry ledger via `guarantee_fund_entries` | `balance_after` column provides running balance without aggregate queries |
| Outcome linking trigger | River job 48h after transaction close | Async — does not block transaction completion |
| Negative signal decay | 50% at 180 days via monthly River cron | Recomputes reputation with decay applied |

---

## Technology Decisions

### Go Backend (New Dependencies)

| Decision | Choice | Import / Module |
|----------|--------|-----------------|
| CV sidecar HTTP client | stdlib `net/http` | No new dependency — simple JSON POST/response |
| LLM model router client | Existing Anthropic SDK wrapper | Already in codebase from Phase 4 agent work |
| Scheduled jobs (late return, SLA, outcome) | River | `github.com/riverqueue/river` (already in go.mod) |
| Hold allocation locking | pgx transactions | `github.com/jackc/pgx/v5` (already in go.mod) |

**Rationale:**
- **stdlib net/http for CV client over a third-party HTTP client:** The sidecar API is three endpoints. No retries, circuit breakers, or connection pooling beyond what net/http provides. Adding a dependency for this is unjustified.
- **River for all scheduled work:** Late return checks, SLA monitoring, outcome linking, and reputation recalculation are all delayed/periodic jobs. River is already wired from Phase 0. No reason to introduce a second job system.

### Python CV Sidecar (New Service)

| Decision | Choice | Package |
|----------|--------|---------|
| Web framework | FastAPI | `fastapi`, `uvicorn[standard]` |
| Computer vision | OpenCV (headless) | `opencv-python-headless` |
| Object segmentation | SAM 2 | `segment-anything-2` |
| Image I/O | Pillow | `Pillow` |
| HTTP client (S3 fetch) | httpx | `httpx` |

**Rationale:**
- **FastAPI over Flask:** Async by default, auto-generated OpenAPI docs, Pydantic validation on request/response. Better fit for an image processing service with potentially long-running requests.
- **opencv-python-headless over opencv-python:** No GUI dependencies. Smaller image. The sidecar runs in a container — no display needed.
- **SAM 2 over SAM 1 / MobileSAM:** SAM 2 handles video and image segmentation with improved accuracy on object boundaries. The model is larger but runs in a dedicated container, so memory is isolated from the Go process.

### React Native (New Dependencies)

No new dependencies for Phase 5. Camera (react-native-vision-camera), gyroscope (expo-sensors), and push notifications (expo-notifications) are already installed from Phases 1 and 3.

---

## Implementation Steps

### Step 5.1 — Photo Diff Pipeline (backend + Python sidecar)

**Create:**
- `cv-service/` (repo root) — Python sidecar project directory
- `cv-service/Dockerfile`:
  ```dockerfile
  FROM python:3.11-slim
  RUN apt-get update && apt-get install -y --no-install-recommends libgl1 libglib2.0-0 && rm -rf /var/lib/apt/lists/*
  WORKDIR /app
  COPY requirements.txt .
  RUN pip install --no-cache-dir -r requirements.txt
  COPY . .
  CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8090"]
  ```
- `cv-service/requirements.txt` — `fastapi`, `uvicorn[standard]`, `opencv-python-headless`, `segment-anything-2`, `Pillow`, `httpx`, `numpy`
- `cv-service/main.py` — FastAPI app with three endpoints (see Python Sidecar API Contract below)
- `cv-service/pipeline/normalize.py` — Resize to 1024px longest side, OpenCV histogram equalization (CLAHE on LAB color space), white balance correction
- `cv-service/pipeline/segment.py` — SAM 2 model loading (lazy, singleton), `segment_item(image_bytes) -> (mask, cropped_rgba)`. Produces isolated item on transparent background
- `cv-service/pipeline/match_angles.py` — `match_pairs(checkin_orientations, checkout_orientations) -> list[(checkin_idx, checkout_idx)]`. Pairs photos by closest gyroscope orientation (Euler angle distance: `sqrt((r1-r2)^2 + (p1-p2)^2 + (y1-y2)^2)`)
- `cv-service/pipeline/quality.py` — Blur detection (Laplacian variance threshold), minimum resolution check (640px shortest side), item-in-frame check (SAM mask must cover >5% of image area)
- `cv-service/models/` — Directory for SAM 2 model weights (mounted via Docker volume or downloaded on first boot)
- `backend/internal/platform/cv/client.go` — Go HTTP client for cv-service:
  - `Preprocess(ctx, checkinImages, checkoutImages, orientations) -> PreprocessResult`
  - `CheckQuality(ctx, imageBytes) -> QualityResult`
  - Health check function
  - Configurable timeout (30s default for preprocessing, 5s for quality check)
- `backend/internal/photodiff/service.go` — PhotoDiffService orchestrating the two-stage pipeline:
  1. Fetch check-in and check-out media from S3 (by transaction ID)
  2. Call cv-service `/preprocess` with images + orientation metadata
  3. Receive paired, normalized, segmented crops
  4. Send paired crops to LLM (Claude via model router) with structural comparison prompt
  5. Parse LLM response: classification + confidence
  6. Store `photo_diff_result` and `photo_diff_confidence` on transaction
- `backend/internal/photodiff/model.go` — PhotoDiffResult enum, PreprocessRequest/Response types, LLMComparisonRequest
- `backend/internal/photodiff/prompt.go` — LLM prompt template for Stage 2 structural comparison. Versioned (stored as const with version string)
- `backend/internal/photodiff/handler.go` — HTTP handler:
  - `POST /api/v1/transactions/:id/photo-diff` (trigger diff manually, admin/debug)
  - `GET /api/v1/transactions/:id/photo-diff` (get diff result)

**Modify:**
- `docker-compose.yml` — Add `cv-service` container on port 8090, health check via `/health`, volume mount for SAM 2 model weights
- `backend/internal/platform/config/config.go` — Add `CVServiceURL` field (default `http://cv-service:8090`)
- `backend/cmd/server/main.go` — Wire PhotoDiffService, inject CV client

**Verify:**
```bash
# Build and start cv-service
docker compose build cv-service
docker compose up -d cv-service
curl -sf http://localhost:8090/health
# Should return {"status":"ok","model_loaded":true}

# Go tests
cd backend && go vet ./...
cd backend && go build ./cmd/server
cd backend && go test ./internal/photodiff/... -v -count=1
cd backend && go test ./internal/platform/cv/... -v -count=1

# Integration test (requires test images in S3):
curl -sf -X POST http://localhost:8080/api/v1/transactions/$TXN_ID/photo-diff \
  -H "Authorization: Bearer $ADMIN_TOKEN"
# Should return 200 with {result, confidence}
```

### Step 5.2 — DisputeAgent with Escalation Gate (backend)

**Create:**
- `backend/internal/dispute/model.go` — Dispute domain type, DisputeInput (reporter, reason, evidence references), DisputeDecision, EscalationRoute enum (`AUTO_RESOLVE`, `AUTO_RESOLVE_AUDIT`, `HUMAN_REVIEW`), HumanReviewItem
- `backend/internal/dispute/repository.go` — Insert dispute, FindByID, FindByTransactionID, FindPendingReview (paginated, ordered by SLA deadline), UpdateStatus, UpdateDecision
- `backend/internal/dispute/service.go` — DisputeService:
  - `FileDispute(ctx, input)` — Create dispute record, transition transaction to `DISPUTED`, trigger evidence gathering
  - `GatherEvidence(ctx, transactionID)` — Assemble in-memory: agreement snapshot (from transaction.agreement_snapshot), check-in/check-out media (from media table), messages (from messages table), proximity proofs (from proximity_proofs table), transaction data, photo diff result. No data duplication — all read from existing tables
  - `RunDisputeAgent(ctx, evidence)` — Call LLM with evidence package, receive decision + confidence
  - `RouteDecision(ctx, decision)` — Apply escalation gate (see routing table below), execute or queue
  - `ExecuteDecision(ctx, decision)` — Hold allocation capture, guarantee fund draw, hold release
  - `HandleInconclusive(ctx, transactionID)` — Prompt both parties for additional photos, schedule 2h River job for re-prompt expiry
- `backend/internal/dispute/escalation.go` — Escalation gate routing logic:
  ```go
  func RouteDecision(confidence float64, chargeAmount float64, photoDiffResult string, hasFraudFlags bool) EscalationRoute
  ```
  Implements the routing table (see Escalation Gate Routing Table below)
- `backend/internal/dispute/hold.go` — Hold allocation capture operations:
  - `CaptureForDamage(ctx, tx pgx.Tx, transactionID, amount)` — SELECT FOR UPDATE on transaction row, validate amount <= remaining, update hold_allocation JSONB, commit
  - `ReleaseRemaining(ctx, tx pgx.Tx, transactionID)` — Release unused hold back to renter via PaymentAdapter
  - `CaptureAndEscalate(ctx, tx pgx.Tx, transactionID, totalDamage)` — When damage > remaining hold: capture all remaining, charge card for difference, draw guarantee fund for any shortfall
- `backend/internal/dispute/handler.go` — HTTP handlers:
  - `POST /api/v1/transactions/:id/disputes` — File a dispute
  - `GET /api/v1/transactions/:id/disputes` — Get dispute(s) for a transaction
  - `GET /api/v1/disputes/:id` — Get dispute by ID
  - `GET /api/v1/admin/disputes/review-queue` — Human review queue (admin only, paginated, ordered by SLA deadline)
  - `POST /api/v1/admin/disputes/:id/resolve` — Human reviewer approves/overrides
  - `POST /api/v1/admin/disputes/:id/request-evidence` — Request more evidence from parties
- `backend/internal/dispute/prompt.go` — DisputeAgent LLM prompt template (versioned). Inputs: agreement terms, photo diff result + confidence, message summary, proximity data, transaction timeline, user reputation scores
- `backend/internal/dispute/jobs.go` — River jobs:
  - `DisputeResolutionJob` — Async dispute processing (evidence gather + agent + route + execute)
  - `RePromptExpiryJob` — Fires 2h after INCONCLUSIVE re-prompt. If new photos submitted: re-run diff. If not: escalate to human review with "re-prompt unanswered" note
  - `SLAMonitorJob` — River periodic job (every 15 min): check all pending disputes against SLA deadlines. At 80% SLA: warning alert. At 100% SLA: auto-escalate + CRITICAL alert

**Modify:**
- `backend/cmd/server/main.go` — Mount dispute router, register River jobs
- `backend/internal/platform/config/config.go` — Add `DisputeSLAActiveHours`, `DisputeSLAPostReturnHours`, `DamageReserveRate` fields

**Verify:**
```bash
cd backend && go vet ./...
cd backend && go build ./cmd/server
cd backend && go test ./internal/dispute/... -v -count=1

# Integration test — file dispute:
curl -sf -X POST http://localhost:8080/api/v1/transactions/$TXN_ID/disputes \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"reason":"Item returned damaged","description":"Screen has crack"}'
# Should return 201 with dispute object

# Verify escalation gate routing:
cd backend && go test ./internal/dispute/ -run TestEscalationGate -v
```

### Step 5.3 — LateReturnAgent (backend)

**Create:**
- `backend/internal/latereturn/model.go` — LateReturnInput, LateReturnDecision, EscalationLevel enum (`CHARGING`, `WARNING`, `ESCALATED_TO_DISPUTE`, `FLAGGED_FOR_REVIEW`)
- `backend/internal/latereturn/service.go` — LateReturnService:
  - `CheckAndCharge(ctx, transactionID)` — Core logic: verify rental still ACTIVE, calculate late duration, compute late fee (hourly rate, double if conflict), capture from hold (respecting damage reserve cap), log AgentDecision
  - `EvaluateEscalation(ctx, transactionID)` — LLM call with: duration overdue, renter message responsiveness, renter reputation, item value, time of day. Returns escalation level
  - `Escalate(ctx, transactionID)` — Hand off to DisputeAgent, capture remaining hold (minus damage reserve), notify host
  - Late fee cap enforcement: `maxLateFeeCapture = holdAmount * (1 - damageReserveRate)`. Cannot exceed `remaining` on hold_allocation. Cannot exceed `holdAmount * 0.6` (default 60%)
- `backend/internal/latereturn/jobs.go` — River jobs:
  - `LateReturnCheckJob` — Scheduled at `scheduled_end` for every ACTIVE rental. On fire: if rental still ACTIVE, run CheckAndCharge. Re-enqueue hourly for continued late charging
  - `LateReturnEscalationJob` — Fired after configurable threshold (default: 4 hours overdue). Runs EvaluateEscalation
- `backend/internal/latereturn/prompt.go` — LateReturnAgent LLM prompt for escalation decisions (versioned)
- `backend/internal/latereturn/handler.go` — HTTP handler:
  - `GET /api/v1/admin/late-returns` — List active late returns (admin only)

**Modify:**
- `backend/cmd/server/main.go` — Register late return River jobs
- `backend/internal/platform/config/config.go` — Add `LateReturnEscalationThresholdHours`, `DamageReserveRate` (shared with dispute)
- `backend/internal/booking/service.go` — On booking acceptance: schedule `LateReturnCheckJob` at `scheduled_end`

**Verify:**
```bash
cd backend && go vet ./...
cd backend && go build ./cmd/server
cd backend && go test ./internal/latereturn/... -v -count=1

# Test late fee cap enforcement:
cd backend && go test ./internal/latereturn/ -run TestLateFeeCap -v
# Verify: late fee capture never exceeds holdAmount * 0.6
# Verify: remaining hold after late fees >= holdAmount * 0.4 (damage reserve)
```

### Step 5.4 — Rating System (backend + RN)

**Create:**
- `backend/internal/rating/model.go` — Rating domain type, CreateRatingInput (transactionID, bubbles array), valid bubble constants:
  - Renter-rates-Host: `GOOD_COMMUNICATION`, `ON_TIME`, `ITEM_AS_DESCRIBED`, `EASY_PICKUP`, `FRIENDLY`
  - Host-rates-Renter: `GOOD_COMMUNICATION`, `ON_TIME_RETURN`, `CAREFUL_WITH_ITEM`, `EASY_HANDOFF`, `RESPECTFUL`
- `backend/internal/rating/repository.go` — Insert, FindByTransactionID, FindByToUserID (paginated), HasUserRated (check UNIQUE constraint before insert)
- `backend/internal/rating/service.go` — RatingService:
  - `SubmitRating(ctx, input)` — Validate transaction is COMPLETED, validate bubbles are from correct set (renter vs host), insert rating, enqueue reputation recalculation job for `to_user_id`
  - `GetRatingsForUser(ctx, userID, page)` — Paginated ratings received by a user
  - `GetRatingBubbleSummary(ctx, userID)` — Aggregate bubble counts for display (e.g., "Good communication (12)")
- `backend/internal/rating/handler.go` — HTTP handlers:
  - `POST /api/v1/transactions/:id/ratings` — Submit a rating
  - `GET /api/v1/transactions/:id/ratings` — Get ratings for a transaction
  - `GET /api/v1/users/:id/ratings` — Get ratings received by a user
  - `GET /api/v1/users/:id/ratings/summary` — Bubble count summary

**Modify:**
- `backend/cmd/server/main.go` — Mount rating router
- `mobile/app/(tabs)/(profile)/index.tsx` — Display rating bubbles on profile
- `mobile/components/rating/RatingBubbles.tsx` — Bubble selection component (tappable pills)
- `mobile/components/rating/RatingPrompt.tsx` — Post-rental rating prompt modal
- `mobile/lib/hooks/useRatings.ts` — TanStack Query hooks for ratings

**Verify:**
```bash
cd backend && go vet ./...
cd backend && go test ./internal/rating/... -v -count=1

# Integration test:
curl -sf -X POST http://localhost:8080/api/v1/transactions/$TXN_ID/ratings \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"bubbles":["GOOD_COMMUNICATION","ON_TIME","ITEM_AS_DESCRIBED"]}'
# Should return 201
# Duplicate attempt should return 409 (unique constraint)

cd mobile && npx tsc --noEmit
```

### Step 5.5 — Reputation Score Recalculation (backend)

**Create:**
- `backend/internal/reputation/model.go` — ReputationSignal types:
  - Positive signals: `COMPLETED_RENTAL` (+15), `POSITIVE_BUBBLE` (+5), `ON_TIME_RETURN` (+10), `ACCOUNT_AGE_30D` (+25), `ACCOUNT_AGE_90D` (+25), `ACCOUNT_AGE_365D` (+25), `KYC_VERIFIED` (+50), `MILESTONE_5` (+50), `MILESTONE_15` (+50), `MILESTONE_50` (+50)
  - Negative signals: `DISPUTE_FILED` (-30), `DISPUTE_LOST` (-50), `CANCELLATION` (-20), `LATE_RETURN` (-15), `FRAUD_FLAG` (-100)
  - Host-specific signals: `HIGH_RESPONSE_RATE` (+25), `HIGH_ACCEPTANCE_RATE` (+25), `ZERO_CANCELLATIONS_90D` (+25), `LOW_RESPONSE_RATE` (-40), `LOW_ACCEPTANCE_RATE` (-40)
- `backend/internal/reputation/service.go` — ReputationService:
  - `Recalculate(ctx, userID)` — Full recalculation from source data:
    1. Count completed rentals (no dispute) -> positive signals
    2. Count positive bubbles received -> positive signals
    3. Count on-time returns -> positive signals
    4. Check account age milestones -> one-time bonuses
    5. Check KYC status -> one-time bonus
    6. Check rental count milestones (5, 15, 50) -> one-time bonuses
    7. Count disputes filed against, disputes lost -> negative signals
    8. Count cancellations as cancelling party -> negative signals
    9. Count late returns -> negative signals
    10. Check fraud flags -> negative signals
    11. Apply 180-day decay: negative signals older than 180 days reduced by 50%
    12. Sum all signals, clamp to [0, 1000]
    13. UPDATE users SET reputation_score = $1 WHERE id = $2
  - `RecalculateHostSignals(ctx, userID)` — Monthly recalculation of host-specific signals (response rate, acceptance rate, cancellation history over rolling windows)
- `backend/internal/reputation/jobs.go` — River jobs:
  - `ReputationRecalcJob` — Triggered by rating submission, dispute resolution, transaction completion. Runs `Recalculate` for the affected user
  - `MonthlyHostReputationJob` — River periodic job (1st of each month). Iterates all users with `host_id` in transactions table, runs `RecalculateHostSignals`
  - `NegativeDecayJob` — River periodic job (daily). Finds negative signals older than 180 days, applies 50% decay, triggers recalculation for affected users

**Modify:**
- `backend/cmd/server/main.go` — Register reputation River jobs
- `backend/internal/rating/service.go` — After rating submission: enqueue `ReputationRecalcJob`
- `backend/internal/dispute/service.go` — After dispute resolution: enqueue `ReputationRecalcJob` for both parties

**Verify:**
```bash
cd backend && go vet ./...
cd backend && go test ./internal/reputation/... -v -count=1

# Test cases:
# 1. New user: reputation = 0
# 2. After 5 clean rentals + KYC: reputation = 5*(15+10) + 50 + 50 = 225
# 3. After dispute lost: reputation -= 50
# 4. After 180 days: dispute penalty decays to -25
# 5. Score never goes below 0 or above 1000
```

### Step 5.6 — Guarantee Fund Accounting (backend)

**Create:**
- `backend/internal/guaranteefund/model.go` — GuaranteeFundEntry domain type, EntryType enum (`CONTRIBUTION`, `CLAIM`, `CARD_RECOVERY`, `COLLECTIONS_REFERRAL`), FundHealth struct (balance, outstandingGaps, reserveRatio, lossRatio)
- `backend/internal/guaranteefund/repository.go` — InsertEntry, GetCurrentBalance (SELECT balance_after FROM guarantee_fund_entries ORDER BY created_at DESC LIMIT 1), GetOutstandingGaps (SUM of guarantee_gap on ACTIVE transactions), GetRolling90DayClaims, GetRolling90DayContributions
- `backend/internal/guaranteefund/service.go` — GuaranteeFundService:
  - `Contribute(ctx, transactionID, amount)` — Insert CONTRIBUTION entry, compute balance_after from previous entry's balance_after + amount. Called by PaymentService on each booking confirmation (guaranteeRate % of platform fee)
  - `Claim(ctx, transactionID, amount)` — Validate fund balance >= amount (fund cannot go negative). Insert CLAIM entry. If amount > balance: disburse available, charge renter's card for remainder, if card fails: insert COLLECTIONS_REFERRAL
  - `RecordCardRecovery(ctx, transactionID, amount)` — Insert CARD_RECOVERY entry (positive, restores balance)
  - `RecordCollectionsReferral(ctx, transactionID, amount)` — Insert COLLECTIONS_REFERRAL entry (tracks amount sent to collections)
  - `GetFundHealth(ctx)` — Calculate current balance, outstanding gaps, reserve ratio, loss ratio
  - `CheckReserveRatio(ctx)` — Enforce reserve ratio thresholds (see table below). Returns action needed (NORMAL, ALERT, RESTRICT_HIGH_VALUE, RESTRICT_ALL_GAP)
- `backend/internal/guaranteefund/jobs.go` — River jobs:
  - `FundHealthCheckJob` — River periodic job (every hour). Runs `CheckReserveRatio`, fires OpsAgent alerts at thresholds
  - `LossRatioCheckJob` — River periodic job (daily). Calculates rolling 90-day loss ratio. If > 0.6 for 30+ consecutive days: fire OpsAgent alert with recommendations
- `backend/internal/guaranteefund/handler.go` — HTTP handlers:
  - `GET /api/v1/admin/guarantee-fund/health` — Fund health dashboard data (admin only)
  - `GET /api/v1/admin/guarantee-fund/entries` — Paginated ledger entries (admin only)

**Modify:**
- `backend/cmd/server/main.go` — Wire guarantee fund service, register River jobs
- `backend/internal/platform/config/config.go` — Add `GuaranteeRate` (default 0.10), `ReserveRatioThresholds` fields
- `backend/internal/dispute/service.go` — On damage exceeding hold: call `GuaranteeFundService.Claim`
- `backend/internal/payment/service.go` — On booking confirmation: call `GuaranteeFundService.Contribute`

**Verify:**
```bash
cd backend && go vet ./...
cd backend && go test ./internal/guaranteefund/... -v -count=1

# Test double-entry integrity:
cd backend && go test ./internal/guaranteefund/ -run TestDoubleEntry -v
# Verify: every entry's balance_after = previous balance_after + amount (contribution) or - amount (claim)
# Verify: fund balance never goes negative
# Verify: reserve ratio alerts fire at correct thresholds
```

### Step 5.7 — Outcome Linking (Agent Learning Framework)

**Create:**
- `backend/internal/outcome/model.go` — OutcomeLinkInput (transactionID, agentDecisionID), OutcomeRule per agent type (maps to PRD 31 table)
- `backend/internal/outcome/service.go` — OutcomeService:
  - `LinkOutcomes(ctx, transactionID)` — For each AgentDecision on the transaction:
    1. Query all AgentDecisions WHERE transaction_id = $1
    2. For each decision, evaluate correctness per agent-specific rules:
       - `DisputeAgent`: correct if not overridden by human reviewer
       - `RiskAgent`: correct if high-risk block was warranted (similar users/items had incidents) or low-risk pass had no incident
       - `AppraisalAgent`: correct if estimated value within 30% of actual damage claim (if any), no host override
       - `LateReturnAgent`: correct if escalation was warranted (renter genuinely non-responsive) vs premature
       - `AgreementAgent`: correct if no dispute arose from agreement gap
       - `VerificationAgent`: correct if verified user not later fraud-flagged
       - `FraudAgent`: correct if flagged accounts confirmed fraudulent
    3. SET outcome_correct on each AgentDecision
  - `UpdateCalibrationMetrics(ctx, agentType)` — Calculate per-confidence-bucket accuracy (rolling 90 days):
    - Buckets: 0.5-0.6, 0.6-0.7, 0.7-0.8, 0.8-0.9, 0.9-1.0
    - For each bucket: expectedAccuracy = midpoint, actualAccuracy = count(correct) / count(all), calibrationError = |expected - actual|
    - Store in Redis as `calibration:{agentType}:{bucket}` with 90-day TTL
- `backend/internal/outcome/jobs.go` — River jobs:
  - `OutcomeLinkJob` — Fires 48h after transaction close (status = COMPLETED or dispute resolved). Runs `LinkOutcomes` + `UpdateCalibrationMetrics` for each agent type encountered
  - `MonthlyCalibrationReportJob` — River periodic job (1st of each month). Aggregates calibration data across all agent types, generates report for OpsAgent dashboard
- `backend/internal/outcome/handler.go` — HTTP handlers:
  - `GET /api/v1/admin/agents/calibration` — Per-agent calibration metrics (admin only)
  - `GET /api/v1/admin/agents/calibration/:agentType` — Detailed calibration for a specific agent
  - `GET /api/v1/admin/agents/decisions` — Paginated agent decisions with outcome data (admin only, filterable by agent type, outcome_correct)

**Modify:**
- `backend/cmd/server/main.go` — Register outcome River jobs, mount handlers
- `backend/internal/dispute/service.go` — After dispute resolution: schedule `OutcomeLinkJob` (48h delay)
- `backend/internal/booking/service.go` — After transaction completion: schedule `OutcomeLinkJob` (48h delay)

**Verify:**
```bash
cd backend && go vet ./...
cd backend && go test ./internal/outcome/... -v -count=1

# Test outcome correctness rules:
cd backend && go test ./internal/outcome/ -run TestDisputeOutcome -v
# Verify: non-overridden dispute decision -> outcomeCorrect = true
# Verify: human-overridden decision -> outcomeCorrect = false
# Verify: calibration metrics update correctly per bucket
```

### Step 5.8 — Post-Rental Flow (RN)

**Create:**
- `mobile/app/(tabs)/(rentals)/return-confirmation.tsx` — Return confirmation screen: shows transaction summary, both parties' check-out photos, photo diff status (pending/complete), hold release status
- `mobile/app/(tabs)/(rentals)/rate.tsx` — Rating prompt screen: displays counterparty name/avatar, bubble selection grid (tappable pills), submit button. Different bubble sets for host vs renter
- `mobile/app/(tabs)/(rentals)/dispute.tsx` — Dispute filing screen: reason selector (damage, missing item, other), description text input, photo evidence reference (links to existing check-in/out photos), submit button
- `mobile/app/(tabs)/(rentals)/dispute-status.tsx` — Dispute tracking screen: status timeline (filed -> evidence gathered -> under review -> resolved), decision display, hold charge/release details, re-prompt photo upload if INCONCLUSIVE
- `mobile/components/rental/HoldStatusCard.tsx` — Hold allocation visualization: bar chart showing authorized / captured-for-late / captured-for-damage / damage-reserve / released / remaining
- `mobile/components/rental/PhotoDiffResult.tsx` — Photo diff result display: side-by-side check-in/out paired photos, classification badge, confidence indicator
- `mobile/components/rental/DisputeTimeline.tsx` — Vertical timeline component for dispute status progression
- `mobile/lib/hooks/useDispute.ts` — TanStack Query hooks for dispute CRUD + status polling
- `mobile/lib/hooks/usePostRental.ts` — Hook orchestrating post-rental flow: detect transaction completion -> show rating prompt -> show hold release status

**Modify:**
- `mobile/app/(tabs)/(rentals)/index.tsx` — Add "Return" action button on active rentals, post-rental state showing rate prompt + hold status
- `mobile/lib/notifications.ts` — Handle post-rental push notification types: `RATING_PROMPT`, `HOLD_RELEASED`, `DAMAGE_DETECTED`, `DISPUTE_FILED`, `DISPUTE_RESOLVED`, `PHOTO_REPROMPT`

**Verify:**
```bash
cd mobile && npx tsc --noEmit
# Manual verification: complete a rental -> verify rating prompt appears -> submit rating -> verify hold release notification
```

---

## Python Sidecar API Contract

### `cv-service` Endpoints

**`GET /health`**
```json
Response 200:
{
  "status": "ok",
  "model_loaded": true,
  "model_name": "sam2_hiera_small",
  "version": "1.0.0"
}
```

**`POST /preprocess`**

Accepts check-in and check-out image sets with orientation metadata. Returns paired, normalized, segmented item crops.

```json
Request:
{
  "checkin_images": [
    {
      "url": "https://s3.../media-originals/01HXYZ...",
      "orientation": {"roll": 15.2, "pitch": 45.0, "yaw": 90.3}
    }
  ],
  "checkout_images": [
    {
      "url": "https://s3.../media-originals/01HABC...",
      "orientation": {"roll": 18.1, "pitch": 42.5, "yaw": 88.7}
    }
  ]
}

Response 200:
{
  "pairs": [
    {
      "checkin_index": 0,
      "checkout_index": 0,
      "angle_distance": 4.87,
      "checkin_crop": "<base64-encoded PNG, transparent background>",
      "checkout_crop": "<base64-encoded PNG, transparent background>"
    }
  ],
  "unpaired_checkin": [],
  "unpaired_checkout": [],
  "processing_time_ms": 2340
}

Response 422:
{
  "error": "insufficient_images",
  "detail": "Minimum 3 check-in images required"
}
```

**`POST /quality-check`**

Validates a single image against quality gate criteria.

```json
Request:
{
  "image_url": "https://s3.../media-originals/01HXYZ...",
  "check_type": "checkin"
}

Response 200:
{
  "passed": true,
  "checks": {
    "blur_score": 145.3,
    "blur_passed": true,
    "resolution": {"width": 3024, "height": 4032},
    "resolution_passed": true,
    "item_detected": true,
    "item_coverage_pct": 34.2
  }
}

Response 200 (failed):
{
  "passed": false,
  "checks": {
    "blur_score": 22.1,
    "blur_passed": false,
    "resolution": {"width": 640, "height": 480},
    "resolution_passed": true,
    "item_detected": true,
    "item_coverage_pct": 12.8
  },
  "rejection_reasons": ["Image is too blurry (score 22.1, threshold 100.0)"]
}
```

---

## Escalation Gate Routing Table

| # | Confidence | Charge Amount | Photo Diff | Fraud Flags | Route | Action |
|---|------------|---------------|------------|-------------|-------|--------|
| 1 | >= 0.85 | <= $200 | Any except INCONCLUSIVE | None | `AUTO_RESOLVE` | Execute immediately. No human review |
| 2 | >= 0.85 | $201 - $1,000 | Any except INCONCLUSIVE | None | `AUTO_RESOLVE_AUDIT` | Execute immediately. Flag for async human audit (post-hoc review) |
| 3 | >= 0.85 | > $1,000 | Any except INCONCLUSIVE | None | `HUMAN_REVIEW` | Queue for human review. Do not execute until approved |
| 4 | < 0.85 | Any | Any | Any | `HUMAN_REVIEW` | Queue for human review. Do not execute until approved |
| 5 | Any | Any | INCONCLUSIVE | Any | `HUMAN_REVIEW` | Queue for human review. Do not execute until approved |
| 6 | Any | Any | Any | Active | `HUMAN_REVIEW` | Queue for human review. Do not execute until approved |

**Priority:** Rules are evaluated top-to-bottom. Rule 6 (fraud flags) overrides all others. Rule 5 (INCONCLUSIVE) overrides rules 1-3. Rule 4 (low confidence) overrides rules 1-3.

**Implementation:**
```go
func RouteDecision(confidence float64, chargeAmount float64, photoDiffResult string, hasFraudFlags bool) EscalationRoute {
    if hasFraudFlags {
        return HumanReview
    }
    if photoDiffResult == "INCONCLUSIVE" {
        return HumanReview
    }
    if confidence < 0.85 {
        return HumanReview
    }
    if chargeAmount > 1000 {
        return HumanReview
    }
    if chargeAmount > 200 {
        return AutoResolveAudit
    }
    return AutoResolve
}
```

---

## Hold Allocation Coordination Pattern

Both LateReturnAgent and DisputeAgent draw from the same hold. Coordination uses `SELECT ... FOR UPDATE` to prevent race conditions.

**Pattern (used by both agents):**

```go
func CaptureFromHold(ctx context.Context, pool *pgxpool.Pool, txnID string, captureType string, amount float64) error {
    tx, _ := pool.Begin(ctx)
    defer tx.Rollback(ctx)

    // 1. Lock the transaction row
    var allocation HoldAllocation
    err := tx.QueryRow(ctx,
        `SELECT hold_allocation FROM transactions WHERE id = $1 FOR UPDATE`,
        txnID,
    ).Scan(&allocation)

    // 2. Validate capture
    if amount > allocation.Remaining {
        return ErrInsufficientHold
    }

    // 3. For late fees: enforce damage reserve cap
    if captureType == "LATE_FEE" {
        maxLateCapture := allocation.TotalAuthorized * (1 - damageReserveRate)
        alreadyCapturedForLate := allocation.CapturedForLateFees
        if alreadyCapturedForLate + amount > maxLateCapture {
            amount = maxLateCapture - alreadyCapturedForLate
        }
        allocation.CapturedForLateFees += amount
    } else {
        allocation.CapturedForDamage += amount
    }

    // 4. Update remaining
    allocation.Remaining -= amount

    // 5. Persist atomically
    _, err = tx.Exec(ctx,
        `UPDATE transactions SET hold_allocation = $1 WHERE id = $2`,
        allocation, txnID,
    )

    // 6. Execute capture via PaymentAdapter
    _, err = paymentAdapter.CaptureHold(holdID, amount)

    return tx.Commit(ctx)
}
```

**Damage reserve enforcement:** When LateReturnAgent captures, the maximum it can take is `holdAmount * (1 - damageReserveRate)` (default 60%). The remaining 40% is reserved for DisputeAgent. DisputeAgent has no such cap -- it can capture whatever remains.

**Overflow handling (damage > remaining hold):**
1. Capture all `remaining` from hold via PaymentAdapter
2. Calculate difference: `damage - remaining`
3. Attempt to charge renter's card for difference via `PaymentAdapter.ChargeRentalFee`
4. If card charge fails: draw from guarantee fund via `GuaranteeFundService.Claim`
5. If guarantee fund insufficient: disburse available, refer remainder to collections

---

## Reputation Signal Tables

### Positive Signals

| Signal | Points | Frequency | Trigger |
|--------|--------|-----------|---------|
| Completed rental (no dispute) | +15 | Per event | Transaction status -> COMPLETED |
| Positive rating bubble received | +5 | Per bubble | Rating submitted |
| On-time return (within 15 min) | +10 | Per event | actual_end <= scheduled_end + 15min |
| Account age > 30 days | +25 | One-time | Daily decay job checks |
| Account age > 90 days | +25 | One-time (stacks) | Daily decay job checks |
| Account age > 365 days | +25 | One-time (stacks) | Daily decay job checks |
| KYC verified | +50 | One-time | identity_status -> VERIFIED |
| 5+ completed rentals, no disputes | +50 | Milestone, one-time | Recalculation checks count |
| 15+ completed rentals, no disputes | +50 | Milestone, one-time | Recalculation checks count |
| 50+ completed rentals, no disputes | +50 | Milestone, one-time | Recalculation checks count |

### Negative Signals

| Signal | Points | Frequency | Decay | Trigger |
|--------|--------|-----------|-------|---------|
| Dispute filed against user | -30 | Per event | 50% at 180 days | Dispute created |
| Dispute lost (decided against) | -50 | Per event | 50% at 180 days | Dispute resolved against user |
| Cancellation (as cancelling party) | -20 | Per event | 50% at 180 days | Transaction cancelled_by = user |
| Late return | -15 | Per event | 50% at 180 days | actual_end > scheduled_end + 15min |
| Fraud flag | -100 | Per event | 50% at 180 days (reset on repeat) | Fraud flag set by FraudAgent |

### Host-Specific Signals (Monthly Recalculation)

| Signal | Points | Condition |
|--------|--------|-----------|
| High response rate | +25 | Response rate > 90% over last 30 days |
| High acceptance rate | +25 | Acceptance rate > 80% over last 30 days |
| Zero host cancellations | +25 | No host-initiated cancellations in 90 days |
| Low response rate | -40 | Response rate < 50% over last 30 days |
| Low acceptance rate | -40 | Acceptance rate < 30% over last 30 days |

### Score Bounds

- **Floor:** 0 (never negative)
- **Ceiling:** 1000
- **New user:** 0
- **Meaningful differentiation:** 5 clean rentals ~ 175 points. 50 clean rentals ~ 750+ points.

---

## Photo Diff Pipeline Architecture

```
Stage 1: CV Preprocessing (Python Sidecar - port 8090)
=======================================================

  Check-in Photos (S3)     Check-out Photos (S3)
        |                         |
        v                         v
  +-----------+             +-----------+
  | Normalize |             | Normalize |
  | - Resize  |             | - Resize  |
  | - CLAHE   |             | - CLAHE   |
  +-----------+             +-----------+
        |                         |
        v                         v
  +-----------+             +-----------+
  | SAM 2     |             | SAM 2     |
  | Segment   |             | Segment   |
  | - Isolate |             | - Isolate |
  | - Crop    |             | - Crop    |
  +-----------+             +-----------+
        |                         |
        +--------- Pair ----------+
        |   (by gyroscope angle   |
        |    closest match)       |
        v                         v
  +----------------------------------+
  | Paired Crops (transparent bg)    |
  | [{checkin_crop, checkout_crop}]  |
  +----------------------------------+
                  |
                  | HTTP Response to Go
                  v

Stage 2: LLM Reasoning (Go Backend - Claude via Model Router)
==============================================================

  +----------------------------------+
  | Paired Crops (from Stage 1)      |
  +----------------------------------+
                  |
                  v
  +----------------------------------+
  | LLM Prompt:                      |
  | - System: "You are a damage      |
  |   assessment expert..."          |
  | - Images: paired crops           |
  | - Task: structural comparison    |
  | - Output: classification +       |
  |   confidence + reasoning         |
  +----------------------------------+
                  |
                  v
  +----------------------------------+
  | Response:                        |
  | - classification: NO_CHANGE |    |
  |   COSMETIC | FUNCTIONAL |        |
  |   MISSING | INCONCLUSIVE         |
  | - confidence: 0.0 - 1.0         |
  | - reasoning: "..."              |
  +----------------------------------+
                  |
                  v
  +----------------------------------+
  | Store on Transaction:            |
  | photo_diff_result                |
  | photo_diff_confidence            |
  +----------------------------------+
                  |
           confidence < 0.7
          or INCONCLUSIVE?
         /              \
       Yes               No
        |                 |
        v                 v
  Re-prompt both      Continue to
  parties (2h)       DisputeAgent
```

---

## API Endpoints

| Method | Path | Auth | Request Body | Response | Errors |
|--------|------|------|-------------|----------|--------|
| POST | `/api/v1/transactions/:id/photo-diff` | Admin | -- | `{result, confidence, pairs}` | 404 transaction, 400 insufficient photos |
| GET | `/api/v1/transactions/:id/photo-diff` | Yes | -- | `{result, confidence}` | 404 |
| POST | `/api/v1/transactions/:id/disputes` | Yes | `{reason, description}` | `{dispute}` | 400 validation, 404 transaction, 409 already disputed |
| GET | `/api/v1/transactions/:id/disputes` | Yes | -- | `{disputes[]}` | 404 transaction |
| GET | `/api/v1/disputes/:id` | Yes | -- | `{dispute, decision?, evidence_summary}` | 404 |
| GET | `/api/v1/admin/disputes/review-queue` | Admin | `?page=1&limit=20` | `{disputes[], total, page}` | -- |
| POST | `/api/v1/admin/disputes/:id/resolve` | Admin | `{action, override_reason?, charge_amount?}` | `{dispute, agent_decision}` | 404, 400 invalid action |
| POST | `/api/v1/admin/disputes/:id/request-evidence` | Admin | `{message}` | `{dispute}` | 404 |
| GET | `/api/v1/admin/late-returns` | Admin | `?page=1&limit=20` | `{late_returns[], total}` | -- |
| POST | `/api/v1/transactions/:id/ratings` | Yes | `{bubbles[]}` | `{rating}` | 400 invalid bubbles, 404 transaction, 409 already rated |
| GET | `/api/v1/transactions/:id/ratings` | Yes | -- | `{ratings[]}` | 404 |
| GET | `/api/v1/users/:id/ratings` | Yes | `?page=1&limit=20` | `{ratings[], total, page}` | 404 user |
| GET | `/api/v1/users/:id/ratings/summary` | Yes | -- | `{bubbles: {name: count}}` | 404 user |
| GET | `/api/v1/admin/guarantee-fund/health` | Admin | -- | `{balance, outstanding_gaps, reserve_ratio, loss_ratio}` | -- |
| GET | `/api/v1/admin/guarantee-fund/entries` | Admin | `?page=1&limit=50` | `{entries[], total, page}` | -- |
| GET | `/api/v1/admin/agents/calibration` | Admin | -- | `{agents: [{type, buckets, calibration_error}]}` | -- |
| GET | `/api/v1/admin/agents/calibration/:agentType` | Admin | -- | `{agent_type, buckets[], decisions_count, accuracy}` | 404 invalid type |
| GET | `/api/v1/admin/agents/decisions` | Admin | `?agent_type=&outcome=&page=1&limit=20` | `{decisions[], total, page}` | -- |

---

## Database Migrations

### Migration 006 — Dispute and Reputation Support

```sql
-- disputes table (new)
CREATE TABLE disputes (
    id                      TEXT PRIMARY KEY,            -- ULID
    transaction_id          TEXT NOT NULL REFERENCES transactions(id),
    reporter_id             TEXT NOT NULL REFERENCES users(id),
    reason                  TEXT NOT NULL
                            CHECK (reason IN ('DAMAGE', 'MISSING_ITEM', 'WRONG_ITEM', 'LATE_RETURN', 'OTHER')),
    description             TEXT NOT NULL DEFAULT '',
    status                  TEXT NOT NULL DEFAULT 'FILED'
                            CHECK (status IN ('FILED', 'EVIDENCE_GATHERED', 'UNDER_REVIEW', 'AWAITING_PHOTOS', 'RESOLVED', 'ESCALATED')),
    escalation_route        TEXT
                            CHECK (escalation_route IN ('AUTO_RESOLVE', 'AUTO_RESOLVE_AUDIT', 'HUMAN_REVIEW')),
    decision_summary        TEXT,
    charge_amount           NUMERIC(10,2),
    resolved_at             TIMESTAMPTZ,
    resolved_by             TEXT,                        -- 'AGENT' or user ID of human reviewer
    sla_deadline            TIMESTAMPTZ,
    reprompt_sent_at        TIMESTAMPTZ,                 -- when INCONCLUSIVE re-prompt was sent
    reprompt_expires_at     TIMESTAMPTZ,                 -- 2h after reprompt_sent_at
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_disputes_transaction_id ON disputes(transaction_id);
CREATE INDEX idx_disputes_status ON disputes(status);
CREATE INDEX idx_disputes_sla_deadline ON disputes(sla_deadline) WHERE status IN ('FILED', 'EVIDENCE_GATHERED', 'UNDER_REVIEW', 'ESCALATED');

-- reputation_events table (new) — audit trail of all reputation changes
CREATE TABLE reputation_events (
    id                      TEXT PRIMARY KEY,            -- ULID
    user_id                 TEXT NOT NULL REFERENCES users(id),
    signal_type             TEXT NOT NULL,               -- e.g., 'COMPLETED_RENTAL', 'DISPUTE_LOST', 'MILESTONE_5'
    points                  INTEGER NOT NULL,            -- positive or negative
    source_transaction_id   TEXT REFERENCES transactions(id),
    source_rating_id        TEXT REFERENCES ratings(id),
    decayed                 BOOLEAN NOT NULL DEFAULT FALSE,
    decayed_at              TIMESTAMPTZ,
    original_points         INTEGER,                     -- points before decay
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_reputation_events_user_id ON reputation_events(user_id);
CREATE INDEX idx_reputation_events_decay ON reputation_events(decayed, created_at) WHERE decayed = FALSE;

-- late_return_charges table (new) — tracks individual late fee captures
CREATE TABLE late_return_charges (
    id                      TEXT PRIMARY KEY,            -- ULID
    transaction_id          TEXT NOT NULL REFERENCES transactions(id),
    amount                  NUMERIC(10,2) NOT NULL,
    rate_multiplier         REAL NOT NULL DEFAULT 1.0,   -- 1.0 = normal, 2.0 = conflict
    period_start            TIMESTAMPTZ NOT NULL,
    period_end              TIMESTAMPTZ NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_late_return_charges_transaction_id ON late_return_charges(transaction_id);

-- Add photo reprompt tracking columns to transactions
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS photo_reprompt_sent_at TIMESTAMPTZ;
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS photo_reprompt_checkin_submitted BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS photo_reprompt_checkout_submitted BOOLEAN NOT NULL DEFAULT FALSE;
```

---

## Reserve Ratio Enforcement Table

| Fund Health | Reserve Ratio | Action |
|-------------|---------------|--------|
| Healthy | >= 15% of outstanding gaps | Normal operation |
| Warning | 10-15% of outstanding gaps | OpsAgent alert. Increase `guaranteeRate` by 5% |
| Restricted (high-value) | 5-10% of outstanding gaps | Restrict new listings with `estimated_value` > $2,000 |
| Restricted (all gap) | < 5% of outstanding gaps | Restrict all new bookings where `guarantee_gap` > $500 until fund recovers |
| Critical (absolute) | Balance < $100 | CRITICAL alert. Restrict all gap bookings regardless of ratio |

---

## Risks and Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| SAM 2 model size (~300MB for small variant) | Slow container start, large image | Use `sam2_hiera_small` (smallest variant). Download model weights on first boot, cache via Docker volume. Lazy-load in sidecar (health check reports `model_loaded` status) |
| Python sidecar reliability (OOM, crash) | Photo diff pipeline blocked | Docker restart policy `unless-stopped`. Go client has 30s timeout with retry (1 retry). Health check endpoint polled by docker-compose. Degrade gracefully: if sidecar unavailable, mark diff as INCONCLUSIVE and route to human review |
| CV preprocessing accuracy on varied lighting/backgrounds | False positives/negatives in damage detection | Stage 1 normalization (CLAHE histogram equalization) handles lighting variance. SAM 2 handles background isolation. Golden test set (see Testing Strategy) validates accuracy floor. LLM in Stage 2 provides reasoning layer that catches CV artifacts |
| LLM hallucination on damage assessment | Incorrect damage classification | Confidence scoring gates auto-resolution. Low-confidence decisions always routed to human review. Prompt includes explicit instruction to return INCONCLUSIVE rather than guess. Outcome linking tracks accuracy over time |
| Hold allocation race condition (LateReturnAgent + DisputeAgent) | Double-capture, over-charge renter | SELECT FOR UPDATE row-level locking. All captures within single pgx transaction. Damage reserve cap prevents late fees from consuming full hold |
| Guarantee fund depletion | Cannot cover damage claims | Reserve ratio monitoring with four-tier alert system. Fund cannot go negative — shortfall charged to renter card, then collections. CRITICAL alert at $100 absolute balance |
| SLA breach on human review queue | Unresolved disputes, poor user experience | River cron job every 15 min checks SLA deadlines. Warning at 80%, auto-escalate at 100%. SLA compliance rate tracked on ops dashboard |
| Negative reputation gaming (create disputes to tank competitor) | Unfair reputation damage | Dispute filed = -30 to responder, but dispute lost = -50 to filer. Filing frivolous disputes is net-negative. FraudAgent monitors dispute patterns |
| SAM 2 dependency stability | Model API changes break sidecar | Pin exact SAM 2 version in requirements.txt. Integration tests in CI validate model output format. Abstraction layer in segment.py allows swapping models |
| Photo re-prompt non-response | Disputes stuck in AWAITING_PHOTOS | 2-hour expiry via River job. Auto-escalate to human review with "re-prompt unanswered" note. Never blocks indefinitely |

---

## Testing Strategy

### Photo Diff Pipeline — Golden Test Set

Maintain a golden test set of 20+ image pairs in `backend/testdata/photodiff/`:
- `no_change/` — 5+ pairs with identical items (different lighting, angles)
- `cosmetic_damage/` — 5+ pairs with scratches, scuffs, minor dents
- `functional_damage/` — 5+ pairs with broken parts, missing components
- `missing_item/` — 3+ pairs where item is absent in return photos
- `inconclusive/` — 3+ pairs with poor quality, heavy lighting differences

**Accuracy targets:**
- NO_CHANGE: >= 90% correct classification
- COSMETIC_DAMAGE: >= 80% correct classification
- FUNCTIONAL_DAMAGE: >= 85% correct classification
- MISSING_ITEM: >= 95% correct classification
- Overall confidence calibration error: < 0.10

**CV sidecar unit tests:**
- Normalization: input image with known histogram -> output histogram within expected range
- Segmentation: input image with known object -> mask covers > 80% of object area
- Angle matching: known orientation pairs -> correct pairing
- Quality gate: blurry image -> rejected, clear image -> passed

### Dispute Resolution Test Cases

| Test Case | Input | Expected Route | Expected Action |
|-----------|-------|----------------|-----------------|
| Low-value, high-confidence damage | confidence=0.92, charge=$150, no fraud | AUTO_RESOLVE | Capture $150 from hold |
| Mid-value, high-confidence damage | confidence=0.88, charge=$500, no fraud | AUTO_RESOLVE_AUDIT | Capture $500, flag for audit |
| High-value, high-confidence damage | confidence=0.90, charge=$1,500, no fraud | HUMAN_REVIEW | Queue, do not capture |
| Low-confidence, any value | confidence=0.70, charge=$100, no fraud | HUMAN_REVIEW | Queue, do not capture |
| INCONCLUSIVE photo diff | confidence=any, diff=INCONCLUSIVE | HUMAN_REVIEW | Queue, trigger re-prompt |
| Fraud-flagged party | confidence=0.95, charge=$50, fraud=true | HUMAN_REVIEW | Queue, do not capture |
| Damage exceeds hold | confidence=0.90, charge=$800, remaining=$300 | AUTO_RESOLVE_AUDIT | Capture $300 hold + charge $500 card |
| Damage exceeds hold + card fails | confidence=0.90, charge=$800, remaining=$300, card_fail=true | AUTO_RESOLVE_AUDIT | Capture $300 + guarantee fund $500 |

### Reputation Calculation Tests

| Scenario | Expected Score |
|----------|---------------|
| New user, just registered | 0 |
| KYC verified only | 50 |
| 1 completed rental, on time, 3 positive bubbles, KYC | 50 + 15 + 10 + 15 = 90 |
| 5 completed rentals, all on time, KYC, 15 bubbles | 50 + 75 + 50 + 75 + 50 = 300 |
| 5 clean rentals + 1 dispute lost | 300 - 50 = 250 |
| Same as above, 180 days later | 300 - 25 = 275 (decay applied) |
| Score cannot exceed 1000 | Verify clamping |
| Score cannot go below 0 | Verify floor |

### Hold Allocation Tests

- Late fee captures respect 60% cap (40% damage reserve protected)
- Concurrent capture attempts: only one succeeds (SELECT FOR UPDATE)
- Damage capture after late fees: can access remaining + damage reserve
- Overflow: capture remaining -> charge card -> guarantee fund
- Guarantee fund: balance_after always consistent, never negative

### Guarantee Fund Tests

- Contribution on booking: balance increases correctly
- Claim: balance decreases, balance_after accurate
- Claim > balance: partial disburse, card charge, collections referral
- Reserve ratio: correct calculation against outstanding gaps
- Loss ratio: rolling 90-day window calculated correctly
- Alert thresholds: fire at 15%, 10%, 5%, $100

### Integration Tests

- **Full loop:** Create rental -> complete -> rate -> verify reputation update -> verify hold release
- **Dispute loop:** Create rental -> return with damage -> photo diff -> dispute filed -> agent decides -> escalation gate routes -> hold captured -> guarantee fund claimed (if needed) -> reputation updated
- **Late return loop:** Create rental -> scheduled_end passes -> LateReturnAgent charges -> damage reserve preserved -> return -> DisputeAgent has funds available
- **Outcome linking:** Complete transaction -> wait 48h (simulate via time override) -> verify AgentDecisions get outcomeCorrect flags

### RN Tests

- TypeScript compilation passes (`npx tsc --noEmit`)
- Rating bubble selection and submission flow
- Dispute filing form validation
- Dispute status timeline rendering
- Hold status card visualization

---

## Implementation Order

| Step | What | Day | Depends On |
|------|------|-----|------------|
| 5.1 | Photo diff pipeline (Python sidecar + Go client + service) | Day 1-4 | Phase 4 complete (media, transactions in place) |
| 5.2 | DisputeAgent with escalation gate | Day 3-6 | 5.1 (needs photo diff results) |
| 5.3 | LateReturnAgent | Day 4-6 | 5.2 (escalation hands off to DisputeAgent) |
| 5.4 | Rating system (backend + RN) | Day 5-7 | Phase 4 complete (independent of 5.1-5.3) |
| 5.5 | Reputation score recalculation | Day 6-8 | 5.4 (needs ratings as input), 5.2 (needs dispute outcomes) |
| 5.6 | Guarantee fund accounting | Day 7-9 | 5.2 (DisputeAgent draws from fund) |
| 5.7 | Outcome linking | Day 8-10 | 5.2, 5.3, 5.5 (needs agent decisions + outcomes to link) |
| 5.8 | Post-rental flow (RN) | Day 7-10 | 5.2, 5.4 (needs dispute + rating APIs) |

Steps 5.4 and 5.1 are independent and can start in parallel (ratings vs photo diff).
Steps 5.8 and 5.7 are independent of each other and can be parallelized (RN vs backend).
The critical path is: 5.1 -> 5.2 -> 5.3 -> 5.5 -> 5.7.
