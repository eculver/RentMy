# Phase 4 — AI Agents / Intelligence Layer Implementation Plan

> **Scope:** Wk 9-12. AI takes over verification, appraisal, risk scoring, and agreement generation. Listings get smart. Bookings get scored.
> **Exit criteria:** AI fills listings, dual-score system active (reputation 0-1000 + per-transaction risk 0-100), generates agreements, verifies identity. Existing data backfilled. Phase 5 unblocked.
> **Blockers:** Phase 3 complete (core transaction loop, payments, proximity, messaging working)

## Resolved Decisions

| Question | Answer | Notes |
|----------|--------|-------|
| LLM provider | Anthropic (Claude) | Single vendor for both tiers (Sonnet + Haiku) via same SDK |
| Cheap model | Anthropic Haiku | Same SDK, same auth — no second provider to manage |
| LLM client SDK | anthropic-sdk-go | Official Go SDK, supports messages API, vision, streaming |
| KYC provider | Stripe Identity | Same vendor as payments — single Stripe relationship, shared webhook infra |
| Prompt storage | Git-versioned files | `backend/prompts/{agent_name}/v{N}.txt` — diffs reviewable in PRs |
| Prompt template engine | Go `text/template` | stdlib, no dependency. Sufficient for variable interpolation |
| Agent module layout | `internal/agent/{name}/` | Follows modular monolith pattern: handler.go, service.go, model.go per agent |
| Model router pattern | Interface + functional options | `Router` interface with `Route(ctx, task, input) -> (response, metadata)` |
| Risk score computation | Deterministic rules engine | No ML in v1. Weighted inputs, explicit thresholds, fully auditable |
| Agreement base template | Versioned JSON in repo | `backend/prompts/agreement/base_template_v{N}.json` — immutable per transaction |
| Backfill strategy | River batch jobs | Separate jobs for listings, users, transactions. Marked `backfilled: true` |
| Cost tracking | Per-call logging to agent_decisions | Model, tokens (input+output), latency stored on every decision record |

---

## Technology Decisions

### Go Backend (New Dependencies)

| Decision | Choice | Import / Module |
|----------|--------|-----------------|
| LLM client | Anthropic Go SDK | `github.com/anthropics/anthropic-sdk-go` |
| KYC / Identity | Stripe Identity (via Stripe Go) | `github.com/stripe/stripe-go/v81` (already in go.mod from Phase 2) |
| Prompt templates | Go stdlib | `text/template` |
| Base64 encoding (images) | Go stdlib | `encoding/base64` |
| JSON schema validation | Go stdlib | `encoding/json` |

**Rationale:**
- **anthropic-sdk-go over generic OpenAI-compatible client:** Type-safe, supports vision (base64 image blocks), tool use, and streaming natively. No adapter layer needed. Haiku and Sonnet accessed via the same client — just change the model string.
- **Stripe Identity over Onfido/Jumio:** Already integrated with Stripe for payments (Phase 2). Single vendor relationship, shared webhook infrastructure, unified dashboard. Stripe Identity uses the same `stripe-go` SDK — no new dependency.
- **text/template over pongo2/jet:** Prompts need variable interpolation, not full template logic. `text/template` is stdlib, zero-dependency, and the team already knows it. Prompts should contain reasoning, not control flow.

### React Native (New Dependencies)

| Decision | Choice | Package |
|----------|--------|---------|
| Stripe Identity SDK | @stripe/stripe-identity-react-native | `@stripe/stripe-identity-react-native` |

No other new RN dependencies — AI autofill and risk display use existing TanStack Query hooks and UI primitives from Phase 1.

---

## Implementation Steps

### Step 4.1 — Model Router (backend)

**Create:**
- `backend/internal/agent/router/model.go` — Core types:
  ```go
  // ModelTier determines which model handles a task.
  type ModelTier int

  const (
      TierNone  ModelTier = iota // deterministic, no LLM
      TierCheap                  // Haiku — fast, low-cost
      TierFull                   // Sonnet — complex reasoning
  )

  // AgentTask identifies a specific agent operation for routing.
  type AgentTask string

  const (
      // AppraisalAgent tasks
      TaskItemIdentification       AgentTask = "appraisal.item_identification"
      TaskTagGeneration            AgentTask = "appraisal.tag_generation"
      TaskValueOverrideReview      AgentTask = "appraisal.value_override_review"

      // DisputeAgent tasks
      TaskEvidenceAnalysis         AgentTask = "dispute.evidence_analysis"
      TaskEvidenceSummary          AgentTask = "dispute.evidence_summary"

      // RiskAgent tasks
      TaskRiskScoring              AgentTask = "risk.scoring"

      // VerificationAgent tasks
      TaskKYCInterpretation        AgentTask = "verification.kyc_interpretation"

      // AgreementAgent tasks
      TaskCustomClauseGeneration   AgentTask = "agreement.custom_clause_generation"
      TaskTemplateRendering        AgentTask = "agreement.template_rendering"

      // LateReturnAgent tasks
      TaskEscalationDecision       AgentTask = "late_return.escalation_decision"
      TaskLateFeeCalculation       AgentTask = "late_return.fee_calculation"

      // FraudAgent tasks
      TaskPatternDetection         AgentTask = "fraud.pattern_detection"
      TaskSignalAggregation        AgentTask = "fraud.signal_aggregation"

      // OpsAgent tasks
      TaskAnomalyDetection         AgentTask = "ops.anomaly_detection"
      TaskHealthReport             AgentTask = "ops.health_report"

      // NotificationService tasks
      TaskNotificationText         AgentTask = "notification.text_generation"

      // DiscoveryService tasks
      TaskSemanticSearch           AgentTask = "discovery.semantic_search"
  )

  // RouteInput is the payload sent to the model.
  type RouteInput struct {
      Task          AgentTask
      SystemPrompt  string
      UserPrompt    string
      Images        [][]byte // base64-encoded images for vision tasks
      MaxTokens     int
  }

  // RouteOutput is the model response with metadata.
  type RouteOutput struct {
      Content       string
      Model         string        // actual model used (e.g., "claude-sonnet-4-20250514")
      PromptVersion string        // e.g., "v3"
      InputTokens   int
      OutputTokens  int
      Latency       time.Duration
      Cached        bool          // whether prompt caching was used
  }
  ```

