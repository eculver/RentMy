# Phase 8 — Visual QA, Bug Fixing & v0 Stabilization

## Status

| Task | Name | Status | Branch | Commit |
|------|------|--------|--------|--------|
| 8.0 | Bootstrap: Full Stack + iOS Simulator + Seed Data | completed | — | see progress.json |
| 8.1 | Audit: Auth Flow (Login + Register) | completed | task-8.1-audit-auth | see progress.json |
| 8.2 | Fix: Auth Flow Bugs | completed | task-8.2-fix-auth-bugs | fd74788 |
| 8.3 | Audit: Feed + Listing Detail + Checkout | completed | task-8.3-audit-feed-listing | a643ee0 |
| 8.4–8.19 | Remaining audit/fix/docs/verification tasks | pending | — | — |

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

9 bugs documented in `thoughts/audits/phase-8-visual-qa/audit-feed-listing.md`.

**Critical for task 8.4:**
1. **B1 (BLOCKER):** All seeded users have `identity_status = 'PENDING'` — KYCGate blocks checkout entirely. Fix: seed users as VERIFIED.
2. **B2 (MAJOR):** `PaymentMethodSelector.native.tsx` passes same Stripe secret for both `customerEphemeralKeySecret` and `setupIntentClientSecret`. Backend must return an ephemeral key.
3. **B3 (MEDIUM):** `confirmation.tsx` is dead code — checkout routes to `booking-status` instead. Fix: route checkout success to `confirmation.tsx`.
4. **B4 (MEDIUM):** "View My Bookings" in `confirmation.tsx` routes to feed instead of rentals.

---

## Graphite Mode

All tasks using Graphite (`gt`). Commands: `gt create`, `gt modify --no-edit`, `gt submit --no-edit`.
