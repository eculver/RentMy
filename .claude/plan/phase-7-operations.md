# Phase 7 — Operations + Growth Implementation Plan

> **Scope:** Wk 13-16. Ops visibility, fraud detection, agent learning, referral growth. Platform is launch-ready.
> **Exit criteria:** Ops dashboard shows real-time metrics + agent calibration, fraud detection active (WiFi compound-only), human review queue operational, referral system pays $20/$20 on first completed rental, alerts fire to Slack/PagerDuty.
> **Blockers:** Phase 6 complete (full transaction lifecycle working)

## Resolved Decisions

| Question | Answer | Notes |
|----------|--------|-------|
| Ops dashboard framework | Vite + React SPA in `/ops` | Internal tool — no SSR needed. NOT Next.js |
| Dashboard UI library | Tailwind CSS + Recharts | Consistent with mobile (NativeWind). Recharts for line/bar/calibration charts |
| Dashboard state management | TanStack Query v5 | Same pattern as mobile — consistent cache/refetch semantics |
| Dashboard auth | Same JWT as mobile | Shared backend issues JWTs. Ops users are `users` rows with an `ops_role` |
| Alert delivery (standard) | Slack webhook via `net/http` POST | No SDK — raw HTTP POST to webhook URL. Simple and zero dependencies |
| Alert delivery (critical) | PagerDuty via `pagerduty-go` | Critical alerts: fraud spike, payout failure, SLA breach, guarantee fund CRITICAL |
| Referral tracking | New `referrals` table in Postgres | Separate from transactions — tracks code, referrer, referee, status, payout |
| FraudAgent scheduling | River cron job + per-transaction hook | Scheduled scan for pattern analysis, inline hook for per-transaction signals |
| OpsAgent scheduling | River cron job (every 15 min) | Generates health snapshots, fires alerts on threshold breaches |
| WiFi signal scoring | Compound-only (+0 alone, +30 with another signal) | Per PRD section 9 — coffee shops, campuses produce false positives alone |
| Calibration window | Rolling 90-day | Per PRD section 31 — enough data for statistical significance |

---

## Technology Decisions

### Go Backend (New Dependencies)

| Decision | Choice | Import / Module |
|----------|--------|-----------------|
| PagerDuty alerts | pagerduty-go | `github.com/PagerDuty/go-pagerduty` |
| Slack alerts | stdlib net/http | `net/http` (POST JSON to webhook URL) |
| Cron scheduling | River periodic jobs | `github.com/riverqueue/river` (already in go.mod) |

**Rationale:**
- **pagerduty-go over raw HTTP:** PagerDuty Events API v2 has specific payload requirements (routing key, severity, dedup key). The SDK handles retry logic and event deduplication. Worth the dependency for critical alerting.
- **Slack via net/http over slack-go SDK:** Incoming webhooks are a single POST with a JSON body. An SDK adds 5+ transitive dependencies for one HTTP call. Not worth it.
- **River periodic jobs over standalone cron:** River is already the job queue. Periodic jobs run inside the same worker lifecycle with the same guarantees (at-least-once, graceful shutdown). No additional infrastructure.

### Ops Dashboard (New Project)

| Decision | Choice | Package |
|----------|--------|---------|
| Build tool | Vite 6 | `vite` |
| UI framework | React 19 | `react`, `react-dom` |
| Styling | Tailwind CSS 4 | `tailwindcss`, `@tailwindcss/vite` |
| Charts | Recharts 2 | `recharts` |
| Server state | TanStack Query v5 | `@tanstack/react-query` |
| HTTP client | ky | `ky` |
| Routing | React Router 7 | `react-router` |
| Date formatting | date-fns | `date-fns` |

**Rationale:**
- **Vite over Next.js:** Internal tool with zero SEO requirements. Vite is faster to scaffold, simpler to deploy (static files + API proxy), and avoids the complexity of SSR/RSC for an admin dashboard.
- **Recharts over Chart.js:** React-native component model, composable, good TypeScript support. Calibration charts (expected vs. actual accuracy) fit naturally into Recharts' line chart API.
- **ky over fetch:** Same HTTP client as mobile — consistent interceptor pattern for JWT injection and error handling.

---

## Project Structure

### Ops Dashboard Layout

```
ops/                                 # Vite React SPA (internal ops dashboard)
  public/
    favicon.svg
  src/
    main.tsx                         # Entry point, React Router + QueryClient
    App.tsx                          # Root layout with sidebar nav
    lib/
      api.ts                         # ky client with JWT auth (same pattern as mobile)
      query.ts                       # TanStack Query client config
      auth.ts                        # Auth store (JWT from shared backend)
      types.ts                       # Shared TypeScript types (metrics, decisions, alerts)
    pages/
      Dashboard.tsx                  # Overview: business/trust/supply/demand metric cards
      ReviewQueue.tsx                # Human review queue: disputes, escalations
      ReviewDetail.tsx               # Single review item: evidence viewer, approve/override
      AgentDecisions.tsx             # Decision log: filter by type, confidence, escalation
      AgentLearning.tsx              # Per-agent calibration charts, outcome rates, overrides
      Alerts.tsx                     # Alert feed: recent OpsAgent alerts, config
      Referrals.tsx                  # Referral stats: codes, conversions, payouts
    components/
      layout/
        Sidebar.tsx                  # Nav: Dashboard, Review Queue, Agents, Alerts, Referrals
        Header.tsx                   # User info, logout
      metrics/
        MetricCard.tsx               # Single metric: value, trend arrow, sparkline
        MetricGrid.tsx               # Grid of MetricCards for a category
      charts/
        CalibrationChart.tsx         # Expected vs. actual accuracy per confidence bucket
        TrendChart.tsx               # Time-series line chart (generic)
        BarChart.tsx                 # Bar chart wrapper (generic)
        GaugeChart.tsx               # Gauge for fund health, fraud rate
      review/
        ReviewList.tsx               # Sortable/filterable list of review items
        EvidenceViewer.tsx           # Photos, agent decision JSON, transaction context
        ActionButtons.tsx            # Approve, Override, Request More Info
      decisions/
        DecisionTable.tsx            # Filterable table of agent decisions
        DecisionDetail.tsx           # Expanded view with input/output/reasoning
      alerts/
        AlertFeed.tsx                # Reverse-chronological alert list
        AlertRuleEditor.tsx          # Edit threshold per alert type
  index.html
  vite.config.ts
  tailwind.config.ts
  tsconfig.json
  package.json
```

### Go Backend (New Packages)