- `backend/internal/agent/router/router.go` — Router interface and default implementation:
  ```go
  // Router dispatches agent tasks to the appropriate model tier.
  type Router interface {
      Route(ctx context.Context, input RouteInput, opts ...Option) (RouteOutput, error)
  }

  // Option configures a single Route call.
  type Option func(*routeConfig)

  func WithMaxRetries(n int) Option { ... }
  func WithTimeout(d time.Duration) Option { ... }
  func WithFallbackTier(t ModelTier) Option { ... }
  ```

  Default implementation:
  - Looks up `AgentTask -> ModelTier` from the task-to-tier matrix (hardcoded map, no config file)
  - `TierNone` tasks return immediately with empty RouteOutput (caller handles deterministic logic)
  - `TierCheap` tasks call Haiku via `anthropic-sdk-go`
  - `TierFull` tasks call Sonnet via `anthropic-sdk-go`
  - Retry logic: exponential backoff, max 3 retries, jitter
  - Timeout: 30s default for Haiku, 120s for Sonnet (vision tasks can be slow)
  - Fallback: if Sonnet unavailable after retries, enqueue a River job for deferred retry — never fail silently, never downgrade to Haiku for money-touching tasks
  - Cost tracking: log model, input/output tokens, latency on every call

- `backend/internal/agent/router/tier_matrix.go` — Complete task-to-tier mapping:

  | Agent | Task | Model Tier |
  |-------|------|------------|
  | AppraisalAgent | Item identification from photos | `TierFull` (Sonnet) |
  | AppraisalAgent | Tag generation | `TierCheap` (Haiku) |
  | AppraisalAgent | Value override justification review | `TierFull` (Sonnet) |
  | DisputeAgent | Evidence analysis + decision | `TierFull` (Sonnet) |
  | DisputeAgent | Evidence summary for human review | `TierCheap` (Haiku) |
  | RiskAgent | Per-transaction risk scoring | `TierCheap` (Haiku, rule-based with ML signal) |
  | VerificationAgent | KYC result interpretation | `TierCheap` (Haiku) |
  | AgreementAgent | Custom clause generation | `TierFull` (Sonnet) |
  | AgreementAgent | Template rendering | `TierNone` (Go template engine) |
  | LateReturnAgent | Escalation decision | `TierFull` (Sonnet) |
  | LateReturnAgent | Late fee calculation | `TierNone` (deterministic) |
  | FraudAgent | Pattern detection across history | `TierFull` (Sonnet) |
  | FraudAgent | Signal aggregation | `TierCheap` (Haiku) |
  | OpsAgent | Anomaly detection | `TierCheap` (Haiku) |
  | OpsAgent | Health report generation | `TierCheap` (Haiku) |
  | NotificationService | Notification text generation | `TierCheap` (Haiku) |
  | DiscoveryService | Semantic search matching | `TierCheap` (Haiku) |

- `backend/internal/agent/router/prompt.go` — Prompt loader: reads `backend/prompts/{agent_name}/v{N}.txt`, parses with `text/template`, caches parsed templates in memory. Detects latest version by scanning directory. Returns prompt version string for decision logging.

- `backend/internal/agent/router/router_test.go` — Unit tests: tier lookup returns correct model for every task, unknown task returns error, prompt loader finds latest version, retry logic triggers on API errors.

- `backend/internal/agent/decision/model.go` — AgentDecision domain type, CreateDecisionInput (wraps the existing `agent_decisions` table)
- `backend/internal/agent/decision/repository.go` — Insert, FindByID, FindByTransactionID, FindByUserID, FindByAgentType, UpdateOutcome (sets outcomeId + outcomeCorrect)
- `backend/internal/agent/decision/service.go` — RecordDecision (called by every agent after a model call), LinkOutcome (called by learning loop jobs)

**Create prompt files:**
- `backend/prompts/appraisal/v1.txt` — Item identification prompt: accepts base64 images, returns JSON with item name, category, estimated value range, suggested hourly/daily price, description, tags
- `backend/prompts/appraisal/override_v1.txt` — Value override review: accepts item photos, AI estimate, host declared value, host justification text. Returns approve/reject + reasoning
- `backend/prompts/risk/v1.txt` — Risk scoring prompt: accepts user profiles, transaction details, behavioral history. Returns risk assessment with per-signal breakdown
- `backend/prompts/verification/v1.txt` — KYC interpretation prompt: accepts Stripe Identity result JSON. Returns verification decision + confidence
- `backend/prompts/agreement/v1.txt` — Custom clause generation: accepts item type, category, value, condition notes. Returns item-specific clauses as JSON array
- `backend/prompts/agreement/base_template_v1.json` — Lawyer-reviewed base agreement template (see Step 4.5 for structure)

**Modify:**
- `backend/cmd/server/main.go` — Initialize Anthropic client, create Router, inject into agents
- `backend/internal/platform/config/config.go` — Add `AnthropicAPIKey`, `AnthropicDefaultModel`, `AnthropicCheapModel` fields

**Verify:**
```bash
cd backend && go vet ./...
cd backend && go build ./cmd/server
cd backend && go test ./internal/agent/... -v -count=1
# Verify tier matrix is complete (all AgentTask constants have a mapping):
cd backend && go test ./internal/agent/router/ -run TestTierMatrixComplete -v
```

### Step 4.2 — VerificationAgent (backend)

**Create:**
- `backend/internal/agent/verification/model.go` — VerificationInput (userID, Stripe Identity session ID), VerificationResult (status, confidence, reason, escalated)
- `backend/internal/agent/verification/service.go` — VerificationAgent:
  - `StartVerification(ctx, userID) -> (sessionURL, error)` — Creates Stripe Identity VerificationSession, returns URL for client to open
  - `HandleWebhook(ctx, event) -> error` — Processes `identity.verification_session.verified` / `requires_input` / `canceled` webhooks from Stripe
  - `InterpretResult(ctx, userID, stripeResult) -> (VerificationResult, error)`:
    1. Receive Stripe Identity result via webhook
    2. If Stripe confidence is high (document verified, selfie matched, no fraud indicators) -> auto-approve, set `identityStatus = VERIFIED`
    3. If Stripe flags fraud indicators (document tampering, selfie mismatch, known fraud database hit) -> auto-reject, set `identityStatus = REJECTED`
    4. Edge cases (expired document, blurry selfie, partial match) -> route to Haiku via model router for interpretation -> if confidence < 0.85 -> escalate to human review queue
    5. Record AgentDecision (agentType=VERIFICATION, input=Stripe result, decision=approve/reject/escalate, confidence, model, promptVersion)
    6. Award +50 reputation on first verification (one-time)
  - River job: `VerificationTimeoutJob` — if Stripe hasn't responded in 10 minutes, retry the session. After 3 retries, escalate to human review
