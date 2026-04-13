# Task 9.6 — E2E: Handoff Flows (Check-In, Active Rental, Check-Out, Return Confirmation)

**Status:** completed  
**Commit:** fb0468c  
**Branch:** task-9.6-handoff-e2e  
**Date:** 2026-04-12

---

## What was built

### Maestro E2E flows (`mobile/e2e/flows/handoff/`)

| File | Flow | Seed |
|------|------|------|
| `check-in.yaml` | API seed → login as renter → Rentals → tap row → tap "Start check-in" → GPS verify → PIN "1234" → 3 fixture photos → Complete → assert "Rental in progress" | `seed-booking-accepted.yaml` |
| `active-rental.yaml` | API seed → login as renter → Rentals → tap ACTIVE row → assert countdown + Navigate + Start check-out buttons | `seed-booking-active.yaml` |
| `check-out.yaml` | API seed → login as renter → Rentals → ACTIVE row → "Start check-out" → GPS verify → 3 fixture photos → Complete → assert return-confirmation screen | `seed-booking-active.yaml` |
| `return-confirmation.yaml` | API seed → login as renter → Rentals → COMPLETED row → assert "Return complete" + rate/dispute/back buttons → tap "Back to rentals" | `seed-booking-completed.yaml` |

### Seed infrastructure

| File | Purpose |
|------|---------|
| `mobile/e2e/scripts/seed-booking.js` | Updated to accept `STATUS` env var (REQUESTED/ACCEPTED/ACTIVE/COMPLETED) |
| `mobile/e2e/helpers/seed-booking-accepted.yaml` | Wraps seed script with STATUS=ACCEPTED |
| `mobile/e2e/helpers/seed-booking-active.yaml` | Wraps seed script with STATUS=ACTIVE |
| `mobile/e2e/helpers/seed-booking-completed.yaml` | Wraps seed script with STATUS=COMPLETED |

### Backend changes

| File | Change |
|------|--------|
| `backend/internal/proximity/service.go` | Added `E2EMode bool` to `Config`; `VerifyGPS` skips distance check when true |
| `backend/app/server.go` | Passes `E2EMode: cfg.E2EMode` to `proximity.Config` |
| `backend/internal/testonly/handler.go` | Added `status` field to request; inserts proximity proofs for ACCEPTED/ACTIVE/COMPLETED states |

**Testonly endpoint behavior by status:**
- `REQUESTED`: unchanged (same as before)
- `ACCEPTED`: inserts transaction + host's `CHECK_IN` proof (verified=true, PIN="1234")
- `ACTIVE`: inserts transaction (actual_start=now) + host/renter `CHECK_IN` proofs (verified) + host `CHECK_OUT` proof (verified)
- `COMPLETED`: inserts transaction (actual_start=now-4h, actual_end=now-1h) + all 4 proofs verified

**Response now includes** `pin` (for ACCEPTED), `listingLat`, `listingLng`.

### testID additions

| Screen/Component | New testIDs |
|-----------------|-------------|
| `CheckInScreen.native.tsx` | `screen-check-in`, `btn-open-camera`, `btn-complete-checkin` |
| `CheckOutScreen.native.tsx` | `screen-check-out`, `btn-open-camera`, `btn-complete-checkout` |
| `active-rental.tsx` | `screen-active-rental`, `active-rental-countdown`, `btn-navigate-return`, `btn-start-checkout`, `btn-report-issue` |
| `return-confirmation.tsx` | `screen-return-confirmation`, `return-status-label`, `btn-rate-rental`, `btn-file-dispute`, `btn-back-to-rentals` |
| `GPSStatus.tsx` | `btn-verify-location` |
| `PINEntry.tsx` | `input-pin-e2e` (E2E single-input), `pin-verified-label` |

### E2E mode bypasses added

| Component/Hook | Bypass |
|---------------|--------|
| `AngleEnforcedCamera.native.tsx` | Fixed: "Use Fixture Photo" stays visible while `captures.length < maxPhotos` (was hiding after 1 photo) |
| `PINEntry.tsx` | E2E renders single `TextInput testID="input-pin-e2e"` instead of 4 digit boxes |
| `useProximity.ts` | Pre-populates `currentLat/Lng` with (34.0522, -118.2437); skips `watchPositionAsync` |
| `proximity.Service` (backend) | Skips Haversine distance check; any coordinates pass GPS verify |

### Rentals list navigation update

