# Audit: Ops Dashboard + Backend Ops Tools
**Task:** 8.15  
**Date:** 2026-04-11  
**Auditor:** Claude Sonnet 4.6  
**Branch:** task-8.15-audit-ops-dashboard

---

## Audit Method

Chrome MCP (`mcp__Control_Chrome__*`) was not available in this environment. The audit was performed using:
- Direct `curl` calls to all backend ops API endpoints (with fresh JWT token)
- Code inspection of all ops dashboard pages (`ops/src/pages/`)
- Code inspection of backend handler, repository, and migration files
- Comparison of dashboard API call shapes vs backend response shapes

**Backend:** Running at `http://localhost:8080` (health: `{"status":"ok","postgres":"connected"}`)  
**Ops Dashboard:** Running at `http://localhost:5173` (Vite dev server started for session)

---

## Summary: Bugs Found

| # | Severity | Area | Description |
|---|----------|------|-------------|
| 1 | HIGH | Backend | `GET /ops/metrics/history` returns 500 "failed to fetch history" |
| 2 | HIGH | Backend | `GET /ops/alerts` returns 500 "failed to fetch alerts" |
| 3 | HIGH | Backend | `GET /ops/alerts/rules` returns 500 "failed to fetch alert rules" |
| 4 | HIGH | Dashboard | `AgentDecisions.tsx` calls wrong path `/ops/agents/decisions` (backend: `/admin/agents/decisions`) |
| 5 | HIGH | Dashboard | `AgentLearning.tsx` calls wrong path `/ops/agents/calibration` (backend: `/admin/agents/calibration`) |
| 6 | HIGH | Dashboard | `AgentLearning.tsx` calls wrong path `/ops/guarantee/health` (backend: `/admin/guarantee-fund/health`) |
| 7 | HIGH | Backend | `GET /api/v1/ops/agents/metrics` — endpoint not implemented (404) |
| 8 | MEDIUM | Dashboard | `Dashboard.tsx` expects `HealthSnapshot[]` from history but backend returns `{"snapshots": [...]}` |
| 9 | MEDIUM | Dashboard | `Alerts.tsx` expects `Alert[]` from alerts but backend returns `{"alerts": [...]}` |
| 10 | MEDIUM | Dashboard | `Alerts.tsx` expects `AlertRule[]` from alert rules but backend returns `{"rules": [...]}` |
| 11 | MEDIUM | Dashboard | `ReviewQueue.tsx` expects `FraudFlag[]` from fraud flags but backend returns `{"flags": [...], "total": ..., "page": ...}` |
| 12 | MEDIUM | Dashboard | `Referrals.tsx` expects `Referral[]` from referrals list but backend returns `{"referrals": [...], ...}` |

---

## Section 1: Ops Dashboard Pages

### 1.1 Dashboard (`/`)

**File:** `ops/src/pages/Dashboard.tsx`

| Query | Endpoint Called | Expected Type | Backend Returns | Status |
|-------|----------------|---------------|-----------------|--------|
| Current metrics | `GET /api/v1/ops/metrics/current` | `HealthSnapshot` | `HealthSnapshot` (direct) | ✓ Route works, 200 |
| History | `GET /api/v1/ops/metrics/history?duration=7d` | `HealthSnapshot[]` | `{"snapshots": HealthSnapshot[]}` | BUG #1 + #8 |

**Bug #1:** History endpoint returns HTTP 500 "failed to fetch history" — root cause is a runtime DB error (likely the `ops_health_snapshots` table has no seed data and/or a DB query is failing; investigation in task 8.16).

**Bug #8:** Even if history returned 200, `Dashboard.tsx:20` does `.json<HealthSnapshot[]>()` but the backend wraps the array: `{"snapshots": snaps}`. The frontend would receive an object, not an array. Charts would fail to render.

---

### 1.2 Alerts (`/alerts`)

**File:** `ops/src/pages/Alerts.tsx`

| Query | Endpoint Called | Expected Type | Backend Returns | Status |
|-------|----------------|---------------|-----------------|--------|
| Alerts list | `GET /api/v1/ops/alerts` | `Alert[]` | `{"alerts": Alert[]}` | BUG #2 + #9 |
| Alert rules | `GET /api/v1/ops/alerts/rules` | `AlertRule[]` | `{"rules": AlertRule[]}` | BUG #3 + #10 |
| Acknowledge (mutation) | `PUT /api/v1/ops/alerts/{alertId}/acknowledge` | — | `{}` | ✓ Route exists |
| Update rule (mutation) | `PUT /api/v1/ops/alerts/rules/{ruleId}` | — | `AlertRule` | ✓ Route exists |

