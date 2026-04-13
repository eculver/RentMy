# Commit f85b033 — Reasoning

**Task:** 8.15 — Audit: Ops Dashboard + Backend Ops Tools

## Why this commit

This is a pure audit commit: no production code was changed. The audit documents the current state of the ops dashboard + backend ops endpoints so task 8.16 has a precise, prioritized bug list to fix.

## What was audited

- All 8 ops dashboard pages were inspected (Dashboard, Alerts, ReviewQueue, ReviewDetail, AgentDecisions, AgentLearning, Referrals, Login)
- All 13 backend ops-area API endpoints were tested via curl with fresh JWT token
- All periodic (River) job registrations were verified in `backend/app/server.go`
- Database migration schemas were cross-checked against repository query columns

## Key findings

1. **3 backend 500 errors** — `history`, `alerts`, and `alert-rules` endpoints all fail at DB query time; the ops tables exist in migrations but the runtime queries are failing (root cause: likely no seed data for alert rules, or a column type mismatch that only surfaces at scan time)
2. **4 dashboard route mismatches** — AgentDecisions and AgentLearning pages call `/ops/agents/*` and `/ops/guarantee/health` but backend routes live at `/admin/agents/*` and `/admin/guarantee-fund/health`; these return 404
3. **1 missing backend endpoint** — `GET /api/v1/ops/agents/metrics` is called by AgentLearning but never implemented
4. **5 response format mismatches** — dashboard expects direct arrays but backend returns wrapped objects (`{"snapshots": []}`, `{"alerts": []}`, `{"rules": []}`, `{"flags": []}`, `{"referrals": []}`)

## What was NOT done

Chrome MCP (`mcp__Control_Chrome__*`) was unavailable in this environment, so visual inspection of rendered pages was replaced with code + API inspection. The audit document notes this limitation. The bugs found are code-level and would manifest as blank/broken UI sections in Chrome.
