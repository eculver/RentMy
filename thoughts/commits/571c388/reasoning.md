# Commit 571c388 — Reasoning

## What
Five Maestro E2E flows for task 9.5 (booking request & status):
- `create-booking-request.yaml` — UI flow through checkout
- `view-booking-status.yaml` — API seed + view REQUESTED status
- `host-accept-booking.yaml` — API seed + host accepts
- `host-decline-booking.yaml` — API seed + host declines
- `cancel-booking.yaml` — API seed + renter cancels

Plus the supporting infrastructure:
- `seed-booking.js` / `seed-booking.yaml` helpers
- `backend/internal/testonly/handler.go` — `POST /api/v1/test/booking`
- `config.E2EMode` bool gated behind `E2E_MODE=true`
- testID additions to `booking-status.tsx`, `checkout.tsx`, `confirmation.tsx`,
  `rentals/index.tsx`, `IncomingRequest.tsx`, `DurationPicker.tsx`
- E2E payment bypass in `PaymentMethodSelector.native.tsx`

## Why

### Payment bypass required
The `PaymentMethodSelector.native.tsx` opens Stripe's native payment sheet —
impossible to automate with Maestro. When `EXPO_PUBLIC_E2E_MODE=true`, the
component immediately calls `onPaymentMethodSelected("e2e_test_card")` via a
`useEffect`, matching the pattern already established by `AngleEnforcedCamera`.

### Backend test endpoint required
Creating bookings via the normal `POST /api/v1/bookings` API calls Stripe to
authorize a payment hold. With a dummy payment method ID (`e2e_test_card`) this
would always fail. Following the pattern the plan described for KYC (`POST
/api/v1/test/verify-user`), I added `POST /api/v1/test/booking` which inserts a
transaction row directly, bypassing the payment adapter. This endpoint is only
mounted when `E2E_MODE=true`; the backend config struct guards it explicitly.

### Booking status flows require pre-seeded data
The host-accept, host-decline, cancel, and view-status flows need a booking
already in REQUESTED state so the test can immediately navigate to the status
screen and act on it. Creating via the app UI would require a complete create
flow + reload, creating unnecessary test surface and brittleness.

## Tradeoffs
- The `e2e_test_card` payment method is intentionally never charged; the backend
  test endpoint stores a placeholder Stripe IDs (`pi_e2e_test`, `""`). This is
  acceptable because the payment flow is covered by unit tests; the E2E tests
  validate the UX and state machine, not payment processing.
- The testonly endpoint does NOT require authentication (by design — seeding before
  login). This is safe because it is only mounted when `E2E_MODE=true`, which should
  never be set in production.
