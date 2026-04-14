# Task 9.9 — Full Regression Suite & Reliability (3 Consecutive Green Runs)

**Status: COMPLETED**
**Branch:** `task-9.9-regression-reliability` (Graphite mode)

---

## Results Summary

| Run | Result | Duration |
|-----|--------|----------|
| 1   | 28/28 Passed | 18m 25s |
| 2   | 28/28 Passed | 18m 24s |
| 3   | 28/28 Passed | 18m 57s |

All 28 flows passed identically across 3 consecutive runs. Zero flaky tests detected.

---

## Bugs Found and Fixed

### Bug 1: iOS EXC_GUARD XPC crash after ~15+ rapid test cycles

**Root cause:** Rapid `clearState` → `launchApp` cycles in Maestro login helpers accumulated stale XPC handles in the iOS simulator. After ~15 flows, the kernel raised `EXC_GUARD` / `GUARD_TYPE_USER` via `_XPC_MISUSE_FAULT`, crashing the app on launch.

**Fix:** Added `stopApp` command before `clearState` in both login helpers (`login-as-renter.yaml`, `login-as-host.yaml`). This explicitly terminates the app process, allowing the simulator to tear down XPC connections cleanly before the next cycle.

**Files:**
- `mobile/e2e/helpers/login-as-renter.yaml` — added `stopApp` before `clearState`
- `mobile/e2e/helpers/login-as-host.yaml` — added `stopApp` before `clearState`

### Bug 2: Stale Maestro driver processes blocking port 7001

**Root cause:** Previous Maestro runs could leave behind `xcodebuild test-without-building` and `maestro-driver-iosUITests-Runner` processes that held port 7001 open. Subsequent runs would fail to connect to the driver.

**Fix:** Added `_e2e-clean-drivers` Makefile prerequisite target that kills stale driver processes before each E2E run.

**Files:**
- `Makefile` — added `_e2e-clean-drivers` target, made `test-mobile-e2e` depend on it

### Bug 3: Minor test reliability improvements from prior tasks

Several flows had testID and seed data fixes carried forward from tasks 9.1–9.8 that contributed to reliability:
- `mobile/e2e/flows/auth/*.yaml` — login/logout flow adjustments for Expo Dev Client overlay handling
- `mobile/e2e/flows/handoff/03-check-in.yaml` — deterministic booking row targeting via `rental-row-king0001`
- `mobile/e2e/flows/disputes/file-dispute.yaml` — `status-active` instead of generic `rental-row`
- `mobile/e2e/seed/setup.sh` — GPS normalization for all listings, deterministic ULID IDs
- `mobile/app/(tabs)/(rentals)/index.tsx` — unique testID per booking row (`rental-row-${booking.id.slice(-8).toLowerCase()}`)

---

## Full Terminal Output

### Run 1

```
[Passed] Auth - Logout (38s)
[Passed] Auth - Register (happy path) (38s)
[Passed] Auth - Login (happy path) (32s)
[Passed] Auth - Register (validation: empty fields) (19s)
[Passed] Auth - Register (error: duplicate email) (26s)
[Passed] Auth - Login (error: wrong password) (20s)
[Passed] Booking - Host accepts booking request (38s)
[Passed] Booking - View booking status (REQUESTED) (36s)
[Passed] Booking - Host declines booking request (38s)
[Passed] Booking - Create booking request (happy path) (47s)
[Passed] Booking - Renter cancels booking request (38s)
[Passed] Handoff - Check-in (GPS + PIN + photos → ACTIVE) (57s)
[Passed] Handoff - Return confirmation (COMPLETED booking summary) (43s)
[Passed] Handoff - Check-out (GPS + photos → COMPLETED) (46s)
[Passed] Handoff - Active rental screen (countdown, navigate, checkout CTA) (36s)
[Passed] Disputes - View dispute status for a pre-seeded disputed booking (38s)
[Passed] Disputes - File a dispute from an active rental (happy path) (47s)
[Passed] Discovery - Browse feed (happy path) (42s)
[Passed] Discovery - Search listings (49s)
[Passed] Discovery - Map view (35s)
[Passed] Ratings - Rate a counterparty after a completed rental (happy path) (39s)
[Passed] Profile - Sign out (37s)
[Passed] Profile - Referrals screen (happy path) (37s)
[Passed] Profile - View profile screen (happy path) (35s)
[Passed] Listing - View listing detail (happy path) (40s)
[Passed] Listing - Create listing (__DEV__ camera bypass) (1m 17s)
[Passed] Messaging - View conversations list (happy path) (36s)
[Passed] Messaging - Send a message (happy path) (41s)

28/28 Flows Passed in 18m 25s
```

### Run 2

