# Phase 9 — Mobile E2E Test Suite — Continuity Ledger

## Phase goal
Build a comprehensive Maestro-based E2E test suite for every core user flow on
iOS Simulator against real backend services. Exit: `make test-mobile-e2e` passes
reliably, every core flow has a happy-path + error-path test, CI runs the suite
on every PR.

---

## Completed tasks

### 9.0 — Maestro Setup & Test Infrastructure
**Commit:** (see 9.1 branch)  
Infrastructure: Maestro CLI, `mobile/e2e/` directory, `dev.env`, root Makefile
targets, `login-as-renter.yaml`, `login-as-host.yaml`, `navigate-to-tab.yaml`.
testIDs wired into login/register screens and feed.

### 9.1 — E2E: Authentication Flows
All 6 auth flows passing: register (happy, duplicate email, validation), login
(happy, wrong password), logout.

### 9.2 — E2E: Discovery Flows
Three discovery flows: `browse-feed.yaml`, `search-listings.yaml`,
`map-view.yaml`. testIDs for tabs, feed list, listing card, search input, map
container.

### 9.3 — E2E: Listing Detail & Creation
Two flows: `view-listing-detail.yaml`, `create-listing.yaml`.
Camera mocked via `EXPO_PUBLIC_E2E_MODE` in `AngleEnforcedCamera.native.tsx`.

### 9.4 — E2E: Profile & Referral Flows
Three flows: `view-profile.yaml`, `referrals.yaml`, `sign-out.yaml`.

### 9.5 — E2E: Booking Request & Status Flows
**Commit:** 571c388  
Five flows covering the booking state machine entry points:

| Flow | Key action |
|------|-----------|
| `create-booking-request.yaml` | Full UI checkout flow |
| `view-booking-status.yaml` | View REQUESTED status |
| `host-accept-booking.yaml` | Host taps Accept → ACCEPTED |
| `host-decline-booking.yaml` | Host taps Decline → DECLINED |
| `cancel-booking.yaml` | Renter cancels → CANCELLED |

Infrastructure additions:
- `POST /api/v1/test/booking` — backend test endpoint (only when E2E_MODE=true)
- `E2EMode bool` config field  
- `seed-booking.js` + `seed-booking.yaml` helper
- E2E payment bypass in `PaymentMethodSelector.native.tsx`
- testIDs in `checkout.tsx`, `confirmation.tsx`, `booking-status.tsx`,
  `rentals/index.tsx`, `IncomingRequest.tsx`, `DurationPicker.tsx`

### 9.6 — E2E: Handoff Flows
**Commit:** fb0468c  
Four flows covering the physical handoff lifecycle:

| Flow | Key action |
|------|-----------|
| `check-in.yaml` | GPS verify (backend bypass) + PIN "1234" + 3 photos → ACTIVE |
| `active-rental.yaml` | Assert countdown + Navigate + Start check-out buttons |
| `check-out.yaml` | GPS verify + 3 photos → COMPLETED → return-confirmation |
| `return-confirmation.yaml` | Assert Return complete + rate/dispute/back buttons |

Infrastructure additions:
- `POST /api/v1/test/booking` now accepts `status` field (REQUESTED/ACCEPTED/ACTIVE/COMPLETED)
- Proximity proofs are pre-inserted for each state to bypass multi-party requirements
- Backend E2E GPS bypass: `proximity.Config.E2EMode=true` skips Haversine distance check
- `PINEntry.tsx`: E2E mode shows single `TextInput testID="input-pin-e2e"` 
- `useProximity.ts`: E2E mode pre-populates GPS coords + skips `watchPositionAsync`
- `AngleEnforcedCamera.native.tsx`: E2E fix — "Use Fixture Photo" stays visible for all photos up to `maxPhotos`
- Rentals list: COMPLETED bookings now navigate to `return-confirmation`
- Seed helpers: `seed-booking-{accepted,active,completed}.yaml`

---

## In-progress / Pending tasks

| Task | Status | Deps |
|------|--------|------|
| 9.7 — Messaging flows | pending | 9.1 |
| 9.8 — Dispute & rating | pending | 9.5 |
| 9.9 — Full regression | pending | 9.1–9.8 |
| 9.10 — CI integration | pending | 9.9 |
| 9.11 — Documentation | pending | 9.10 |

---

## Key architectural decisions

1. **E2E_MODE pattern** — `EXPO_PUBLIC_E2E_MODE=true` in the app bypasses native
   hardware (camera, Stripe payment sheet). Always build for E2E with this flag.

2. **Backend E2E mode** — `E2E_MODE=true` on the backend mounts test-only
   endpoints. Never set in production (checked with `cfg.E2EMode` guard).

3. **Seed helpers vs full UI flows** — Flows that need a specific booking state
   (host-accept, host-decline, etc.) use the `seed-booking.yaml` helper to create
   state directly via API, keeping the test focused on the action under test.

4. **Seeded users** — `alice@test.com` (host, 5 listings) and `bob@test.com`
   (renter, 0 listings), both VERIFIED (migration 016). These are the only E2E
   test accounts; all flows use them.

5. **Graphite mode** — All branches are created with `/opt/homebrew/bin/gt create`.