```
backend/internal/
  agent/
    ops/
      agent.go                       # OpsAgent: health metrics, anomaly detection, alerts
      metrics.go                     # Metric collection queries (aggregate SQL)
      alerts.go                      # Alert evaluation, threshold comparison, routing
      slack.go                       # Slack webhook sender (net/http POST)
      pagerduty.go                   # PagerDuty event sender (pagerduty-go)
      handler.go                     # HTTP handlers for ops API endpoints
      repository.go                  # DB queries: metric snapshots, alert history
      model.go                       # HealthSnapshot, AlertRule, Alert, MetricValue types
      jobs.go                        # River periodic job: RunHealthCheck
      learning.go                    # Calibration calculations, outcome rates, fund health
      review.go                      # Human review queue logic
    fraud/
      agent.go                       # FraudAgent: signal detection, pattern analysis
      signals.go                     # Individual signal detectors (device, payment, phone, wifi)
      patterns.go                    # Cross-transaction pattern analysis (collusion, velocity)
      handler.go                     # HTTP handlers for fraud review endpoints
      repository.go                  # DB queries: fraud flags, signal history
      model.go                       # FraudSignal, FraudFlag, SignalBundle types
      jobs.go                        # River periodic job: RunPatternScan
  referral/
    service.go                       # Referral logic: generate code, track, validate, payout
    handler.go                       # HTTP handlers for referral endpoints
    repository.go                    # DB queries: referrals CRUD
    model.go                         # Referral, ReferralCode, ReferralPayout types
```

---

## Implementation Steps

### Step 7.1 — OpsAgent (backend)

**Create:**
- `backend/internal/agent/ops/model.go` — Domain types:
  - `HealthSnapshot` — timestamped collection of all metric categories (business, trust, supply, demand)
  - `MetricValue` — `{Name, Value float64, PreviousValue float64, Trend string, Period string}`
  - `AlertRule` — `{ID, MetricName, Operator (GT/LT/DEVIATION), Threshold float64, Severity (INFO/WARNING/CRITICAL), Channel (SLACK/PAGERDUTY/BOTH), Enabled bool}`
  - `Alert` — `{ID, RuleID, MetricName, CurrentValue, Threshold, Severity, Channel, FiredAt, AcknowledgedAt, AcknowledgedBy}`
- `backend/internal/agent/ops/repository.go` — Postgres queries:
  - `InsertHealthSnapshot` — store snapshot as JSONB in `ops_health_snapshots` table
  - `GetLatestSnapshot` — most recent snapshot
  - `GetSnapshotHistory(duration)` — snapshots within time range for trend charts
  - `ListAlertRules` — all configured alert rules
  - `UpsertAlertRule` — create or update alert rule
  - `InsertAlert` — record fired alert
  - `ListAlerts(filters)` — paginated alert history with severity/status filters
  - `AcknowledgeAlert(id, userID)` — mark alert as acknowledged
- `backend/internal/agent/ops/metrics.go` — Metric collection (raw SQL aggregations):
  - `CollectBusinessMetrics(ctx)` — queries: `COUNT(listings) WHERE status='ACTIVE'`, `COUNT(DISTINCT users) WHERE last_active_at > now()-interval '7 days'`, booking conversion `COUNT(status='COMPLETED')/COUNT(*)`, revenue sums, average transaction value, host payout velocity (avg time from completion to payout)
  - `CollectTrustMetrics(ctx)` — queries: fraud flag rate `COUNT(risk_flags != '[]')/COUNT(users)`, dispute rate `COUNT(status='DISPUTED')/COUNT(transactions)`, average agent confidence `AVG(confidence) FROM agent_decisions`, collusion alert count
  - `CollectSupplyMetrics(ctx)` — queries: new host signups (7d rolling), host churn rate (hosts with no activity in 30d / total hosts), listings per area (PostGIS grid), response rate distribution
  - `CollectDemandMetrics(ctx)` — queries: search-to-book conversion (requires search events, tracked via `search_events` or derived from transaction funnel), repeat renter rate, failed booking rate (auto-declines + cancellations)
  - `DetectAnomalies(current, history)` — compare current snapshot against rolling 7-day average. Flag any metric deviating >2 standard deviations from the mean
- `backend/internal/agent/ops/alerts.go` — Alert evaluation and routing:
  - `EvaluateRules(snapshot, rules)` — compare each metric in snapshot against configured thresholds. Returns list of triggered alerts
  - `RouteAlert(alert)` — dispatches to Slack, PagerDuty, or both based on alert severity and rule config
  - Alert severity mapping: `INFO` -> Slack only, `WARNING` -> Slack only, `CRITICAL` -> Slack + PagerDuty
- `backend/internal/agent/ops/slack.go` — Slack webhook sender:
  - `SendSlackAlert(webhookURL, alert)` — `net/http` POST with JSON payload: `{"text": "...", "blocks": [{"type": "section", "text": {"type": "mrkdwn", "text": "..."}}]}`. Includes metric name, current value, threshold, severity, timestamp. Retry once on 5xx
- `backend/internal/agent/ops/pagerduty.go` — PagerDuty event sender:
  - `SendPagerDutyAlert(routingKey, alert)` — uses `pagerduty-go` SDK to create Events API v2 trigger event. Dedup key is `alert.RuleID + alert.MetricName + truncated-hour` to prevent duplicate pages within the same hour
- `backend/internal/agent/ops/agent.go` — OpsAgent orchestrator:
  - `Run(ctx)` — collects all metrics, builds snapshot, stores in DB, evaluates alert rules, routes triggered alerts. Called by River periodic job
- `backend/internal/agent/ops/jobs.go` — River periodic job registration:
  - `HealthCheckArgs` — River job args struct
  - `HealthCheckWorker` — calls `agent.Run(ctx)`. Registered as periodic job running every 15 minutes
- `backend/internal/agent/ops/handler.go` — HTTP handlers returning `chi.Router`:
  - `GET /api/v1/ops/metrics/current` — latest health snapshot
  - `GET /api/v1/ops/metrics/history` — snapshot history (query param: `duration=7d|30d|90d`)
  - `GET /api/v1/ops/alerts` — paginated alert list (query params: `severity`, `acknowledged`, `page`, `limit`)
  - `PUT /api/v1/ops/alerts/:id/acknowledge` — acknowledge alert
  - `GET /api/v1/ops/alerts/rules` — list all alert rules
  - `PUT /api/v1/ops/alerts/rules/:id` — update alert rule (threshold, severity, enabled)

**Modify:**
- `backend/cmd/server/main.go` — Mount ops router at `/api/v1/ops`, register River periodic job for health checks
- `backend/internal/platform/config/config.go` — Add `SlackWebhookURL`, `PagerDutyRoutingKey`, `OpsHealthCheckInterval` fields

**Verify:**
```bash
cd backend && go vet ./...
cd backend && go build ./cmd/server
cd backend && go test ./internal/agent/ops/... -v -count=1
# Integration test:
docker compose up -d
cd backend && make dev &
sleep 3
# Trigger manual health check:
curl -sf http://localhost:8080/api/v1/ops/metrics/current \
  -H "Authorization: Bearer $OPS_TOKEN"
# Should return 200 with health snapshot JSON
# Verify alert rules:
curl -sf http://localhost:8080/api/v1/ops/alerts/rules \
  -H "Authorization: Bearer $OPS_TOKEN"
# Should return default alert rules
kill %1
```

