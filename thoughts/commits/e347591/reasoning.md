# Commit e347591 — Reasoning

## What
Audit document for Phase 8 Task 8.11: Rentals + Disputes + Ratings screens. 9 bugs documented, including 3 critical protocol mismatches between Go backend JSON serialization and TypeScript frontend type expectations.

## Why this approach
iOS Simulator was not running (no active Expo dev server), so the audit was performed via static analysis: reading all five rentals screens, four rental/rating components, two backend packages (dispute, rating), and four mobile hooks.

Static analysis was sufficient to uncover structural bugs that would not have been obvious from visual screenshots alone (field name mismatches, wrong status values, missing navigation branches).

## Key decisions
- Prioritized backend/frontend schema mismatches over visual issues — these would produce silent failures at runtime that are harder to notice visually.
- Noted that `PhotoDiffResult` is a complete, tested component that is simply never rendered — an integration gap, not a component bug.
- Did not flag the `primary-*` NativeWind color classes as a bug; confirmed `primary` is defined in `tailwind.config.js`.
- Noted seeding limitations (no bookings possible without Stripe) that prevent live visual testing. This is pre-existing and documented in the Phase 8 appendix.

## Bugs to fix in 8.12
See `thoughts/audits/phase-8-visual-qa/audit-rentals-disputes.md` for full details. Priority order:
1. BUG-1: Add JSON tags to `Rating` struct (critical — hasRated always false)
2. BUG-2: Align dispute status values (critical — timeline always shows wrong state)
3. BUG-3: Fix dispute field name mismatches (critical — charge/confidence/verdict all null)
4. BUG-4: checkout navigates to wrong screen (high — return-confirmation unreachable)
5. BUG-5: DISPUTED row navigates to wrong screen (high — dispute-status unreachable)
6. BUG-6: Hardcoded zero hold allocation (medium)
7. BUG-7: PhotoDiffResult never rendered (medium)
8. BUG-8: INCONCLUSIVE re-prompt stub (low)
9. BUG-9: Dead textColor prop in HoldStatusCard (low)
