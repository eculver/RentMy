# RentMy Mobile E2E Tests

Maestro-based end-to-end test suite covering all core user flows in the RentMy iOS app.

---

## Quick Start

### Prerequisites

| Requirement | Version | Check |
|-------------|---------|-------|
| Maestro CLI | ≥ 2.4 | `~/.maestro/bin/maestro --version` |
| iOS Simulator | Booted | `xcrun simctl list devices | grep Booted` |
| Docker services | Running | `docker compose ps` |
| Go backend | Running (E2E_MODE=true) | `curl localhost:8080/health` |
| App build | Installed on simulator | `xcrun simctl listapps <device-id> \| grep rentmy` |

### Install Maestro

```bash
curl -Ls https://get.maestro.mobile.dev | bash
```

### Start services

```bash
# From repo root
docker compose up -d

# From backend/
E2E_MODE=true make dev
```

### Build and install the app

The app must be compiled with the `EXPO_PUBLIC_E2E_MODE=true` flag to enable:
- Camera bypass (uses fixture photos instead of native camera)
- KYC bypass (auto-approves Stripe Identity)
- E2E payment method (no Stripe sheet)

```bash
# First time (full build, ~5-10 min)
cd mobile && EXPO_PUBLIC_E2E_MODE=true npx expo run:ios

# Subsequent runs (incremental, ~30 s)
cd mobile && EXPO_PUBLIC_E2E_MODE=true npx expo run:ios --no-build-cache false
```

The simulator must be **booted** before running this command.

### Run the full suite

```bash
# All 28 flows
make test-mobile-e2e

# Individual categories
make test-mobile-e2e-auth
make test-mobile-e2e-discovery
make test-mobile-e2e-listing
make test-mobile-e2e-booking
make test-mobile-e2e-handoff
make test-mobile-e2e-messaging
make test-mobile-e2e-profile
make test-mobile-e2e-disputes
make test-mobile-e2e-ratings
```

---

## Test Coverage

| Category | Flows | Happy path | Error path | Notes |
|----------|-------|------------|------------|-------|
| Auth | 6 | login, register, logout | wrong password, duplicate email, empty fields | |
| Discovery | 3 | browse feed, search, map | — | Map marker tap requires manual verification (native maps) |
| Listing | 2 | view detail, create | — | Create uses camera bypass |
| Booking | 5 | create request, view status, host accept, host decline, cancel | — | |
| Handoff | 4 | check-in, active rental, check-out, return confirmation | — | GPS bypass via `setLocation` |
| Messaging | 2 | view conversations, send message | — | |
| Profile | 3 | view profile, referrals, sign out | — | |
| Disputes | 2 | file dispute, view dispute status | — | |
| Ratings | 1 | rate counterparty | — | |
| **Total** | **28** | | | |

---

## Directory Structure

```
mobile/e2e/
  flows/
    auth/             6 flows (login, register x3, logout, login-wrong-password)
    discovery/        3 flows (browse-feed, search-listings, map-view)
    listing/          2 flows (view-listing-detail, create-listing)
    booking/          5 flows (create-request, view-status, host-accept, host-decline, cancel)
    handoff/          4 flows (check-in, active-rental, check-out, return-confirmation)
    messaging/        2 flows (view-conversations, send-message)
    profile/          3 flows (view-profile, referrals, sign-out)
    disputes/         2 flows (file-dispute, view-dispute-status)
    ratings/          1 flow  (rate-counterparty)
  helpers/
    login-as-renter.yaml    clearState → launchApp → login as bob@test.com
    login-as-host.yaml      clearState → launchApp → login as alice@test.com
    seed-booking.yaml       Seed a REQUESTED booking via API
    seed-booking-accepted.yaml   Seed an ACCEPTED booking
    seed-booking-active.yaml     Seed an ACTIVE booking
    seed-booking-completed.yaml  Seed a COMPLETED booking
    seed-conversation.yaml  Seed a conversation with messages
    seed-dispute.yaml       Seed a DISPUTED booking
    navigate-to-tab.yaml    Tap a tab bar item by label text
  scripts/
    seed-booking.js         HTTP helper: POST /api/v1/test/booking
    seed-conversation.js    HTTP helper: POST /api/v1/test/conversation
    seed-dispute.js         HTTP helper: POST /api/v1/test/dispute
    gen-unique-email.js     Generates a unique email for registration flows
  config/
    dev.env                 Environment variables for local runs
  fixtures/                 Static photos for camera bypass
```

---

## Test Data

Tests run against real backend services using two seeded accounts:

| Account | Email | Role | Notes |
|---------|-------|------|-------|
| Renter | `bob@test.com` | Renter | No listings, can book |
| Host | `alice@test.com` | Host | 5 active listings |

Seeded data is created by Phase 8 migrations (`backend/migrations/`). Flows that require specific booking states (ACCEPTED, ACTIVE, etc.) call `POST /api/v1/test/booking` with the desired status — this endpoint is only available when the backend runs with `E2E_MODE=true`.

### Idempotency

Each seeding call creates a **new** booking. Because bob accumulates bookings across runs, the Rentals tab always shows the most recently created booking first (backend sorts by `created_at DESC`). Tests that tap `rental-row` always hit the freshest booking from that run.

If test data becomes stale and causes unexpected state, reset with:

```bash
# Wipe and reseed the database
cd backend && make migrate-down && make migrate-up && make seed
```

---

## Adding New Tests

