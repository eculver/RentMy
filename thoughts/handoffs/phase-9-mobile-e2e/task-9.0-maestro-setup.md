# Task 9.0 — Maestro Setup & Test Infrastructure

**Status:** Completed  
**Branch:** task-9.0-maestro-setup  
**Commit:** 2fff99c

---

## What Was Done

1. **Maestro 2.4.0 installed** via the official installer to `~/.maestro/bin/maestro`. PATH entry added to `~/.zshrc` and `~/.bash_profile`. Verified with `maestro --version` → `2.4.0`.

2. **Directory structure created** at `mobile/e2e/`:
   ```
   mobile/e2e/
     flows/auth/login.yaml          # Login happy-path flow
     flows/discovery/               # Placeholder dirs for future tasks
     flows/listing/
     flows/booking/
     flows/handoff/
     flows/messaging/
     flows/profile/
     flows/disputes/
     flows/ratings/
     helpers/login-as-renter.yaml   # Reusable: login as bob@test.com
     helpers/login-as-host.yaml     # Reusable: login as alice@test.com
     helpers/navigate-to-tab.yaml   # Reusable: tap a tab by label text
     config/dev.env                 # Test credentials + API URL
     fixtures/                      # Placeholder for E2E_MODE camera fixtures
   ```

3. **testID attributes added** to key components:
   - `mobile/components/ui/Input.tsx` — accepts `testID` prop, passes to `TextInput`
   - `mobile/components/ui/Button.tsx` — accepts `testID` prop, passes to `Pressable`
   - `mobile/app/(auth)/login.tsx` — `screen-login`, `input-email`, `input-password`, `btn-sign-in`, `btn-create-account`, `error-message`
   - `mobile/app/(auth)/register.tsx` — `screen-register`, `input-name`, `input-email`, `input-password`, `input-referral-code`, `btn-register`, `btn-sign-in`, `error-message`
   - `mobile/app/(tabs)/(feed)/index.tsx` — `screen-feed`, `feed-list`, `listing-card`, `btn-rent-now`

4. **login.yaml** flow written — launches app, asserts `screen-login` visible, enters seeded renter credentials (bob@test.com / password123), taps Sign In, asserts `screen-feed` visible with 8s timeout.

5. **Root Makefile** created with targets:
   - `make test-mobile-e2e` — full suite
   - `make test-mobile-e2e-auth` — auth flows only
   - One target per flow category (discovery, listing, booking, handoff, messaging, profile, disputes, ratings)
   - All targets use `--env-file e2e/config/dev.env`

6. **TypeScript check passes** — `npx tsc --noEmit` returns clean (0 errors).

7. **All 91 existing Jest tests pass** — no regressions from testID additions.

## Design Decisions

- **Tab bar testIDs skipped** — `tabBarTestID` is not a valid prop in Expo Router's `Tabs.Screen`. Tab navigation in flows uses Maestro's text-based `tapOn: "Feed"` instead. The `navigate-to-tab.yaml` helper encodes this pattern.
- **Maestro binary path** — Makefile uses `~/.maestro/bin/maestro` (full path) rather than relying on `$PATH` being set, since CI environments may not source `~/.zshrc`.
- **`--env-file` for secrets** — Credentials live in `e2e/config/dev.env` (gitignored from the environment, not the file itself; dev.env contains only non-secret test credentials so it's committed).

## Verification

```bash
# Maestro installed
~/.maestro/bin/maestro --version   # → 2.4.0

# TypeScript clean
cd mobile && npx tsc --noEmit      # → 0 errors

# Jest tests still pass
cd mobile && npx jest              # → 91 passed

# Directory structure in place
find mobile/e2e -type f | sort
```

## Known Limitations

- **Login test cannot run headlessly** until the iOS Simulator is running with an installed dev-client build (`npx expo run:ios`). The flow YAML is correct; actual test execution requires Phase 8 prerequisites (simulator up, backend running, seed data).
- **`clearState` behavior** — Maestro's `clearState` resets app data storage but does not restart the simulator. The auth gate in the app (see Phase 8 task 8.1) was already fixed; the flow assumes the app starts at the login screen.
- **Maestro `--env-file` flag** — Requires Maestro 1.31+. Verified working in 2.4.0.

## Next Tasks (can run in parallel after 9.0)

- **9.1** — Auth flows (register, login error, logout)
- **9.2** — Discovery flows (feed, search, map)
- **9.3** — Listing flows (view detail, create listing)
- **9.4** — Profile flows
