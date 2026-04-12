# Commit 2d4491d — Reasoning

**Task:** 8.16 — Fix: Ops Dashboard + Backend Ops Tools Bugs  
**Branch:** task-8.16-fix-ops-dashboard-bugs

## Why this commit

All 12 bugs identified in the audit (task 8.15) are addressed in a single commit because they are a cohesive set of targeted fixes with no logical dependencies on each other. Splitting into multiple commits would produce noise without clarity.

## Key design decisions

### BUG-7 — Agent metrics endpoint implementation

The `GET /ops/agents/metrics` endpoint is implemented entirely within the `ops` package rather than adding it to the `outcome` package (which already has calibration data). The ops handler has a single dependency (`opsRepo *Repository`), and adding a new cross-package dependency would require changing `app/server.go` and the handler constructor. Instead, the ops repository directly queries `agent_decisions` — this is acceptable in a monolith where all tables live in the same database. The query is a 90-day aggregate that is read-only and index-friendly (filtered by `created_at` and `agent_type`).

### nil slice vs empty slice

Go's `nil` slice serializes to `null` in JSON. Returning `null` from list endpoints is technically valid per JSON spec, but React Query callbacks calling `.map()` on null throw errors. Initializing with `make([]T, 0)` is the idiomatic fix and costs nothing at runtime.

### CalibrationBucket field names

The `types.ts` type was independently authored and didn't match the Go model's JSON tags. Since there were no existing integration tests catching this (the endpoint was 404ing due to wrong URL), it was never caught. Fixed in place — no backward-compat concern since this is an internal admin tool.

### FundHealth type mismatch

The dashboard's `FundHealth` interface referenced `currentBalance`, `reserveRequired`, `contributions`, `claims` — none of which match the backend's JSON output (`balance`, `outstandingGaps`, `reserveRatio`, `lossRatio`, `action`). Updated to match. The gauge semantics are preserved: balance shown against outstanding-gaps as the "target", and loss ratio as a health indicator.
