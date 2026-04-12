# Handoff: Task 8.5 — Audit: Search + Map

**Status:** Completed  
**Branch:** `task-8.5-audit-search-map`  
**Commit:** `035f291`  
**Date:** 2026-04-11  
**Mode:** Graphite (gt)

---

## What Was Done

Static code audit of the search and map screens, their shared components, hooks, and backend discovery service. Identified 5 bugs in priority order.

**Recovery note:** progress.json for task 8.4 was not updated in the previous session. Fixed at session start (task 8.4 → `completed`, commitSha `9be90b4`) before creating this branch.

---

## Bugs Found

| ID | Severity | File | Description |
|----|----------|------|-------------|
| B1 | MAJOR | MapScreen.native.tsx | No markers on initial load — bounds stay null until user pans |
| B2 | MEDIUM | MapScreen.native.tsx | Map error state missing Retry button |
| B3 | MEDIUM | (search)/index.tsx | Shows "No results" while location is loading (TanStack v5: isLoading=false when disabled) |
| B4 | MINOR | (search)/index.tsx | Search TextInput uncontrolled — no `value` prop |
| B5 | MINOR | useDiscovery.ts + service.go | Pagination cursor from re-ranked list, not DB-ordered list |

Full details with root cause, code excerpts, and fixes: `thoughts/audits/phase-8-visual-qa/audit-search-map.md`

---

## Files Changed

| File | Change |
|------|--------|
| `thoughts/audits/phase-8-visual-qa/audit-search-map.md` | New — full audit report |
| `.claude/progress.json` | task 8.4 → completed (delayed from prev session); task 8.5 → completed |

---

## Key Context for Task 8.6

- **B1 fix location:** `MapScreen.native.tsx` — add `useEffect` to initialize bounds from `lat`/`lng` on mount
- **B2 fix location:** `MapScreen.native.tsx` — destructure `retry` from `useLocation()`, render `Pressable` in error state
- **B3 fix location:** `(search)/index.tsx` — add `loading: locationLoading` from `useLocation()`, guard results render
- **B4 fix location:** `(search)/index.tsx` — add local `inputValue` state, bind `value={inputValue}`
- **B5 fix location:** Backend and/or frontend — cursor pagination needs to align with sort order; simplest fix is to not re-sort server-side and return DB-ordered results, or switch to offset pagination (more expensive)

---

## Next Task

**8.6 — Fix: Search + Map Bugs** — implement fixes for all 5 bugs above.