### Step 7.2 — FraudAgent (backend)

**Create:**
- `backend/internal/agent/fraud/model.go` — Domain types:
  - `FraudSignal` — `{Type (DEVICE_FINGERPRINT|PAYMENT_INSTRUMENT|CARRIER_BATCH|SIMULTANEOUS_CREATION|EXCLUSIVE_PAIR|WIFI_NETWORK|DAMAGE_PATTERN|VALUE_SPIKE), UserID, RelatedUserID, Score int, IsCompoundOnly bool, Evidence JSONB, DetectedAt}`
  - `SignalBundle` — `{UserID, Signals []FraudSignal, CompoundScore int, HasNonCompoundSignal bool}` — aggregates signals per user. WiFi signal contributes to CompoundScore only when `HasNonCompoundSignal` is true
  - `FraudFlag` — `{ID, UserID, Signals []FraudSignal, TotalScore int, Action (MONITOR|FLAG|SUSPEND), CreatedAt, ResolvedAt, ResolvedBy}`
- `backend/internal/agent/fraud/signals.go` — Individual signal detectors:
  - `DetectSharedDeviceFingerprint(ctx, userID)` — query: `SELECT DISTINCT user_id FROM users WHERE device_fingerprint = (SELECT device_fingerprint FROM users WHERE id = $1) AND id != $1`. Score: +40
  - `DetectLinkedPaymentInstrument(ctx, userID)` — query against Stripe customer metadata (payment method fingerprints). Score: +50
  - `DetectCarrierBatchPhone(ctx, userID)` — check if phone number is within known carrier batch ranges (sequential numbers, same prefix + incrementing suffix). Score: +30
  - `DetectSimultaneousCreation(ctx, userID)` — query: `SELECT id FROM users WHERE ABS(EXTRACT(EPOCH FROM created_at - (SELECT created_at FROM users WHERE id = $1))) < 300 AND id != $1`. Score: +35 (accounts created within 5 minutes of each other)
  - `DetectWiFiNetwork(ctx, userID)` — query against signup metadata for matching network BSSID/SSID. Score: +30 BUT `IsCompoundOnly = true`. This score is added to CompoundScore ONLY when the bundle already contains at least one non-compound signal
  - `RunAllSignals(ctx, userID)` — runs all detectors, returns `SignalBundle`. WiFi scoring logic: if bundle has any signal where `IsCompoundOnly == false`, then WiFi's +30 is included in total. Otherwise WiFi is recorded but contributes 0 to the total score
- `backend/internal/agent/fraud/patterns.go` — Cross-transaction pattern analysis (runs on schedule):
  - `DetectExclusivePairs(ctx)` — query: find user pairs where >80% of their transactions are with each other and they have >3 transactions total. Score: +45
  - `DetectDamageAmountGaming(ctx)` — query: find users whose damage claims are within 5% of the hold amount on >50% of their claims. Score: +50
  - `DetectSerialDamage(ctx)` — query: find items where >60% of rentals result in damage claims. Score: +40
  - `DetectNewAccountValueSpike(ctx)` — query: find accounts <30 days old listing items with `estimated_value > $500` and >3 such listings. Score: +35
  - `RunPatternAnalysis(ctx)` — runs all pattern detectors across the platform
- `backend/internal/agent/fraud/agent.go` — FraudAgent orchestrator:
  - `EvaluateTransaction(ctx, transactionID)` — runs signal detection for both renter and host on a transaction. Writes `AgentDecision` with `agent_type = 'FRAUD'`. If total score exceeds threshold (configurable, default 80), flags the account
  - `EvaluateUser(ctx, userID)` — runs full signal + pattern analysis for a single user
  - `RunScheduledScan(ctx)` — runs pattern analysis across all active users, flags new detections
- `backend/internal/agent/fraud/repository.go` — DB queries:
  - `InsertFraudFlag(flag)` — insert into user's `risk_flags` JSONB array and create a `FraudFlag` record
  - `ListFraudFlags(filters)` — paginated list with status filters
  - `ResolveFraudFlag(id, resolvedBy, outcome)` — mark flag as resolved, update `outcome_correct` on linked `AgentDecision`
  - `GetUserFraudHistory(userID)` — all signals and flags for a user
- `backend/internal/agent/fraud/handler.go` — HTTP handlers returning `chi.Router`:
  - `GET /api/v1/ops/fraud/flags` — paginated fraud flags (query params: `status`, `severity`, `page`, `limit`)
  - `GET /api/v1/ops/fraud/flags/:id` — single fraud flag with full signal detail
  - `PUT /api/v1/ops/fraud/flags/:id/resolve` — resolve flag (body: `{outcome, notes}`)
  - `GET /api/v1/ops/fraud/users/:id/signals` — all fraud signals for a user
- `backend/internal/agent/fraud/jobs.go` — River periodic job:
  - `PatternScanArgs` — River job args
  - `PatternScanWorker` — calls `agent.RunScheduledScan(ctx)`. Registered as periodic job running every 6 hours

**Modify:**
- `backend/cmd/server/main.go` — Mount fraud router at `/api/v1/ops/fraud`, register River periodic job for pattern scans
- Booking flow (Phase 3 code) — add hook to call `fraud.EvaluateTransaction()` after transaction creation, before acceptance. Write `AgentDecision` record with results

**Verify:**
```bash
cd backend && go vet ./...
cd backend && go build ./cmd/server
cd backend && go test ./internal/agent/fraud/... -v -count=1
# Unit test WiFi compound-only logic:
# - WiFi signal alone -> score = 0 (recorded but not counted)
# - WiFi + shared device -> score = 40 (device) + 30 (wifi) = 70
# - WiFi + WiFi -> score = 0 (compound-only + compound-only = nothing)
# Integration test:
curl -sf http://localhost:8080/api/v1/ops/fraud/flags \
  -H "Authorization: Bearer $OPS_TOKEN"
# Should return 200 with paginated fraud flags
```

### Step 7.3 — Ops Dashboard (web)

**Create:**
- Vite project in `/ops`:
  ```bash
  cd ops && npm create vite@latest . -- --template react-ts
  npm install tailwindcss @tailwindcss/vite recharts @tanstack/react-query ky react-router date-fns
  ```
