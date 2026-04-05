# Phase 7 — Test Infrastructure + Retroactive Coverage

## Goal

Set up integration and component testing infrastructure, then write retroactive tests for Phases 1-4. This phase exists because Phases 1-4 were built without integration tests. Going forward (Phases 5-6), every task includes tests as part of the task itself.

## Execution Order

Phase 7 tasks run **before** Phase 5. Tasks 7.1 and 7.2 set up infrastructure; 7.3-7.5 add retroactive coverage; 7.6 wires tests into CI.

---

## Step 7.1 — Backend Integration Test Infrastructure

**Goal:** Set up testcontainers-go so integration tests can run against real Postgres + Redis.

**PRD refs:** cross-cutting.md (Testing section)

### What to build

1. **Add dependencies to `go.mod`:**
   ```
   github.com/testcontainers/testcontainers-go
   github.com/testcontainers/testcontainers-go/modules/postgres
   github.com/testcontainers/testcontainers-go/modules/redis
   ```

2. **Create `backend/tests/integration/helpers.go`:**
   - `TestMain(m *testing.M)` — starts Postgres + Redis containers, runs migrations, stores DSN in package var
   - `NewTestDB(t *testing.T) *pgxpool.Pool` — returns a connection pool to the test database
   - `NewTestRedis(t *testing.T) *redis.Client` — returns a Redis client
   - `NewTestServer(t *testing.T) (*httptest.Server, *http.Client)` — wires up the full chi router with all services, returns a test HTTP server and client
   - `CleanupDB(t *testing.T, pool *pgxpool.Pool)` — truncates all tables between tests (TRUNCATE CASCADE)

3. **Create factory functions in `backend/tests/integration/factories.go`:**
   - `CreateTestUser(t, pool) User` — inserts a user with bcrypt password, returns the model
   - `CreateTestListing(t, pool, ownerID) Listing` — inserts a listing with valid location
   - `CreateTestBooking(t, pool, renterID, listingID) Booking` — inserts a booking in REQUESTED state
   - `LoginTestUser(t, client, email, password) string` — calls POST /auth/login, returns JWT token
   - `AuthHeader(token string) http.Header` — returns `Authorization: Bearer <token>`

4. **Create `backend/tests/integration/smoke_test.go`:**
   - One test that starts the server, hits GET /health, asserts 200
   - Proves the infrastructure works before adding more tests

### Verification

```bash
cd backend && go test ./tests/integration/... -v -count=1 -timeout 120s
```

The smoke test must pass. Docker must be running for testcontainers.

---

## Step 7.2 — Mobile Test Infrastructure

**Goal:** Set up Jest + React Native Testing Library + MSW so mobile screen/component tests can run.

**PRD refs:** cross-cutting.md (Testing section)

### What to build

1. **Install dependencies:**
   ```bash
   cd mobile && npm install --save-dev \
     jest \
     @testing-library/react-native \
     @testing-library/jest-native \
     msw \
     react-test-renderer \
     @types/jest \
     jest-expo
   ```

2. **Create `mobile/jest.config.ts`:**
   ```typescript
   import type { Config } from 'jest';

   const config: Config = {
     preset: 'jest-expo',
     setupFilesAfterSetup: ['<rootDir>/__tests__/setup.ts'],
     testMatch: ['<rootDir>/__tests__/**/*.test.{ts,tsx}'],
     transformIgnorePatterns: [
       'node_modules/(?!((jest-)?react-native|@react-native(-community)?)|expo(nent)?|@expo(nent)?/.*|@expo-google-fonts/.*|react-navigation|@react-navigation/.*|@sentry/react-native|native-base|react-native-svg|nativewind|react-native-reanimated)',
     ],
   };
   export default config;
   ```

