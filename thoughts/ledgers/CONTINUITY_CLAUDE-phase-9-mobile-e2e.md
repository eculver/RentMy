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