```
[Passed] Auth - Logout (37s)
[Passed] Auth - Register (happy path) (38s)
[Passed] Auth - Login (happy path) (32s)
[Passed] Auth - Register (validation: empty fields) (19s)
[Passed] Auth - Register (error: duplicate email) (26s)
[Passed] Auth - Login (error: wrong password) (21s)
[Passed] Booking - Host accepts booking request (38s)
[Passed] Booking - View booking status (REQUESTED) (37s)
[Passed] Booking - Host declines booking request (37s)
[Passed] Booking - Create booking request (happy path) (46s)
[Passed] Booking - Renter cancels booking request (39s)
[Passed] Handoff - Check-in (GPS + PIN + photos → ACTIVE) (58s)
[Passed] Handoff - Return confirmation (COMPLETED booking summary) (43s)
[Passed] Handoff - Check-out (GPS + photos → COMPLETED) (46s)
[Passed] Handoff - Active rental screen (countdown, navigate, checkout CTA) (36s)
[Passed] Disputes - View dispute status for a pre-seeded disputed booking (38s)
[Passed] Disputes - File a dispute from an active rental (happy path) (47s)
[Passed] Discovery - Browse feed (happy path) (42s)
[Passed] Discovery - Search listings (49s)
[Passed] Discovery - Map view (36s)
[Passed] Ratings - Rate a counterparty after a completed rental (happy path) (39s)
[Passed] Profile - Sign out (37s)
[Passed] Profile - Referrals screen (happy path) (36s)
[Passed] Profile - View profile screen (happy path) (34s)
[Passed] Listing - View listing detail (happy path) (40s)
[Passed] Listing - Create listing (__DEV__ camera bypass) (1m 16s)
[Passed] Messaging - View conversations list (happy path) (36s)
[Passed] Messaging - Send a message (happy path) (41s)

28/28 Flows Passed in 18m 24s
```

### Run 3

```
[Passed] Auth - Logout (37s)
[Passed] Auth - Register (happy path) (38s)
[Passed] Auth - Login (happy path) (32s)
[Passed] Auth - Register (validation: empty fields) (19s)
[Passed] Auth - Register (error: duplicate email) (26s)
[Passed] Auth - Login (error: wrong password) (21s)
[Passed] Booking - Host accepts booking request (38s)
[Passed] Booking - View booking status (REQUESTED) (36s)
[Passed] Booking - Host declines booking request (37s)
[Passed] Booking - Create booking request (happy path) (46s)
[Passed] Booking - Renter cancels booking request (39s)
[Passed] Handoff - Check-in (GPS + PIN + photos → ACTIVE) (58s)
[Passed] Handoff - Return confirmation (COMPLETED booking summary) (43s)
[Passed] Handoff - Check-out (GPS + photos → COMPLETED) (46s)
[Passed] Handoff - Active rental screen (countdown, navigate, checkout CTA) (36s)
[Passed] Disputes - View dispute status for a pre-seeded disputed booking (38s)
[Passed] Disputes - File a dispute from an active rental (happy path) (47s)
[Passed] Discovery - Browse feed (happy path) (41s)
[Passed] Discovery - Search listings (48s)
[Passed] Discovery - Map view (36s)
[Passed] Ratings - Rate a counterparty after a completed rental (happy path) (39s)
[Passed] Profile - Sign out (37s)
[Passed] Profile - Referrals screen (happy path) (37s)
[Passed] Profile - View profile screen (happy path) (1m 5s)
[Passed] Listing - View listing detail (happy path) (41s)
[Passed] Listing - Create listing (__DEV__ camera bypass) (1m 19s)
[Passed] Messaging - View conversations list (happy path) (36s)
[Passed] Messaging - Send a message (happy path) (41s)

28/28 Flows Passed in 18m 57s
```

---

## Files Changed

- `mobile/e2e/helpers/login-as-renter.yaml` — added `stopApp` before `clearState`
- `mobile/e2e/helpers/login-as-host.yaml` — added `stopApp` before `clearState`
- `Makefile` — added `_e2e-clean-drivers` target, restructured E2E targets
- `mobile/e2e/run1-output.txt` — Run 1 terminal output
- `mobile/e2e/run2-output.txt` — Run 2 terminal output
- `mobile/e2e/run3-output.txt` — Run 3 terminal output

Plus reliability fixes carried forward from tasks 9.1–9.8:
- `mobile/e2e/flows/auth/*.yaml` — Expo Dev Client overlay handling
- `mobile/e2e/flows/handoff/02-check-out.yaml`, `03-check-in.yaml`, `04-return-confirmation.yaml` — deterministic testIDs
- `mobile/e2e/flows/disputes/file-dispute.yaml` — `status-active` targeting
- `mobile/e2e/seed/setup.sh` — GPS normalization, deterministic IDs
- `mobile/app/(tabs)/(rentals)/index.tsx` — unique testID per booking row