- `ops/vite.config.ts` — Vite config with Tailwind plugin, API proxy to `http://localhost:8080` in dev
- `ops/src/lib/api.ts` — ky client with `prefixUrl: '/api/v1'`, JWT auth header hook (reads token from localStorage)
- `ops/src/lib/query.ts` — TanStack Query client (staleTime: 30s for real-time feel, refetchInterval: 60s on dashboard)
- `ops/src/lib/auth.ts` — Auth state: login via shared backend `POST /api/v1/auth/login`, store JWT in localStorage, redirect to login on 401
- `ops/src/lib/types.ts` — TypeScript types mirroring Go models: `HealthSnapshot`, `MetricValue`, `Alert`, `AlertRule`, `AgentDecision`, `FraudFlag`, `Referral`, `CalibrationBucket`
- `ops/src/main.tsx` — App entry: React Router with QueryClientProvider, auth gate
- `ops/src/App.tsx` — Root layout: sidebar nav + content area
- `ops/src/components/layout/Sidebar.tsx` — Navigation: Dashboard, Review Queue, Agent Decisions, Agent Learning, Alerts, Referrals
- `ops/src/components/layout/Header.tsx` — Current user, logout button
- `ops/src/components/metrics/MetricCard.tsx` — Single metric display: large value, trend arrow (up/down/flat), percentage change, optional sparkline via Recharts `<Sparkline>`
- `ops/src/components/metrics/MetricGrid.tsx` — CSS Grid of MetricCards, 2-4 columns responsive
- `ops/src/pages/Dashboard.tsx` — Overview page:
  - Four metric grids: Business (active listings, active users, booking conversion, revenue, avg transaction, payout velocity), Trust & Safety (fraud flag rate, dispute rate, avg confidence, collusion alerts), Supply (new hosts 7d, churn rate, listings per area, response rate), Demand (search-to-book, repeat renter, failed booking rate)
  - TanStack Query hook: `useQuery({queryKey: ['metrics', 'current'], queryFn: () => api.get('ops/metrics/current').json(), refetchInterval: 60_000})`
  - Trend charts below grids: 7-day rolling line charts for key metrics
- `ops/src/pages/ReviewQueue.tsx` — Human review queue:
  - Table: transaction ID, type (dispute/escalation/fraud flag), submitted at, severity, status (pending/reviewed/overridden), assigned to
  - Filters: type, severity, status, date range
  - Sort: by severity (desc), by date (desc)
  - TanStack Query hook: `useQuery({queryKey: ['reviews', filters], queryFn: ...})`
- `ops/src/pages/ReviewDetail.tsx` — Single review item:
  - Transaction context: renter, host, listing, dates, amounts
  - Evidence viewer: check-in/check-out photos (side-by-side), agent decision JSON (collapsible), message history
  - Agent decision chain: all `AgentDecision` records for this transaction, ordered by `created_at`
  - Action buttons: Approve (confirm agent decision), Override (opens override form with reason select + notes), Request More Info (sends message to user)
  - Override form writes `AgentDecision` with `agent_type = 'HUMAN_OVERRIDE'`, `override_of = original_decision_id`
- `ops/src/pages/AgentDecisions.tsx` — Decision log viewer:
  - Filterable table: agent type, confidence range, escalated (yes/no), outcome (correct/incorrect/pending), date range
  - Columns: ID, agent type, transaction ID, confidence, escalated, outcome, created at
  - Click row to expand: full input JSON, decision JSON, model, prompt version, reasoning
- `ops/src/pages/AgentLearning.tsx` — Agent learning dashboard (PRD section 31):
  - Agent selector: dropdown to pick agent type (Risk, Verification, Appraisal, Dispute, Agreement, Fraud, LateReturn)
  - **Confidence calibration chart** (per agent): Recharts `<LineChart>` with two lines:
    - X-axis: confidence buckets (0.5-0.6, 0.6-0.7, 0.7-0.8, 0.8-0.9, 0.9-1.0)
    - Blue line: expected accuracy (bucket midpoint: 0.55, 0.65, 0.75, 0.85, 0.95)
    - Orange line: actual accuracy (`COUNT(outcome_correct=true) / COUNT(*)` per bucket)
    - Perfect calibration = lines overlap. Gap = calibration error
    - Shaded region between lines showing calibration error magnitude
  - **Outcome correctness rate**: bar chart per agent — percentage of decisions where `outcome_correct = true` over rolling 90-day window
  - **Override rate**: bar chart per agent — percentage of decisions that were overridden by human (`override_of IS NOT NULL`)
  - **Per-agent evaluation metrics** table (from PRD section 31): primary metric, secondary metric, current value, alert threshold, status (OK/WARNING/CRITICAL)
  - **Loss ratio trend**: line chart — guarantee fund claims / contributions over time
  - **Guarantee fund health**: gauge showing current balance vs. reserve ratio requirement
  - TanStack Query hooks:
    - `useQuery({queryKey: ['agents', 'calibration', agentType], queryFn: ...})` — calls `GET /api/v1/ops/agents/calibration?agent_type=RISK`
    - `useQuery({queryKey: ['agents', 'metrics', agentType], queryFn: ...})` — calls `GET /api/v1/ops/agents/metrics?agent_type=RISK`
- `ops/src/pages/Alerts.tsx` — Alert feed and configuration:
  - Reverse-chronological alert list: severity badge, metric name, current value vs. threshold, timestamp, acknowledged status
  - Click to acknowledge (PUT request)
  - Alert rule editor: table of rules with inline editing for threshold, severity, enabled toggle
- `ops/src/pages/Referrals.tsx` — Referral stats:
  - Summary metrics: total codes generated, total conversions, total payouts, conversion rate
  - Table: referrer name, referee name, code, status (pending/completed/paid/fraudulent), created at, completed at, payout amount
- `ops/src/components/charts/CalibrationChart.tsx` — Reusable calibration chart:
  - Props: `{buckets: CalibrationBucket[], agentType: string}`
  - Renders expected vs. actual accuracy with calibration error shading
  - Tooltip shows bucket details: decision count, correct count, expected %, actual %
- `ops/src/components/charts/TrendChart.tsx` — Generic time-series line chart
- `ops/src/components/charts/BarChart.tsx` — Generic bar chart wrapper
- `ops/src/components/charts/GaugeChart.tsx` — Semi-circular gauge for fund health / fraud rate
- `ops/src/components/review/EvidenceViewer.tsx` — Side-by-side photo viewer with zoom, plus JSON viewer for agent decisions
- `ops/src/components/review/ActionButtons.tsx` — Approve, Override, Request More Info buttons with confirmation modals

**Modify:**
- `docker-compose.yml` — (optional) add nginx service for local ops dashboard proxying in dev, OR rely on Vite dev server proxy

**Verify:**
```bash
cd ops && npm run build
cd ops && npx tsc --noEmit
# Dev verification:
cd ops && npm run dev &
# Navigate to http://localhost:5173
# Verify: Dashboard loads with metric cards
# Verify: Review Queue shows pending items
# Verify: Agent Learning shows calibration charts
# Verify: Alerts page shows alert feed
kill %1
```

### Step 7.4 — Referral System (backend + RN)

**Create:**
- `backend/internal/referral/model.go` — Domain types:
  - `ReferralCode` — `{Code string (8-char alphanumeric), UserID, CreatedAt, ExpiresAt, MaxUses int, UseCount int}`
  - `Referral` — `{ID, ReferralCodeID, ReferrerID, RefereeID, Status (PENDING|SIGNED_UP|FIRST_RENTAL_COMPLETED|PAID|FRAUDULENT), ReferrerPayout, RefereePayout, CompletedAt, PaidAt, CreatedAt}`
  - `ReferralPayout` — `{ID, ReferralID, UserID, Amount (fixed $20.00), Status (PENDING|PROCESSING|PAID|FAILED|BLOCKED), StripeTransferID, CreatedAt}`
