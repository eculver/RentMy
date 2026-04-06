# Commit Reasoning: 9bc1807

## What
Agent Learning Framework outcome linking and calibration (PRD §31, Task 6.7).

## Why
Every agent decision needs a feedback loop to measure accuracy. Without outcome linking, we can't know if agents are making correct decisions or if their confidence scores are well-calibrated. This is the foundation for prompt evolution and operational monitoring.

## Design Decisions
1. **Redis for calibration metrics** — Calibration data is computed from rolling 90-day windows and accessed frequently by the admin dashboard. Redis provides fast reads with automatic TTL cleanup, avoiding stale data.

2. **ScheduleOutcomeLink as a package-level function** — Follows the same pattern as `latereturn.ScheduleLateReturnCheck()` and `notification.SchedulePickupApproaching()`. This allows both dispute and booking services to schedule outcome linking without importing each other.

3. **Agent-specific evaluation in the service, not per-agent packages** — Centralizing evaluation rules avoids circular imports (outcome→dispute→outcome) and keeps all correctness logic in one place for easy auditing.

4. **48h delay via River ScheduledAt** — Matches PRD specification. Gives time for real-world outcomes to materialize (e.g., disputes to be filed, returns to complete) before evaluating correctness.

5. **No new migration needed** — The `agent_decisions` table already has `outcome_id` and `outcome_correct` columns from migration 006. Calibration data lives in Redis.
