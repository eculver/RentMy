# Task 8.15 Handoff — Audit: Ops Dashboard + Backend Ops Tools

**Status:** Completed  
**Commit:** f85b033  
**Branch:** task-8.15-audit-ops-dashboard  
**Date:** 2026-04-11

---

## What Was Done

Performed a full audit of the ops dashboard (`ops/src/`) and backend ops API endpoints (`/api/v1/ops/*`, `/api/v1/admin/agents/*`, `/api/v1/admin/guarantee-fund/*`).

**Audit method:** Code inspection + direct curl API tests (Chrome MCP unavailable in this environment).

---

## Bugs Found (12 total, for task 8.16 to fix)

### High Severity — Backend 500s

**BUG-1:** `GET /api/v1/ops/metrics/history?duration=7d` → HTTP 500 "failed to fetch history"  
- Handler: `backend/internal/agent/ops/handler.go:71`  
- Likely cause: DB query against `ops_health_snapshots` fails at runtime; table is empty and scan may be returning a `rows.Err()` value

**BUG-2:** `GET /api/v1/ops/alerts` → HTTP 500 "failed to fetch alerts"  
- Handler: `backend/internal/agent/ops/handler.go:106`

**BUG-3:** `GET /api/v1/ops/alerts/rules` → HTTP 500 "failed to fetch alert rules"  
- Handler: `backend/internal/agent/ops/handler.go:146`

### High Severity — Dashboard 404s (Path Prefix Mismatch)

**BUG-4:** `AgentDecisions.tsx:19` calls `api.get('ops/agents/decisions')` → 404  
- Backend route is at `GET /api/v1/admin/agents/decisions` (`outcome/handler.go:27`)  
- Fix: change dashboard call to `admin/agents/decisions`

**BUG-5:** `AgentLearning.tsx:42` calls `api.get('ops/agents/calibration')` → 404  
- Backend route is at `GET /api/v1/admin/agents/calibration` (`outcome/handler.go:25-26`)  
- Fix: change dashboard call to `admin/agents/calibration`

**BUG-6:** `AgentLearning.tsx:52` calls `api.get('ops/guarantee/health')` → 404  
- Backend route is at `GET /api/v1/admin/guarantee-fund/health` (`guaranteefund/handler.go:26`)  
- Fix: change dashboard call to `admin/guarantee-fund/health`

**BUG-7:** `AgentLearning.tsx:47` calls `api.get('ops/agents/metrics')` → 404  
- Endpoint not implemented in backend at all  
- Fix: implement `GET /api/v1/ops/agents/metrics` handler (returns per-agent metrics: accuracy rate, decision count, avg confidence)

### Medium Severity — Dashboard Response Format Mismatches

**BUG-8:** `Dashboard.tsx:20` does `.json<HealthSnapshot[]>()` on history — backend returns `{"snapshots": HealthSnapshot[]}`  
- Fix: change to `.json<{snapshots: HealthSnapshot[]}>()` then extract `.snapshots`

**BUG-9:** `Alerts.tsx:26` does `.json<Alert[]>()` — backend `ops/handler.go:110` returns `{"alerts": Alert[]}`  
- Fix: `.json<{alerts: Alert[]}>()` then extract `.alerts`

**BUG-10:** `Alerts.tsx:33` does `.json<AlertRule[]>()` — backend returns `{"rules": AlertRule[]}`  
- Fix: `.json<{rules: AlertRule[]}>()` then extract `.rules`

**BUG-11:** `ReviewQueue.tsx:30` does `.json<FraudFlag[]>()` — backend `fraud/handler.go:67` returns `{"flags": FraudFlag[], "total": int, "page": int}`  
- Fix: `.json<{flags: FraudFlag[]}>()` then extract `.flags`

**BUG-12:** `Referrals.tsx:28` does `.json<Referral[]>()` — backend `referral/handler.go:132` returns `{"referrals": Referral[], "page": int, "limit": int}`  
- Fix: `.json<{referrals: Referral[]}>()` then extract `.referrals`

---

## What's Working

- `GET /api/v1/ops/metrics/current` → 200, correct data
- `GET /api/v1/ops/fraud/flags` → 200 (response wrapping bug but route works)
- `GET /api/v1/ops/fraud/flags/{id}` → 200, direct object
- `GET /api/v1/ops/referrals/stats` → 200, direct object
- `GET /api/v1/admin/agents/decisions` → 200 (wrong path in dashboard)
- `GET /api/v1/admin/agents/calibration` → 200 (wrong path in dashboard)
- `GET /api/v1/admin/guarantee-fund/health` → 200 (wrong path in dashboard)
- Login page → renders correctly, auth flow functional
- All 4 periodic jobs registered: `ops_health_check`, `fraud_pattern_scan`, `guarantee_fund_health_check`, `guarantee_fund_loss_ratio_check`

---

## Audit Deliverable

`thoughts/audits/phase-8-visual-qa/audit-ops-tools.md` — full audit document with section-by-section breakdown, endpoint matrix, and prioritized fix list.

---

## Next Task

**Task 8.16 — Fix: Ops Dashboard + Backend Ops Tools Bugs**  
Dependencies: 8.15 (this task)  
All 12 bugs above are scoped for 8.16.
