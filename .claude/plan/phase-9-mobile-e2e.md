# Phase 9 — Mobile E2E Test Suite

> **Scope:** Build a comprehensive Maestro-based E2E test suite that exercises every core user flow on iOS Simulator against real backend services. Fix all bugs discovered during test authoring. Integrate into CI.
> **Exit criteria:** `make test-mobile-e2e` passes reliably. Every core user flow has at least one happy-path test and one error-path test. All discovered bugs are fixed. CI runs the suite on every PR.
> **Blockers:** Phase 8 tasks through 8.4 completed (app compiles, backend runs, seed data exists).
> **Recommended model:** Opus (extended thinking) for tasks 9.0, 9.3, 9.5, 9.6, 9.7, 9.10, 9.11. Sonnet acceptable for 9.1, 9.2, 9.4, 9.8, 9.9.

---

## Why Maestro

Maestro is the recommended E2E framework for this project:

- **First-class Expo support** — works with expo-dev-client builds out of the box
- **YAML-based flows** — trivial for AI agents to write, read, debug, and iterate on
- **Lowest flakiness** (<1%) — smart waits, automatic retries, no manual synchronization
- **Simple CI** — single CLI install, no Appium server or WebDriver
- **Black-box testing** — tests what the user sees, catches real UX bugs
- **Built-in simulator control** — `setLocation`, `clearState`, screenshots on failure

If gray-box native module testing is later needed (e.g., asserting on internal Stripe state), Detox can be added for those specific cases only.

---

## Conventions

### Test Organization
```
mobile/e2e/
  flows/
    auth/
      login.yaml
      register.yaml
      logout.yaml
      token-refresh.yaml
    discovery/
      browse-feed.yaml
      search-listings.yaml
      map-view.yaml
    listing/
      create-listing.yaml
      view-listing-detail.yaml
    booking/
      create-booking-request.yaml
      view-booking-status.yaml
      host-accept-booking.yaml
      host-decline-booking.yaml
      cancel-booking.yaml
    handoff/
      check-in.yaml
      active-rental.yaml
      check-out.yaml
      return-confirmation.yaml
    messaging/
      view-conversations.yaml
      send-message.yaml
    profile/
      view-profile.yaml
      referrals.yaml
      sign-out.yaml
    disputes/
      file-dispute.yaml
      view-dispute-status.yaml
    ratings/
      rate-counterparty.yaml
  helpers/
    login-as-renter.yaml       # Reusable: login as seeded renter
    login-as-host.yaml         # Reusable: login as seeded host
    seed-booking.yaml          # Reusable: create booking via API
    navigate-to-tab.yaml       # Reusable: tab navigation
  config/
    dev.env                    # Environment config (API URL, test credentials)
  Makefile                     # Targets: test-all, test-auth, test-booking, etc.
```

### Naming
- Flow files: `{action}.yaml` (e.g., `login.yaml`, `create-booking-request.yaml`)
- Helpers: `{verb}-{noun}.yaml` (e.g., `login-as-renter.yaml`)
- Test IDs in app code: `testID="screen-{name}"`, `testID="btn-{action}"`, `testID="input-{field}"`

### Bug Fix Protocol
When a test reveals a bug:
1. Document the bug inline in the flow YAML as a comment (`# BUG: description`)
2. Fix the bug in the app code (mobile and/or backend)
3. Remove the comment once the fix is verified
4. The task is NOT complete until the test passes

### Test Data
Tests run against real backend services with seeded data (from Phase 8.0). Tests that mutate state (create bookings, send messages) must:
- Use dedicated test user accounts (seeded in 8.0)
- Clean up after themselves where possible
- Be idempotent — running twice produces the same result

---

## User Flows Covered

All flows are derived from the PRD (rentmy-prd-v8.md) and implementations across Phases 1-7.

### Authentication (3 flows)
| Flow | Steps | API Endpoints |
|------|-------|---------------|
| Register | Enter name/email/password → Submit → Redirect to feed | `POST /api/v1/auth/register` |
| Login | Enter email/password → Submit → Redirect to feed | `POST /api/v1/auth/login` |
| Logout | Tap Sign Out → Redirect to login | `POST /api/v1/auth/logout` |

