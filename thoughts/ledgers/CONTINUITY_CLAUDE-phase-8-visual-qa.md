# Phase 8 — Visual QA, Bug Fixing & v0 Stabilization

## Status

| Task | Name | Status | Branch | Commit |
|------|------|--------|--------|--------|
| 8.0 | Bootstrap: Full Stack + iOS Simulator + Seed Data | completed | — | see progress.json |
| 8.1 | Audit: Auth Flow (Login + Register) | completed | task-8.1-audit-auth | see progress.json |
| 8.2 | Fix: Auth Flow Bugs | completed | task-8.2-fix-auth-bugs | fd74788 |
| 8.3 | Audit: Feed + Listing Detail + Checkout | completed | task-8.3-audit-feed-listing | a643ee0 |
| 8.4 | Fix: Feed + Listing Detail + Checkout Bugs | completed | task-8.4-fix-feed-listing-checkout-bugs | 2bb354a |
| 8.5 | Audit: Search + Map | completed | task-8.5-audit-search-map | 035f291 |
| 8.6 | Fix: Search + Map Bugs | completed | task-8.6-fix-search-map-bugs | 5cbb9a8 |
| 8.7 | Audit: Booking + Handoff | completed | task-8.7-audit-booking-handoff | 3bd3319 |
| 8.8 | Fix: Booking + Handoff Bugs | completed | task-8.8-fix-booking-handoff-bugs | c33a4ec |
| 8.9–8.19 | Remaining fix/audit/docs/verification tasks | pending | — | — |

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

## Search + Map Audit Findings (task 8.5)

5 bugs documented in `thoughts/audits/phase-8-visual-qa/audit-search-map.md`.

## Search + Map Fixes (task 8.6)

All 5 audit bugs resolved. Key context for future tasks:

- `MapScreen.native.tsx`: `useEffect` initializes `bounds` from lat/lng on mount; `retry` wired to error state Retry button
- `(search)/index.tsx`: location-loading guard before result render; TextInput now controlled with `inputValue` state
- `handler.go`: `minID(listings)` helper; `feed` + `search` endpoints return `nextCursor` (min ULID = last in DB order)
- `useDiscovery.ts`: `FeedResponse.nextCursor?: string`; `getNextPageParam` uses `nextCursor` with fallback
- Pre-existing TS error in `(profile)/index.tsx` (router path types) — not introduced by 8.6

---

## Booking + Handoff Audit Findings (task 8.7)

8 bugs documented in `thoughts/audits/phase-8-visual-qa/audit-booking-handoff.md`. 3 medium, 5 low severity. No critical bugs. All flows structurally sound; full simulator testing not possible (no seeded bookings). Key bugs for task 8.8:

- BUG-BH-1: cost breakdown $0 on first start date selection (onChangeStart else-branch doesn't call setAmounts)
- BUG-BH-3: Maps URL uses listing ULID not coordinates (backend needs to return listing address)
- BUG-BH-4: "Report an issue" shows stale placeholder Alert (should navigate to dispute screen)

---

## Graphite Mode

All tasks using Graphite (`gt`). Commands: `gt create`, `gt modify --no-edit`, `gt submit --no-edit`.