- `backend/internal/agent/verification/handler.go` — HTTP handlers:
  - `POST /api/v1/verification/start` — Initiates KYC for authenticated user
  - `GET /api/v1/verification/status` — Returns current verification status
  - `POST /api/v1/webhooks/stripe-identity` — Stripe Identity webhook receiver (validates signature)
- `backend/internal/agent/verification/repository.go` — Store/retrieve verification attempts, idempotency check (KYC triggered only once per user unless rejected)
- `backend/prompts/verification/v1.txt` — Edge case interpretation prompt

**Modify:**
- `backend/cmd/server/main.go` — Mount verification router, register webhook handler, register River job
- `backend/internal/user/service.go` — Add `UpdateIdentityStatus(ctx, userID, status)` method

**Verify:**
```bash
cd backend && go vet ./...
cd backend && go test ./internal/agent/verification/... -v -count=1
# Integration test with Stripe test mode:
curl -sf -X POST http://localhost:8080/api/v1/verification/start \
  -H "Authorization: Bearer $TOKEN"
# Should return 200 with {sessionUrl, sessionId}
# Simulate webhook:
curl -sf -X POST http://localhost:8080/api/v1/webhooks/stripe-identity \
  -H 'Stripe-Signature: test_sig' \
  -d '{"type":"identity.verification_session.verified","data":{"object":{"id":"vs_test","status":"verified"}}}'
# User identity_status should be VERIFIED
# agent_decisions should have a VERIFICATION record
```

### Step 4.3 — AppraisalAgent (backend)

**Create:**
- `backend/internal/agent/appraisal/model.go` — AppraisalInput (listingID, imageURLs), AppraisalResult (itemName, category, estimatedValue, suggestedPricePerHour, suggestedPricePerDay, description, tags []string, confidence), OverrideInput (listingID, aiEstimate, hostDeclaredValue, justification), OverrideResult (approved bool, reasoning string)
- `backend/internal/agent/appraisal/service.go` — AppraisalAgent:
  - `Appraise(ctx, input AppraisalInput) -> (AppraisalResult, error)`:
    1. Fetch images from S3 (original URLs from media records)
    2. Base64 encode images for vision API
    3. Load prompt `appraisal/v{latest}.txt`, render with `text/template` (inject image count, any prior listing data)
    4. Route through model router: `TaskItemIdentification` -> Sonnet (vision)
    5. Parse structured JSON response (item name, value, pricing, description, tags)
    6. Route `TaskTagGeneration` -> Haiku for additional semantic tags (cheaper, can process text-only)
    7. Merge results, record AgentDecision
    8. Update listing: set `ai_generated_tags`, `estimated_value`, title/description if empty
    9. Return AppraisalResult
  - `ReviewOverride(ctx, input OverrideInput) -> (OverrideResult, error)`:
    1. Host declared value >100% of AI estimate triggers this
    2. Load prompt `appraisal/override_v{latest}.txt`
    3. Route through model router: `TaskValueOverrideReview` -> Sonnet
    4. Include: item photos, AI estimate, host value, host justification text
    5. AI evaluates justification autonomously (rare/vintage items, recent purchase receipt reference, custom modifications)
    6. Record AgentDecision (input=override request, decision=approve/reject, confidence)
    7. If approved: listing uses host_declared_value for hold calculation
    8. If rejected: respond with reasoning, host can accept AI estimate or provide more evidence
  - River job: `AppraisalJob` — async processing for listing creation (non-blocking). Timeout: 30s for vision call. On failure: retry up to 3 times, then mark listing with `appraisal_status: FAILED` for manual review
- `backend/internal/agent/appraisal/handler.go` — HTTP handlers:
  - `POST /api/v1/listings/:id/appraise` — Trigger appraisal (called automatically on listing creation, or manually for re-appraisal)
  - `POST /api/v1/listings/:id/override` — Submit value override with justification
  - `GET /api/v1/listings/:id/appraisal` — Get appraisal result for a listing
- `backend/internal/agent/appraisal/repository.go` — Store/retrieve appraisal results linked to listings

**Modify:**
- `backend/cmd/server/main.go` — Mount appraisal router, register River job
- `backend/internal/listing/service.go` — After listing creation, enqueue `AppraisalJob` via River
- `backend/internal/listing/model.go` — Add `AppraisalStatus` field (PENDING, COMPLETE, FAILED)

**Verify:**
```bash
cd backend && go vet ./...
cd backend && go test ./internal/agent/appraisal/... -v -count=1
# Golden set test (mock LLM):
cd backend && go test ./internal/agent/appraisal/ -run TestGoldenSet -v
# Integration test:
# Create listing with photos -> appraisal job runs -> listing fields populated
curl -sf http://localhost:8080/api/v1/listings/$LISTING_ID/appraisal \
  -H "Authorization: Bearer $TOKEN"
# Should return appraisal result with estimatedValue, tags, description
# Override test:
curl -sf -X POST http://localhost:8080/api/v1/listings/$LISTING_ID/override \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"declaredValue":500,"justification":"Vintage 1960s model, recently appraised"}'
# Should return approve/reject decision
```

### Step 4.4 — RiskAgent (backend)