### Discovery (3 flows)
| Flow | Steps | API Endpoints |
|------|-------|---------------|
| Browse Feed | Location acquired → Feed loads → Scroll → Cards render | `GET /api/v1/discovery/feed` |
| Search | Type query → Debounce → Results load → Apply filters | `GET /api/v1/discovery/search` |
| Map View | Map renders → Pan/zoom → Markers appear → Tap marker → Callout | `GET /api/v1/discovery/map` |

### Listing (2 flows)
| Flow | Steps | API Endpoints |
|------|-------|---------------|
| View Detail | Tap card → Detail loads → Photos/price/host render | `GET /api/v1/listings/:id` |
| Create Listing | Location → Photos → AI autofill → Edit → Publish | `POST /api/v1/listings`, `POST /api/v1/media/upload` |

### Booking (5 flows)
| Flow | Steps | API Endpoints | State Transitions |
|------|-------|---------------|-------------------|
| Create Request | Select dates → Cost breakdown → Confirm | `POST /api/v1/bookings` | → REQUESTED |
| View Status | See pending request → Timer → Status updates | `GET /api/v1/bookings/:id` | REQUESTED → ... |
| Host Accept | View request → Tap Accept → PIN generated | `POST /api/v1/bookings/:id/accept` | REQUESTED → ACCEPTED |
| Host Decline | View request → Tap Decline → Hold released | `POST /api/v1/bookings/:id/decline` | REQUESTED → DECLINED |
| Cancel | View booking → Tap Cancel → Fee calculated | `POST /api/v1/bookings/:id/cancel` | → CANCELLED |

### Handoff (4 flows)
| Flow | Steps | API Endpoints | State Transitions |
|------|-------|---------------|-------------------|
| Check-In | GPS verify → PIN entry → Photo capture → Confirm | `POST /api/v1/proximity/verify`, `POST /api/v1/proximity/pin` | ACCEPTED → ACTIVE |
| Active Rental | Countdown timer → Navigate button → Check-out CTA | `GET /api/v1/bookings/:id` | ACTIVE |
| Check-Out | GPS verify → Photo capture → Diff triggered | `POST /api/v1/proximity/verify` | ACTIVE → COMPLETED |
| Return Confirm | Side-by-side photos → Hold release status | (display only) | COMPLETED |

### Messaging (2 flows)
| Flow | Steps | API Endpoints |
|------|-------|---------------|
| View Conversations | Open Messages tab → List renders → Tap conversation | `GET /api/v1/conversations` |
| Send Message | Open conversation → Type → Send → Message appears | `POST /api/v1/messages` |

### Profile (3 flows)
| Flow | Steps | API Endpoints |
|------|-------|---------------|
| View Profile | Open Profile tab → Name/email/listings render | `GET /api/v1/users/me` |
| Referrals | Tap Invite → Stats render → Share link | `GET /api/v1/users/me/referrals` |
| Sign Out | Tap Sign Out → Redirect to login | (local state clear) |

### Disputes (2 flows)
| Flow | Steps | API Endpoints | State Transitions |
|------|-------|---------------|-------------------|
| File Dispute | Select reason → Describe → Submit | `POST /api/v1/transactions/:id/disputes` | ACTIVE → DISPUTED |
| View Status | Open dispute → Timeline renders → Status updates | `GET /api/v1/transactions/:id/disputes` | DISPUTED → COMPLETED |

### Ratings (1 flow)
| Flow | Steps | API Endpoints |
|------|-------|---------------|
| Rate Counterparty | Open completed rental → Select bubbles → Submit | `POST /api/v1/ratings` |

**Total: 25 user flows, ~50+ individual test assertions**

---

## Step 9.0 — Maestro Setup & Test Infrastructure

**Goal:** Install Maestro, configure it for the Expo app, create the test directory structure, add reusable helpers, wire up `make test-mobile-e2e`, and verify the whole pipeline works end-to-end with one trivial test.

