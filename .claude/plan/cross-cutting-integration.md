# Cross-Cutting Concerns — Per-Phase Integration Matrix

> Maps each cross-cutting concern to specific deliverables in each phase. Reference this when writing phase plans and during implementation to ensure nothing is missed.

## Unit Tests

| Phase | What to Test |
|-------|-------------|
| 1 | UserService: bcrypt hashing, JWT issuance/validation, input validation. ListingService: 7-day ceiling enforcement, CRUD. MediaService: thumbnail generation, orientation metadata |
| 2 | DiscoveryService: ranking formula (all 4 factors), tiered hold calculation, fulltext search scoring. PaymentService: hold tiers, guarantee fund contribution math, HoldAllocation ledger |
| 3 | BookingService: every state machine transition (valid + invalid), cancellation fee calculation, auto-decline timer, fraud velocity rules. ProximityService: distance calculation, PIN generation/validation |
| 4 | RiskAgent: reputation signal math (positive/negative/decay), per-transaction risk score, milestone triggers. AppraisalAgent: value override threshold. AgreementAgent: guardrail enforcement (cannot remove liability/arbitration) |
| 5 | Photo diff: confidence scoring, damage classification. DisputeAgent: escalation gate routing (all 6 conditions). LateReturnAgent: late fee cap (60% of hold), damage reserve preservation. Reputation: decay at 180 days, milestone bonuses. Guarantee fund: reserve ratio calculation, loss ratio |
| 6 | FraudAgent: WiFi compound-only rule (never scores alone), collusion pattern detection. Referral: $20 payout triggers, fraud prevention rules |

## Integration Tests (testcontainers-go — real Postgres + Redis)

Tests live in `backend/tests/integration/`. Infrastructure set up in Phase 5.

| Phase | What to Test | Test File |
|-------|-------------|-----------|
| 5 (retro) | Register → Login → Get Profile. Upload photo → verify in S3 → retrieve thumbnail. Create listing → verify in DB with media | `user_api_test.go`, `listing_api_test.go` |
| 5 (retro) | Search → results ranked correctly. Create booking → hold authorized (Stripe test mode). Guarantee fund contribution recorded | `discovery_api_test.go`, `payment_api_test.go` |
| 5 (retro) | Full booking lifecycle: request → accept → check-in → check-out → complete. Auto-decline fires after timeout. Cancellation charges correct fee. Fraud velocity blocks violating booking | `booking_api_test.go` |
| 5 (retro) | Listing creation → AI autofill (mock LLM). KYC flow → identity status updated. Risk score computed on booking | `agent_api_test.go` |
| 6 | Return photos → CV preprocessing → LLM diff → dispute filed → escalation gate → hold captured. Reputation updated after completed transaction. Outcome linking fires 48h after close | `dispute_api_test.go`, `reputation_api_test.go` |
| 7 | OpsAgent cron → health report generated → alert fired (Slack webhook mock). End-to-end referral: create code → referred user signs up → completes rental → both get $20 | `ops_api_test.go`, `referral_api_test.go` |

## Mobile Component Tests (Jest + RNTL + MSW)

Tests live in `mobile/__tests__/`. Infrastructure set up in Phase 5.

| Phase | What to Test | Test File |
|-------|-------------|-----------|
| 5 (retro) | Auth screens: login/register validation, navigation. Listing creation: form fields, submission. Feed/search: data rendering, pagination | `screens/auth.test.tsx`, `screens/listing-create.test.tsx`, `screens/feed.test.tsx` |
| 5 (retro) | Booking flow: price breakdown, submission. Map: marker rendering | `screens/booking.test.tsx`, `screens/map.test.tsx` |
| 6 | Post-rental flow: photo return, dispute filing, rating submission | `screens/post-rental.test.tsx` |
| 7 | Referral UI: code display, sharing, status | `screens/referral.test.tsx` |

**Rule:** Every new task that adds backend endpoints or mobile screens MUST add corresponding tests. Tests are part of the task deliverable, not optional.

## Structured Logging (slog)

