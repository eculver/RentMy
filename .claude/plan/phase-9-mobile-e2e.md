# Phase 9 — Mobile E2E Test Suite

> **Scope:** Get the Maestro E2E test suite running against the real app on iOS Simulator. Fix every bug discovered. Achieve reliable, repeatable test runs.
> **Exit criteria:** `make test-mobile-e2e` passes 3 consecutive times with identical results. Maestro test output logs are captured as evidence for every task.
> **Blockers:** Backend compiles and serves requests. App builds and runs in iOS Simulator.
> **Required model:** Opus for ALL tasks. Sonnet has demonstrated it will fake verification rather than debug real failures.

---

## CRITICAL: Verification Rules

**These rules exist because a previous Phase 9 attempt fabricated all test results. Every rule is mandatory.**

1. **No task is complete without Maestro test output.** The handoff doc MUST include the full terminal output of `maestro test` showing pass/fail for each flow. Screenshots of test output are NOT acceptable — paste the actual text.

2. **If Maestro is not installed, install it first.** Run `maestro --version` at the start of every session. If it fails, run `curl -Ls https://get.maestro.mobile.dev | bash` before doing anything else.

3. **If the app doesn't build, fix the build first.** Do not write test flows for an app that doesn't compile. `npx expo run:ios` must succeed before any test work begins.

4. **If the backend doesn't start, fix the backend first.** `curl -sf http://localhost:8080/health` must return 200 before any test work begins.

5. **Do NOT create test-only backend endpoints.** Tests run against the real API. If a test requires data in a specific state, use the real API endpoints to create it (register user, create listing, create booking, etc.) or use a SQL seed script in `mobile/e2e/seed/`.

6. **Do NOT add global E2E_MODE switches.** No `IS_E2E`, no `EXPO_PUBLIC_E2E_MODE`, no `E2E_MODE` environment variables that bypass real code paths. If a specific flow requires a narrow override (e.g., camera hardware), document it in the flow's YAML comments and implement it as a `__DEV__`-only code path in that specific component. Every override must be justified in the handoff doc.

7. **Tests must fail when the app is broken.** If you can make all tests pass without fixing any app bugs, your tests are fake. A properly working test suite WILL find bugs. Document every bug found and fixed.

8. **Run each test individually first, then as a suite.** Don't run the full suite until individual flows pass. When a flow fails, fix the bug, re-run THAT flow, confirm it passes, then move on.

---

## Why Maestro

- **First-class Expo support** — works with expo-dev-client builds
- **YAML-based flows** — AI agents can write, read, debug, and iterate
- **<1% flakiness** — smart waits, automatic retries
- **Black-box testing** — tests what the user actually sees
- **Built-in simulator control** — `setLocation`, `clearState`, screenshots on failure
- **CLI:** installed at `~/.maestro/bin/maestro` (or `maestro` if on PATH)

---

## Test Organization

```
mobile/e2e/
  flows/               # All test flows organized by feature
    auth/              # Login, register, logout
    discovery/         # Feed, search, map
    listing/           # View detail, create listing
    booking/           # Create, status, accept, decline, cancel
    handoff/           # Check-in, active rental, check-out, return
    messaging/         # Conversations, send message
    profile/           # View profile, referrals, sign out
    disputes/          # File dispute, view status
    ratings/           # Rate counterparty
  helpers/             # Reusable sub-flows (login-as-renter, etc.)
  config/
    dev.env            # Test credentials and API URL
  seed/                # SQL or shell scripts to seed test data
```

---

## Override Policy (Per-Component, Not Global)

Most tests require ZERO app-side overrides. The real app, real backend, real API.

**Overrides are allowed ONLY for hardware that Maestro cannot control:**

| Component | Override | Justification | Implementation |
|-----------|----------|---------------|----------------|
| Camera (AngleEnforcedCamera) | Skip native camera, use gallery picker | Maestro cannot trigger hardware camera | `__DEV__` check in the component only |
| Stripe Identity (KYC) | Pre-verify test users in seed data | Maestro cannot scan physical ID documents | SQL seed sets `identity_status = 'VERIFIED'` |
| GPS Location | None needed | Maestro's `setLocation` + `xcrun simctl location` handles this | N/A |
| Stripe Payments | None needed | Stripe test mode + test card numbers work normally | N/A |
| Push Notifications | None needed | Not tested in E2E (out of scope) | N/A |

**All other code paths must be exercised as-is.** If a test fails because real code is broken, fix the real code.

---

## Test Data Strategy

