# Phase 9 — Mobile E2E Test Suite — Continuity Ledger

## Task 9.0: Get the App Running (Build + Backend + Simulator + First Test)
- **Status:** Completed
- **Branch:** `task-9.0-maestro-setup`
- **Bugs fixed:** 6 (missing testIDs, auth redirect pattern, keychain clearing, Expo Dev Client overlays, iOS Save Password prompt, invalid Maestro timeout syntax)
- **Key decisions:**
  - Changed auth pattern from conditional `Stack.Screen` to `<Redirect>` for reliable Expo Router navigation
  - All E2E flows use `clearState` + `clearKeychain` to fully reset auth state
  - Expo Dev Client flow handled via: tap dev server → dismiss welcome → close dev tools panel
  - `testID="screen-feed"` added to all FeedScreen return paths (loading, error, main)
- **Verification:** `maestro test mobile/e2e/flows/auth/login.yaml` passes with all 18 steps COMPLETED

## Task 9.1: E2E Authentication Flows
- **Status:** Completed
- **Branch:** `task-9.1-e2e-auth-flows`
- **Bugs fixed:** 6 (missing testIDs on register/profile/error, 15-tab bar bug, iOS Strong Password autofill, runScript variable issue, tab text matching, YAML missing clearKeychain/Dev Client handling)
- **Key decisions:**
  - Added `_layout.tsx` Stack navigators to `(feed)`, `(map)`, `(messages)`, `(search)` groups to fix 15-tab bar bug
  - Use `eraseText: 30` before `inputText` on register password field to defeat iOS Strong Password autofill
  - Use `evalScript` + `${output.VARIABLE}` instead of `runScript` for dynamic test data
  - Tab navigation via `tapOn: text: "TabName, tab.*"` matching iOS accessibility text
  - Keyboard dismissal on register form: tap non-interactive subtitle text instead of `hideKeyboard`
- **Verification:** `maestro test mobile/e2e/flows/auth/` — 6/6 Flows Passed in 3m 12s

