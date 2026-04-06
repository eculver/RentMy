# Task 5.5 — Phase 1-4 Mobile Component Tests

## Summary

Wrote screen and component tests for the core mobile screens and UI primitives built in Phases 1-4.

## What Was Done

### New files

**Test helper:**
- `mobile/__tests__/helpers/renderWithProviders.tsx` — Wraps components in a `QueryClientProvider` with `retry: false` for reliable async test behavior.

**Screen tests:**
- `mobile/__tests__/screens/auth.test.tsx` — 15 tests for `LoginScreen` and `RegisterScreen`: field rendering, zod validation errors (email format, password rules), store spy calls, API error states (401, 409).
- `mobile/__tests__/screens/feed.test.tsx` — 5 tests for `FeedScreen`: empty state, listing cards from MSW, navigation on card press, "Rent Now" shortcut, location loading state.
- `mobile/__tests__/screens/search.test.tsx` — 5 tests for `SearchScreen`: search input renders, idle empty state, results from MSW, "no results" empty state, navigation on result press.
- `mobile/__tests__/screens/booking.test.tsx` — 11 tests for `BookingRequestScreen` (6) and `BookingStatusScreen` (5): title from params, form sections, disabled/enabled send button, successful booking submission, status labels (REQUESTED/ACCEPTED), error state, booking details section.
- `mobile/__tests__/screens/listing-create.test.tsx` — 7 tests: camera step renders, advance to form step, all form fields render, validation errors for missing title and short description, successful API POST, AI suggestions display.

**Component test:**
- `mobile/__tests__/components/ui.test.tsx` — 11 tests for `Button` and `Input`: all variants, onPress handler, disabled state, loading spinner, label/error display, onChange.

### Total: 54 new tests across 6 new test files

## Key Decisions

- **Store spies for API error tests**: Rather than relying on MSW to intercept the auth module's separate `authApi` ky instance, error-state tests (401, 409) inject a spy via `useAuthStore.setState()` that throws a real `HTTPError`. This is more reliable and tests the component's error handling logic directly.

- **Zustand store seeding for location**: `useLocationStore.setState({ lat, lng })` in `beforeEach` so `useLocation` skips the async permission flow and `useFeed` is immediately enabled.

- **Direct query store seeding for search**: `useSearchStore.setState({ query: 'drill' })` bypasses the debounce (300ms), making search result tests synchronous.

- **MockCamera component**: `AngleEnforcedCamera` is mocked with a "Done with photos" pressable button so tests can advance the multi-step `CreateListingScreen` from the camera step to the form step without needing native camera access.

- **Module-level mocks**: `expo-router`, `@expo/vector-icons`, `@stripe/stripe-react-native`, `pusher-js/react-native`, `expo-linking`, `@gorhom/bottom-sheet` are all mocked per test file. These are not in `setup.ts` to avoid breaking the existing API and smoke tests.

- **Git fallback mode**: `gt create` failed with empty commit message error; used `git checkout -b` for branch creation.

## Verification

```
cd mobile && npx tsc --noEmit   # 0 errors
cd mobile && npx jest --verbose  # 59 tests, 8 suites, all pass
```

Test counts:
- components/smoke.test.tsx: 1 (existing)
- lib/api.test.ts: 4 (existing)
- components/ui.test.tsx: 11 (new)
- screens/auth.test.tsx: 15 (new)
- screens/feed.test.tsx: 5 (new)
- screens/search.test.tsx: 5 (new)
- screens/booking.test.tsx: 11 (new)
- screens/listing-create.test.tsx: 7 (new)

## Next Task

5.6 — CI Pipeline Update (depends on 5.1, 5.2)