**Create:**
- `backend/internal/agent/risk/model.go` — Core types:
  ```go
  // ReputationSignal represents a single event that modifies reputation.
  type ReputationSignal struct {
      Type   SignalType
      Points int       // positive = add, negative = subtract
      UserID string
      At     time.Time
  }

  type SignalType string

  const (
      // Positive signals
      SignalCompletedNoDispute   SignalType = "completed_no_dispute"     // +15
      SignalPositiveRating       SignalType = "positive_rating"          // +5 per bubble
      SignalOnTimeReturn         SignalType = "on_time_return"           // +10
      SignalAccountAge30         SignalType = "account_age_30"           // +25 one-time
      SignalAccountAge90         SignalType = "account_age_90"           // +25 one-time
      SignalAccountAge365        SignalType = "account_age_365"          // +25 one-time
      SignalKYCVerified          SignalType = "kyc_verified"             // +50 one-time
      SignalMilestone5           SignalType = "milestone_5_rentals"      // +50 one-time
      SignalMilestone15          SignalType = "milestone_15_rentals"     // +50 one-time
      SignalMilestone50          SignalType = "milestone_50_rentals"     // +50 one-time

      // Negative signals
      SignalDisputeFiled         SignalType = "dispute_filed"            // -30
      SignalDisputeLost          SignalType = "dispute_lost"             // -50
      SignalCancellation         SignalType = "cancellation"             // -20
      SignalLateReturn           SignalType = "late_return"              // -15
      SignalFraudFlag            SignalType = "fraud_flag"               // -100

      // Host-specific (recalculated monthly)
      SignalHighResponseRate     SignalType = "high_response_rate"       // +25
      SignalHighAcceptanceRate   SignalType = "high_acceptance_rate"     // +25
      SignalZeroCancellations90  SignalType = "zero_cancellations_90d"   // +25
      SignalLowResponseRate      SignalType = "low_response_rate"        // -40
      SignalLowAcceptanceRate    SignalType = "low_acceptance_rate"      // -40
  )
  ```

  Reputation score computation:
  ```
  reputationScore = clamp(0, 1000,
      sum(positive signals)
    - sum(active negative signals)        // full penalty within 180d
    - sum(decayed negative signals) * 0.5 // 50% after 180d, unless repeated
  )
  ```

  Risk score computation (per-transaction, deterministic rules engine):
  ```
  riskScore = clamp(0, 100,
      baseRisk(identityStatus, accountAge, reputationScore)
    + transactionRisk(itemValue, duration, bookingHour)
    + counterpartyRisk(counterpartyReputation, counterpartyAccountAge)
    + behavioralRisk(recentCancellations, recentDisputes, geoConsistency)
    + fraudSignals(deviceFingerprint, networkSignals, velocityFlags)
  )
  ```

  Complete risk input weights:
  ```
  Base risk:
    identityStatus == PENDING                           = +20
    accountAge < 7 days                                 = +15
    reputationScore < 100                               = +15
    reputationScore 100-300                              = +5

  Transaction risk:
    itemValue > $1,000                                  = +15
    itemValue > $3,000                                  = +25 (replaces +15)
    bookingHour is 00:00-05:00 local                    = +10

  Counterparty risk:
    both users have reputationScore < 50                = +30
    counterparty accountAge < 14 days                   = +10

  Behavioral risk:
    2+ cancellations in last 60 days                    = +20
    1+ disputes in last 60 days                         = +25
    transaction outside user's usual geo radius          = +10

  Fraud signals:
    same device fingerprint as another account          = +50
    compound network signals (WiFi + one other)         = +30
    velocity flag (exceeds frequency threshold)         = +20
  ```

  Controls:
  ```
  0-30   -> fast payout, standard hold
  31-70  -> standard escrow, 48h delayed payout
  71+    -> block booking or require additional verification
  ```

- `backend/internal/agent/risk/service.go` — RiskAgent:
  - `ComputeRiskScore(ctx, renterID, hostID, listingID, txDetails) -> (int, RiskBreakdown, error)`:
    1. Fetch both user profiles (reputation, identity status, account age, risk flags, device fingerprint)
    2. Fetch transaction details (item value, scheduled start time, location)
    3. Fetch behavioral history (cancellations, disputes in last 60 days)
    4. Fetch fraud signals (device fingerprint matches, network signals)
    5. Compute each risk component using deterministic rules (no LLM call for v1)
    6. Sum and clamp to 0-100
    7. Record AgentDecision (agentType=RISK, input=all signals, decision=score+breakdown, model=null for rules-only)
    8. Return score + per-signal breakdown for transparency
  - `RecalculateReputation(ctx, userID) -> (int, error)`:
    1. Fetch all reputation signals for user (from signal log table)
    2. Apply decay: negative signals older than 180 days contribute at 50% unless repeated within window
    3. Sum positive signals, subtract active/decayed negative signals
    4. Clamp to 0-1000
    5. Update user.reputation_score
    6. Return new score
  - `EmitSignal(ctx, signal ReputationSignal) -> error`:
    1. Insert signal into reputation_signals table
    2. Check one-time signals (KYC, milestones, account age) for idempotency
    3. Trigger RecalculateReputation
  - River jobs:
    - `MonthlyReputationJob` — Recalculates host-specific signals (response rate, acceptance rate, cancellations) for all hosts. Runs on 1st of month
    - `DecayCheckJob` — Weekly scan for negative signals crossing the 180-day mark. Recalculates affected users' reputation

- `backend/internal/agent/risk/repository.go` — reputation_signals CRUD, risk score queries, behavioral history aggregation
- `backend/internal/agent/risk/handler.go` — HTTP handlers:
  - `GET /api/v1/users/:id/reputation` — Get user reputation score + signal history
  - `GET /api/v1/transactions/:id/risk` — Get transaction risk score + breakdown
  - `POST /api/v1/admin/reputation/:userId/recalculate` — Force reputation recalculation (admin only)

**Modify:**
- `backend/cmd/server/main.go` — Mount risk router, register River cron jobs
- `backend/internal/booking/service.go` — Call `ComputeRiskScore` during booking request. Store on transaction. Apply controls (block if 71+, delay payout if 31-70)
- `backend/internal/booking/service.go` — After booking completes, call `EmitSignal` for completed_no_dispute, on_time_return, etc.

**Verify:**
```bash
cd backend && go vet ./...
cd backend && go test ./internal/agent/risk/... -v -count=1
# Golden set tests:
cd backend && go test ./internal/agent/risk/ -run TestRiskScoring -v
# Test cases:
# - New user (pending KYC, 0 reputation) renting $500 item = high risk
# - Verified user (500 reputation) renting $50 item = low risk
# - Two new users transacting with each other = very high risk (counterparty risk)
# - High-value item at 3am = elevated risk
# Reputation tests:
cd backend && go test ./internal/agent/risk/ -run TestReputationSignals -v
# - 5 clean rentals = ~175 reputation
# - Dispute lost decays by 50% after 180 days
# - One-time signals (KYC, milestones) never double-count
# - Score never goes below 0 or above 1000
```

### Step 4.5 — AgreementAgent (backend)