3. **Create `mobile/__tests__/setup.ts`:**
   - Import and configure MSW with a set of default API handlers (login, register, listings feed)
   - Mock `expo-location`, `expo-camera`, `expo-image-picker` (they don't work in Jest)
   - Set up `@testing-library/jest-native` matchers

4. **Create `mobile/__tests__/lib/api.test.ts`:**
   - Test that the `ky` client sends auth headers
   - Test that token refresh works on 401
   - Proves MSW is wired up correctly

5. **Create `mobile/__tests__/components/smoke.test.tsx`:**
   - Render a simple component, assert it renders without crashing

6. **Add test script to `mobile/package.json`:**
   ```json
   "scripts": {
     "test": "jest",
     "test:watch": "jest --watch"
   }
   ```

### Verification

```bash
cd mobile && npx jest --passWithNoTests
```

Both smoke tests must pass.

---

## Step 7.3 — Phase 1-2 Backend Integration Tests

**Goal:** Test the user, media, listing, discovery, and payment API surfaces end-to-end.

**Dependencies:** [7.1]

### Tests to write in `backend/tests/integration/`

**`user_api_test.go`:**
- `TestRegisterAndLogin` — POST /auth/register → POST /auth/login → verify JWT claims
- `TestRegisterDuplicateEmail` — second register with same email returns 409
- `TestGetProfile` — GET /users/me with valid token returns user data
- `TestUpdateProfile` — PATCH /users/me updates fields
- `TestInvalidToken` — requests with bad/expired token return 401

**`listing_api_test.go`:**
- `TestCreateListing` — POST /listings creates listing with location
- `TestListingSevenDayCeiling` — listings with rentals in next 7 days show correct availability
- `TestGetListing` — GET /listings/:id returns listing with media
- `TestUpdateListing` — PATCH /listings/:id updates fields, verifies owner-only
- `TestDeleteListing` — DELETE /listings/:id soft-deletes, returns 404 on re-fetch

**`discovery_api_test.go`:**
- `TestFeedNearby` — GET /discovery/feed returns listings sorted by rank score
- `TestFeedPagination` — offset/limit pagination works correctly
- `TestSearchFulltext` — GET /discovery/search?q=... returns matching listings
- `TestMapBoundingBox` — GET /discovery/map with bbox returns listings in bounds
- `TestLocationFuzzing` — returned coordinates differ from stored coordinates by ~500m

**`payment_api_test.go`:**
- `TestTieredHoldCalculation` — verify hold tiers (≤$50, ≤$200, >$200)
- Note: Stripe API calls should be mocked at the HTTP level (httptest) or skipped with build tag

### Verification

```bash
cd backend && go test ./tests/integration/... -v -count=1 -timeout 180s
```

All new tests must pass. Existing unit tests must still pass.

---

## Step 7.4 — Phase 3-4 Backend Integration Tests

**Goal:** Test the booking lifecycle, proximity, notifications, messaging, and AI agent surfaces.

**Dependencies:** [7.1, 7.3]

### Tests to write

**`booking_api_test.go`:**
- `TestBookingLifecycle` — create user A (host), user B (renter), create listing, request booking, confirm, check-in, check-out, complete
- `TestBookingStateMachineRejections` — invalid transitions return 400
- `TestAutoDecline72Hours` — booking not confirmed in 72h auto-declines (mock time or use River test mode)
- `TestCancellationFees` — cancel at various points, verify correct fee tier
- `TestFraudVelocityCheck` — rapid booking requests trigger velocity block

**`proximity_api_test.go`:**
- `TestPINGeneration` — check-in generates 6-digit PIN
- `TestPINValidation` — correct PIN succeeds, wrong PIN fails
- `TestProximityCheck` — within radius returns true, outside returns false

**`notification_api_test.go`:**
- `TestNotificationCreation` — booking events create notification records
- `TestNotificationPreferences` — quiet hours suppress delivery
- `TestNotificationList` — GET /notifications returns user's notifications

**`agent_api_test.go`:**
- `TestModelRouterTierSelection` — verify correct tier selection for different task types
- `TestDecisionAuditLog` — agent decisions are recorded in audit log
- `TestVerificationAgentFlow` — submit KYC → agent processes → decision recorded

### Verification

```bash
cd backend && go test ./tests/integration/... -v -count=1 -timeout 180s
```

---

## Step 7.5 — Phase 1-4 Mobile Component Tests

**Goal:** Write screen tests for the core mobile screens built in Phases 1-4.

**Dependencies:** [7.2]

### Tests to write in `mobile/__tests__/`

**`screens/auth.test.tsx`:**
- Login screen renders email + password fields
- Login with invalid email shows validation error
- Login with valid credentials navigates to home (mock navigation)
- Register screen renders all required fields
- Register with mismatched passwords shows error

**`screens/listing-create.test.tsx`:**
- Listing creation form renders all fields
- Submit with missing title shows validation error
- Successful submission calls API (MSW handler) and navigates

**`screens/feed.test.tsx`:**
- Feed screen renders list of listings from API (MSW)
- Pull-to-refresh triggers new API call
- Empty state renders when no listings

**`screens/search.test.tsx`:**
- Search input debounces API calls
- Results render with correct listing data
- Filter toggles update query params

**`screens/booking.test.tsx`:**
- Booking request screen shows listing details and price breakdown
- Submit booking calls API
- Booking status screen shows correct state

**`components/ui.test.tsx`:**
- Shared UI components render without crashes
- Button press handlers fire correctly

### Verification

```bash
cd mobile && npx jest --verbose
```

---

## Step 7.6 — CI Pipeline Update

**Goal:** Add integration tests and mobile tests to GitHub Actions CI.

**Dependencies:** [7.1, 7.2]

### What to build

1. **Update `.github/workflows/backend.yml`:**
   - Add a job `integration-tests` that runs after `lint` and `test`
   - Uses `services:` to start Postgres and Redis (or relies on testcontainers with Docker-in-Docker)
   - Runs `go test ./tests/integration/... -v -count=1 -timeout 300s`
   - If using testcontainers: needs Docker socket access

2. **Update `.github/workflows/mobile.yml` (or create if missing):**
   - Add a job `test` that runs `npx jest --ci --coverage`
   - Reports coverage as a PR comment (optional)

3. **Add a `docker-compose.test.yml`** (optional, if testcontainers isn't used in CI):
   - Minimal Postgres + Redis for CI
   - No volumes, no persistent state

### Verification

```bash
# Locally simulate CI:
cd backend && go test ./tests/integration/... -v -count=1 -timeout 300s
cd mobile && npx jest --ci
```

---

## Testing Standards (for this phase and all future phases)

### What makes a good integration test

1. **Tests the contract, not the implementation** — assert on HTTP status codes, response bodies, and database state. Don't assert on internal function calls.
2. **Each test is independent** — uses factory functions to create its own data. No shared state between tests.
3. **Tests the sad path too** — 401 for unauthorized, 404 for missing, 409 for duplicates, 400 for bad input.
4. **Minimal mocking** — the whole point of integration tests is real dependencies. Only mock external services (Stripe, OSRM, Anthropic API).

### What makes a good mobile test

1. **Renders the real component** — not a shallow render. Use `render()` from RNTL.
2. **Interacts like a user** — `fireEvent.press()`, `fireEvent.changeText()`, not direct state manipulation.
3. **Asserts on visible output** — `getByText()`, `getByTestId()`, not internal component state.
4. **Mocks the network, not the components** — MSW intercepts API calls. Components render for real.
