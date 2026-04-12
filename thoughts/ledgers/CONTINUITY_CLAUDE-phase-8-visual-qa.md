# Phase 8 — Visual QA, Bug Fixing & v0 Stabilization

## Status

| Task | Name | Status | Branch | Commit |
|------|------|--------|--------|--------|
| 8.0 | Bootstrap: Full Stack + iOS Simulator + Seed Data | completed | — | see progress.json |
| 8.1 | Audit: Auth Flow (Login + Register) | completed | task-8.1-audit-auth | see progress.json |
| 8.2 | Fix: Auth Flow Bugs | completed | task-8.2-fix-auth-bugs | fd74788 |
| 8.3 | Audit: Feed + Listing Detail + Checkout | completed | task-8.3-audit-feed-listing | a643ee0 |
| 8.4 | Fix: Feed + Listing Detail + Checkout Bugs | completed | task-8.4-fix-feed-listing-checkout-bugs | 2bb354a |
| 8.5–8.19 | Remaining audit/fix/docs/verification tasks | pending | — | — |

---

## Key Context

### Test Seed Data (from task 8.0)

| User | Email | Password | User ID |
|------|-------|----------|---------|
| Alice Test | alice@test.com | password123 | 01KNZ4649E3NV0PJB82YHG38CQ |
| Bob Renter | bob@test.com | password123 | 01KNZ468WYRDRAGT1VSA3B4GHA |

**5 listings seeded** (all owned by Alice) — DeWalt Drill, Canon R6 Camera, Paddleboard, Tent, Pressure Washer. No bookings or messages (Stripe placeholder keys prevent booking creation).

### Known Limitations from 8.0

- No bookings created (Stripe placeholder keys)
- No message threads (require a booking)
- Simulator location: 34.0522, -118.2437 (Los Angeles)
- App bypasses auth gate on fresh install (goes to feed instead of login)

---

## Auth Flow Fixes (task 8.2)

7 bugs fixed across login/register flow. See `thoughts/handoffs/phase-8-visual-qa/` for details.

---

## Feed + Listing + Checkout Audit Findings (task 8.3)

9 bugs documented in `thoughts/audits/phase-8-visual-qa/audit-feed-listing.md`. All 9 fixed in task 8.4.

## Feed + Listing + Checkout Fixes (task 8.4)

All 9 audit bugs resolved. Checkout flow is now end-to-end testable with seeded users.

Key changes for future context:
- Migration 016 sets seeded users to VERIFIED (unblocks KYCGate)
- Backend `POST /api/v1/payments/setup` now returns `{customerId, ephemeralKey, clientSecret}`
- `GET /api/v1/listings/:id` now returns `photos: string[]`
- Checkout routes to `confirmation.tsx` (not `booking-status`) after booking success
- `useLocation` exposes a `retry()` callback

---

## Graphite Mode

All tasks using Graphite (`gt`). Commands: `gt create`, `gt modify --no-edit`, `gt submit --no-edit`.
