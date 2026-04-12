# Task 8.16 Handoff — Fix: Ops Dashboard + Backend Ops Tools Bugs

**Status:** Completed  
**Commit:** 2d4491d  
**Branch:** task-8.16-fix-ops-dashboard-bugs  
**Date:** 2026-04-12

---

## What Was Done

Fixed all 12 bugs documented in audit task 8.15. Changes span backend (ops package) and ops dashboard frontend (React/TypeScript).

---

## Backend Changes

### BUG-1/2/3: Nil slice JSON serialization (`backend/internal/agent/ops/repository.go`)

`GetSnapshotHistory`, `ListAlerts`, and `ListAlertRules` were returning `nil` Go slices when tables were empty. This serializes to JSON `null` instead of `[]`, breaking client-side array assumptions.

**Fix:** Changed `var snaps []T` → `snaps := make([]T, 0)` in all three methods.

### BUG-7: Missing `GET /api/v1/ops/agents/metrics` endpoint

The endpoint was called by `AgentLearning.tsx` but didn't exist in the backend. Added:

- **`model.go`**: `AgentMetrics` struct (JSON response type), `agentMetricSpec` struct, and `agentSpecs` map defining per-agent primary/secondary metric thresholds.
- **`repository.go`**: `GetAgentMetrics()` method — queries `agent_decisions` (90-day window, excluding HUMAN_OVERRIDE and OPS pseudo-agents), computes correctness rate, override rate, and status (OK/WARNING/CRITICAL) using `metricStatus()` helper.
- **`handler.go`**: `handleGetAgentMetrics()` handler + route registration at `GET /ops/agents/metrics`.

Status thresholds per agent type:
| Agent Type   | Correctness Threshold | Override Threshold |
|--------------|----------------------|-------------------|
| RISK         | 85%                  | 20%               |
| VERIFICATION | 95%                  | 5%                |
| APPRAISAL    | 80%                  | 10%               |
| DISPUTE      | 85%                  | 15%               |
| AGREEMENT    | 90%                  | 12%               |
| FRAUD        | 90%                  | 8%                |
| LATE_RETURN  | 85%                  | 25%               |

---

## Frontend Changes

### BUG-4: `AgentDecisions.tsx` — wrong URL + query param name

- URL: `ops/agents/decisions` → `admin/agents/decisions`
- Query param: `agent_type` → `agentType` (matches backend `r.URL.Query().Get("agentType")`)
- Defensive rendering: `d.transactionId.slice(0, 10)` → `d.transactionId ? ...slice : '-'`; `d.confidence * 100` → `d.confidence != null ? ... : '-'`

### BUG-5: `AgentLearning.tsx` — calibration wrong URL + response format

- URL: `ops/agents/calibration` (with `agent_type` query param) → `admin/agents/calibration/${selected}` (path param)
- Response: `.json<CalibrationBucket[]>()` → `.json<{ buckets: CalibrationBucket[] }>().then(r => r.buckets)`

### BUG-6: `AgentLearning.tsx` — guarantee fund wrong URL + type mismatch

- URL: `ops/guarantee/health` → `admin/guarantee-fund/health`
- `FundHealth` interface updated from `{currentBalance, reserveRequired, contributions, claims}` → `{balance, outstandingGaps, reserveRatio, lossRatio, action}` (matches backend `guaranteefund.FundHealth`)
- Gauge usage updated to `fund.balance` and `fund.outstandingGaps`

### BUG-8: `Dashboard.tsx` — metrics history response wrapper

- `.json<HealthSnapshot[]>()` → `.json<{ snapshots: HealthSnapshot[] }>().then(r => r.snapshots)`

### BUG-9: `Alerts.tsx` — alerts list response wrapper

- `.json<Alert[]>()` → `.json<{ alerts: Alert[] }>().then(r => r.alerts)`

### BUG-10: `Alerts.tsx` — alert rules response wrapper

- `.json<AlertRule[]>()` → `.json<{ rules: AlertRule[] }>().then(r => r.rules)`

### BUG-11: `ReviewQueue.tsx` — fraud flags response wrapper

- `.json<FraudFlag[]>()` → `.json<{ flags: FraudFlag[] }>().then(r => r.flags)`

### BUG-12: `Referrals.tsx` — referrals list response wrapper

- `.json<Referral[]>()` → `.json<{ referrals: Referral[] }>().then(r => r.referrals)`

### Additional: `CalibrationBucket` type + `CalibrationChart` field names

The `CalibrationBucket` type in `types.ts` had wrong field names vs backend:
- `bucketMin` → `bucketLow`
- `bucketMax` → `bucketHigh`
- `decisionCount` → `totalDecisions`
- `correctCount` → `correctDecisions`

Added: `agentType`, `calibrationError`, `updatedAt` fields.

`CalibrationChart.tsx` updated to use correct field names.

---

## Verification

- `cd backend && go vet ./...` → passed
- `cd backend && go build -o /dev/null ./cmd/server` → passed
- `cd backend && go test ./internal/agent/ops/...` → passed
- `cd ops && tsc --noEmit` → passed

---

## Next Task

**Task 8.17 — Documentation Cleanup & Consolidation**  
Dependencies: none (parallel with completed bug fix tasks)