- `backend/internal/referral/repository.go` — Postgres queries:
  - `InsertReferralCode(code)` — create referral code for user
  - `FindReferralCodeByCode(code)` — lookup by code string
  - `FindReferralCodeByUser(userID)` — lookup existing code for user (one per user)
  - `InsertReferral(referral)` — create referral tracking record
  - `UpdateReferralStatus(id, status)` — advance referral state
  - `FindReferralByReferee(userID)` — check if user was referred
  - `ListReferralsByReferrer(userID, page, limit)` — paginated referrals for a user
  - `InsertReferralPayout(payout)` — create payout record
  - `ListReferralPayouts(filters)` — paginated payouts for ops dashboard
- `backend/internal/referral/service.go` — Business logic:
  - `GenerateCode(ctx, userID)` — generate 8-char uppercase alphanumeric code, store in DB. Idempotent: returns existing code if user already has one
  - `ApplyReferralCode(ctx, refereeUserID, code)` — called during registration. Validates code exists, is not expired, has not exceeded max uses, referrer != referee. Creates `Referral` with status `SIGNED_UP`. Fraud check: shared device fingerprint between referrer and referee -> reject
  - `OnFirstRentalCompleted(ctx, userID)` — called by transaction completion hook. Checks if user has a referral with status `SIGNED_UP`. If yes: advances to `FIRST_RENTAL_COMPLETED`, triggers payout evaluation
  - `EvaluateAndPayout(ctx, referralID)` — fraud checks before payout:
    1. Shared device detection: referrer and referee have same `device_fingerprint` -> block, set status `FRAUDULENT`
    2. Same network detection: referrer and referee signed up from same WiFi network AND any other fraud signal -> block
    3. Velocity limit: referrer has received >5 referral payouts in the last 30 days -> delay (do not block, queue for manual review)
    4. If all checks pass: create two `ReferralPayout` records ($20 each for referrer and referee), enqueue River job to process Stripe transfers, advance status to `PAID`
  - `ProcessPayout(ctx, payoutID)` — River job handler: initiate Stripe transfer (debit platform balance, credit connected account). Update payout status to `PAID` or `FAILED`
- `backend/internal/referral/handler.go` — HTTP handlers returning `chi.Router`:
  - `POST /api/v1/referrals/code` — generate referral code for authenticated user
  - `GET /api/v1/referrals/code` — get user's existing referral code
  - `POST /api/v1/referrals/apply` — apply referral code during registration (body: `{code}`)
  - `GET /api/v1/referrals/mine` — list user's referrals (as referrer) with status
  - `GET /api/v1/ops/referrals` — (ops) paginated list of all referrals with fraud status
  - `GET /api/v1/ops/referrals/stats` — (ops) summary stats: total, conversion rate, payout total

**Modify:**
- `backend/cmd/server/main.go` — Mount referral router at `/api/v1/referrals` (user-facing) and `/api/v1/ops/referrals` (ops-facing)
- `backend/internal/user/service.go` (or registration flow) — call `referral.ApplyReferralCode()` when registration includes a referral code
- Transaction completion hook (Phase 3 code) — call `referral.OnFirstRentalCompleted()` after a user's first rental completes

**Create (React Native):**
- `mobile/app/(tabs)/(profile)/referrals.tsx` — Referral screen:
  - Referral code display with copy button
  - Share button: triggers RN Share sheet with deep link `rentmy://join?ref=CODE`
  - Referral status list: FlatList of referrals showing referee name, status, payout amount
  - Empty state: explains the $20/$20 reward
- `mobile/components/referral/ReferralCard.tsx` — Single referral row: referee avatar, name, status badge (pending/completed/paid), payout amount
- `mobile/lib/hooks/useReferrals.ts` — TanStack Query hooks:
  - `useReferralCode()` — `GET /api/v1/referrals/code` with auto-generation via `POST` if none exists
  - `useMyReferrals()` — `GET /api/v1/referrals/mine` with pagination

**Modify (React Native):**
- `mobile/app/(auth)/register.tsx` — Add optional referral code field. If deep link contains `ref` param, pre-fill the field. Calls `POST /api/v1/referrals/apply` after successful registration
- `mobile/app/(tabs)/(profile)/index.tsx` — Add "Invite Friends" button linking to referrals screen

**Verify:**
```bash
cd backend && go vet ./...
cd backend && go build ./cmd/server
cd backend && go test ./internal/referral/... -v -count=1
cd mobile && npx tsc --noEmit
# Integration test — full referral flow:
# 1. User A generates referral code
curl -sf -X POST http://localhost:8080/api/v1/referrals/code \
  -H "Authorization: Bearer $TOKEN_A"
# Returns: {"code": "ABCD1234"}
# 2. User B registers with referral code
curl -sf -X POST http://localhost:8080/api/v1/referrals/apply \
  -H "Authorization: Bearer $TOKEN_B" \
  -H 'Content-Type: application/json' \
  -d '{"code":"ABCD1234"}'
# Returns: 201 with referral record (status: SIGNED_UP)
# 3. User B completes first rental -> triggers payout evaluation
# 4. Both users receive $20 payout (verify via payout records)
```

### Step 7.5 — Agent Learning Endpoints (backend)

**Create:**
- `backend/internal/agent/ops/learning.go` — Agent learning metric calculations:
  - `CalculateCalibration(ctx, agentType, window)` — SQL query:
    ```sql
    SELECT
      CASE
        WHEN confidence >= 0.9 THEN '0.9-1.0'
        WHEN confidence >= 0.8 THEN '0.8-0.9'
        WHEN confidence >= 0.7 THEN '0.7-0.8'
        WHEN confidence >= 0.6 THEN '0.6-0.7'
        ELSE '0.5-0.6'
      END as bucket,
      COUNT(*) as total,
      COUNT(*) FILTER (WHERE outcome_correct = true) as correct,
      AVG(confidence) as avg_confidence
    FROM agent_decisions
    WHERE agent_type = $1
      AND outcome_correct IS NOT NULL
      AND created_at > NOW() - $2::interval
    GROUP BY bucket
    ORDER BY bucket
    ```
    Returns `[]CalibrationBucket{Bucket, Total, Correct, ExpectedAccuracy, ActualAccuracy, CalibrationError}`
  - `CalculateOutcomeRates(ctx, agentType, window)` — correctness rate, override rate, escalation rate per agent
  - `CalculateAgentEvalMetrics(ctx, agentType)` — per-agent primary and secondary metrics from PRD section 31 (e.g., AppraisalAgent: value accuracy + host override rate)
  - `CalculateGuaranteeFundHealth(ctx)` — current balance, reserve ratio, claims vs. contributions trend
  - `CalculateLossRatio(ctx, window)` — guarantee fund claims / contributions over time buckets

