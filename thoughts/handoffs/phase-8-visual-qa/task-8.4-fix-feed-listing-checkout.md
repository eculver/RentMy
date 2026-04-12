# Handoff: Task 8.4 — Fix: Feed + Listing Detail + Checkout Bugs

**Status:** Completed  
**Branch:** `task-8.4-fix-feed-listing-checkout-bugs`  
**Commit:** `2bb354a`  
**Date:** 2026-04-11  
**Mode:** Graphite (gt)

---

## What Was Done

Implemented fixes for all 9 bugs documented in the Phase 8.3 audit (`thoughts/audits/phase-8-visual-qa/audit-feed-listing.md`), in priority order.

---

## Fixes Applied

### B1 — BLOCKER: KYCGate blocks checkout for all seeded test users
**Fix:** New migration `backend/migrations/016_dev_seed_verify_users.sql` patches `identity_status` to `VERIFIED` for `alice@test.com` and `bob@test.com`. The migration is a no-op in production (those emails don't exist there). The `-- +goose Down` reverses it cleanly.

### B2 — MAJOR: Stripe ephemeral key / setup intent secret conflict
**Backend changes:**
- `PaymentAdapter` interface gained `CreateEphemeralKey(ctx, customerID) (string, error)`
- `StripeAdapter` implements it using `sc.EphemeralKeys.New` with API version `2024-06-20`
- `SetupPaymentResult` gained `EphemeralKey string` field (JSON: `ephemeralKey`)
- `SetupRenterPayment` now calls `CreateEphemeralKey` before `CreateSetupIntent` and returns both

**Frontend change:**
- `PaymentMethodSelector.native.tsx` `SetupResponse` interface gained `ephemeralKey` field
- `initPaymentSheet` now correctly maps `customerEphemeralKeySecret: setup.ephemeralKey` and `setupIntentClientSecret: setup.clientSecret`

**Test change:**
- `service_test.go` `mockAdapter` gained `createEphemeralKeyFn` field and interface method

### B3 — MEDIUM: confirmation.tsx was dead code
**Fix:** `checkout.tsx` `handleConfirm` now routes to `/(tabs)/(feed)/listing/[id]/confirmation` passing: `id`, `transactionId`, `holdAmount`, `rentalFee`, `platformFee`, `totalImpact`, `scheduledStart`, `scheduledEnd` as route params.

### B4 — MEDIUM: "View My Bookings" routed to feed
**Fix:** `confirmation.tsx` "View My Bookings" `onPress` changed from `/(tabs)/(feed)` to `/(tabs)/(rentals)`.

### B5 — MINOR: Location error had no retry
**Fix:**
- `useLocation.ts` gains `retryCount` state and exposes `retry: () => void` in the returned object. `retryCount` is added to `useEffect` deps so calling `retry()` re-runs the permission+location request.
- `LocationState` interface updated with `retry: () => void`
- Feed `index.tsx` destructures `retryLocation` and adds a "Retry" `Pressable` to the error state.

### B6 — MINOR: Platform fee not shown in CostBreakdown
**Fix:** `confirmation.tsx` now receives `platformFee` as a route param and renders it as a line item in the booking summary card when non-zero (between rental fee and hold). The `ConfirmationParams` type was updated accordingly.

### B7 — MINOR: Hold shows $0.00 until estimate loads
**Fix:** `checkout.tsx` wraps the `CostBreakdown` render with a conditional: when `scheduledStart && scheduledEnd` but `holdEstimate == null`, renders an `ActivityIndicator` with "Calculating costs…" instead of the breakdown with `$0.00`.

### B8 — MINOR: "Edit Listing" button is a no-op
**Fix:** `listing/[id].tsx` host CTA now renders a `disabled` Pressable styled `bg-gray-300` with label "Edit Listing (Coming Soon)". Silent no-op is replaced with clear visual feedback.

### B9 — MINOR: Listing detail shows only one photo
**Backend:**
- `Listing` model gained `Photos []string \`json:"photos"\``
- `FindByID` in `repository.go` runs a second query after the main scan to fetch all `LISTING_PHOTO` media URLs (preferring `thumbnail_url` over `original_url`), ordered by `created_at`. Empty result yields `[]string{}` (not null).

**Frontend:**
- `ListingDetail` interface gained `photos: string[]`
- `listing/[id].tsx` builds the `photos` array from `listing?.photos` when non-empty, falling back to the route param thumbnail, falling back to `[]`.

---

## Files Changed

| File | Change |
|------|--------|
| `backend/migrations/016_dev_seed_verify_users.sql` | New — verify seeded test users |
| `backend/internal/payment/model.go` | `CreateEphemeralKey` in interface; `EphemeralKey` in result |
| `backend/internal/payment/service.go` | Call `CreateEphemeralKey` in `SetupRenterPayment` |
| `backend/internal/payment/service_test.go` | Add `createEphemeralKeyFn` to `mockAdapter` |
| `backend/internal/payment/stripe.go` | Implement `CreateEphemeralKey` |
| `backend/internal/listing/model.go` | Add `Photos []string` to `Listing` |
| `backend/internal/listing/repository.go` | Fetch photos in `FindByID` |
| `mobile/components/checkout/PaymentMethodSelector.native.tsx` | Fix Stripe key mapping |
| `mobile/app/(tabs)/(feed)/listing/[id]/checkout.tsx` | B3+B7: routing + hold loading state |
| `mobile/app/(tabs)/(feed)/listing/[id]/confirmation.tsx` | B4+B6: rentals route, platform fee |
| `mobile/app/(tabs)/(feed)/listing/[id].tsx` | B8+B9: edit button, photos array |
| `mobile/app/(tabs)/(feed)/index.tsx` | B5: retry button in location error |
| `mobile/lib/hooks/useLocation.ts` | B5: add retry to hook |
| `mobile/lib/hooks/useListing.ts` | B9: add photos to ListingDetail |
| `.claude/progress.json` | task 8.4 → completed |

---

## Verification

- `cd backend && go vet ./...` — clean
- `cd backend && go build -o /dev/null ./cmd/server` — OK
- `cd mobile && npx tsc --noEmit` — 2 pre-existing errors in `(profile)/index.tsx` unrelated to task 8.4 (router path types for profile sub-routes)
- `cd mobile && npx expo export --platform web` — exported successfully, all routes including `/(tabs)/(feed)/listing/[id]/confirmation` resolved
- `cd mobile && npx jest` — 91 tests passed

---

## Next Task

**8.5 — Audit: Search + Map** — visually inspect search and map screens.
