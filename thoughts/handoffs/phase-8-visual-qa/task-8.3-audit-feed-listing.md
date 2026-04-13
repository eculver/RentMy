# Handoff: Task 8.3 — Audit: Feed + Listing Detail + Checkout

**Status:** Completed  
**Branch:** `task-8.3-audit-feed-listing`  
**Commit:** `a643ee0`  
**Date:** 2026-04-11  
**Mode:** Graphite (gt)

---

## What Was Done

Static code audit of the feed, listing detail, and checkout screens plus all components under test. The iOS Simulator was not actively running (infra not live), so the audit was performed by reading the source code against the phase plan checklist and PRD requirements.

**Deliverable:** `thoughts/audits/phase-8-visual-qa/audit-feed-listing.md`

---

## Bugs Found (9 total)

| ID | Severity | Summary |
|----|----------|---------|
| B1 | BLOCKER | KYCGate blocks checkout for all PENDING seeded users |
| B2 | MAJOR | PaymentMethodSelector passes same secret for both Stripe fields |
| B3 | MEDIUM | confirmation.tsx is dead code — checkout routes to booking-status instead |
| B4 | MEDIUM | "View My Bookings" in confirmation.tsx routes to feed, not rentals |
| B5 | MINOR | Location error state has no retry button |
| B6 | MINOR | Platform fee not shown in CostBreakdown |
| B7 | MINOR | Hold shows $0.00 until estimate loads (no loading state) |
| B8 | MINOR | "Edit Listing" button is a no-op for hosts |
| B9 | MINOR | Listing detail shows only one photo (thumbnail-only, no photo array in API) |

---

## Key Findings for Task 8.4

The most impactful fix is **B1** (seed users as VERIFIED) because it unblocks the entire checkout flow for manual testing. Without it, no one can reach the payment or booking submission steps.

**B2** (Stripe secret mapping) requires both frontend and backend changes:
- Backend `POST /api/v1/payments/setup` must create a Stripe Customer Ephemeral Key and return it separately from the SetupIntent client secret
- Frontend `PaymentMethodSelector.native.tsx` must use the ephemeral key for `customerEphemeralKeySecret`

**B3** (confirmation.tsx dead code) needs a routing decision: the confirmation screen at `listing/[id]/confirmation.tsx` already has the right UI and expects the right params — the fix is to route there from checkout success instead of `booking-status`.

---

## Files Touched

- `.claude/progress.json` — task 8.3 set to `completed`
- `thoughts/audits/phase-8-visual-qa/audit-feed-listing.md` — audit deliverable (new)

---

## Next Task

**8.4 — Fix: Feed + Listing Detail + Checkout Bugs** — implement fixes for all 9 bugs documented in audit-feed-listing.md, prioritized in the order listed in the audit's "Priority Order for Fixes" section.