**Modify:**
- `backend/internal/agent/ops/handler.go` — Add endpoints:
  - `GET /api/v1/ops/agents/calibration` — query params: `agent_type`, `window` (default 90d). Returns calibration buckets
  - `GET /api/v1/ops/agents/metrics` — query params: `agent_type`. Returns eval metrics (primary, secondary, threshold, status)
  - `GET /api/v1/ops/agents/outcomes` — query params: `agent_type`, `window`. Returns outcome rates (correctness, override, escalation)
  - `GET /api/v1/ops/agents/fund-health` — returns guarantee fund health metrics
  - `GET /api/v1/ops/agents/loss-ratio` — query params: `window`. Returns loss ratio trend data

**Verify:**
```bash
cd backend && go test ./internal/agent/ops/... -v -count=1
# Integration test:
curl -sf "http://localhost:8080/api/v1/ops/agents/calibration?agent_type=RISK&window=90d" \
  -H "Authorization: Bearer $OPS_TOKEN"
# Should return calibration buckets with expected/actual accuracy
curl -sf "http://localhost:8080/api/v1/ops/agents/metrics?agent_type=DISPUTE" \
  -H "Authorization: Bearer $OPS_TOKEN"
# Should return: {primaryMetric: "Human override rate", primaryValue: 0.12, ...}
```

### Step 7.6 — Human Review Queue (backend)

**Create:**
- `backend/internal/agent/ops/review.go` — Review queue logic:
  - `ReviewItem` — `{ID, TransactionID, Type (DISPUTE|ESCALATION|FRAUD_FLAG), AgentDecisionID, Severity (LOW|MEDIUM|HIGH|CRITICAL), Status (PENDING|IN_REVIEW|RESOLVED|OVERRIDDEN), AssignedTo, Evidence JSONB, CreatedAt, ResolvedAt}`
  - `CreateReviewItem(ctx, item)` — called when any agent escalates (confidence below threshold) or when fraud flag is created
  - `ListReviewItems(ctx, filters)` — paginated, filterable by type/severity/status/assignee
  - `GetReviewItem(ctx, id)` — full review item with transaction context, all agent decisions, photos, messages
  - `ResolveReviewItem(ctx, id, resolution)` — mark resolved. If overriding: create `AgentDecision` with `agent_type = 'HUMAN_OVERRIDE'` and `override_of` pointing to original decision. Set `outcome_correct = false` on the overridden decision
  - `AssignReviewItem(ctx, id, userID)` — assign to ops team member

**Modify:**
- `backend/internal/agent/ops/handler.go` — Add endpoints:
  - `GET /api/v1/ops/reviews` — paginated review queue (query params: `type`, `severity`, `status`, `assignee`, `page`, `limit`)
  - `GET /api/v1/ops/reviews/:id` — single review item with full context
  - `PUT /api/v1/ops/reviews/:id/resolve` — resolve (body: `{action: "approve"|"override"|"request_info", reason?, notes?, overrideDecision?}`)
  - `PUT /api/v1/ops/reviews/:id/assign` — assign to user (body: `{assigneeId}`)
- Escalation hooks in existing agents (Phase 3-5 code) — when any agent sets `escalated = true` on an `AgentDecision`, also call `review.CreateReviewItem()` to enqueue for human review

**Verify:**
```bash
cd backend && go test ./internal/agent/ops/... -v -count=1
# Integration test:
curl -sf "http://localhost:8080/api/v1/ops/reviews?status=PENDING&severity=HIGH" \
  -H "Authorization: Bearer $OPS_TOKEN"
# Should return paginated list of pending high-severity review items
# Test override flow:
curl -sf -X PUT "http://localhost:8080/api/v1/ops/reviews/$REVIEW_ID/resolve" \
  -H "Authorization: Bearer $OPS_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"action":"override","reason":"Agreement clause misinterpreted","notes":"Renter not liable per custom section"}'
# Should return 200, create HUMAN_OVERRIDE decision, set outcome_correct=false on original
```

---

## API Endpoints

| Method | Path | Auth | Request Body | Response | Errors |
|--------|------|------|-------------|----------|--------|
| GET | `/api/v1/ops/metrics/current` | Yes (ops) | — | `{snapshot}` | 401, 403 |
| GET | `/api/v1/ops/metrics/history` | Yes (ops) | `?duration=7d` | `{snapshots[]}` | 401, 403 |
| GET | `/api/v1/ops/alerts` | Yes (ops) | `?severity=CRITICAL&page=1` | `{alerts[], total, page}` | 401, 403 |
| PUT | `/api/v1/ops/alerts/:id/acknowledge` | Yes (ops) | — | `{alert}` | 401, 403, 404 |
| GET | `/api/v1/ops/alerts/rules` | Yes (ops) | — | `{rules[]}` | 401, 403 |
| PUT | `/api/v1/ops/alerts/rules/:id` | Yes (ops) | `{threshold?, severity?, enabled?}` | `{rule}` | 400, 401, 403, 404 |
| GET | `/api/v1/ops/fraud/flags` | Yes (ops) | `?status=OPEN&page=1` | `{flags[], total, page}` | 401, 403 |
| GET | `/api/v1/ops/fraud/flags/:id` | Yes (ops) | — | `{flag, signals[]}` | 401, 403, 404 |
| PUT | `/api/v1/ops/fraud/flags/:id/resolve` | Yes (ops) | `{outcome, notes}` | `{flag}` | 400, 401, 403, 404 |
| GET | `/api/v1/ops/fraud/users/:id/signals` | Yes (ops) | — | `{signals[]}` | 401, 403, 404 |
| GET | `/api/v1/ops/reviews` | Yes (ops) | `?type=DISPUTE&severity=HIGH&page=1` | `{items[], total, page}` | 401, 403 |
| GET | `/api/v1/ops/reviews/:id` | Yes (ops) | — | `{item, transaction, decisions[], media[]}` | 401, 403, 404 |
| PUT | `/api/v1/ops/reviews/:id/resolve` | Yes (ops) | `{action, reason?, notes?}` | `{item, decision?}` | 400, 401, 403, 404 |
| PUT | `/api/v1/ops/reviews/:id/assign` | Yes (ops) | `{assigneeId}` | `{item}` | 400, 401, 403, 404 |
| GET | `/api/v1/ops/agents/calibration` | Yes (ops) | `?agent_type=RISK&window=90d` | `{buckets[]}` | 400, 401, 403 |
| GET | `/api/v1/ops/agents/metrics` | Yes (ops) | `?agent_type=RISK` | `{primary, secondary, threshold, status}` | 400, 401, 403 |
| GET | `/api/v1/ops/agents/outcomes` | Yes (ops) | `?agent_type=RISK&window=90d` | `{correctnessRate, overrideRate, escalationRate}` | 400, 401, 403 |
| GET | `/api/v1/ops/agents/fund-health` | Yes (ops) | — | `{balance, reserveRatio, claimsTrend[]}` | 401, 403 |
| GET | `/api/v1/ops/agents/loss-ratio` | Yes (ops) | `?window=90d` | `{dataPoints[]}` | 401, 403 |
| POST | `/api/v1/referrals/code` | Yes | — | `{code}` | 401, 409 already exists |
| GET | `/api/v1/referrals/code` | Yes | — | `{code}` | 401, 404 |
| POST | `/api/v1/referrals/apply` | Yes | `{code}` | `{referral}` | 400 invalid/expired, 401, 409 already referred |
| GET | `/api/v1/referrals/mine` | Yes | `?page=1&limit=20` | `{referrals[], total, page}` | 401 |
| GET | `/api/v1/ops/referrals` | Yes (ops) | `?status=PAID&page=1` | `{referrals[], total, page}` | 401, 403 |
| GET | `/api/v1/ops/referrals/stats` | Yes (ops) | — | `{totalCodes, totalConversions, totalPayouts, conversionRate}` | 401, 403 |

