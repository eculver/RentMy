# Commit Reasoning: LateReturnAgent

## What
Added the LateReturnAgent — an autonomous agent that monitors rentals past their scheduled end, auto-charges hourly late fees from the pre-authorized hold, and escalates to DisputeAgent when warranted.

## Why
PRD §19 requires automated late return handling to reduce host anxiety and ensure timely fee collection. The agent eliminates manual intervention for the common case (renter is slightly late) while providing intelligent escalation for serious situations (potential theft, unresponsive renter).

## Key Design Decisions

### Damage reserve cap (60/40 split)
Late fee captures are capped at `holdAmount * 0.6` (configurable via `DAMAGE_RESERVE_RATE_BPS`). This ensures DisputeAgent always has at least 40% of the hold available for damage claims, even after maximum late fees. The cap is enforced in `maxLateFeeCap()` with comprehensive test coverage.

### Two-stage job architecture
1. **LateReturnCheckJob** — fires at `scheduled_end`, charges the first hourly increment, then re-enqueues itself every 60 minutes for continued charging.
2. **LateReturnEscalationJob** — fires when `hoursOverdue >= escalationThreshold` (default 4h). Calls Claude Sonnet for a nuanced escalation decision considering renter responsiveness, reputation, item value, and time of day.

This separation keeps the deterministic fee calculation separate from the LLM-based escalation judgment, following the existing pattern from DisputeAgent.

### Conservative escalation defaults
The LLM prompt emphasizes conservative behavior — premature escalation damages trust. FLAGGED_FOR_REVIEW (potential theft) is designed to be extremely rare. When the LLM call fails, the system defaults to WARNING rather than escalating.

### Double rate for conflicts
Per PRD §19, if the late rental causes a conflict with another ACCEPTED/ACTIVE booking for the same listing, the hourly rate doubles. This is checked via `HasConflictingBooking()`.

## Files Changed
- **New:** `backend/internal/latereturn/` — full domain package (model, repository, service, jobs, prompt, handler)
- **New:** `backend/migrations/012_late_returns.sql` — late_returns table
- **Modified:** `backend/app/server.go` — wire LateReturnAgent workers and handler
- **Modified:** `backend/internal/platform/config/config.go` — add late return config fields
- **Modified:** `backend/internal/booking/service.go` — schedule LateReturnCheckJob on booking acceptance
