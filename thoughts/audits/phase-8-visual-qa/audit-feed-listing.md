# Audit: Feed + Listing Detail + Checkout
**Task:** 8.3  
**Date:** 2026-04-11  
**Method:** Static code inspection (iOS Simulator not live — building environment from task 8.0 seed data notes)

---

## Screens Audited

| Screen | File |
|--------|------|
| Feed | `mobile/app/(tabs)/(feed)/index.tsx` |
| Listing Detail | `mobile/app/(tabs)/(feed)/listing/[id].tsx` |
| Checkout | `mobile/app/(tabs)/(feed)/listing/[id]/checkout.tsx` |
| Confirmation | `mobile/app/(tabs)/(feed)/listing/[id]/confirmation.tsx` |

## Components Audited

| Component | File |
|-----------|------|
| ListingFeedCard | `mobile/components/listing/ListingFeedCard.tsx` |
| PhotoGallery | `mobile/components/listing/PhotoGallery.tsx` |
| HostInfoCard | `mobile/components/listing/HostInfoCard.tsx` |
| HoldExplainer | `mobile/components/listing/HoldExplainer.tsx` |
| AvailabilityCalendar | `mobile/components/listing/AvailabilityCalendar.tsx` |
| DurationPicker | `mobile/components/checkout/DurationPicker.tsx` |
| CostBreakdown | `mobile/components/checkout/CostBreakdown.tsx` |
| PaymentMethodSelector | `mobile/components/checkout/PaymentMethodSelector.native.tsx` |
| KYCGate | `mobile/components/verification/KYCGate.tsx` |

---

## Bugs Found

### B1 — BLOCKER: KYCGate blocks checkout for all seeded test users

**File:** `mobile/app/(tabs)/(feed)/listing/[id]/checkout.tsx` (line 220), `mobile/components/verification/KYCGate.tsx`

**Description:** `CheckoutScreen` wraps its content in `<KYCGate>`. KYCGate only renders children when `user.identityStatus === "VERIFIED"`. All newly registered users (including alice@test.com and bob@test.com from seed data) are created with `identity_status = 'PENDING'` (backend default: `IdentityStatusPending` at `backend/internal/user/service.go:92`). Tapping "Rent Now" on any listing shows the "Identity Verification Required" interstitial instead of the checkout form.

**Impact:** Checkout flow is completely untestable with seeded users without a manual DB update.

**Fix:** Update the seed script to patch alice and bob's `identity_status` to `'VERIFIED'` after registration, or add a dev-only API endpoint to bypass KYC.

---

### B2 — MAJOR: PaymentMethodSelector conflates Stripe ephemeral key and setup intent secrets

**File:** `mobile/components/checkout/PaymentMethodSelector.native.tsx` (lines 32–38)

**Description:** The component calls `initPaymentSheet` with `setup.clientSecret` for **both** `customerEphemeralKeySecret` and `setupIntentClientSecret`. Stripe requires these to be distinct:
- `customerEphemeralKeySecret`: a short-lived key for reading the customer's saved payment methods
- `setupIntentClientSecret`: the client secret of the SetupIntent

The backend's `POST /api/v1/payments/setup` returns `{ customerId, clientSecret }` where `clientSecret` is the SetupIntent client secret. There is no ephemeral key field in the response. Passing the same secret for both fields will cause Stripe SDK errors at runtime.

```typescript
// Current (wrong):
customerEphemeralKeySecret: setup.clientSecret,
setupIntentClientSecret: setup.clientSecret,

// Correct — backend must return a separate ephemeral key:
customerEphemeralKeySecret: setup.ephemeralKey,
setupIntentClientSecret: setup.clientSecret,
```

**Fix:** Backend `POST /api/v1/payments/setup` must create and return a Stripe Customer Ephemeral Key alongside the SetupIntent client secret. Frontend must map both fields separately.

---

### B3 — MEDIUM: confirmation.tsx is dead code

**File:** `mobile/app/(tabs)/(feed)/listing/[id]/confirmation.tsx`

**Description:** After a successful booking, `checkout.tsx` routes to `/(tabs)/(feed)/booking-status` (line 121). The `confirmation.tsx` screen at `listing/[id]/confirmation.tsx` is never navigated to anywhere in the codebase. It exists but is unreachable.

**Fix:** Either (a) wire checkout to route to `confirmation.tsx` (passing `transactionId`, `holdAmount`, `rentalFee`, `totalImpact`, `scheduledStart`, `scheduledEnd` as params — the screen already expects them), or (b) delete `confirmation.tsx` and keep the `booking-status` flow as canonical. Option (a) is preferred because `confirmation.tsx` provides a richer post-booking summary with dates and amounts.

---

### B4 — MEDIUM: "View My Bookings" CTA in confirmation.tsx routes to feed, not rentals

**File:** `mobile/app/(tabs)/(feed)/listing/[id]/confirmation.tsx` (line 110)

**Description:** The "View My Bookings" button calls `router.replace("/(tabs)/(feed)")`, which navigates to the feed tab. It should navigate to `/(tabs)/(rentals)` where the user's bookings are tracked.

```typescript
// Current (wrong):
router.replace("/(tabs)/(feed)");

// Correct:
router.replace("/(tabs)/(rentals)");
```

**Fix:** Change route destination from `/(tabs)/(feed)` to `/(tabs)/(rentals)`.

---

