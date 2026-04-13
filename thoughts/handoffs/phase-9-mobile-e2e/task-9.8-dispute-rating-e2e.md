# Task 9.8 — E2E: Dispute & Rating Flows

**Status:** completed  
**Commit:** ea0ac4a  
**Branch:** task-9.8-dispute-rating-e2e  
**Date:** 2026-04-12

---

## What was built

### Maestro E2E flows

| File | Flow | Seed |
|------|------|------|
| `disputes/file-dispute.yaml` | Seed ACTIVE booking → login as renter → Rentals → tap ACTIVE row → tap "Report an issue" → dispute screen → select DAMAGE → enter description → submit → assert dispute-status screen + timeline | `seed-booking-active.yaml` |
| `disputes/view-dispute-status.yaml` | Seed DISPUTED booking → login as renter → Rentals → tap DISPUTED row → dispute-status screen → assert timeline + "Filed" step → back to rentals | `seed-dispute.yaml` |
| `ratings/rate-counterparty.yaml` | Seed COMPLETED booking → login as renter → Rentals → tap COMPLETED row → return-confirmation → tap "Rate this rental" → rate screen → select GOOD_COMMUNICATION bubble → submit → assert success message | `seed-booking-completed.yaml` |

### New seed infrastructure

| File | Purpose |
|------|---------|
| `mobile/e2e/scripts/seed-dispute.js` | Calls `POST /api/v1/test/dispute` — creates COMPLETED booking + PENDING dispute, sets transaction status to DISPUTED. Outputs `TRANSACTION_ID` and `DISPUTE_ID`. |
| `mobile/e2e/helpers/seed-dispute.yaml` | Maestro helper wrapping `seed-dispute.js` |

### Backend changes

| File | Change |
|------|--------|
| `backend/internal/testonly/handler.go` | Added `createTestDispute` handler + `POST /test/dispute` route. Creates COMPLETED-state transaction (yesterday's dates), inserts dispute row with `PENDING` status and `DAMAGE` reason, updates transaction status to `DISPUTED`. Returns `transactionId` and `disputeId`. |

### testID additions

| Screen/Component | New testIDs |
|-----------------|-------------|
| `dispute.tsx` | `screen-dispute`, `dispute-reason-{DAMAGE,MISSING_ITEM,OTHER}` (on each reason Pressable), `input-dispute-description`, `btn-submit-dispute` |
| `dispute-status.tsx` | `screen-dispute-status`, `btn-back-to-rentals-dispute` |
| `DisputeTimeline.tsx` | `dispute-timeline` (on outer container View) |
| `rate.tsx` | `screen-rate`, `rating-bubbles-container`, `rating-success-message`, `btn-submit-rating`, `btn-skip-rating` |
| `RatingBubbles.tsx` | `rating-bubble-{BUBBLE}` (dynamic: `rating-bubble-GOOD_COMMUNICATION`, `rating-bubble-ON_TIME`, etc.) |

---

## How to run

```bash
# Prerequisites:
# 1. Docker services running: docker compose up -d
# 2. Backend in E2E mode: cd backend && E2E_MODE=true make dev
# 3. App built in E2E mode: cd mobile && EXPO_PUBLIC_E2E_MODE=true npx expo run:ios

make test-mobile-e2e-disputes   # file-dispute + view-dispute-status
make test-mobile-e2e-ratings    # rate-counterparty
```

---

## Decision log

### Why two separate dispute flows

`file-dispute.yaml` exercises the full filing UI (reason selector, description
input, submit, redirect). `view-dispute-status.yaml` exercises the status
screen directly — testing that the Rentals list correctly routes DISPUTED
bookings to `dispute-status` and that the timeline renders for a pre-existing
dispute. Running only one flow would leave the other screen untested.

### Why the seed endpoint creates a COMPLETED booking (not ACTIVE)

The `FileDispute` service requires the transaction to already exist (any
status). The dispute endpoint seeds a COMPLETED transaction to represent a
post-return dispute, which is the most common real-world case. The status is
then set to DISPUTED via a direct SQL update. ACTIVE-state disputes are tested
implicitly by `file-dispute.yaml` (which actually triggers the status transition
via the normal API flow).

### Why `rating-bubble-GOOD_COMMUNICATION` not a generic bubble tap

Tapping by text (e.g. `assertVisible: "Good communication"`) is fragile if
the label changes. Tapping by `testID="rating-bubble-GOOD_COMMUNICATION"` ties
the test to the bubble's canonical value (a `RatingBubble` enum member), which
is stable API surface. The label can be renamed without breaking the test.

---

## Known limitations

- The dispute filing flow only tests the DAMAGE reason path. MISSING_ITEM and
  OTHER are not covered by separate flows but the reason selector testIDs
  (`dispute-reason-MISSING_ITEM`, `dispute-reason-OTHER`) are wired up for
  future tests.
- `view-dispute-status.yaml` asserts the dispute is at "Filed" step (PENDING
  status). Asserting the resolved/INCONCLUSIVE states would require either
  faking the AI pipeline output or adding more seed-endpoint variants — deferred
  to task 9.9 (regression/reliability work).
- `rate-counterparty.yaml` tests only the renter's bubble set. The host rating
  path (HOST_BUBBLES) is not covered in a separate flow.

---

## Files changed

```
backend/internal/testonly/handler.go               — POST /test/dispute endpoint
mobile/app/(tabs)/(rentals)/dispute-status.tsx     — testID additions
mobile/app/(tabs)/(rentals)/dispute.tsx            — testID additions
mobile/app/(tabs)/(rentals)/rate.tsx               — testID additions
mobile/components/rating/RatingBubbles.tsx         — testID on each bubble Pressable
mobile/components/rental/DisputeTimeline.tsx       — testID on container View
mobile/e2e/flows/disputes/file-dispute.yaml        — new flow
mobile/e2e/flows/disputes/view-dispute-status.yaml — new flow
mobile/e2e/flows/ratings/rate-counterparty.yaml    — new flow
mobile/e2e/helpers/seed-dispute.yaml               — new helper
mobile/e2e/scripts/seed-dispute.js                 — new script
.claude/progress.json                              — 9.8 in_progress → completed
```
