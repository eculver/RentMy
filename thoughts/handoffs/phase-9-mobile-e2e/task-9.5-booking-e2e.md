# Task 9.5 — E2E: Booking Request & Status Flows

**Status:** completed  
**Commit:** 571c388  
**Branch:** task-9.5-booking-e2e  
**Date:** 2026-04-12

---

## What was built

### Maestro E2E flows (`mobile/e2e/flows/booking/`)

| File | Flow | Seed |
|------|------|------|
| `create-booking-request.yaml` | Login → feed → listing detail → Rent Now → checkout → select dates → confirm → booking confirmation | None (full UI) |
| `view-booking-status.yaml` | API seed → login → Rentals tab → tap row → assert "Waiting for host" | `seed-booking.js` |
| `host-accept-booking.yaml` | API seed → login as host → Rentals → tap row → tap Accept → assert "Booking accepted" | `seed-booking.js` |
| `host-decline-booking.yaml` | API seed → login as host → Rentals → tap row → tap Decline → assert "Booking declined" | `seed-booking.js` |
| `cancel-booking.yaml` | API seed → login as renter → Rentals → tap row → tap "Cancel booking" → modal confirm → assert "Booking cancelled" | `seed-booking.js` |

### Seed infrastructure

- **`mobile/e2e/scripts/seed-booking.js`** — Maestro `runScript` that calls `POST /api/v1/test/booking` and stores the `TRANSACTION_ID` in an output variable
- **`mobile/e2e/helpers/seed-booking.yaml`** — Wraps `seed-booking.js` as a reusable Maestro helper
- **`backend/internal/testonly/handler.go`** — `POST /api/v1/test/booking` endpoint that inserts a transaction in REQUESTED state, bypassing Stripe. Only mounted when `cfg.E2EMode` is true.
- **`backend/internal/platform/config/config.go`** — Added `E2EMode bool` field, controlled by `E2E_MODE` env var (default false)
- **`backend/app/server.go`** — Mounts `testonly.Handler` under `/api/v1` when `E2EMode=true`

### testID additions

| Screen/Component | New testIDs |
|-----------------|-------------|
| `checkout.tsx` | `screen-checkout`, `btn-confirm-booking` |
| `confirmation.tsx` | `screen-booking-confirmation`, `btn-message-host`, `btn-view-bookings` |
| `booking-status.tsx` | `screen-booking-status`, `booking-status-label`, `btn-navigate-pickup`, `btn-start-checkin`, `btn-manage-active-rental`, `btn-cancel-booking` |
| `rentals/index.tsx` | `screen-rentals`, `rentals-list`, `rental-row` |
| `IncomingRequest.tsx` | `incoming-request-timer`, `btn-accept`, `btn-decline` |
| `DurationPicker.tsx` | `btn-start-increase`, `btn-start-decrease`, `lbl-start-date`, `btn-end-increase`, `btn-end-decrease`, `lbl-end-date` |

### E2E payment bypass

`mobile/components/checkout/PaymentMethodSelector.native.tsx` — when `EXPO_PUBLIC_E2E_MODE=true`, a `useEffect` immediately calls `onPaymentMethodSelected("e2e_test_card")`, skipping Stripe's native payment sheet. The selected state renders "Saved payment method" and hides the "Change" button. The `payment-method-selected` testID is shown, which the `create-booking-request.yaml` flow asserts.

---

## How to run

```bash
# Prerequisites:
# 1. Docker services running: docker compose up -d
# 2. Backend in E2E mode: cd backend && E2E_MODE=true make dev
# 3. App built in E2E mode: cd mobile && EXPO_PUBLIC_E2E_MODE=true npx expo run:ios

make test-mobile-e2e-booking    # all 5 booking flows
```

---

## Decision log

### Why `POST /api/v1/test/booking` instead of seeding via normal flow

The normal `POST /api/v1/bookings` calls Stripe `AuthorizeHold` + `ChargeRentalFee`. Using `e2e_test_card` as the payment method ID would fail Stripe API validation even in test mode, because test mode still requires a real `pm_*` ID attached to a real test customer. Rather than add real Stripe test customer IDs to the seed data (which couples tests to Stripe account state), the test-only endpoint inserts directly into `transactions` with placeholder Stripe IDs.

### Why E2E bypass in PaymentMethodSelector instead of using Stripe test card in the UI

Stripe's native payment sheet is presented as a `UIViewController` that Maestro cannot interact with (same constraint as `UIImagePickerController`). The only reliable option is to bypass it, matching `AngleEnforcedCamera`'s `EXPO_PUBLIC_E2E_MODE` pattern.

### Dependency: flow 9.6 (handoff) depends on this

Task 9.6 (handoff flows) builds on top of the seed helpers and booking status screen established here. The `TRANSACTION_ID` output from `seed-booking.yaml` is the link between the two.

---

## Known limitations

- The `create-booking-request.yaml` flow asserts on `screen-booking-confirmation` but does NOT navigate to the booking-status screen from there. This is intentional: the confirmation screen has a "View My Bookings" button, but the E2E flow ends at confirmation to keep the test focused.
- The host-accept flow doesn't verify the PIN display (PIN is shown to the host after accept). This is covered in task 9.6 (check-in flow).
- `cancel-booking.yaml` uses `index: 1` to tap the "Cancel booking" text inside the modal, since the outer button and the modal confirm button share the same label. This may need to be made more robust if the rendering order changes — adding a `testID="btn-confirm-cancel"` to `CancelConfirmation.tsx` would be cleaner.

---

## Files changed

```
backend/app/server.go                              — mount testonly handler
backend/internal/platform/config/config.go        — add E2EMode field
backend/internal/testonly/handler.go               — new test-only endpoint
mobile/app/(tabs)/(feed)/booking-status.tsx        — testID additions
mobile/app/(tabs)/(feed)/listing/[id]/checkout.tsx — testID additions
mobile/app/(tabs)/(feed)/listing/[id]/confirmation.tsx — testID additions
mobile/app/(tabs)/(rentals)/index.tsx              — testID additions
mobile/components/booking/IncomingRequest.tsx      — testID additions
mobile/components/checkout/DurationPicker.tsx      — testID additions + testIDPrefix prop
mobile/components/checkout/PaymentMethodSelector.native.tsx — E2E bypass
mobile/e2e/config/dev.env                          — add backend E2E_MODE note
mobile/e2e/flows/booking/cancel-booking.yaml       — new flow
mobile/e2e/flows/booking/create-booking-request.yaml — new flow
mobile/e2e/flows/booking/host-accept-booking.yaml  — new flow
mobile/e2e/flows/booking/host-decline-booking.yaml — new flow
mobile/e2e/flows/booking/view-booking-status.yaml  — new flow
mobile/e2e/helpers/seed-booking.yaml               — new helper
mobile/e2e/scripts/seed-booking.js                 — new seed script
.claude/progress.json                              — 9.5 in_progress → completed
```
