# Handoff: Task 8.11 — Audit: Rentals + Disputes + Ratings

**Completed:** 2026-04-11  
**Branch:** `task-8.11-audit-rentals-disputes-ratings`  
**Commit:** e347591  
**Mode:** Graphite (`gt create` succeeded)

---

## What was done

Static code audit of all five rentals screens (`index`, `dispute`, `dispute-status`, `rate`, `return-confirmation`), four rental/rating UI components, and the backend `dispute` and `rating` packages.

Deliverable: `thoughts/audits/phase-8-visual-qa/audit-rentals-disputes.md`

---

## Bugs found (9 total)

### Critical (3)

**BUG-1:** `backend/internal/rating/model.go` — `Rating` struct has no JSON tags. Go serializes as `FromUserID`, `TransactionID` (PascalCase). Frontend expects `fromUserId`, `transactionId` (camelCase). The `hasRated` check in `index.tsx:158` always returns false → "Rate" button never disappears, duplicate rating submissions hit 409.

**BUG-2:** `DisputeStatus` type mismatch. Backend has: `PENDING`, `GATHERING`, `ANALYZING`, `AUTO_RESOLVED`, `AUDIT_QUEUED`, `HUMAN_REVIEW`, `RESOLVED`, `INCONCLUSIVE`. Frontend has: `PENDING`, `EVIDENCE_GATHERING`, `UNDER_REVIEW`, `RESOLVED`, `CLOSED`. `DisputeTimeline`'s `stepIndex()` falls back to "Resolved" for any unrecognized status.

**BUG-3:** Dispute JSON field mismatches: `route` vs `escalationRoute`, `chargeAmount` vs `damageChargeCents`, `confidence` vs `agentConfidence`, `agentDecisionId` (UUID) vs `agentDecision` (verdict string), `reviewerId` vs `resolvedBy`. All these fields render as null/undefined in the dispute-status screen.

### High (2)

**BUG-4:** `CheckOutScreen.native.tsx:55-58` — After checkout, navigates to `/(tabs)/(feed)/booking-status` instead of `/(tabs)/(rentals)/return-confirmation`. The return-confirmation screen is only reachable via push notification.

**BUG-5:** `index.tsx:229-240` — DISPUTED bookings navigate to `booking-status` (falls through the `else` branch in `handleBookingPress`). Should route to `/(tabs)/(rentals)/dispute-status`.

### Medium (2)

**BUG-6:** `return-confirmation.tsx:86-92` — Hold allocation hardcoded to all zeros. `HoldStatusCard` always shows $0.00 / "pending".

**BUG-7:** `PhotoDiffResult` component is complete and tested but is never imported or rendered in any rentals screen.

### Low (2)

**BUG-8:** INCONCLUSIVE re-prompt "Upload photos" button fires an `Alert.alert()` stub instead of a real upload flow.

**BUG-9:** `HoldStatusCard` `textColor` prop accepted but never applied — dead code.

---

## For the next agent (Task 8.12)

Fix all bugs above. Priority: BUG-1 through BUG-5 are must-fix before any visual testing. BUG-6 and BUG-7 require backend cooperation (hold allocation API) or a design decision (hide the card until data is available).

Full bug descriptions with code references and suggested fixes are in the audit doc.

## Seeding note

No bookings exist in the test database (Stripe payment required). The bugs above were found via static analysis. Visual verification of the rentals screens will need either:
- A test seed endpoint that bypasses Stripe payment
- Manual creation of a booking via backend test utilities