Tests use real API endpoints to seed data. NO test-only backend endpoints.

**Seed data options (prefer in this order):**
1. **SQL seed script** (`mobile/e2e/seed/e2e_seed.sql`) — run before suite via `psql`. Creates test users, listings, and bookings in specific states.
2. **Shell script** (`mobile/e2e/seed/setup.sh`) — calls real API endpoints via `curl` to register users, create listings, create bookings.
3. **Maestro `runScript`** — for flows that need dynamic data, use JavaScript in YAML to call APIs.

**Test accounts (seeded):**
- `alice@test.com` / `password123` — host, owns 5 listings, identity VERIFIED
- `bob@test.com` / `password123` — renter, no listings, identity VERIFIED

---

## Step 9.0 — Get the App Running (Build + Backend + Simulator)

**Goal:** Verify the entire stack works. This is the prerequisite for ALL other tasks.

**What to do (in order):**
1. `maestro --version` — confirm Maestro is installed
2. `docker compose up -d` — start Postgres, Redis, MinIO, Soketi
3. `cd backend && make dev` — start backend (runs migrations)
4. `curl -sf http://localhost:8080/health` — verify backend health
5. `cd mobile && npm ci` — install mobile deps
6. `cd mobile && npx expo run:ios` — build and launch in simulator
7. Wait for app to appear in simulator
8. `xcrun simctl io booted screenshot /tmp/rentmy-9.0-app-running.png` — capture proof
9. Try logging in manually with `bob@test.com` / `password123` via the simulator
10. If login fails → **FIX IT.** This is the most basic flow. Debug the backend response, check the database, fix whatever is broken.
11. Run the simplest Maestro test: `maestro test mobile/e2e/flows/auth/login.yaml`
12. If it passes → task done. If it fails → fix the bug and re-run.

**Handoff doc MUST include:**
- Output of `maestro --version`
- Output of `curl -sf http://localhost:8080/health`
- Output of `maestro test mobile/e2e/flows/auth/login.yaml` (full terminal output)
- Screenshot of the app in the simulator AFTER successful login
- List of every bug found and fixed (with file paths and descriptions)

**Verification:**
```bash
maestro --version
curl -sf http://localhost:8080/health
maestro test mobile/e2e/flows/auth/login.yaml
```

---

## Step 9.1 — Auth Flows

**Goal:** All auth E2E flows pass against the real app.

**Flows:** `mobile/e2e/flows/auth/` (login, register, logout, validation errors, wrong password, duplicate email)

**What to do:**
1. Run each flow individually: `maestro test mobile/e2e/flows/auth/login.yaml`
2. When a flow fails, read the error output. Debug the app code. Fix the bug. Re-run.
3. After all individual flows pass, run the suite: `maestro test mobile/e2e/flows/auth/`
4. All must pass.

**Handoff doc MUST include:**
- Full output of `maestro test mobile/e2e/flows/auth/` showing all flows pass
- List of every bug found and fixed

**Verification:**
```bash
maestro test mobile/e2e/flows/auth/
```

---

## Step 9.2 — Discovery Flows (Feed, Search, Map)

**Goal:** All discovery E2E flows pass against the real app.

**Flows:** `mobile/e2e/flows/discovery/` (browse feed, search, map view)

**Prerequisites:** Auth flows pass (9.1), seeded listings exist in database.

**What to do:**
1. Set simulator location: `xcrun simctl location booted set 34.0522 -118.2437`
2. Run each flow individually
3. Fix all bugs found
4. Run suite: `maestro test mobile/e2e/flows/discovery/`

**Expected bugs:** Feed not loading, search not returning results, map markers not rendering. These are real bugs — fix them.

**Handoff doc MUST include:** Full Maestro output + bug list.

**Verification:**
```bash
maestro test mobile/e2e/flows/discovery/
```

---

## Step 9.3 — Listing Flows

**Goal:** Listing detail and listing creation E2E flows pass.

**Flows:** `mobile/e2e/flows/listing/`

**Camera override (ONLY override in this task):**
- The create-listing flow requires the AngleEnforcedCamera, which Maestro cannot drive.
- Add a `__DEV__`-only "Use test photos" button in AngleEnforcedCamera that loads fixture images from the gallery instead of opening the native camera.
- This override MUST be behind `__DEV__` (not a custom env var) and MUST still upload the photos through the real media upload pipeline.
- Document this override in the handoff doc with justification.

**Verification:**
```bash
maestro test mobile/e2e/flows/listing/
```

---

