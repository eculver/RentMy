# Task 9.6 — E2E: Handoff flows (check-in, active, check-out)

## Status: COMPLETED

## Branch: task-9.6-e2e-handoff-flows

## What was done

Got all 4 handoff Maestro E2E flows passing against the real app on iOS Simulator:

1. **01-active-rental.yaml** — Views ACTIVE booking, asserts countdown/buttons
2. **02-check-out.yaml** — Full check-out: GPS verify + 3 photos + complete → COMPLETED
3. **03-check-in.yaml** — Full check-in: GPS verify + PIN entry + 3 photos → ACTIVE
4. **04-return-confirmation.yaml** — Views COMPLETED booking summary, back navigation

## Files changed

### Screen testIDs added
- `mobile/app/(tabs)/(feed)/active-rental.tsx` — screen-active-rental, btn-start-checkout, btn-navigate-return, btn-report-issue, active-rental-countdown
- `mobile/app/(tabs)/(feed)/booking-status.tsx` — btn-start-checkin
- `mobile/app/(tabs)/(rentals)/return-confirmation.tsx` — screen-return-confirmation, return-status-label, btn-rate-rental, btn-file-dispute, btn-back-to-rentals
- `mobile/components/screens/CheckInScreen.native.tsx` — screen-check-in, btn-open-camera, btn-complete-checkin
- `mobile/components/screens/CheckOutScreen.native.tsx` — screen-check-out, btn-open-camera, btn-complete-checkout
- `mobile/components/handoff/GPSStatus.tsx` — btn-verify-location
- `mobile/components/handoff/PINEntry.tsx` — input-pin-{0-3}

### Navigation fixes
- `mobile/app/(tabs)/(rentals)/index.tsx` — COMPLETED bookings route to return-confirmation
- `mobile/app/(tabs)/(rentals)/return-confirmation.tsx` — `router.back()` instead of `router.replace()`

### Rendering fixes
- SafeAreaView → View on active-rental, CheckInScreen, CheckOutScreen, return-confirmation (RN 0.81.5 rendering bug)
- React Query cache invalidation before navigation in CheckInScreen + CheckOutScreen

### Seed data
- `mobile/e2e/seed/setup.sh` — `seed_handoff_bookings()` creates ACCEPTED, ACTIVE, COMPLETED bookings with proximity proofs. GPS coordinates normalized to match Maestro setLocation. Date ordering ensures seeded ACTIVE sorts first.

### Maestro flows
- `mobile/e2e/flows/handoff/01-active-rental.yaml` — renamed, rewritten
- `mobile/e2e/flows/handoff/02-check-out.yaml` — new, full check-out flow
- `mobile/e2e/flows/handoff/03-check-in.yaml` — new, full check-in flow with PIN
- `mobile/e2e/flows/handoff/04-return-confirmation.yaml` — renamed, rewritten

## Bugs encountered and fixed

1. **SafeAreaView rendering**: Tab navigator headerShown:true + SafeAreaView = blank content
2. **GPS proximity failure**: Seeded listings had wrong PostGIS coordinates; normalized to (34.0522, -118.2437)
3. **Stale cache after check-out**: React Query showed cached ACTIVE after API returned COMPLETED; added invalidateQueries
4. **Maestro execution order**: Flows don't run alphabetically by filename — Maestro uses its own ordering. Check-in ran before check-out, creating a second ACTIVE booking. Fixed seed date ordering so seeded ACTIVE always sorts first.
5. **Button behind tab bar**: "Back to rentals" button on return-confirmation was untappable; added scroll before tap

## Verification

- `npx tsc --noEmit` — passes
- `npx expo export --platform ios` — passes
- `maestro test e2e/flows/handoff/` — 4/4 flows passed

## Graphite mode

Used git fallback mode (branch created with `git checkout -b`).