**Create:**
- `backend/internal/agent/agreement/model.go` — AgreementInput (transactionID, listingID, itemType, itemValue, itemCategory, itemConditionNotes), AgreementOutput (fullAgreement JSON, customClauses []Clause, version string), Clause (type, text, category)

  Base template structure (`base_template_v1.json`):
  ```json
  {
    "version": "1.0",
    "sections": [
      {
        "id": "platform_terms",
        "title": "Platform Terms of Use",
        "content": "...",
        "mutable": false
      },
      {
        "id": "liability",
        "title": "Liability Framework",
        "content": "Renter assumes full liability for item during rental period...",
        "mutable": false
      },
      {
        "id": "arbitration",
        "title": "Arbitration Clause",
        "content": "...",
        "mutable": false
      },
      {
        "id": "late_fees",
        "title": "Late Fee Structure",
        "content": "...",
        "mutable": false
      },
      {
        "id": "hold_disclosure",
        "title": "Pre-Authorization Hold Disclosure",
        "content": "...",
        "mutable": false
      },
      {
        "id": "data_usage",
        "title": "Data Usage and Retention",
        "content": "...",
        "mutable": false
      },
      {
        "id": "ai_disclosure",
        "title": "AI Agent Disclosure",
        "content": "You acknowledge that AI agents make operational decisions...",
        "mutable": false
      },
      {
        "id": "item_specific",
        "title": "Item-Specific Terms",
        "content": null,
        "mutable": true,
        "generated_by": "AgreementAgent"
      }
    ]
  }
  ```

- `backend/internal/agent/agreement/service.go` — AgreementAgent:
  - `GenerateAgreement(ctx, input AgreementInput) -> (AgreementOutput, error)`:
    1. Load base template (`base_template_v1.json`)
    2. Fetch listing details (item type, value, category, photos, condition)
    3. Load prompt `agreement/v{latest}.txt`, render with item context
    4. Route through model router: `TaskCustomClauseGeneration` -> Sonnet
    5. AI generates item-specific clauses:
       - Condition notes (current state of item, pre-existing wear)
       - Exclusions (e.g., water damage for electronics, engine damage for vehicles)
       - Handling instructions (e.g., "Do not submerge" for camera gear)
       - Custom damage thresholds based on item type and value
       - Anticipated damage scenarios for the item category
    6. **Guardrail validation** — programmatic check that custom clauses do not:
       - Contradict any `mutable: false` section
       - Remove or weaken liability language
       - Remove or modify arbitration clause
       - Alter payment terms, hold amounts, or fee structure
       - Contain legally problematic language (regex filter for banned patterns)
    7. If guardrail violation detected: strip violating clause, log warning, re-generate with explicit constraint
    8. Merge custom clauses into base template's `item_specific` section
    9. Route `TaskTemplateRendering` -> `TierNone` (Go template renders final JSON)
    10. Record AgentDecision (agentType=AGREEMENT)
    11. Store immutable agreement snapshot on transaction (agreement_snapshot JSONB)
    12. Return complete agreement
  - `ValidateAcceptance(ctx, transactionID, userID) -> error` — Records user acceptance (timestamp, IP, device ID). Both parties must accept before booking confirms
- `backend/internal/agent/agreement/handler.go` — HTTP handlers:
  - `GET /api/v1/transactions/:id/agreement` — Get agreement for a transaction
  - `POST /api/v1/transactions/:id/agreement/accept` — User accepts agreement
  - `GET /api/v1/transactions/:id/agreement/status` — Check if both parties have accepted
- `backend/internal/agent/agreement/repository.go` — Agreement snapshot storage, acceptance tracking

**Modify:**
- `backend/cmd/server/main.go` — Mount agreement router
- `backend/internal/booking/service.go` — After booking accepted by host, trigger `GenerateAgreement`. Block transition to ACTIVE until both parties accept

**Verify:**
```bash
cd backend && go vet ./...
cd backend && go test ./internal/agent/agreement/... -v -count=1
# Guardrail tests:
cd backend && go test ./internal/agent/agreement/ -run TestGuardrails -v
# - Custom clause that contradicts liability -> stripped
# - Custom clause that removes arbitration -> stripped
# - Custom clause modifying payment terms -> stripped
# - Valid item-specific clause -> kept
# Golden set test (mock LLM):
cd backend && go test ./internal/agent/agreement/ -run TestGoldenSet -v
# - Electronics item -> generates water damage exclusion
# - Camera gear -> generates missing accessory clause
# - Furniture -> generates cosmetic damage threshold
```

### Step 4.6 — Wire KYC into Booking Flow (RN)

**Install:**
```bash
cd mobile && npx expo install @stripe/stripe-identity-react-native
```

**Create:**
- `mobile/app/(tabs)/(profile)/verify.tsx` — KYC verification screen:
  - Renders Stripe Identity verification sheet
  - States: idle, processing, verified, rejected
  - On verified: dismiss and return to previous screen
  - On rejected: show reason, allow retry
- `mobile/components/verification/KYCGate.tsx` — Component that wraps booking checkout:
  - Checks user's `identityStatus` from auth store
  - If `VERIFIED`: renders children (booking proceeds)
  - If `PENDING` or not started: triggers KYC flow before allowing booking
  - If `REJECTED`: shows rejection message with retry button
- `mobile/lib/hooks/useVerification.ts` — TanStack Query hook:
  - `useStartVerification()` — mutation calling `POST /api/v1/verification/start`
  - `useVerificationStatus()` — query polling `GET /api/v1/verification/status` every 3s during verification

**Modify:**
- `mobile/app/(tabs)/(feed)/checkout.tsx` — Wrap checkout flow with `KYCGate`. If user not verified, show KYC before proceeding
- `mobile/lib/auth.ts` — Add `identityStatus` to auth store, update on verification completion

**Verify:**
```bash
cd mobile && npx tsc --noEmit
# Manual test:
# 1. New user taps "Rent Now" -> KYC screen appears
# 2. Complete Stripe Identity flow (test mode)
# 3. Returns to checkout -> booking proceeds
# 4. Second booking attempt -> no KYC prompt (already verified)
```

### Step 4.7 — Wire AI Autofill into Listing Creation (RN)

**Create:**
- `mobile/components/listing/AIAutofillOverlay.tsx` — Loading overlay shown after photo capture while AI processes:
  - Skeleton UI for title, description, price, tags fields
  - Animated shimmer effect during loading
  - On completion: fields populate with smooth transition
  - Error state: "AI couldn't identify this item. Please fill in manually."