## Step 9.4 — Profile & Referral Flows

**Goal:** Profile, referral, and sign-out E2E flows pass.

**Flows:** `mobile/e2e/flows/profile/`

**Verification:**
```bash
maestro test mobile/e2e/flows/profile/
```

---

## Step 9.5 — Booking Flows

**Goal:** All booking E2E flows pass (create request, view status, host accept, host decline, cancel).

**Flows:** `mobile/e2e/flows/booking/`

**Test data:** These flows need bookings in specific states. Seed them using:
- Real API calls via shell script (`mobile/e2e/seed/setup.sh`)
- OR SQL seed script (`mobile/e2e/seed/e2e_seed.sql`)
- Do NOT create test-only backend endpoints.

**Payment:** Stripe test mode handles this. Use test card `4242 4242 4242 4242`. If the Stripe sheet is problematic for Maestro, document the specific issue and propose a targeted fix (not a global bypass).

**Verification:**
```bash
maestro test mobile/e2e/flows/booking/
```

---

## Step 9.6 — Handoff Flows (Check-In, Active, Check-Out)

**Goal:** All handoff E2E flows pass.

**Flows:** `mobile/e2e/flows/handoff/`

**GPS:** Use Maestro's `setLocation` or `xcrun simctl location booted set` — no app-side bypass needed.

**Camera override:** Same `__DEV__` approach as Step 9.3 — the check-in and check-out flows that require photo capture use the same AngleEnforcedCamera component.

**PIN:** Use real PIN from the booking seed data (whatever PIN was generated when the host accepted the booking).

**Test data:** Bookings in ACCEPTED and ACTIVE states must be seeded via real API calls or SQL.

**Verification:**
```bash
maestro test mobile/e2e/flows/handoff/
```

---

## Step 9.7 — Messaging Flows

**Goal:** Conversation list and send-message E2E flows pass.

**Flows:** `mobile/e2e/flows/messaging/`

**Test data:** Seed at least one conversation between test users via real API.

**Verification:**
```bash
maestro test mobile/e2e/flows/messaging/
```

---

## Step 9.8 — Dispute & Rating Flows

**Goal:** Dispute and rating E2E flows pass.

**Flows:** `mobile/e2e/flows/disputes/`, `mobile/e2e/flows/ratings/`

**Test data:** Seed bookings in ACTIVE (for dispute) and COMPLETED (for rating) states.

**Verification:**
```bash
maestro test mobile/e2e/flows/disputes/
maestro test mobile/e2e/flows/ratings/
```

---

## Step 9.9 — Full Regression Suite & Reliability

**Goal:** Run the entire E2E suite 3 consecutive times with identical results.

**What to do:**
1. `make test-mobile-e2e` — Run 1
2. `make test-mobile-e2e` — Run 2
3. `make test-mobile-e2e` — Run 3
4. If any run differs: fix the flaky test or underlying bug.
5. All 3 runs must be green.

**Handoff doc MUST include:**
- Full terminal output of all 3 runs
- If any test was flaky, document what caused it and how it was fixed

**Verification:**
```bash
make test-mobile-e2e  # Run 1
make test-mobile-e2e  # Run 2
make test-mobile-e2e  # Run 3
```

---

## Step 9.10 — CI Integration (Maestro Cloud + GitHub Actions)

**Goal:** E2E suite runs automatically on pushes to main.

**What to do:**
1. Set up Maestro Cloud (free tier)
2. Create `.github/workflows/e2e-mobile.yml` — trigger on push to main only
3. Add `make test-mobile-e2e-cloud` target
4. Test the workflow

**Verification:**
- Push to main → CI runs → All pass → Green check

---

## Step 9.11 — Documentation & CLAUDE.md Update

**Goal:** Document the E2E suite and update project instructions.

**What to do:**
1. Write `mobile/e2e/README.md` — install, run, add tests, debug failures
2. Update root `README.md`
3. Update `CLAUDE.md` — add E2E to "What Done Means", add Maestro conventions

**Verification:**
- `test -f mobile/e2e/README.md`
- CLAUDE.md references E2E tests

---

## Dependencies

```
9.0  → no deps (get the stack running)
9.1  → 9.0
9.2  → 9.0
9.3  → 9.0
9.4  → 9.0
9.5  → 9.1, 9.3
9.6  → 9.5
9.7  → 9.1
9.8  → 9.5
9.9  → all of 9.1–9.8
9.10 → 9.9
9.11 → 9.10
```

Tasks 9.1–9.4 can run in parallel after 9.0.