**What to do:**
1. Install Maestro CLI: `curl -Ls https://get.maestro.mobile.dev | bash`
2. Create `mobile/e2e/` directory structure (see Conventions above)
3. Add testID attributes to key app components (at minimum: login form, register form, tab bar items, feed list) — these are the hooks Maestro uses to find elements
4. Write `mobile/e2e/flows/auth/login.yaml` — the simplest possible flow:
   - Launch app
   - Assert login screen visible
   - Enter seeded user credentials
   - Tap sign in
   - Assert feed screen visible
5. Create helper flows:
   - `login-as-renter.yaml` — reusable login with renter credentials
   - `login-as-host.yaml` — reusable login with host credentials
6. Create `mobile/e2e/config/dev.env` with test credentials and API URL
7. Add Makefile targets:
   ```makefile
   # Root Makefile
   test-mobile-e2e:
   	cd mobile && maestro test e2e/flows/

   test-mobile-e2e-auth:
   	cd mobile && maestro test e2e/flows/auth/

   test-mobile-e2e-booking:
   	cd mobile && maestro test e2e/flows/booking/
   # ... etc for each category
   ```
8. Build the app for simulator: `cd mobile && npx expo run:ios`
9. Run the login test: `maestro test mobile/e2e/flows/auth/login.yaml`
10. Fix any bugs found (there will be bugs — auth flow, navigation, missing testIDs)

**Prerequisites:**
- Docker services running (`docker compose up -d`)
- Backend running with seeded data (`cd backend && make dev`)
- App built for iOS Simulator (`cd mobile && npx expo run:ios`)

**Deliverable:** Maestro installed, directory structure created, login test passing, `make test-mobile-e2e` wired up.

**Verification:**
```bash
maestro --version                                    # Maestro CLI installed
cd mobile && maestro test e2e/flows/auth/login.yaml  # Login flow passes
make test-mobile-e2e-auth                            # Makefile target works
```

**Recommended model:** Opus (extended thinking) — this task sets architectural patterns for all subsequent tests.

---

## Step 9.1 — E2E: Authentication Flows

**Goal:** Full test coverage for register, login, and logout. Fix all bugs found.

**Flows to write:**
1. `auth/register.yaml` — Happy path: fill form → submit → lands on feed
2. `auth/register.yaml` — Error path: duplicate email → error message shown
3. `auth/register.yaml` — Validation: empty fields → validation errors shown
4. `auth/login.yaml` — Happy path: existing user → submit → lands on feed
5. `auth/login.yaml` — Error path: wrong password → error message shown
6. `auth/logout.yaml` — Sign out → redirected to login → cannot access tabs

**testIDs to add (if missing):**
- `input-email`, `input-password`, `input-name`
- `btn-sign-in`, `btn-register`, `btn-sign-out`
- `screen-login`, `screen-register`, `screen-feed`
- `error-message` (validation/API errors)

**Bug fix protocol:** Every test that fails = a bug to fix. Fix the code, re-run the test, move on only when green.

**Verification:**
```bash
cd mobile && maestro test e2e/flows/auth/
```
All auth flows pass.

---

## Step 9.2 — E2E: Discovery Flows (Feed, Search, Map)

**Goal:** Test the three discovery paths. Fix all bugs found.

**Flows to write:**
1. `discovery/browse-feed.yaml`
   - Login → Feed tab loads → At least 1 listing card visible
   - Scroll down → More cards load (infinite scroll)
   - Pull to refresh → Feed reloads
   - Tap card → Navigate to listing detail

2. `discovery/search-listings.yaml`
   - Login → Search tab → Type query → Results appear
   - Clear query → Results clear
   - Apply filter (price range) → Results filter

3. `discovery/map-view.yaml`
   - Login → Map tab → Map renders
   - Set simulated location: `maestro setLocation 34.05 -118.24`
   - Markers visible on map
   - Tap marker → Callout appears

**testIDs to add:**
- `tab-feed`, `tab-search`, `tab-map`, `tab-messages`, `tab-profile`
- `feed-list`, `listing-card`, `search-input`, `filter-sheet`
- `map-container`, `map-marker`, `map-callout`

**Verification:**
```bash
cd mobile && maestro test e2e/flows/discovery/
```

---