### 1. Pick the right template

- **Happy path flow:** Use `login-as-renter.yaml` or `login-as-host.yaml` as the first step, then navigate and assert.
- **Flow that needs a booking in state X:** Use the matching `seed-booking-*.yaml` helper, then `login-as-renter.yaml`.
- **Error path:** Use `clearState` + `launchApp` directly, skip the login helper.

### 2. Add testIDs to the app

All Maestro element selectors use `testID` props. Add them in the component:

```tsx
<Pressable testID="btn-my-action" onPress={...}>
```

Naming conventions:
- Screens: `screen-{route-name}` (e.g. `screen-feed`, `screen-checkout`)
- Buttons: `btn-{action}` (e.g. `btn-confirm-booking`, `btn-sign-out`)
- Inputs: `input-{field}` (e.g. `input-email`, `input-dispute-description`)
- Lists: `{noun}-list`, rows: `{noun}-row` (e.g. `conversation-list`, `rental-row`)
- Labels: `{noun}-label` (e.g. `booking-status-label`)

### 3. Write the flow YAML

```yaml
appId: ${APP_ID}
name: "Category - Action description"
env:
  APP_ID: com.rentmy.app
---
# Step comments explain WHY, not just what the YAML does.

- runFlow: e2e/helpers/login-as-renter.yaml

- tapOn: "Rentals"

- assertVisible:
    id: "screen-rentals"
    timeout: 5000
```

Timeout guidelines:
- Tab navigation / screen transitions: `4000`–`6000`
- API responses (local backend): `6000`–`8000`
- Complex state transitions (booking confirm, AI appraisal): `10000`–`20000`

### 4. Place the file

```
mobile/e2e/flows/{category}/{action}.yaml
```

Maestro discovers all YAML files under `e2e/flows/` automatically.

---

## Debugging Failures

### View screenshots

Maestro writes screenshots on failure to `~/.maestro/tests/<timestamp>/`. Run:

```bash
open ~/.maestro/tests/
```

### Run a single flow

```bash
cd mobile && ~/.maestro/bin/maestro test e2e/flows/auth/login.yaml --env-file e2e/config/dev.env
```

### Verbose output

```bash
cd mobile && ~/.maestro/bin/maestro test e2e/flows/ --env-file e2e/config/dev.env --debug-output /tmp/maestro-debug
```

### Common failure patterns

| Symptom | Likely cause | Fix |
|---------|-------------|-----|
| `Element not found: id=screen-X` | Screen not loaded yet | Increase `timeout` on the assertion |
| `Element not found: id=btn-Y` | TestID missing in app | Add `testID="btn-Y"` to the component |
| `seed-booking.js failed (404)` | Backend not in E2E_MODE | Start with `E2E_MODE=true make dev` |
| `seed-booking.js failed (500)` | Stale DB state | Run `make migrate-down && make migrate-up && make seed` |
| Test taps wrong `rental-row` | Many accumulated bookings | Reset the DB (see Test Data above) |
| Map markers not visible | Simulator location not set | `setLocation` before asserting map |

---

## Known Limitations

| Limitation | Reason | Workaround |
|-----------|--------|------------|
| Map marker / callout tap | React Native Maps renders markers in native MapKit outside the accessibility tree; Maestro cannot interact with them | Manual verification only |
| Native camera | Vision Camera uses native capture; no JS bridge for Maestro to hook | `EXPO_PUBLIC_E2E_MODE=true` activates fixture-photo bypass |
| Stripe Identity KYC | Native Stripe SDK; cannot be automated | Backend bypasses identity check when `E2E_MODE=true` |
| Real-time Pusher events | Tests don't assert on push-triggered UI updates | Soketi (local Pusher) runs via Docker; manual verification if flaky |
| Shared test accounts | `bob@test.com` accumulates bookings across runs | Tests rely on backend's `ORDER BY created_at DESC` sort |

---

## Pass Rate (Task 9.9 Validation)

Three consecutive full suite runs on 2026-04-12:

| Run | Result | Duration | Notes |
|-----|--------|----------|-------|
| 1 | ✅ 28/28 passed | ~12 min | Baseline |
| 2 | ✅ 28/28 passed | ~11 min | — |
| 3 | ✅ 28/28 passed | ~12 min | — |

**Reliability fixes applied in this task (task 9.9):**

1. `booking/cancel-booking.yaml` — Fixed invalid YAML syntax: `assertVisible: "text"\n    timeout: N` (4-space indent under scalar) rewritten as proper nested mapping.
2. `handoff/check-in.yaml` — Fixed two malformed timeout annotations: `assertVisible: "text"\n  timeout: N` (sibling key, not child) rewritten as `assertVisible: { text: ..., timeout: ... }`.
3. `handoff/check-out.yaml` — Same fix as check-in (Location verified assertion).
4. `discovery/search-listings.yaml` — Added `timeout: 5000` to `assertNotVisible: search-results` so the debounce can fire before the assertion runs.
5. `discovery/browse-feed.yaml` — Increased post-refresh `listing-card` timeout from 8 s to 12 s to absorb the extra network round-trip after pull-to-refresh.
6. `booking/create-booking-request.yaml` — Increased `screen-checkout` timeout from 5 s to 8 s (KYC gate evaluation on navigation).
7. `messaging/send-message.yaml` — Removed broken `assertNotVisible: { text: "..." }` step (the same text is visible in the message bubble just asserted above, so the assertion would always fail).
