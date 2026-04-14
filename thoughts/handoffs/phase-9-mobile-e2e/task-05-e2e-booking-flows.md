# Task 9.5 — E2E: Booking request & status flows

## Summary

All 5 booking Maestro E2E flows pass:

1. **create-booking-request** — Login as renter, tap listing, checkout, select dates, confirm booking, verify "Waiting for host" status
2. **view-booking-status** — Login as renter, Rentals tab, tap REQUESTED booking, verify status label
3. **host-accept-booking** — Login as host, Rentals tab, tap incoming request, accept, verify "Booking confirmed"
4. **host-decline-booking** — Login as host, Rentals tab, tap incoming request, decline, verify "Request declined"
5. **cancel-booking** — Login as renter, Rentals tab, tap REQUESTED booking, cancel, confirm modal, verify "Booking cancelled"

## Key Changes

### Seed script (`mobile/e2e/seed/setup.sh`)
- Added `seed_bookings()` function that creates REQUESTED bookings from bob to alice's listings
- Fixed cascade delete: child tables (`proximity_proofs`, `agent_decisions`, `media`, etc.) have FK constraints on `transactions` — must delete from all 10 child tables before deleting transactions
- Backdate alice's account by 60 days to bypass fraud "new-to-new lockout" (both parties can't be newer than 30 days)
- Added `ensure_stripe_customers()` to set stub Stripe customer IDs

### Backend (`backend/internal/payment/stub.go`, `backend/app/server.go`)
- New stub payment adapter for dev: returns success for all payment operations when `StripeSecretKey == "sk_test_placeholder"`
- Wired into server bootstrap — used automatically in dev without real Stripe credentials

### Mobile screens — testIDs & SafeAreaView fix
- **SafeAreaView rendering bug**: RN 0.81.5 + tab navigator `headerShown: true` causes SafeAreaView-wrapped ScrollView content to not render. Fixed by using `View` instead in `booking-status.tsx` and `checkout.tsx`
- Added testIDs across: `screen-booking-status`, `booking-status-label`, `btn-cancel-booking`, `btn-confirm-cancel`, `screen-checkout`, `btn-rent-now`, `screen-listing-detail`, `lbl-start-date`, `btn-start-increase`, `lbl-end-date`, `btn-end-increase`, `payment-method-selected`, `btn-confirm-booking`, `screen-rentals`, `status-requested`, `btn-accept-request`, `btn-decline-request`

### Rentals tab navigation
- Created `mobile/app/(tabs)/(rentals)/booking-status.tsx` — re-exports feed's booking-status so it works within the Rentals tab stack
- Updated Rentals index to push within its own tab stack instead of cross-tab navigation

### Payment method auto-select
- `PaymentMethodSelector.native.tsx` auto-selects `__DEV__` bypass payment method, showing "payment-method-selected" testID

### Maestro flow YAMLs
- All 5 flows rewritten to use testID selectors and `extendedWaitUntil` for async waits
- `create-booking-request` includes `setLocation` step (simulator GPS for feed)
- Host flows use `login-as-host.yaml` helper

### Pusher hook
- `usePusher.ts` wrapped in try-catch for graceful degradation when Soketi isn't available

## Bugs Fixed

| Bug | Root Cause | Fix |
|-----|-----------|-----|
| Empty booking-status screen | SafeAreaView + ScrollView in RN 0.81 with tab `headerShown: true` | Replaced SafeAreaView with View |
| 403 on booking creation | Fraud "new-to-new lockout" — both accounts < 30 days | Backdate alice's `created_at` by 60 days in seed |
| 409 listing unavailable | Stale bookings from prior runs not cleaned up | CASCADE delete through 10 child FK tables before re-seeding |
| Cancel modal tap ambiguity | "Cancel booking" text matched both button and modal button | Added `testID="btn-confirm-cancel"` |
| Feed empty after clearState | Location permission reset between flows | Added `setLocation` step to create-booking-request |

## Verification

```
5/5 Flows Passed in 3m 5s
- [Passed] Booking - Host accepts booking request
- [Passed] Booking - View booking status (REQUESTED)
- [Passed] Booking - Host declines booking request
- [Passed] Booking - Create booking request (happy path)
- [Passed] Booking - Renter cancels booking request
```

- `npx tsc --noEmit` — clean
- `npx expo export --platform web` — clean

## Mode

Graphite mode (gt) for branch management.
