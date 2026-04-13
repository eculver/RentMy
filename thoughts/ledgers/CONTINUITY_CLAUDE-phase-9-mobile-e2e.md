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
<<<<<<< HEAD

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
||||||| parent of cc34520 (feat: get all auth E2E flows passing — fix testIDs, tab bar, iOS autofill)
=======

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
>>>>>>> cc34520 (feat: get all auth E2E flows passing — fix testIDs, tab bar, iOS autofill)
