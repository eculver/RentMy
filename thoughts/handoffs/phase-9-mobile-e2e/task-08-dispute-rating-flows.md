# Task 9.8 — E2E: Dispute & Rating Flows

**Status: COMPLETED**
**Branch:** `task-9.8-dispute-rating-flows` (Graphite mode)

---

## Verification Output

```
Disputes suite (2/2):
[Passed] Disputes - View dispute status for a pre-seeded disputed booking (39s)
[Passed] Disputes - File a dispute from an active rental (happy path) (44s)
2/2 Flows Passed in 1m 23s

Ratings suite (1/1):
[Passed] Ratings - Rate a counterparty after a completed rental (happy path) (40s)
1/1 Flow Passed in 40s
```

---

## Bugs Found and Fixed

### Bug 1: Seed scripts call non-existent backend test endpoints

**Root cause:** The Maestro seed helper YAMLs (`seed-booking-active.yaml`, `seed-booking-completed.yaml`, `seed-dispute.yaml`) ran JS scripts that called `POST /api/v1/test/booking` and `POST /api/v1/test/dispute` — endpoints that don't exist in the backend.

**Fix:** Added a `seed_dispute_rating_bookings()` function to `mobile/e2e/seed/setup.sh` that uses raw SQL to create the needed test data:
- ACTIVE booking (for file-dispute flow)
- DISPUTED booking + PENDING dispute record (for view-dispute-status flow)
- COMPLETED booking with no ratings (for rate-counterparty flow)

Each booking includes proper proximity proofs. The Maestro seed helper YAMLs were converted to no-ops.

**Files:**
- `mobile/e2e/seed/setup.sh` — added `seed_dispute_rating_bookings()` and verification counts
- `mobile/e2e/helpers/seed-booking-active.yaml` — converted to no-op
- `mobile/e2e/helpers/seed-booking-completed.yaml` — converted to no-op
- `mobile/e2e/helpers/seed-dispute.yaml` — converted to no-op

### Bug 2: Missing testIDs on dispute and rating screens

**Root cause:** Dispute, dispute-status, and rate screens lacked testIDs for Maestro element selectors.

**Fix:** Added testIDs to all interactive and assertion-target elements:
- `screen-dispute`, `dispute-reason-{DAMAGE|MISSING_ITEM|OTHER}`, `input-dispute-description`, `btn-submit-dispute`, `btn-cancel-dispute` on dispute.tsx
- `screen-dispute-status`, `dispute-timeline`, `btn-back-to-rentals-dispute` on dispute-status.tsx
- `screen-rate`, `rating-bubbles-container`, `btn-submit-rating`, `rating-success-message` on rate.tsx
- `rating-bubble-{BUBBLE_NAME}` on RatingBubbles.tsx
- `btn-view-dispute` on rentals index (DISPUTED badge)

**Files:**
- `mobile/app/(tabs)/(rentals)/dispute.tsx`
- `mobile/app/(tabs)/(rentals)/dispute-status.tsx`
- `mobile/app/(tabs)/(rentals)/rate.tsx`
- `mobile/app/(tabs)/(rentals)/index.tsx`
- `mobile/components/rating/RatingBubbles.tsx`

### Bug 3: SafeAreaView testID not propagating on iOS

**Root cause:** React Native's `SafeAreaView` doesn't reliably forward `testID` to the native accessibility tree on iOS, so Maestro can't find elements by testID on SafeAreaView.

**Fix:** Changed `SafeAreaView` to `View` on dispute, dispute-status, and rate screens (all 3 screens plus loading/error states on rate screen).

### Bug 4: Tapping text inside FlatList rows doesn't propagate to parent Pressable on iOS

**Root cause:** When Maestro taps on a `Text` element (e.g., "Disputed" status label) inside a FlatList row, the iOS accessibility tree doesn't propagate the tap to the parent `Pressable`'s `onPress`. The tap registers as COMPLETED in Maestro but the handler never fires.

**Fix:** Converted the DISPUTED status badge in the rentals list from a plain `View` to a `Pressable` with its own `testID="btn-view-dispute"` and `onPress` handler (with `e.stopPropagation()`). The E2E flow taps this dedicated Pressable instead of the row text.

**Files:** `mobile/app/(tabs)/(rentals)/index.tsx`

### Bug 5: `handleReportIssue` was a placeholder Alert

**Root cause:** The "Report an issue" button on the active-rental screen showed a "Coming soon" Alert instead of navigating to the dispute filing screen.

**Fix:** Changed `handleReportIssue` to use `router.push()` to navigate to the dispute screen with the `transactionId` param.

**Files:** `mobile/app/(tabs)/(feed)/active-rental.tsx`

### Bug 6: Missing DISPUTED case in handleBookingPress

**Root cause:** The rentals index screen's `handleBookingPress` didn't handle the DISPUTED status, so tapping a disputed rental row did nothing (fell through to booking-status).

**Fix:** Added an `else if (booking.status === "DISPUTED")` branch that navigates to `dispute-status` with the `transactionId`.

**Files:** `mobile/app/(tabs)/(rentals)/index.tsx`

### Bug 7: Keyboard covering submit button on dispute screen

**Root cause:** After entering text in the multiline description TextInput, the keyboard stayed up and covered the submit button. Maestro's `hideKeyboard` doesn't work on multiline TextInputs.

**Fix:** Added `keyboardDismissMode="on-drag"` to the dispute screen's ScrollView. The E2E flow taps on the "What happened?" static text to dismiss the keyboard, then uses `scrollUntilVisible` which also triggers dismiss via the drag mode.

**Files:** `mobile/app/(tabs)/(rentals)/dispute.tsx`

---

## E2E Flow YAML Changes

All three flow files were rewritten to use correct Maestro patterns:
- `extendedWaitUntil` (with timeout) instead of `assertVisible`
- testID-based selectors instead of text matching for interactive elements
- `scrollUntilVisible` before tapping off-screen buttons
- `runFlow` with relative paths (`../../helpers/login-as-renter.yaml`)
- Keyboard dismiss via tapping static text + scroll drag mode

**Files:**
- `mobile/e2e/flows/disputes/file-dispute.yaml`
- `mobile/e2e/flows/disputes/view-dispute-status.yaml`
- `mobile/e2e/flows/ratings/rate-counterparty.yaml`

---

## Build Verification

- `npx tsc --noEmit` — passes (exit 0)
- `npx expo export --platform web` — passes (Metro bundle succeeds)
- `go vet ./...` — passes (no backend changes)
