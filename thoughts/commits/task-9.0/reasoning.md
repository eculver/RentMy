# Commit Reasoning — Task 9.0: Get the App Running

## What changed
- Added testID props to Button component and login/feed screens for Maestro E2E targeting
- Fixed Expo Router auth redirect to use `<Redirect>` pattern instead of conditional `Stack.Screen`
- Fixed Maestro YAML flows for Expo Dev Client compatibility (clearKeychain, dev server reconnection, overlay dismissal)
- Fixed invalid `timeout` property on Maestro `assertVisible` commands

## Why
Task 9.0 requires the full stack (backend + iOS simulator + Maestro) working end-to-end with a passing login test. The existing YAML flows and app code had several issues preventing this: missing testIDs, broken auth navigation, incorrect Maestro syntax, and iOS-specific overlay handling.

## Decisions
- Used `clearState` + `clearKeychain` (not just `clearState`) because expo-secure-store uses iOS Keychain which persists across clearState
- Changed from conditional Stack.Screen to Redirect pattern because the former doesn't reliably control Expo Router navigation
- Added testID to all FeedScreen return paths (loading, error, and main) so tests work regardless of location availability
