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