`mobile/app/(tabs)/(rentals)/index.tsx`: COMPLETED bookings now navigate to `return-confirmation` instead of `booking-status`. This enables the `return-confirmation.yaml` flow to reach the screen from the rentals list.

---

## How to run

```bash
# Prerequisites:
# 1. Docker services running: docker compose up -d
# 2. Backend in E2E mode: cd backend && E2E_MODE=true make dev
# 3. App built in E2E mode: cd mobile && EXPO_PUBLIC_E2E_MODE=true npx expo run:ios

make test-mobile-e2e-handoff    # all 4 handoff flows
```

---

## Decision log

### Why backend E2E GPS bypass instead of Maestro setLocation

`setLocation` in Maestro sets the iOS Simulator location, but the test listing's coordinates are dynamic (determined at seed time) and may not be set at all in CI. A backend bypass — accepting any coordinates when `E2E_MODE=true` — is deterministic and requires no coordination between the Maestro flow and the listing's actual lat/lng.

The bypass is safe: it is guarded by `cfg.E2EMode` which only enables when `E2E_MODE=true`, and the testonly handler (which uses E2E mode) is only mounted when that same flag is set.

### Why pre-verify the host's CHECK_IN proof in ACCEPTED seed

`CheckHandoffComplete(CHECK_IN)` requires both host and renter to be verified before the booking can transition to ACTIVE. In a real check-in, the host arrives first and does GPS verify. In the E2E flow, only the renter goes through the check-in screen. Pre-verifying the host's proof in the test seed simulates the host having already completed their side.

### Why pre-verify the host's CHECK_OUT proof in ACTIVE seed

Same reasoning for check-out: both parties must verify GPS before `CheckHandoffComplete(CHECK_OUT)` returns true. Pre-verifying the host's CHECK_OUT proof means only the renter needs to go through the check-out screen.

### Why 4 E2E bypasses instead of 1

Each bypass targets a different layer:
1. **GPS backend bypass**: ensures the proximity API call succeeds without real location
2. **PINEntry E2E input**: makes the 4-digit PIN entry automatable in Maestro
3. **useProximity GPS pre-population**: prevents the "Verify my location" button from being disabled while waiting for location acquisition
4. **AngleEnforcedCamera multi-photo fix**: allows 3+ photos in a single camera session

---

## Known limitations

- The `check-in.yaml` flow only tests the renter's path. The host's check-in path (showing PIN, GPS verify as host) is not covered in this task.
- Return-confirmation's photo comparison section (`isCompleted && !hasOpenDispute` → sky banner) is rendered but not specifically asserted in the E2E flow — it depends on the backend returning `hasOpenDispute=false`, which is the default for new test bookings.
- `actual_start`/`actual_end` in ACTIVE/COMPLETED seed bookings use `now` and `now-4h`. Bookings seeded in rapid succession may have identical timestamps but this does not affect test correctness.

---

## Files changed

```
backend/app/server.go                              — pass E2EMode to proximity.Config
backend/internal/proximity/service.go             — E2EMode field + GPS bypass
backend/internal/testonly/handler.go               — multi-state seed + proximity proofs
mobile/app/(tabs)/(feed)/active-rental.tsx         — testID additions
mobile/app/(tabs)/(rentals)/index.tsx              — COMPLETED → return-confirmation routing
mobile/app/(tabs)/(rentals)/return-confirmation.tsx — testID additions
mobile/components/camera/AngleEnforcedCamera.native.tsx — multi-photo E2E fix
mobile/components/handoff/GPSStatus.tsx            — testID on verify button
mobile/components/handoff/PINEntry.tsx             — E2E single-input mode
mobile/components/screens/CheckInScreen.native.tsx — testID additions
mobile/components/screens/CheckOutScreen.native.tsx — testID additions
mobile/e2e/flows/handoff/active-rental.yaml        — new flow
mobile/e2e/flows/handoff/check-in.yaml             — new flow
mobile/e2e/flows/handoff/check-out.yaml            — new flow
mobile/e2e/flows/handoff/return-confirmation.yaml  — new flow
mobile/e2e/helpers/seed-booking-accepted.yaml      — new helper
mobile/e2e/helpers/seed-booking-active.yaml        — new helper
mobile/e2e/helpers/seed-booking-completed.yaml     — new helper
mobile/e2e/scripts/seed-booking.js                 — STATUS parameter support
mobile/lib/hooks/useProximity.ts                   — E2E GPS pre-population
.claude/progress.json                              — 9.6 in_progress → completed
```