**Bug #2:** `GET /ops/alerts` returns HTTP 500. The DB query against `ops_alerts` is failing at runtime.

**Bug #3:** `GET /ops/alerts/rules` returns HTTP 500. The DB query against `ops_alert_rules` is failing at runtime.

**Bug #9:** Response format mismatch — `Alerts.tsx:26` does `.json<Alert[]>()` but backend wraps: `{"alerts": alerts}`. Even after the 500 is fixed, the frontend will receive `undefined` when trying to loop over the response as an array.

**Bug #10:** Response format mismatch — `Alerts.tsx:33` does `.json<AlertRule[]>()` but backend wraps: `{"rules": rules}`.

---

### 1.3 Review Queue (`/reviews`)

**File:** `ops/src/pages/ReviewQueue.tsx`

| Query | Endpoint Called | Expected Type | Backend Returns | Status |
|-------|----------------|---------------|-----------------|--------|
| Fraud flags | `GET /api/v1/ops/fraud/flags` | `FraudFlag[]` | `{"flags": FraudFlag[], "total": int, "page": int}` | BUG #11 |

**Bug #11:** `ReviewQueue.tsx:30` calls `.json<FraudFlag[]>()` but `fraud/handler.go:67` returns `{"flags": flags, "total": total, "page": page}`. The table will render empty.

---

### 1.4 Review Detail (`/reviews/{flagId}`)

**File:** `ops/src/pages/ReviewDetail.tsx`

| Query | Endpoint Called | Status |
|-------|----------------|--------|
| Single flag | `GET /api/v1/ops/fraud/flags/{flagId}` | ✓ Returns direct object |

No bugs found in this page.

---

### 1.5 Agent Decisions (`/agents/decisions`)

**File:** `ops/src/pages/AgentDecisions.tsx`

| Query | Endpoint Called | Backend Path | Status |
|-------|----------------|--------------|--------|
| Decisions | `GET /api/v1/ops/agents/decisions` | `/api/v1/admin/agents/decisions` | BUG #4 — 404 |

**Bug #4:** Path prefix mismatch. Dashboard calls `/ops/agents/decisions` but backend registers the route at `/admin/agents/decisions` (`outcome/handler.go:27`). Returns 404.

---

### 1.6 Agent Learning (`/agents/learning`)

**File:** `ops/src/pages/AgentLearning.tsx`

| Query | Endpoint Called | Backend Path | Status |
|-------|----------------|--------------|--------|
| Calibration | `GET /api/v1/ops/agents/calibration` | `/api/v1/admin/agents/calibration` | BUG #5 — 404 |
| Agent metrics | `GET /api/v1/ops/agents/metrics` | NOT IMPLEMENTED | BUG #7 — 404 |
| Guarantee health | `GET /api/v1/ops/guarantee/health` | `/api/v1/admin/guarantee-fund/health` | BUG #6 — 404 |

**Bug #5:** Calibration path mismatch — `/ops/agents/calibration` → `/admin/agents/calibration`.

**Bug #6:** Guarantee health path mismatch — `/ops/guarantee/health` → `/admin/guarantee-fund/health`.

**Bug #7:** `GET /ops/agents/metrics` is not implemented anywhere in the backend. No handler exists for this endpoint. The `AgentLearning` page's agent performance bar charts will always fail to load.

---

### 1.7 Referrals (`/referrals`)

**File:** `ops/src/pages/Referrals.tsx`

| Query | Endpoint Called | Expected Type | Backend Returns | Status |
|-------|----------------|---------------|-----------------|--------|
| Stats | `GET /api/v1/ops/referrals/stats` | `ReferralStats` | `ReferralStats` (direct) | ✓ Route works |
| List | `GET /api/v1/ops/referrals` | `Referral[]` | `{"referrals": Referral[], "page": int, "limit": int}` | BUG #12 |

**Bug #12:** `Referrals.tsx:28` calls `.json<Referral[]>()` but `referral/handler.go:132` returns `{"referrals": refs, "page": page, "limit": limit}`. The table will render empty.

---

### 1.8 Login (`/login`)

**File:** `ops/src/pages/Login.tsx`

No API issues identified. The 401-redirect is handled in `lib/api.ts` via `afterResponse` hook. Login page render itself is not blocked.

---

## Section 2: Backend Ops Endpoints

### 2.1 Endpoint Availability Matrix