- `mobile/components/listing/ValueOverridePrompt.tsx` — Modal shown when host changes value >100% of AI estimate:
  - Displays AI estimate vs host value
  - Text input for justification
  - Submit button triggers `POST /api/v1/listings/:id/override`
  - Shows approve/reject result with reasoning
- `mobile/lib/hooks/useAppraisal.ts` — TanStack Query hooks:
  - `useAppraisal(listingId)` — query for appraisal result, polls while status is PENDING
  - `useOverride(listingId)` — mutation for value override submission

**Modify:**
- `mobile/app/(tabs)/(profile)/create-listing.tsx` — After photo capture step:
  1. Show `AIAutofillOverlay` while appraisal runs
  2. On completion: populate form fields (title, description, price per hour, price per day, tags)
  3. All AI-filled fields are editable by host
  4. On value field change: if new value > 2x AI estimate, show `ValueOverridePrompt`
- `mobile/components/listing/ListingForm.tsx` — Add `aiSuggested` indicator on each field (subtle "AI" badge). Add `onValueChange` handler that checks override threshold

**Verify:**
```bash
cd mobile && npx tsc --noEmit
# Manual test:
# 1. Create listing -> capture photos -> AI autofill overlay shows
# 2. Fields populate with AI suggestions
# 3. Edit title -> "AI" badge disappears on that field
# 4. Change value to 3x AI estimate -> override prompt appears
# 5. Submit justification -> approve/reject shown
```

### Step 4.8 — Backfill Existing Data (backend)

**Create:**
- `backend/internal/agent/backfill/jobs.go` — River batch jobs:
  - `BackfillAppraisalJob`:
    1. Query all listings where `ai_generated_tags IS NULL OR ai_generated_tags = '[]'` OR `estimated_value IS NULL`
    2. Process in batches of 10 (rate limit: 1 appraisal/second to avoid API throttling)
    3. For each listing: call `AppraisalAgent.Appraise()`
    4. Mark result with `backfilled: true` in the AgentDecision metadata
    5. On failure: log error, skip listing, continue batch
    6. Progress tracking: log `processed/total` every 10 listings
  - `BackfillReputationJob`:
    1. Query all users with any transaction history
    2. For each user: replay all reputation signals from their complete transaction history
    3. Call `RiskAgent.RecalculateReputation()` with full signal history
    4. Mark with `backfilled: true`
    5. Process in batches of 50 users
  - `BackfillRiskScoreJob`:
    1. Query all transactions where `risk_score IS NULL`
    2. For each: call `RiskAgent.ComputeRiskScore()` with historical data
    3. Store score on transaction with `backfilled: true` metadata in AgentDecision
    4. These scores are for analytics only — they do not retroactively affect payout timing or hold amounts
    5. Process in batches of 50 transactions

- `backend/internal/agent/backfill/handler.go` — Admin-only HTTP handlers:
  - `POST /api/v1/admin/backfill/appraisals` — Enqueue BackfillAppraisalJob
  - `POST /api/v1/admin/backfill/reputation` — Enqueue BackfillReputationJob
  - `POST /api/v1/admin/backfill/risk-scores` — Enqueue BackfillRiskScoreJob
  - `GET /api/v1/admin/backfill/status` — Returns progress for active backfill jobs

**Modify:**
- `backend/cmd/server/main.go` — Mount backfill admin router (behind admin auth middleware), register River jobs

**Verify:**
```bash
cd backend && go vet ./...
cd backend && go test ./internal/agent/backfill/... -v -count=1
# Integration test (with seeded data):
# 1. Seed 5 listings without AI tags
# 2. Trigger backfill:
curl -sf -X POST http://localhost:8080/api/v1/admin/backfill/appraisals \
  -H "Authorization: Bearer $ADMIN_TOKEN"
# 3. Check progress:
curl -sf http://localhost:8080/api/v1/admin/backfill/status \
  -H "Authorization: Bearer $ADMIN_TOKEN"
# 4. Verify all 5 listings now have ai_generated_tags and estimated_value
# 5. Verify agent_decisions records have backfilled metadata
```

---

## API Endpoints

| Method | Path | Auth | Request Body | Response | Errors |
|--------|------|------|-------------|----------|--------|
| POST | `/api/v1/verification/start` | Yes | -- | `{sessionUrl, sessionId}` | 400 already verified, 401 |
| GET | `/api/v1/verification/status` | Yes | -- | `{status, updatedAt}` | 401 |
| POST | `/api/v1/webhooks/stripe-identity` | Stripe sig | Stripe webhook event | `200 OK` | 400 bad signature |
| POST | `/api/v1/listings/:id/appraise` | Yes | -- | `{appraisal}` | 403 not owner, 404 |
| GET | `/api/v1/listings/:id/appraisal` | Yes | -- | `{appraisal}` | 404 |
| POST | `/api/v1/listings/:id/override` | Yes | `{declaredValue, justification}` | `{approved, reasoning}` | 400 no AI estimate yet, 403 not owner |
| GET | `/api/v1/users/:id/reputation` | Yes | -- | `{score, signals[], breakdown}` | 404 |
| GET | `/api/v1/transactions/:id/risk` | Yes | -- | `{score, breakdown}` | 403 not party, 404 |
| POST | `/api/v1/admin/reputation/:userId/recalculate` | Admin | -- | `{oldScore, newScore}` | 403 not admin |
| GET | `/api/v1/transactions/:id/agreement` | Yes | -- | `{agreement}` | 403 not party, 404 |
| POST | `/api/v1/transactions/:id/agreement/accept` | Yes | -- | `{accepted, bothAccepted}` | 403 not party |
| GET | `/api/v1/transactions/:id/agreement/status` | Yes | -- | `{renterAccepted, hostAccepted}` | 403 not party |
| POST | `/api/v1/admin/backfill/appraisals` | Admin | -- | `{jobId}` | 403 not admin |
| POST | `/api/v1/admin/backfill/reputation` | Admin | -- | `{jobId}` | 403 not admin |
| POST | `/api/v1/admin/backfill/risk-scores` | Admin | -- | `{jobId}` | 403 not admin |
| GET | `/api/v1/admin/backfill/status` | Admin | -- | `{jobs[]}` | 403 not admin |

---

## Database Migrations

Migration `005_intelligence_layer.sql`:

```sql
-- Reputation signals log (tracks every event that modifies reputation)
CREATE TABLE reputation_signals (
    id                      TEXT PRIMARY KEY,            -- ULID
    user_id                 TEXT NOT NULL REFERENCES users(id),
    signal_type             TEXT NOT NULL,               -- matches SignalType constants
    points                  INTEGER NOT NULL,            -- positive or negative
    source_transaction_id   TEXT REFERENCES transactions(id),
    source_rating_id        TEXT REFERENCES ratings(id),
    decayed                 BOOLEAN NOT NULL DEFAULT FALSE,
    decayed_at              TIMESTAMPTZ,
    backfilled              BOOLEAN NOT NULL DEFAULT FALSE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_reputation_signals_user_id ON reputation_signals(user_id);
CREATE INDEX idx_reputation_signals_type ON reputation_signals(signal_type);
CREATE INDEX idx_reputation_signals_created_at ON reputation_signals(created_at);

-- Verification attempts (KYC tracking)
CREATE TABLE verification_attempts (
    id                      TEXT PRIMARY KEY,            -- ULID
    user_id                 TEXT NOT NULL REFERENCES users(id),
    stripe_session_id       TEXT NOT NULL,
    status                  TEXT NOT NULL DEFAULT 'PENDING'
                            CHECK (status IN ('PENDING', 'PROCESSING', 'VERIFIED',
                                              'REJECTED', 'EXPIRED')),
    stripe_result           JSONB,
    agent_decision_id       TEXT REFERENCES agent_decisions(id),
    retry_count             INTEGER NOT NULL DEFAULT 0,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at            TIMESTAMPTZ
);
CREATE INDEX idx_verification_attempts_user_id ON verification_attempts(user_id);
CREATE INDEX idx_verification_attempts_status ON verification_attempts(status);
CREATE UNIQUE INDEX idx_verification_attempts_stripe_session
    ON verification_attempts(stripe_session_id);

-- Agreement acceptances (tracks both parties accepting)
CREATE TABLE agreement_acceptances (
    id                      TEXT PRIMARY KEY,            -- ULID
    transaction_id          TEXT NOT NULL REFERENCES transactions(id),
    user_id                 TEXT NOT NULL REFERENCES users(id),
    ip_address              TEXT,
    device_id               TEXT,
    accepted_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(transaction_id, user_id)
);
CREATE INDEX idx_agreement_acceptances_transaction_id
    ON agreement_acceptances(transaction_id);

-- Appraisal results (cached per listing, re-run on photos change)
CREATE TABLE appraisal_results (
    id                      TEXT PRIMARY KEY,            -- ULID
    listing_id              TEXT NOT NULL REFERENCES listings(id),
    item_name               TEXT,
    item_category           TEXT,
    estimated_value         NUMERIC(10,2),
    suggested_price_hour    NUMERIC(10,2),
    suggested_price_day     NUMERIC(10,2),
    generated_description   TEXT,
    generated_tags          JSONB NOT NULL DEFAULT '[]',
    confidence              REAL,
    agent_decision_id       TEXT REFERENCES agent_decisions(id),
    backfilled              BOOLEAN NOT NULL DEFAULT FALSE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_appraisal_results_listing_id ON appraisal_results(listing_id);

-- Add appraisal_status to listings
ALTER TABLE listings ADD COLUMN appraisal_status TEXT DEFAULT 'PENDING'
    CHECK (appraisal_status IN ('PENDING', 'COMPLETE', 'FAILED'));

-- Add cost tracking columns to agent_decisions
ALTER TABLE agent_decisions ADD COLUMN IF NOT EXISTS prompt_version TEXT;
ALTER TABLE agent_decisions ADD COLUMN IF NOT EXISTS tokens_input INTEGER;
ALTER TABLE agent_decisions ADD COLUMN IF NOT EXISTS tokens_output INTEGER;
ALTER TABLE agent_decisions ADD COLUMN IF NOT EXISTS latency_ms INTEGER;
ALTER TABLE agent_decisions ADD COLUMN IF NOT EXISTS backfilled
    BOOLEAN NOT NULL DEFAULT FALSE;

-- Backfill job progress tracking
CREATE TABLE backfill_jobs (
    id                      TEXT PRIMARY KEY,            -- ULID
    job_type                TEXT NOT NULL
                            CHECK (job_type IN ('APPRAISAL', 'REPUTATION', 'RISK_SCORE')),
    status                  TEXT NOT NULL DEFAULT 'QUEUED'
                            CHECK (status IN ('QUEUED', 'RUNNING', 'COMPLETED', 'FAILED')),
    total_items             INTEGER NOT NULL DEFAULT 0,
    processed_items         INTEGER NOT NULL DEFAULT 0,
    failed_items            INTEGER NOT NULL DEFAULT 0,
    started_at              TIMESTAMPTZ,
    completed_at            TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

## Risks and Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| LLM latency (Sonnet vision: 5-15s per call) | Slow listing creation UX | Async processing via River. Client shows shimmer/skeleton UI. Appraisal is non-blocking — listing saves immediately, AI fills in background |
| LLM cost ($3/$15 per MTok for Sonnet input/output) | Unexpected API bills | Cost tracking per call in agent_decisions. Daily cost aggregation in OpsAgent health report. Hard budget alerts at $X/day. Haiku for everything that does not touch money |
| LLM accuracy / hallucination on value estimates | Incorrect hold amounts, user trust erosion | Golden set testing with known items. Host can always override with justification. AppraisalAgent tracks override rate — high override rate triggers prompt revision per section 31 |
| Hallucinated agreement clauses | Legally problematic custom terms | Programmatic guardrail validation post-generation. Custom clauses cannot contradict immutable base template. Banned pattern regex filter. Human legal review of guardrail rules quarterly |
| Stripe Identity API downtime | Users cannot verify, bookings blocked | 10-minute timeout with retry. After 3 retries, escalate to human review. Users can browse freely while verification is pending — only booking is blocked |
| Risk score false positives (blocking legitimate bookings) | Lost revenue, user frustration | Conservative thresholds at launch (only block at 71+). All risk inputs logged for audit. Admin override endpoint. Track false positive rate per section 31 learning framework |
| Risk score false negatives (allowing fraudulent bookings) | Financial loss | Guarantee fund backstop. FraudAgent pattern detection runs post-hoc. All inputs instrumented for future ML model training |
| Prompt injection via user-provided text (justification, messages) | Agent makes incorrect decisions | Never include raw user text in system prompts. User text always in user message blocks. Input sanitization (strip control characters). Output JSON schema validation |
| Backfill job overwhelming Anthropic API rate limits | 429 errors, incomplete backfill | Rate limiting: 1 appraisal/second, 60 risk scores/second. Exponential backoff on 429. Batch processing with progress tracking. Resumable jobs |
| Model version changes (Anthropic deprecates model) | Agent behavior changes unexpectedly | Pin model versions in config. Prompt version tracking enables A/B comparison. Test golden set against new model before switching |

---

## Testing Strategy

### Golden Set Test Pattern

Every agent has a golden set: predefined inputs with expected classifications. These run in CI on every PR that touches agent code or prompts.

**How it works:**
1. Define test fixtures: real-world-representative inputs (item photos for appraisal, user profiles for risk, KYC results for verification)
2. Define expected outputs: the correct classification, value range, or decision for each input
3. Mock the LLM response: use recorded responses from the real model (captured once, stored as fixtures)
4. Assert: agent produces the expected output given the mocked model response

**Why mock instead of calling the real API in CI:**
- Deterministic: same input always produces same output, no flaky tests
- Fast: no network calls, tests run in milliseconds
- Free: no API costs in CI
- Real model validation happens in the golden set recording step (manual, periodic)

### Per-Agent Test Specifications

**Model Router:**
- Unit: every AgentTask maps to the correct ModelTier
- Unit: unknown task returns error
- Unit: prompt loader finds latest version, parses template correctly
- Unit: retry logic triggers on 5xx, does not retry on 4xx
- Unit: timeout enforcement (context cancellation)
- Integration: mock Anthropic server receives correct model string per tier

**VerificationAgent:**
- Unit: high-confidence Stripe result -> auto-approve, identity_status = VERIFIED
- Unit: fraud indicators in Stripe result -> auto-reject, identity_status = REJECTED
- Unit: edge case (expired doc) -> Haiku call -> escalation if confidence < 0.85
- Unit: +50 reputation signal emitted on first verification only
- Unit: second verification attempt for already-verified user returns error
- Integration: Stripe webhook signature validation (valid sig accepted, invalid rejected)
- Golden set: 5 Stripe Identity result fixtures covering approve/reject/edge cases

**AppraisalAgent:**
- Unit (golden set): kayak photos -> item="Kayak", category="Water Sports", value in $200-$800 range
- Unit (golden set): MacBook photos -> item="MacBook Pro", category="Electronics", value in $800-$2500 range
- Unit (golden set): folding chair photos -> item="Folding Chair", category="Furniture", value in $20-$80 range
- Unit: override with value 3x AI estimate + valid justification -> approved
- Unit: override with value 3x AI estimate + empty justification -> rejected
- Unit: appraisal failure after 3 retries -> listing marked FAILED
- Unit: tags are generated via Haiku (TierCheap), not Sonnet

**RiskAgent:**
- Unit: new user (PENDING KYC, 0 rep) renting $500 item = score > 50
- Unit: verified user (500 rep) renting $50 item = score < 15
- Unit: two new users (both < 50 rep) transacting = counterparty risk +30
- Unit: booking at 3am = +10 timing risk
- Unit: same device fingerprint as another account = +50
- Unit: compound signal (WiFi + device fingerprint) = +30 (WiFi contributes only with compound)
- Unit: reputation after 5 clean rentals with positive ratings = ~175
- Unit: dispute lost (-50) decays to -25 after 180 days
- Unit: repeated dispute within 180 days resets decay clock
- Unit: one-time signals (KYC +50) never double-counted
- Unit: score clamped to 0-1000 (reputation) and 0-100 (risk)
- Unit: risk controls: 0-30 fast, 31-70 delayed, 71+ blocked

**AgreementAgent:**
- Unit: electronics item generates water damage exclusion clause
- Unit: camera gear generates missing accessory clause
- Unit: furniture generates cosmetic damage threshold
- Unit: custom clause contradicting liability section -> stripped by guardrail
- Unit: custom clause removing arbitration -> stripped by guardrail
- Unit: custom clause modifying payment terms -> stripped by guardrail
- Unit: both parties must accept before booking transitions to ACTIVE
- Unit: acceptance is idempotent (accepting twice does not error)
- Unit: agreement snapshot is immutable after creation

**Backfill Jobs:**
- Unit: job processes listings in batches of 10
- Unit: failed appraisal skips listing, continues batch
- Unit: progress tracking updates after each batch
- Unit: backfilled records are distinguishable (backfilled=true in agent_decisions)
- Integration: seed 3 listings without tags -> run backfill -> all 3 have tags

### General Testing

- **Repository tests:** All new tables (reputation_signals, verification_attempts, agreement_acceptances, appraisal_results, backfill_jobs) against test Postgres via testcontainers
- **Integration tests:** Full flows: create listing -> appraisal runs -> fields populated. Start booking -> risk score computed -> controls applied. KYC flow -> verification recorded -> reputation signal emitted
- **RN:** TypeScript check passes. Manual verification of KYC gate, AI autofill overlay, and value override prompt on simulator

---

## Implementation Order

| Step | What | Day | Depends On |
|------|------|-----|------------|
| 4.1 | Model router + prompt loader + AgentDecision service | Day 1-3 | Phase 3 complete |
| 4.2 | VerificationAgent + Stripe Identity integration | Day 3-5 | 4.1 (needs router) |
| 4.3 | AppraisalAgent + vision pipeline | Day 4-7 | 4.1 (needs router) |
| 4.4 | RiskAgent + dual-score system | Day 5-8 | 4.1 (needs router + decision service) |
| 4.5 | AgreementAgent + template system | Day 7-9 | 4.1, 4.3 (needs router + item data from appraisal) |
| 4.6 | Wire KYC into booking flow (RN) | Day 5-7 | 4.2 (needs verification API) |
| 4.7 | Wire AI autofill into listing creation (RN) | Day 7-9 | 4.3 (needs appraisal API) |
| 4.8 | Backfill existing data | Day 9-10 | 4.3, 4.4 (needs appraisal + risk agents) |

Steps 4.2 and 4.3 are independent of each other — can be parallelized after 4.1 is complete.
Steps 4.6 and 4.7 are independent (mobile vs mobile, different screens) — can be parallelized.
Step 4.4 can start in parallel with 4.3 once the router (4.1) is done.
Step 4.8 must be last — it depends on all agents being functional.
