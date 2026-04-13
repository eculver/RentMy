# Task 8.7 Handoff — Audit: Booking + Handoff

**Status:** Completed  
**Commit:** 3bd3319  
**Branch:** task-8.7-audit-booking-handoff  
**Date:** 2026-04-11

## What was done

Static code audit of all booking and handoff screens and their components:

- `booking-request.tsx` — booking creation flow with DurationPicker, CostBreakdown, PaymentMethodSelector
- `booking-status.tsx` — live status screen with IncomingRequest (host), cancel modal, navigation CTAs
- `active-rental.tsx` — countdown timer, navigate-to-return, check-out CTA, report issue
- `CheckInScreen.native.tsx` — GPS + PIN + photo flow for check-in
- `CheckOutScreen.native.tsx` — GPS + photo flow for check-out
- All associated components: `BookingCard`, `IncomingRequest`, `CancelConfirmation`, `GPSStatus`, `PINDisplay`, `PINEntry`, `PhotoGrid`, `AngleEnforcedCamera`

## Method

All analysis was static (code review only). No live simulator testing was possible because bookings cannot be seeded — `POST /api/v1/bookings` requires a valid Stripe payment method ID and fails with placeholder Stripe API keys.

## Bugs documented

8 bugs found; all captured in `thoughts/audits/phase-8-visual-qa/audit-booking-handoff.md`:

| ID | Severity | Description |
|----|----------|-------------|
| BUG-BH-1 | Medium | Cost breakdown shows $0 when user picks start date without interacting with end date |
| BUG-BH-2 | Low | `IncomingRequest` / `BookingCard` hardcoded `mx-4` causes double-padding in padded containers |
| BUG-BH-3 | Medium | Maps navigation URL uses listing ULID as search query (not coordinates) |
| BUG-BH-4 | Medium | "Report an issue" shows stale placeholder alert instead of navigating to dispute screen |
| BUG-BH-5 | Low | "Message host/renter" goes to inbox tab, not specific conversation |
| BUG-BH-6 | Low | `IncomingRequest` missing renter name/reputation (backend doesn't return it) |
| BUG-BH-7 | Low | `isRenter` is `false` during data load, initializing `useProximity` with wrong role |
| BUG-BH-8 | Low | PIN SMS input accepts any phone string with no E.164 format validation |

## Key observations

- **No critical bugs** — the core flows (request → accept → check-in → check-out) are architecturally sound.
- **`useProximity` hook** is well-designed; GPS + PIN + photo state management is clean and correct.
- **`AngleEnforcedCamera`** angle-diversity logic is correct. `angularDistance` handles yaw wrap-around properly.
- **TypeScript clean** — no errors in booking/handoff files (`npx tsc --noEmit` only shows 2 pre-existing profile screen errors).
- **Maps URL** (BUG-BH-3) affects both `booking-status.tsx` and `active-rental.tsx` — fix requires backend to return listing address or coordinates in the booking response.

## Next task

**Task 8.8 — Fix: Booking + Handoff Bugs** — resolve the bugs documented in this audit, starting with the 3 medium-severity items (BH-1, BH-3, BH-4).

## Branch mode

Graphite mode — `gt create` succeeded, will use `gt submit` for push.