| Endpoint | HTTP Method | HTTP Status | Notes |
|----------|-------------|-------------|-------|
| `/api/v1/ops/metrics/current` | GET | 200 | Working, returns full `HealthSnapshot` |
| `/api/v1/ops/metrics/history?duration=7d` | GET | **500** | "failed to fetch history" — DB error |
| `/api/v1/ops/alerts` | GET | **500** | "failed to fetch alerts" — DB error |
| `/api/v1/ops/alerts/rules` | GET | **500** | "failed to fetch alert rules" — DB error |
| `/api/v1/ops/fraud/flags` | GET | 200 | Returns wrapped `{"flags": [...]}` |
| `/api/v1/ops/fraud/flags/{id}` | GET | 200 | Returns direct object |
| `/api/v1/ops/fraud/users/{id}/signals` | GET | 200 | Returns wrapped `{"signals": [...], "flags": [...]}` |
| `/api/v1/ops/referrals/stats` | GET | 200 | Returns direct `ReferralStats` object |
| `/api/v1/ops/referrals` | GET | 200 | Returns wrapped `{"referrals": [...]}` |
| `/api/v1/admin/agents/decisions` | GET | 200 | Working (wrong path in dashboard) |
| `/api/v1/admin/agents/calibration` | GET | 200 | Working (wrong path in dashboard) |
| `/api/v1/ops/agents/metrics` | GET | **404** | Not implemented |
| `/api/v1/admin/guarantee-fund/health` | GET | 200 | Working (wrong path in dashboard) |

### 2.2 Root Cause Analysis for 500 Errors

The three endpoints returning 500 (`history`, `alerts`, `alert rules`) all query tables that require the `ops_health_check` periodic job to have run at least once (to populate `ops_health_snapshots`) or for alert rules to be seeded. The most likely root cause is:

- **`ops_health_snapshots`**: Empty table (job not run yet). The `GetSnapshotHistory` query returns no rows but should not error unless the table is missing or a column type mismatch exists.
- **`ops_alert_rules` / `ops_alerts`**: Empty tables would return nil without error. A 500 suggests either the table doesn't exist (migration not applied) or a runtime scan error.

The root cause investigation and fixes belong in task 8.16.

---

## Section 3: Periodic Jobs

Verified registration in `backend/app/server.go`:

| Job ID | Schedule | Worker Location | Status |
|--------|----------|-----------------|--------|
| `ops_health_check` | Configurable interval (env: `OPS_HEALTH_CHECK_INTERVAL_M`) | `agent/ops/jobs.go` | ✓ Registered |
| `fraud_pattern_scan` | Every 6 hours | `agent/fraud/jobs.go` | ✓ Registered |
| `guarantee_fund_health_check` | Every 1 hour | `guaranteefund/jobs.go` | ✓ Registered |
| `guarantee_fund_loss_ratio_check` | Every 24 hours | `guaranteefund/jobs.go` | ✓ Registered |

Both required jobs (`ops_health_check` and `fraud_pattern_scan`) are registered. The 15-minute default for `ops_health_check` depends on `OPS_HEALTH_CHECK_INTERVAL_M` env var — if unset, the server may panic or skip registration.

---

## Section 4: Coherence Checks

| Check | Result |
|-------|--------|
| `metrics/current` reflects actual DB state (5 listings, 3 users) | Partial — `activeListings: 0` despite 5 listings seeded; `activeUsers: 3` ✓ |
| Fraud flags reference real users | N/A — no fraud flags in DB (no completed bookings) |
| Agent decisions link to real transactions | N/A — no agent decisions in DB yet |
| Guarantee fund health matches ledger entries | Endpoint path broken (404 via `/ops/guarantee/health`) |
| Alert rules cover 16 documented metrics | 500 error prevents verification |

**Coherence Bug:** `activeListings: 0` despite 5 seeded listings. This means `OpsAgent.HealthCheckJob` hasn't run and/or the metric counting query for active listings is incorrect.

---

## Bug List for Task 8.16

Priority order for fixes:

1. **Fix 3 backend 500s** — investigate DB errors for history, alerts, and alert rules endpoints; likely schema mismatch or missing seed data
2. **Fix 3 path mismatches in dashboard** — unify prefix: either rename backend routes from `/admin/*` to `/ops/*`, or update dashboard API calls to `/admin/*`
3. **Implement `GET /api/v1/ops/agents/metrics`** — add backend handler for agent performance metrics used by AgentLearning page
4. **Fix 5 response format mismatches** — unwrap arrays in frontend queries for history, alerts, alert rules, fraud flags, and referrals list
5. **Investigate `activeListings: 0`** — verify the OpsAgent listing-count SQL query matches actual listings table state