---

## Database Migrations

**Migration: `007_phase6_ops_referrals.sql`**

```sql
-- Ops health snapshots (time-series of platform health)
CREATE TABLE ops_health_snapshots (
    id              TEXT PRIMARY KEY,                -- ULID
    metrics         JSONB NOT NULL,                  -- Full HealthSnapshot: {business: {...}, trust: {...}, supply: {...}, demand: {...}}
    anomalies       JSONB NOT NULL DEFAULT '[]',     -- Detected anomalies: [{metricName, currentValue, expectedValue, deviation}]
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_ops_health_snapshots_created_at ON ops_health_snapshots(created_at DESC);

-- Alert rules (configurable thresholds)
CREATE TABLE alert_rules (
    id              TEXT PRIMARY KEY,                -- ULID
    metric_name     TEXT NOT NULL,                   -- e.g., "fraud_flag_rate", "booking_conversion"
    operator        TEXT NOT NULL CHECK (operator IN ('GT', 'LT', 'DEVIATION')),
    threshold       NUMERIC(10,4) NOT NULL,          -- e.g., 0.05 for 5% fraud rate
    severity        TEXT NOT NULL DEFAULT 'WARNING' CHECK (severity IN ('INFO', 'WARNING', 'CRITICAL')),
    channel         TEXT NOT NULL DEFAULT 'SLACK' CHECK (channel IN ('SLACK', 'PAGERDUTY', 'BOTH')),
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Fired alerts (audit trail)
CREATE TABLE alerts (
    id              TEXT PRIMARY KEY,                -- ULID
    rule_id         TEXT NOT NULL REFERENCES alert_rules(id),
    metric_name     TEXT NOT NULL,
    current_value   NUMERIC(10,4) NOT NULL,
    threshold       NUMERIC(10,4) NOT NULL,
    severity        TEXT NOT NULL,
    channel         TEXT NOT NULL,
    fired_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    acknowledged_at TIMESTAMPTZ,
    acknowledged_by TEXT REFERENCES users(id)
);
CREATE INDEX idx_alerts_fired_at ON alerts(fired_at DESC);
CREATE INDEX idx_alerts_severity ON alerts(severity);

-- Fraud flags (per-user fraud investigations)
CREATE TABLE fraud_flags (
    id              TEXT PRIMARY KEY,                -- ULID
    user_id         TEXT NOT NULL REFERENCES users(id),
    signals         JSONB NOT NULL DEFAULT '[]',     -- [{type, score, evidence, isCompoundOnly}]
    total_score     INTEGER NOT NULL DEFAULT 0,
    action          TEXT NOT NULL DEFAULT 'MONITOR' CHECK (action IN ('MONITOR', 'FLAG', 'SUSPEND')),
    agent_decision_id TEXT REFERENCES agent_decisions(id),
    resolved_at     TIMESTAMPTZ,
    resolved_by     TEXT REFERENCES users(id),
    resolution_notes TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_fraud_flags_user_id ON fraud_flags(user_id);
CREATE INDEX idx_fraud_flags_action ON fraud_flags(action);
CREATE INDEX idx_fraud_flags_created_at ON fraud_flags(created_at DESC);

-- Review queue items (human review of escalated decisions)
CREATE TABLE review_items (
    id                  TEXT PRIMARY KEY,            -- ULID
    transaction_id      TEXT REFERENCES transactions(id),
    type                TEXT NOT NULL CHECK (type IN ('DISPUTE', 'ESCALATION', 'FRAUD_FLAG')),
    agent_decision_id   TEXT REFERENCES agent_decisions(id),
    severity            TEXT NOT NULL DEFAULT 'MEDIUM' CHECK (severity IN ('LOW', 'MEDIUM', 'HIGH', 'CRITICAL')),
    status              TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'IN_REVIEW', 'RESOLVED', 'OVERRIDDEN')),
    assigned_to         TEXT REFERENCES users(id),
    evidence            JSONB NOT NULL DEFAULT '{}', -- Contextual data for the reviewer
    resolved_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_review_items_status ON review_items(status);
CREATE INDEX idx_review_items_severity ON review_items(severity);
CREATE INDEX idx_review_items_created_at ON review_items(created_at DESC);

-- Referral codes (one per host)
CREATE TABLE referral_codes (
    id              TEXT PRIMARY KEY,                -- ULID
    user_id         TEXT NOT NULL REFERENCES users(id) UNIQUE,
    code            TEXT NOT NULL UNIQUE,            -- 8-char alphanumeric, e.g., "ABCD1234"
    max_uses        INTEGER NOT NULL DEFAULT 100,
    use_count       INTEGER NOT NULL DEFAULT 0,
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_referral_codes_code ON referral_codes(code);

-- Referrals (tracks referrer -> referee relationship and status)
CREATE TABLE referrals (
    id              TEXT PRIMARY KEY,                -- ULID
    referral_code_id TEXT NOT NULL REFERENCES referral_codes(id),
    referrer_id     TEXT NOT NULL REFERENCES users(id),
    referee_id      TEXT NOT NULL REFERENCES users(id),
    status          TEXT NOT NULL DEFAULT 'PENDING'
                    CHECK (status IN ('PENDING', 'SIGNED_UP', 'FIRST_RENTAL_COMPLETED', 'PAID', 'FRAUDULENT')),
    completed_at    TIMESTAMPTZ,
    paid_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(referee_id)                               -- A user can only be referred once
);
CREATE INDEX idx_referrals_referrer_id ON referrals(referrer_id);
CREATE INDEX idx_referrals_status ON referrals(status);

-- Referral payouts (tracks individual $20 payouts)
CREATE TABLE referral_payouts (
    id              TEXT PRIMARY KEY,                -- ULID
    referral_id     TEXT NOT NULL REFERENCES referrals(id),
    user_id         TEXT NOT NULL REFERENCES users(id),  -- Who receives the payout
    amount          NUMERIC(10,2) NOT NULL DEFAULT 20.00,
    status          TEXT NOT NULL DEFAULT 'PENDING'
                    CHECK (status IN ('PENDING', 'PROCESSING', 'PAID', 'FAILED', 'BLOCKED')),
    stripe_transfer_id TEXT,
    failure_reason  TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_referral_payouts_referral_id ON referral_payouts(referral_id);
CREATE INDEX idx_referral_payouts_user_id ON referral_payouts(user_id);
CREATE INDEX idx_referral_payouts_status ON referral_payouts(status);

-- Seed default alert rules
INSERT INTO alert_rules (id, metric_name, operator, threshold, severity, channel) VALUES
    ('01SEED0001ALERTRULE00001', 'fraud_flag_rate', 'GT', 0.01, 'CRITICAL', 'BOTH'),
    ('01SEED0001ALERTRULE00002', 'booking_conversion', 'LT', 0.50, 'WARNING', 'SLACK'),
    ('01SEED0001ALERTRULE00003', 'supply_density_min', 'LT', 5.0, 'WARNING', 'SLACK'),
    ('01SEED0001ALERTRULE00004', 'agent_confidence_avg', 'LT', 0.70, 'WARNING', 'SLACK'),
    ('01SEED0001ALERTRULE00005', 'payout_failure_rate', 'GT', 0.05, 'CRITICAL', 'BOTH'),
    ('01SEED0001ALERTRULE00006', 'guarantee_fund_ratio', 'LT', 0.20, 'CRITICAL', 'BOTH');
```