| Phase | Additions |
|-------|-----------|
| 0 | Base setup: RequestID middleware, structured JSON output (DONE) |
| 1 | Add `user_id` to log context after auth. Log all auth events (register, login, failed login) |
| 2 | Add `listing_id`, `transaction_id` to log context. Log payment events (hold authorized, captured, released) |
| 3 | Add booking state transitions to logs. Log proximity verification results. Log notification delivery |
| 4 | Log agent decisions with model, confidence, latency, token count. Log prompt version used |
| 5 | Log dispute lifecycle events. Log photo diff results with confidence. Log reputation score changes |
| 6 | Log fraud signals detected. Log alert delivery (Slack webhook response). Dashboard access logs |

## Request Tracing (OpenTelemetry)

| Phase | Additions |
|-------|-----------|
| 1 | Add OpenTelemetry middleware to chi router. Trace spans for auth, DB queries, S3 uploads |
| 2 | Trace spans for Stripe API calls, PostGIS queries, ranking computation |
| 3 | Trace spans for state machine transitions, push notification delivery, SMS sends |
| 4 | Trace spans for LLM API calls (include model, tokens, latency), KYC verification |
| 5 | Trace spans for CV pipeline (sidecar HTTP), photo diff end-to-end, dispute resolution |
| 6 | Trace spans for OpsAgent cron runs, fraud analysis, alert delivery |

## Metrics (Prometheus)

| Phase | Metrics |
|-------|---------|
| 2 | Request latency histograms, error rate counters, active connections gauge |
| 3 | Queue depths (River), booking conversion rate, notification delivery rate |
| 4 | LLM latency histogram, LLM cost counter (tokens * price), agent confidence histogram |
| 5 | Dispute resolution time histogram, escalation rate counter, guarantee fund balance gauge, loss ratio gauge |
| 6 | Dashboard: expose all metrics via Grafana. Fraud signal counters. Alert delivery latency |

## Rate Limiting (Redis token bucket)

| Phase | Endpoints |
|-------|-----------|
| 1 | Auth: 10 req/min per IP (register, login). Profile: 30 req/min per user |
| 2 | Search: 60 req/min per user. Feed/Map: 30 req/min per user. Payment mutations: 5 req/min per user |
| 3 | Booking creation: 5 req/min per user. Messages: 30 req/min per user |
| 4 | N/A (agent services are internal) |
| 5 | Dispute filing: 3 req/min per user. Rating: 10 req/min per user |
| 6 | Dashboard: 60 req/min per user. Referral: 10 req/min per user |

## Idempotency Keys

| Phase | Operations |
|-------|------------|
| 2 | All payment operations (AuthorizeHold, CaptureHold, ChargeRentalFee, PayoutHost, Refund). Store idempotency key in Redis with 24h TTL |
| 3 | Booking creation (prevent double-tap duplicate bookings). Key: `booking:{renter_id}:{listing_id}:{scheduled_start}` |
| 5 | Dispute filing (prevent duplicate disputes per transaction). Hold capture operations |

## Error Handling

| Phase | Patterns |
|-------|----------|
| 1 | Standard error responses: `{"error": "message", "code": "ERROR_CODE"}`. Wrap all errors: `fmt.Errorf("context: %w", err)` |
| 2 | Stripe error wrapping with user-friendly messages. Retry with exponential backoff on Stripe 5xx |
| 3 | State machine transition errors (400 with current state + attempted transition). Push notification delivery retry (3 attempts) |
| 4 | LLM timeout: 30s deadline, circuit breaker after 3 consecutive failures, queue for retry via River. Never fail silently on agent decisions |
| 5 | CV sidecar errors: circuit breaker, fallback to LLM-only (degraded mode). Dead letter queue for failed River jobs (outcome linking, reputation calc) |
| 6 | Alert delivery errors: retry 3x with backoff. Dashboard API errors: standard error responses |

## Circuit Breakers

| Phase | Dependencies |
|-------|-------------|
| 2 | Stripe API |
| 3 | Push notification provider (Expo Push), Twilio SMS |
| 4 | Anthropic API (LLM), Stripe Identity (KYC) |
| 5 | CV sidecar HTTP, Anthropic API |
| 6 | Slack webhook, PagerDuty API |