### B5 — MINOR: Location error state has no retry mechanism

**File:** `mobile/app/(tabs)/(feed)/index.tsx` (lines 74–83)

**Description:** When location permission is denied or `expo-location` fails, the feed renders a static error card ("Location unavailable") with no retry button, refresh gesture, or fallback to a default location. Users are stuck on an unusable screen.

**Fix:** Add a "Retry" `Pressable` that re-triggers the location permission request, or fall back to a configurable default location (e.g., the seeded LA coordinates).

---

### B6 — MINOR: Platform fee not shown in cost breakdown

**File:** `mobile/components/checkout/CostBreakdown.tsx`, `mobile/app/(tabs)/(feed)/listing/[id]/checkout.tsx` (line 117)

**Description:** The `BookingResponse` includes a `platformFee` field, but `checkout.tsx` ignores it when routing to `booking-status`. `CostBreakdown` only displays `rentalFee` and `holdAmount`. Users see no indication of the platform service fee before confirming.

**Fix:** Thread `platformFee` through `CostBreakdown` (add a `platformFee` prop) and display it as a separate line item between rental fee and hold.

---

### B7 — MINOR: Hold amount displays $0.00 until estimate loads

**File:** `mobile/app/(tabs)/(feed)/listing/[id]/checkout.tsx` (lines 134, 170–172)

**Description:** `displayHold` falls back to `holdAmount` from the checkout store (default: 0) until `useHoldEstimate` resolves. When dates are selected before the estimate loads, `CostBreakdown` shows `$0.00` for the hold with no loading indicator. This is confusing — users may think there is no hold.

**Fix:** Add a loading skeleton or spinner to the `CostBreakdown` section when `holdEstimate` is undefined but dates are selected. Alternatively, gate `CostBreakdown` render on `holdEstimate != null`.

---

### B8 — MINOR: "Edit Listing" button is a no-op for hosts

**File:** `mobile/app/(tabs)/(feed)/listing/[id].tsx` (lines 174–178)

**Description:** The CTA for hosts renders "Edit Listing" but the `onPress` handler is empty — there's a comment: `// Navigate to edit listing — wired in Phase 3+`. Hosts tapping "Edit Listing" see no response.

**Impact:** Low (hosts would not typically be viewing their own listings from the feed), but it is a dead button with no user feedback.

**Fix:** Navigate to an edit listing screen or, if not yet implemented, disable the button with a toast "Coming soon" rather than silently failing.

---

### B9 — MINOR: Listing detail shows at most one photo (thumbnail only)

**File:** `mobile/app/(tabs)/(feed)/listing/[id].tsx` (line 54), `mobile/lib/hooks/useListing.ts`

**Description:** The `photos` array passed to `PhotoGallery` is built exclusively from the `thumbnailUrl` route param:
```typescript
const photos = thumbnailUrl ? [thumbnailUrl] : [];
```

`ListingDetail` (the API response type) does not include a `photos` field. The gallery always shows at most one image. If no thumbnail was passed in route params (e.g., navigation from search or map), the gallery shows a placeholder.

**Fix:** Add a `photos: string[]` field to `ListingDetail` and return photo URLs from `GET /api/v1/listings/:id`. Use `listing.photos` (falling back to thumbnail) to populate the gallery.

---

## Checklist Summary

| Item | Status | Notes |
|------|--------|-------|
| Feed renders listing cards | PASS | Skeleton, empty state, and card layout are correct |
| Feed shows title, price, image placeholder | PASS | ListingFeedCard handles missing thumbnails gracefully |
| Feed "Rent Now" shortcut present | PASS | Button routes to `booking-request.tsx` correctly |
| Listing detail renders after tap | PASS | Loading/error states handled |
| Photo gallery renders | PARTIAL | Shows thumbnail only — no multi-photo support (B9) |
| Host info card renders | PASS | Name, rating, verified badge all correct |
| Hold explainer appears for renters | PASS | Only shown when `!isHost && holdEstimate` |
| Availability calendar renders | PASS | Empty slots shows "Available anytime" message |
| Checkout accessible from detail | BLOCKED | KYCGate blocks all PENDING users (B1) |
| DurationPicker renders | PASS | +/- hour steps, 7-day ceiling enforced |
| CostBreakdown renders | PARTIAL | Missing platform fee (B6), $0 hold before estimate (B7) |
| PaymentMethodSelector renders | FAIL | Stripe initPaymentSheet will fail — wrong secret mapping (B2) |
| Confirm Booking submits | BLOCKED | Requires working payment method (B2) |
| Post-booking confirmation screen | FAIL | confirmation.tsx is dead code, never navigated to (B3) |
| Location error handling | PARTIAL | Shows error message but no retry (B5) |

---

## Priority Order for Fixes (task 8.4)

1. **B1** — Seed users as VERIFIED (unblocks testing the entire checkout flow)
2. **B2** — Fix Stripe secret mapping in PaymentMethodSelector + backend setup endpoint
3. **B3** — Wire checkout success to `confirmation.tsx` (or delete confirmation.tsx)
4. **B4** — Fix "View My Bookings" route in confirmation.tsx
5. **B5** — Add retry to location error state
6. **B6** — Add platform fee to CostBreakdown
7. **B7** — Add loading state for hold estimate in checkout
8. **B8** — Disable or remove no-op "Edit Listing" button
9. **B9** — Add photos array to listing API response and ListingDetail type