## Task 9.2: E2E Discovery Flows (Feed, Search, Map)
- **Status:** Completed
- **Branch:** `task-9.2-e2e-discovery-flows`
- **Bugs fixed:** 8 (missing testIDs on all discovery screens, no seeded listings, invalid Maestro syntax in YAML flows, Google Maps SDK crash, broken listing detail back navigation, listing detail testID only on success state, keyboard covers FilterSheet, BottomSheet invisible to Maestro)
- **Key decisions:**
  - Switched from Google Maps (`PROVIDER_GOOGLE`) to Apple Maps (default) — no API key needed
  - Added native Stack header with "Back" button to listing detail screen (removed custom absolute-positioned back button)
  - Added `Keyboard.dismiss()` when opening the filter sheet
  - Used coordinate-based taps for BottomSheet elements (Maestro can't find elements inside `@gorhom/bottom-sheet`)
  - Extended seed script to create 5 keyword-rich listings (camera, tent, etc.) near LA and activate them via `docker exec` SQL
  - Added `testID="screen-listing-detail"` to all 3 render states (loading, error, success)
- **Verification:** `maestro test mobile/e2e/flows/discovery/` — 3/3 Flows Passed in 1m 57s. Auth regression: 6/6 Passed in 2m 51s.

## Task 9.3: E2E Listing Flows (View Detail + Create Listing)
- **Status:** Completed
- **Branch:** `task-9.3-e2e-listing-flows`
- **Bugs fixed:** 3 new (API response wrapper mismatch, lat/lng keyboard dismiss, continue button z-index) + 2 from prior WIP session (missing testIDs on ListingForm, listing detail route refactored)
- **Key decisions:**
  - `__DEV__ && !device` camera bypass in `AngleEnforcedCamera.native.tsx` — writes real JPEG to cache, goes through normal media-upload pipeline. No global E2E_MODE.
  - Fixed `MediaResponse` and `ListingResponse` types to match backend's `{ media: {...} }` / `{ listing: {...} }` wrapper pattern
  - Moved "Continue without AI suggestions" button to `z-20 absolute` positioning above the `AIAutofillOverlay` (z-10 absolute)
  - Keyboard dismiss for numeric fields: tap "Listing Details" heading (far from inputs) instead of nearby labels
  - Listing detail route refactored from `[id].tsx` to `[id]/index.tsx` directory for future checkout route
- **Verification:** `maestro test mobile/e2e/flows/listing/` — 2/2 Flows Passed in 1m 55s. Auth regression: 6/6 Passed. Discovery regression: 3/3 Passed.

## Task 9.4: E2E Profile & Referral Flows
- **Status:** Completed
- **Branch:** `task-9.4-profile-referral-flows`
- **Bugs fixed:** 0 app bugs. All fixes were missing testIDs and invalid Maestro syntax in pre-existing YAML templates.
- **Key decisions:**
  - Added 7 testIDs across profile screen (`profile-name`, `profile-email`, `btn-invite-friends`, `profile-listings-empty`) and referrals screen (`screen-referrals`, `referral-code`, `btn-share-referral`)
  - Added `referrals` route to profile Stack layout (was relying on implicit Expo Router auto-routing without proper header)
  - Fixed all 3 YAML flows: `assertVisible` → `extendedWaitUntil`, `assertNotVisible` → `extendedWaitUntil: notVisible`, `tapOn: "Profile"` → `tapOn: text: "Profile, tab.*"` (consistent with patterns from 9.1)
  - No overrides needed — profile/referral flows work entirely against real app + real backend
  - Referral code auto-generates via GET (404) → POST fallback in `useReferralCode` hook
- **Flakiness note:** 1/3 suite runs saw transient `kAXErrorInvalidUIElement` from iOS XCTest during Expo Dev Client transition. Not an app issue. Subsequent runs pass cleanly.
- **Verification:** `maestro test mobile/e2e/flows/profile/` — 3/3 Flows Passed in 1m 41s. Auth regression: 6/6 Passed. Discovery regression: 3/3 Passed. Listing regression: 2/2 Passed.

## Task 9.5: E2E Booking Request & Status Flows
- **Status:** Completed
- **Branch:** `task-9.4-profile-referral-flows` (stacked)
- **Bugs fixed:** 5 (SafeAreaView+ScrollView rendering bug, fraud new-to-new lockout, FK cascade delete for seed cleanup, cancel modal tap ambiguity, feed empty after clearState)
- **Key decisions:**
  - SafeAreaView replaced with View in booking-status.tsx and checkout.tsx — RN 0.81.5 bug with tab headerShown:true causes ScrollView content to not render inside SafeAreaView
  - Created `backend/internal/payment/stub.go` — stub payment adapter auto-selected when StripeSecretKey == "sk_test_placeholder", no global E2E_MODE
  - `PaymentMethodSelector.native.tsx` auto-selects __DEV__ bypass payment method
  - Seed script cascade-deletes from 10 child FK tables (agent_decisions, media, proximity_proofs, messages, ratings, guarantee_fund_entries, risk_scores, agreements, disputes, late_returns) before deleting transactions
  - Backdate alice's created_at by 60 days to bypass fraud velocity "new-to-new lockout" (30-day threshold)
  - Created `(rentals)/booking-status.tsx` re-export to keep navigation within Rentals tab stack
  - All 5 YAML flows use testID selectors and extendedWaitUntil for async waits
- **Verification:** `maestro test mobile/e2e/flows/booking/` — 5/5 Flows Passed in 3m 5s. TypeScript clean. Metro bundler clean.

## Task 9.6: E2E Handoff Flows (Check-in, Active, Check-out)
- **Status:** Completed
- **Branch:** `task-9.6-e2e-handoff-flows`
- **Bugs fixed:** 5 (SafeAreaView rendering on 4 screens, GPS proximity coordinate mismatch, React Query stale cache after check-in/check-out, Maestro non-alphabetical execution order causing wrong ACTIVE booking selection, button behind tab bar untappable)
- **Key decisions:**
  - Added testIDs to 7 screens/components: CheckInScreen, CheckOutScreen, ActiveRental, ReturnConfirmation, GPSStatus, PINEntry, booking-status (start-checkin button)
  - Extended `seed_handoff_bookings()` with ACCEPTED (+ host CHECK_IN proof w/ PIN=1234), ACTIVE (+ all CHECK_IN + host CHECK_OUT proofs), COMPLETED (+ all 4 proofs)
  - Normalized handoff listing GPS coordinates to (34.0522, -118.2437) via `UPDATE listings SET location = ST_SetSRID(ST_MakePoint(...))`
  - `router.back()` instead of `router.replace("/(tabs)/(rentals)")` for return-confirmation back navigation — more reliable with Expo Router stack
  - Seed ACTIVE booking has `scheduledStart = NOW()` and ACCEPTED has `scheduledStart = NOW() - 3 hours` — ensures seeded ACTIVE always sorts first in rentals list regardless of Maestro execution order
  - Added `- scroll` before tapping "Back to rentals" button in flow 04 to avoid tab bar overlap
  - COMPLETED bookings in rentals index now route to `return-confirmation` instead of `booking-status`
  - React Query invalidation added to both CheckInScreen and CheckOutScreen `handleComplete` before navigation
- **Maestro quirk:** `maestro test <dir>` does NOT execute flows alphabetically by filename — it uses its own ordering (03, 04, 02, 01 in our case). This matters when flows mutate DB state.
- **Verification:** `maestro test mobile/e2e/flows/handoff/` — 4/4 Flows Passed in 2m 49s. TypeScript clean. Metro bundler clean.

## Task 9.7: E2E Messaging Flows
- **Status:** Completed
- **Branch:** `task-9.7-e2e-messaging-flows` (Graphite mode)
- **Bugs fixed:** 7 (missing conversations endpoint, sendMessage response mismatch, missing testIDs, SafeAreaView testID propagation, non-existent seed endpoint, YAML syntax issues, conversation ordering)
- **Key decisions:**
  - Created `GET /api/v1/users/me/conversations` backend endpoint (handler, service, repository) with `LEFT JOIN LATERAL` for last message
  - Fixed `useSendMessage` hook — backend returns raw `Message`, not `{ message: Message }`
  - Used `View` instead of `SafeAreaView` for testID containers (iOS accessibility tree issue)
  - Moved conversation seeding from JS API calls to SQL in `setup.sh` — Maestro `http.post()` only supports 2 args, can't set auth headers
  - Seed messages use `NOW() + INTERVAL` timestamps to ensure they sort above all booking creation times in conversations list
  - Tab navigation uses `tapOn: text: "Messages, tab.*"` regex pattern for iOS accessibility
  - `assertVisible` replaced with `extendedWaitUntil: visible:` for timeout support
- **Verification:** `maestro test mobile/e2e/flows/messaging/` — 2/2 Flows Passed in 1m 14s. TypeScript clean. go vet clean.

## Task 9.8: E2E Dispute & Rating Flows
- **Status:** Completed
- **Branch:** `task-9.8-dispute-rating-flows` (Graphite mode)
- **Bugs fixed:** 7 (non-existent seed endpoints, missing testIDs on 5 screens/components, SafeAreaView testID propagation on 3 screens, FlatList row tap propagation on iOS, placeholder handleReportIssue, missing DISPUTED case in handleBookingPress, keyboard covering submit button)
- **Key decisions:**
  - Extended `setup.sh` with `seed_dispute_rating_bookings()` — creates ACTIVE (for dispute filing), DISPUTED + PENDING dispute (for viewing), COMPLETED with no ratings (for rating). All with proximity proofs.
  - Converted 3 seed helper YAMLs to no-ops (data comes from SQL seeding)
  - DISPUTED badge in rentals list converted from View to Pressable with own `testID="btn-view-dispute"` and `onPress` — avoids iOS accessibility tree tap propagation issue where Text taps inside FlatList rows don't reach parent Pressables
  - `handleReportIssue` in active-rental.tsx changed from placeholder Alert to `router.push()` to dispute screen
  - Added `keyboardDismissMode="on-drag"` to dispute screen ScrollView; E2E flow taps "What happened?" text to dismiss keyboard before scrolling to submit
  - `SafeAreaView` → `View` on dispute.tsx, dispute-status.tsx, rate.tsx (iOS testID propagation fix)
  - Added testIDs: `screen-dispute`, `dispute-reason-*`, `input-dispute-description`, `btn-submit-dispute`, `btn-cancel-dispute`, `screen-dispute-status`, `dispute-timeline`, `btn-back-to-rentals-dispute`, `screen-rate`, `rating-bubbles-container`, `btn-submit-rating`, `rating-success-message`, `rating-bubble-*`, `btn-view-dispute`
- **Verification:** `maestro test mobile/e2e/flows/disputes/` — 2/2 Flows Passed in 1m 23s. `maestro test mobile/e2e/flows/ratings/` — 1/1 Flow Passed in 40s. TypeScript clean. Metro bundler clean. go vet clean.

## Task 9.9: Full Regression Suite & Reliability (3 Consecutive Green Runs)
- **Status:** Completed
- **Branch:** `task-9.9-regression-reliability` (Graphite mode)
- **Bugs fixed:** 2 (iOS EXC_GUARD XPC crash after 15+ rapid test cycles, stale Maestro driver processes blocking port 7001)
- **Key decisions:**
  - Added `stopApp` before `clearState` in both login helpers to prevent XPC handle accumulation
  - Added `_e2e-clean-drivers` Makefile prerequisite target that kills stale `maestro-driver` and `xcodebuild` processes before each run
  - Re-seed data between each run to ensure identical starting state
- **Results:** 3 consecutive green runs — 28/28 Passed in 18m 25s, 18m 24s, 18m 57s
- **Verification:** `make test-mobile-e2e` × 3 — all 28/28 Passed. Zero flaky tests.
