# Commit 1f53335 — feat: get all handoff E2E flows passing

## Why

Task 9.6 requires all 4 handoff Maestro E2E flows to pass against the real app
on iOS Simulator: active-rental view, check-out, check-in, and return-confirmation.

## Key decisions

1. **SafeAreaView → View**: RN 0.81.5 + tab navigator `headerShown: true` causes
   SafeAreaView-wrapped ScrollView content to not render. Replaced with plain View
   on active-rental, CheckInScreen, CheckOutScreen, and return-confirmation.

2. **React Query cache invalidation**: After check-in/check-out API calls, the
   booking-status screen showed stale cached data. Added explicit
   `queryClient.invalidateQueries` before `router.replace` in both handlers.

3. **Seed date ordering**: Maestro runs flows in non-alphabetical order (by flow name,
   not filename). Check-in (03) runs before check-out (02), creating a second ACTIVE
   booking. Fixed by making the seeded ACTIVE booking's `scheduledStart` more recent
   than the ACCEPTED booking's, so it always sorts first in the rentals list.

4. **Scroll before tap**: The "Back to rentals" button on return-confirmation was behind
   the tab bar. Added `- scroll` in the Maestro flow before tapping to bring it into view.

5. **GPS coordinate normalization**: Seed script updates handoff listing locations to
   exact coordinates (34.0522, -118.2437) matching Maestro `setLocation` values, ensuring
   the 100m proximity threshold is met.

6. **router.back() over router.replace()**: For return-confirmation's "Back to rentals"
   navigation, `router.back()` is more reliable than `router.replace("/(tabs)/(rentals)")`.