## Step 9.3 — E2E: Listing Detail & Creation

**Goal:** Test viewing a listing and the full listing creation flow. Fix all bugs found.

**Flows to write:**
1. `listing/view-listing-detail.yaml`
   - Login → Feed → Tap listing → Detail screen loads
   - Photos render (gallery swipe)
   - Price, description, host info visible
   - "Rent Now" button visible
   - Back navigation works

2. `listing/create-listing.yaml`
   - Login as host → Profile tab → Tap "Create Listing"
   - Location step renders
   - Camera step: `E2E_MODE=true` bypasses native camera, uses fixture photos
   - AI autofill returns data → Fields populated
   - Edit fields → Submit → Listing created
   - Verify listing appears in host's profile

**Camera mocking:** The AngleEnforcedCamera uses react-native-vision-camera which cannot be automated by Maestro. When `E2E_MODE=true`, the camera component skips native capture and uses pre-uploaded test fixture photos from `e2e/fixtures/`. See [Appendix: Mocking Strategy](#appendix-mocking-strategy).

**Recommended model:** Opus — complex mocking decisions.

**Verification:**
```bash
cd mobile && maestro test e2e/flows/listing/
```

---

## Step 9.4 — E2E: Profile & Referral Flows

**Goal:** Test profile screen rendering and referral system. Fix all bugs found.

**Flows to write:**
1. `profile/view-profile.yaml`
   - Login → Profile tab → Name, email, avatar visible
   - My Listings section renders (or empty state)
   - Create Listing button visible

2. `profile/referrals.yaml`
   - Login → Profile → Tap "Invite Friends"
   - Referral screen loads → Stats render
   - Share button visible

3. `profile/sign-out.yaml`
   - Login → Profile → Tap Sign Out
   - Redirected to login screen
   - Attempt to navigate to feed → Blocked (auth gate)

**Verification:**
```bash
cd mobile && maestro test e2e/flows/profile/
```

---

## Step 9.5 — E2E: Booking Request & Status Flows

**Goal:** Test the full booking creation flow from "Rent Now" through status tracking. Fix all bugs found.

**Flows to write:**
1. `booking/create-booking-request.yaml`
   - Login as renter → Feed → Listing detail → Tap "Rent Now"
   - KYC gate: `E2E_MODE=true` mocks Stripe Identity verification (auto-approves). Test both paths: already-verified user skips gate, unverified user hits mocked KYC and gets verified.
   - Date picker → Select start/end dates
   - Cost breakdown renders (rental fee + hold)
   - Payment method selector → Select test card
   - Confirm → Booking created → Navigate to status screen

2. `booking/view-booking-status.yaml`
   - Pre-create booking via API helper
   - Login as renter → Rentals tab → Tap booking
   - Status screen shows "Waiting for host response"
   - Timer countdown visible

3. `booking/host-accept-booking.yaml`
   - Pre-create booking via API (status: REQUESTED)
   - Login as host → Rentals tab → Tap pending request
   - Tap Accept → Status changes to ACCEPTED
   - PIN displayed to host

4. `booking/host-decline-booking.yaml`
   - Pre-create booking via API (status: REQUESTED)
   - Login as host → Tap Decline → Status changes to DECLINED
   - Verify renter sees DECLINED status

5. `booking/cancel-booking.yaml`
   - Pre-create booking via API (status: REQUESTED)
   - Login as renter → Tap Cancel → Booking cancelled
   - Cancellation fee displayed (if applicable)

**API helpers needed:** These flows require creating bookings in specific states. Write a shell script or Maestro `runScript` that calls the backend API to seed bookings.

**Recommended model:** Opus — booking state machine is the most complex flow in the app.

**Verification:**
```bash
cd mobile && maestro test e2e/flows/booking/
```

---

## Step 9.6 — E2E: Handoff Flows (Check-In, Active Rental, Check-Out)

**Goal:** Test the physical handoff flow. This is the most complex flow in the app and the most likely to have bugs. Fix all bugs found.

**Flows to write:**
1. `handoff/check-in.yaml`
   - Pre-create booking in ACCEPTED state via API
   - Login as renter
   - Navigate to check-in screen
   - Set simulated location near listing: `setLocation 34.05 -118.24`
   - GPS verification step → Success
   - PIN entry → Enter correct PIN → Success
   - Photo step: use `__DEV__` bypass or pre-seeded photos
   - Verify transition to ACTIVE state

2. `handoff/active-rental.yaml`
   - Pre-create booking in ACTIVE state via API
   - Login as renter → Navigate to active rental
   - Countdown timer visible and ticking
   - "Navigate to return" button visible
   - "Start check-out" button visible

3. `handoff/check-out.yaml`
   - Pre-create booking in ACTIVE state via API
   - Login as renter → Navigate to check-out
   - GPS verification → Success
   - Photo capture → Use bypass/pre-seeded
   - Verify transition to COMPLETED state

4. `handoff/return-confirmation.yaml`
   - Pre-create booking in COMPLETED state via API
   - Login as renter → Navigate to return confirmation
   - Check-in vs check-out photos render
   - Hold release status visible

**GPS mocking:** Maestro supports `setLocation` to simulate GPS coordinates. Set both renter and listing to same coords to pass proximity check.

**Recommended model:** Opus — handoff has the most moving parts (GPS, PIN, photos, state transitions, real-time updates).

**Verification:**
```bash
cd mobile && maestro test e2e/flows/handoff/
```

---

## Step 9.7 — E2E: Messaging Flows

**Goal:** Test conversation listing and message sending. Fix all bugs found.

**Flows to write:**
1. `messaging/view-conversations.yaml`
   - Pre-seed messages between test users via API
   - Login as renter → Messages tab
   - Conversation list renders with counterparty name
   - Last message preview visible
   - Tap conversation → Opens message thread

2. `messaging/send-message.yaml`
   - Pre-seed conversation via API
   - Login as renter → Messages → Open conversation
   - Type message → Tap send
   - Message appears in thread
   - Verify message bubble styling (sent vs received)

**Verification:**
```bash
cd mobile && maestro test e2e/flows/messaging/
```

---

## Step 9.8 — E2E: Dispute & Rating Flows

**Goal:** Test dispute filing and post-rental rating. Fix all bugs found.

**Flows to write:**
1. `disputes/file-dispute.yaml`
   - Pre-create booking in ACTIVE state via API
   - Login as renter → Active rental → Tap "Report an issue"
   - Select reason → Enter description → Submit
   - Dispute confirmation screen renders

2. `disputes/view-dispute-status.yaml`
   - Pre-create disputed booking via API
   - Login as renter → Rentals → Tap disputed booking
   - Timeline renders → Status visible

3. `ratings/rate-counterparty.yaml`
   - Pre-create COMPLETED booking via API (no rating yet)
   - Login as renter → Rentals → Tap completed rental
   - "Rate" button visible → Tap
   - Select rating bubbles → Submit
   - Confirmation shown

**Verification:**
```bash
cd mobile && maestro test e2e/flows/disputes/
cd mobile && maestro test e2e/flows/ratings/
```

---

## Step 9.9 — Full Regression Suite & Reliability

**Goal:** Run the entire E2E suite end-to-end 3 times consecutively. Fix any flaky tests. Ensure deterministic results.

**What to do:**
1. Run `make test-mobile-e2e` — all flows, start to finish
2. If any test fails:
   - Is it a real bug? Fix the app code.
   - Is it flaky? Fix the test (add waits, improve selectors, fix race conditions).
3. Run the full suite 3 more times. If any run has a different result than the others, the suite is not stable — keep fixing.
4. Document final pass rate and any known limitations in `mobile/e2e/README.md`

**Exit criteria:** 3 consecutive full runs with identical results (all green).

**Verification:**
```bash
make test-mobile-e2e  # Run 1 — all pass
make test-mobile-e2e  # Run 2 — all pass
make test-mobile-e2e  # Run 3 — all pass
```

---

## Step 9.10 — CI Integration (Maestro Cloud + GitHub Actions)

**Goal:** Run the E2E suite automatically on pushes to main. The primary consumer of these tests is the AI agent — it needs to know immediately if a change breaks existing functionality.

**What to do:**
1. Set up Maestro Cloud (free tier):
   - Create account at https://cloud.maestro.dev
   - Configure project + upload app binary
   - Store API key in GitHub Secrets (`MAESTRO_CLOUD_API_KEY`)
2. Create `.github/workflows/e2e-mobile.yml`:
   - Trigger: `push` to `main` only (not PRs — saves cost)
   - Steps:
     - Checkout code
     - Build iOS app: `npx expo run:ios --no-install` (or use cached build)
     - Upload flows + app to Maestro Cloud
     - Wait for results
     - Upload failure screenshots as artifacts
   - Fallback: if Maestro Cloud is unavailable, run locally on `macos-latest` runner
3. Add a `make test-mobile-e2e-cloud` target for manual cloud runs
4. Test the workflow on a branch first, then merge

**Agent usage:** Before pushing changes, the agent should run `make test-mobile-e2e` locally. The CI run on main is the backstop — if the agent missed something, the CI catch prevents accumulating regressions.

**Recommended model:** Opus — CI configuration is tricky to get right.

**Verification:**
- Push to main → GitHub Actions triggers Maestro Cloud → All pass → Green check
- `make test-mobile-e2e-cloud` works from local dev

---

## Step 9.11 — Documentation & Handoff

**Goal:** Document the E2E test suite so future developers (human or agent) can maintain it.

**What to do:**
1. Write `mobile/e2e/README.md`:
   - How to install Maestro
   - How to run tests locally
   - How to add new tests (template + conventions)
   - How to debug failures (screenshots, logs)
   - Known limitations (camera, Stripe Identity)
2. Update root `README.md` with E2E test section
3. Update `CLAUDE.md`:
   - Add `make test-mobile-e2e` to verification commands
   - Add E2E test requirement to "What Done Means" section
   - Add Maestro conventions to coding standards
4. Write handoff doc + update progress.json

**Verification:**
- README is clear and actionable
- CLAUDE.md updated
- A new developer could follow the README and run the suite

---

## Dependencies

```
9.0  (setup)           → no deps (just Phase 8.0 seeded data)
9.1  (auth)            → 9.0
9.2  (discovery)       → 9.0
9.3  (listing)         → 9.0
9.4  (profile)         → 9.0
9.5  (booking)         → 9.1 (needs auth helpers), 9.3 (needs listing detail working)
9.6  (handoff)         → 9.5
9.7  (messaging)       → 9.1
9.8  (disputes/rating) → 9.5
9.9  (reliability)     → 9.1 through 9.8 (all flows must exist)
9.10 (CI)              → 9.9
9.11 (docs)            → 9.10
```

Tasks 9.1, 9.2, 9.3, 9.4 can run in parallel after 9.0.

---

## Appendix: Mocking Strategy

### GPS Location
Maestro supports `setLocation lat lng`. Also available: `xcrun simctl location booted set lat lng`. Use listing coordinates from seed data so proximity checks pass.

### Camera (AngleEnforcedCamera)
Cannot be automated via Maestro (native camera hardware). Strategy:
- Add a `__DEV__` + `E2E_MODE` environment variable check in AngleEnforcedCamera
- When `E2E_MODE=true`: skip camera, use pre-uploaded test photos from `e2e/fixtures/`
- This is standard practice for E2E testing of camera features

### Stripe Payments
Use Stripe test mode (already configured). Test card: `4242 4242 4242 4242`.

### Stripe Identity (KYC)
When `E2E_MODE=true`, the VerifyScreen bypasses the native Stripe Identity SDK and auto-sets `identityStatus=VERIFIED` via a direct API call to a test-only endpoint (`POST /api/v1/test/verify-user`). This endpoint only exists when the backend is started with `E2E_MODE=true`. It must NOT be available in production builds.

### Real-Time (Pusher)
Soketi (local Pusher replacement) is already running via Docker Compose. Real-time updates should work in tests. If flaky, add explicit waits after state-changing API calls.

### Test Data Isolation
Each flow that creates data should use a unique identifier (timestamp or random suffix) to avoid conflicts with other test runs. Helper scripts clean up test data between runs.
