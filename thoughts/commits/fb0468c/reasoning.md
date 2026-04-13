# Commit fb0468c — feat: add Maestro E2E handoff flows (task 9.6)

## Why this commit

Task 9.6 extends the Maestro E2E suite with four flows covering the physical handoff lifecycle: check-in (ACCEPTED → ACTIVE), active rental screen assertions, check-out (ACTIVE → COMPLETED), and return confirmation display.

## Key decisions

### Backend E2E GPS bypass (proximity service)

The GPS proximity check (`POST /api/v1/proximity/verify`) normally requires the renter to be within 100 m of the listing location. In E2E tests, we cannot guarantee the simulator's location matches the seeded listing's coordinates (which are dynamic and may not be set at all).

Added `E2EMode bool` to `proximity.Config`. When true, `VerifyGPS` skips the Haversine distance calculation and marks any location as verified. The proof record creation/update still runs — this is critical because the renter's `CHECK_IN` proximity proof must exist in the DB before `VerifyPIN` can be called (VerifyPIN looks up the renter's record to mark it as verified).

This mirrors the existing pattern: Stripe payment bypass in `PaymentMethodSelector`, camera bypass in `AngleEnforcedCamera`.

### Testonly handler: multi-state booking seed

The existing `POST /api/v1/test/booking` only created `REQUESTED` bookings. The handoff flows need bookings in three additional states:

- **ACCEPTED**: transaction + host `CHECK_IN` proof (verified=true, PIN="1234"). The renter enters "1234" via the E2E PIN input. The host's proof being pre-verified means `CheckHandoffComplete(CHECK_IN)` returns true once the renter also verifies.
- **ACTIVE**: transaction (with `actual_start`) + host/renter `CHECK_IN` proofs + host `CHECK_OUT` proof (pre-verified). The renter then only needs to do their own GPS verify for check-out.
- **COMPLETED**: transaction (with `actual_start`/`actual_end`) + all four proximity proofs verified.

All states are selected via a `status` field in the request body.

### PINEntry E2E mode

The normal `PINEntry` component uses four separate `TextInput` boxes with auto-advance focus. Maestro cannot reliably type into multiple native inputs in sequence. Added an `IS_E2E` render path that shows a single `TextInput` with `testID="input-pin-e2e"` where the user types the full 4-digit PIN. The `onChangeText` callback auto-submits when `length === 4`.

### AngleEnforcedCamera E2E fix

The existing E2E bypass only allowed one fixture photo (showed "Use Fixture Photo" only when `captures.length === 0`, then switched to "Continue"). Check-in and check-out require `MIN_PHOTOS = 3`. Fixed by keeping "Use Fixture Photo" visible while `captures.length < maxPhotos`.

### useProximity E2E GPS pre-population

Added `IS_E2E` check: when true, `currentLat`/`currentLng` are initialized to `(34.0522, -118.2437)` (hardcoded test coords) instead of `null`. The `watchPositionAsync` subscription is skipped. This means `hasLocation` is always true in E2E mode — the "Verify my location" button is immediately enabled without waiting for the simulator GPS signal.

### COMPLETED bookings → return-confirmation

The rentals list previously routed all non-ACTIVE, non-DISPUTED bookings to `booking-status`. COMPLETED bookings now route to `return-confirmation` — the more relevant screen after a rental ends. This also enables the `return-confirmation.yaml` E2E flow to navigate there directly from the rentals list.

## Files changed

- `backend/internal/proximity/service.go` — E2EMode in Config, bypass in VerifyGPS
- `backend/app/server.go` — pass E2EMode to proximity.Config
- `backend/internal/testonly/handler.go` — multi-state booking seed + proximity proof insertion
- `mobile/components/camera/AngleEnforcedCamera.native.tsx` — multi-photo E2E bypass fix
- `mobile/components/screens/CheckInScreen.native.tsx` — testIDs
- `mobile/components/screens/CheckOutScreen.native.tsx` — testIDs
- `mobile/components/handoff/GPSStatus.tsx` — testID on verify button
- `mobile/components/handoff/PINEntry.tsx` — E2E single-input mode
- `mobile/app/(tabs)/(feed)/active-rental.tsx` — testIDs
- `mobile/app/(tabs)/(rentals)/return-confirmation.tsx` — testIDs
- `mobile/app/(tabs)/(rentals)/index.tsx` — COMPLETED → return-confirmation routing
- `mobile/lib/hooks/useProximity.ts` — E2E GPS pre-population
- `mobile/e2e/scripts/seed-booking.js` — STATUS parameter support
- `mobile/e2e/helpers/seed-booking-{accepted,active,completed}.yaml` — new helpers
- `mobile/e2e/flows/handoff/{check-in,active-rental,check-out,return-confirmation}.yaml` — new flows
