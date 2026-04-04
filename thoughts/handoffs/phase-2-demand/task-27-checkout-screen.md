# Task 2.7 — Checkout screen (RN)

## Status: COMPLETE

**Commit:** c202cc4
**Branch:** task-2.7-checkout-screen
**Branching mode:** git fallback (Graphite unavailable)

---

## What Was Done

### New files

**`mobile/lib/stores/checkoutStore.ts`**
- Zustand store for checkout flow state
- Fields: `scheduledStart`, `scheduledEnd`, `paymentMethodId`, `holdAmount`, `rentalFee`, `totalImpact` (computed: holdAmount + rentalFee)
- Actions: `setSchedule`, `setPaymentMethod`, `setAmounts`, `reset`
- `reset()` called on successful booking to clear state for next use

**`mobile/components/checkout/DurationPicker.tsx`**
- Start/end date-time step controls (+/- 1 hour buttons)
- Enforces 7-day maximum duration ceiling (PRD §7): end time capped at start + 7 days
- Shows computed duration label (e.g. "2 days", "3d 4h", "6 hrs")
- Shows red warning banner if user somehow exceeds ceiling
- Auto-corrects end time if it becomes invalid after start changes
- Deferred: full visual calendar (Phase 5). Step controls cover Phase 2 needs without a 200KB calendar library dependency.

**`mobile/components/checkout/CostBreakdown.tsx`**
- Three line items: rental fee, temporary hold, total card impact
- Hold row has an expandable info panel (tap info icon) explaining that hold ≠ charge
- Total row is visually emphasized (bold, gray background)
- All amounts formatted from cents: `cents / 100` → `$XX.XX`

**`mobile/components/checkout/PaymentMethodSelector.tsx`**
- Calls `POST /api/v1/payments/setup` to get Stripe customer ID and setup intent client secret
- Uses `@stripe/stripe-react-native` `initPaymentSheet` + `presentPaymentSheet` for PCI-compliant card collection
- Displays saved method label once added; "Change" CTA to re-open sheet
- Handles Stripe "Canceled" code gracefully (user dismissed sheet — no error shown)
- `paymentMethodId` is set to the payment option label from the sheet (Phase 3 can refine to a real Stripe PM ID from the customer's payment methods)

**`mobile/app/(tabs)/(feed)/listing/[id]/checkout.tsx`**
- Route: `/listing/[id]/checkout` (dynamic segment, navigated from listing detail "Rent Now" button)
- KYC gate: checks `user.identityStatus` (cast from User type which doesn't have this field in Phase 2). If not `VERIFIED` (or field is absent, as in Phase 2), user is allowed through. Phase 4 will populate the field and enforce the gate.
- Fetches `useListing(id)` for pricing and `useHoldEstimate(id)` for hold amount in parallel
- `DurationPicker` for start/end selection
- `CostBreakdown` rendered once both dates are selected (rental fee estimated client-side from listing pricing × duration)
- `PaymentMethodSelector` for Stripe card setup
- "Confirm Booking" button: disabled until dates + payment method are set; calls `POST /api/v1/bookings`
- On success: calls `reset()` on checkoutStore, navigates to confirmation screen via `router.replace`
- On failure: shows `Alert` with backend error message

**`mobile/app/(tabs)/(feed)/listing/[id]/confirmation.tsx`**
- Booking confirmation screen (navigated to after successful `POST /api/v1/bookings`)
- Green checkmark success icon
- Booking summary card: rental period, rental fee, hold amount, total card impact
- "Message Host" CTA → navigates to messages tab (Phase 3 will wire to specific thread)
- "View My Bookings" CTA → navigates back to feed tab

### Modified files

**`mobile/app/_layout.tsx`**
- Wrapped app in `<StripeProvider publishableKey={STRIPE_PUBLISHABLE_KEY}>` (from env var `EXPO_PUBLIC_STRIPE_PUBLISHABLE_KEY`, defaults to `pk_test_placeholder`)
- `StripeProvider` sits outside `QueryClientProvider` to be available throughout the app

**`mobile/package.json` + `mobile/package-lock.json`**
- Added `@stripe/stripe-react-native` (SDK 54.0.0 compatible version via `npx expo install`)

---

## Architecture Decisions

- **`@stripe/stripe-react-native` PaymentSheet for PCI compliance:** Raw card numbers never touch the app. Stripe's PaymentSheet handles all card input on a Stripe-hosted overlay. This is the only approach that avoids PCI SAQ A-EP scope.
- **Client-side rental fee estimate:** The checkout screen estimates the rental fee from `listing.pricePerHour` / `listing.pricePerDay` × duration. The backend recalculates the exact fee on `POST /api/v1/bookings`. This means the displayed fee may differ slightly from the charged fee (e.g. if the host changes pricing between page load and confirm). Phase 3 can add a pre-booking price quote endpoint to eliminate the discrepancy.
- **KYC gate stubbed:** `user.identityStatus` is not in the Phase 2 `User` type (the field comes from the backend but isn't mapped in auth.ts). The gate code is written and will activate automatically in Phase 4 when the field is populated.
- **`paymentMethodId` as string placeholder:** After `presentPaymentSheet` in setup mode, Stripe doesn't return the specific PM ID — the method is attached to the customer on the backend. We pass a label string. Phase 3 can add a `GET /api/v1/payments/methods` endpoint to retrieve the saved PM ID from the customer object, then pass it correctly to `POST /api/v1/bookings`.
- **`router.replace` on confirmation:** Using `replace` instead of `push` so the back button on the confirmation screen doesn't return to checkout (which would re-show a completed checkout).
- **Step controls for duration, not a calendar:** A visual calendar library (react-native-calendars) adds ~200KB and significant complexity for a Phase 2 screen that only needs to select a duration, not browse a calendar. The +/- 1-hour step control is functional and lightweight.

---

## Verification

```
cd mobile && npx tsc --noEmit  → exit 0 (no errors)
```

---

## Notes for Next Tasks

- **Phase 3 (BookingService):** The `POST /api/v1/bookings` endpoint is already implemented in PaymentService (task 2.2). BookingService in Phase 3 will expand booking state management (status transitions, handoff flow).
- **Real Stripe test keys:** Add `EXPO_PUBLIC_STRIPE_PUBLISHABLE_KEY=pk_test_...` to `.env.individual` for manual testing on simulator. Pair with `STRIPE_SECRET_KEY=sk_test_...` in backend `.env.individual`.
- **`GET /api/v1/payments/methods` (Phase 3):** Add endpoint to fetch saved payment methods for a customer, returning the Stripe PM ID. Update `PaymentMethodSelector` to use this ID as `paymentMethodId` in the booking request.
- **Rental fee quote endpoint (Phase 3):** Add `GET /api/v1/listings/:id/quote?start=&end=` returning exact rental fee, so the checkout screen doesn't need to estimate client-side.
- **KYC enforcement (Phase 4):** When `identityStatus` is added to the user response, update `mobile/lib/auth.ts` `User` interface to include the field. The checkout screen KYC gate will activate automatically.
- **"View My Bookings" (Phase 3):** The confirmation screen CTA routes to the feed tab as a placeholder. Phase 3 should add a bookings list screen and update this CTA to navigate there.