---

## Risks and Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| Metric aggregation queries slow on large datasets | Dashboard latency, DB load | Use materialized views or pre-computed snapshots (OpsAgent already stores snapshots). Aggregate queries run in River job, not in HTTP request path |
| Slack webhook rate limiting | Missed alerts during spikes | Batch alerts within a 60-second window. Deduplicate by rule+metric within the same hour |
| PagerDuty alert fatigue | On-call ignores real incidents | Strict CRITICAL-only routing. Dedup key prevents repeated pages for the same issue within one hour. Review thresholds monthly |
| WiFi compound-only logic edge cases | False negatives (real collusion missed) or false positives | Comprehensive unit tests for all signal combinations. WiFi+WiFi = 0, WiFi+device = 70, WiFi alone = 0. Log all signal bundles for audit |
| Referral fraud (self-referral) | Loss of $40 per fraudulent referral | Multi-signal detection: shared device fingerprint, same IP at registration, same payment instrument. Velocity limit: max 5 payouts per referrer per 30 days |
| Ops dashboard auth bypass | Unauthorized access to internal data | Same JWT auth as mobile. Add `ops_role` check middleware — only users with ops role can access `/api/v1/ops/*` endpoints |
| Calibration data insufficient early on | Meaningless calibration charts | Show "insufficient data" state when a bucket has <10 decisions. Require 90 days of data before surfacing calibration error alerts |
| Recharts bundle size | Slow dashboard load | Recharts is ~200KB gzipped. Acceptable for internal tool. Lazy-load chart pages with React.lazy if needed |
| Referral code collision | Duplicate codes fail unique constraint | Generate code, retry on conflict (up to 3 attempts). 8-char alphanumeric = 36^8 = 2.8 trillion combinations — collision probability is negligible |
| Guarantee fund health calculation accuracy | Incorrect reserve ratio display | Use database SUM aggregations (not application-level math). Cross-verify with Stripe balance in daily reconciliation job |

---

## Testing Strategy

- **Unit tests (Go):**
  - WiFi compound-only scoring: all combinations (WiFi alone = 0, WiFi + device = 70, WiFi + carrier = 60, WiFi + WiFi = 0, device alone = 40)
  - Alert rule evaluation: GT/LT/DEVIATION operators with edge cases (exactly at threshold, one above, one below)
  - Calibration calculation: mock decision data with known outcomes, verify bucket accuracy and calibration error
  - Referral code generation: uniqueness, format validation (8-char alphanumeric)
  - Referral fraud detection: shared device -> blocked, velocity limit -> delayed, clean referral -> paid
  - Payout amount: always $20.00 for both referrer and referee
  - Signal bundle scoring: verify CompoundScore correctly includes/excludes WiFi based on HasNonCompoundSignal flag

- **Integration tests (Go):**
  - OpsAgent end-to-end: seed test data -> run health check -> verify snapshot stored -> verify alerts fired for threshold breaches
  - FraudAgent end-to-end: create two users with shared device fingerprint -> run signal detection -> verify fraud flag created with correct score
  - Referral flow: create code -> apply code -> complete rental -> verify payouts created with correct amounts and statuses
  - Review queue: create escalated agent decision -> verify review item auto-created -> resolve with override -> verify HUMAN_OVERRIDE decision created and original outcome_correct set to false
  - Slack webhook: mock HTTP server -> send alert -> verify payload format matches Slack block kit schema
  - PagerDuty: mock pagerduty-go client -> send critical alert -> verify event payload includes routing key, severity, dedup key

- **Repository tests:**
  - All new tables: CRUD operations against test Postgres
  - Alert rules: upsert idempotency
  - Referral constraints: unique referee, unique code per user
  - Fraud flags: compound queries for signal aggregation

- **Ops dashboard (TypeScript):**
  - `npx tsc --noEmit` — full type check
  - Verify all pages render without errors (manual or Playwright smoke test)
  - Calibration chart renders correctly with mock data (expected vs. actual lines, shaded error region)
  - Review queue CRUD operations work end-to-end through the dashboard UI

- **React Native:**
  - `npx tsc --noEmit` — type check
  - Referral screen: code display, share sheet trigger, referral list rendering
  - Registration with referral code: pre-fill from deep link, submission to API

---

## Implementation Order

| Step | What | Day | Depends On |
|------|------|-----|------------|
| 6.1 | OpsAgent + metrics + alerts (backend) | Day 1-4 | Phase 6 complete |
| 6.2 | FraudAgent + signals + patterns (backend) | Day 3-6 | 6.1 (shares ops alert routing) |
| 6.3 | Ops dashboard (web) | Day 5-10 | 6.1, 6.2 (needs API endpoints to render) |
| 6.4 | Referral system (backend + RN) | Day 5-8 | Phase 6 complete (needs transaction completion hook) |
| 6.5 | Agent learning endpoints (backend) | Day 7-9 | 6.1 (extends ops handler) |
| 6.6 | Human review queue (backend) | Day 8-10 | 6.1, 6.2 (review items created by agent escalations) |

Steps 6.1 and 6.4 are independent and can start in parallel (ops metrics vs. referral system).
Step 7.3 (dashboard) can start UI scaffolding on Day 5 while backend endpoints are still being built — mock data first, wire to real API as endpoints land.
Steps 6.5 and 6.6 extend 6.1's handler and can be developed in parallel with each other once 6.1 is complete.
