# Commit 356ec39 — Rating system

**Task:** 6.4 — Rating System (backend + RN)

## Why this commit exists

PRD §15 requires structured bubble-tag ratings after each completed rental. This commit
adds the full vertical slice: DB → service → HTTP → mobile components.

## Key decisions

1. **Reputation integration via EmitSignal (no River job)** — `riskSvc.EmitSignal` already
   handles insert + recalculate synchronously. A separate River job would be redundant and
   add complexity for no gain.

2. **Non-fatal signal emit on failure** — Rating submission and reputation update are
   decoupled: a transient signal failure doesn't roll back the rating. Monthly reputation
   recalculation provides a safety net.

3. **Handler uses auth.UserIDFromContext** — Consistent with every other authed handler in
   the codebase. No service-level user context injection needed.
