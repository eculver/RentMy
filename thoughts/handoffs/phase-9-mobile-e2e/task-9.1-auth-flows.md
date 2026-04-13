# Task 9.1 — E2E: Authentication Flows

**Status:** Completed  
**Branch:** task-9.1-auth-flows  
**Commit:** 2b7f0a9

---

## What Was Done

### New Maestro flows (mobile/e2e/flows/auth/)

| File | Scenario |
|------|----------|
| `register.yaml` | Happy path: unique email via runScript → lands on feed |
| `register-duplicate-email.yaml` | Error path: seeded email (bob@test.com) → 409 → error-message visible |
| `register-validation.yaml` | Client-side validation: empty submit + invalid email → stays on register screen |
| `login-wrong-password.yaml` | Error path: valid email + wrong password → 401 → error-message visible |
| `logout.yaml` | Login → Profile tab → Sign Out → back to login → feed/profile not visible |

`login.yaml` (happy path) was already written in task 9.0 and required no changes.

### New script (mobile/e2e/scripts/gen-unique-email.js)

Maestro `runScript` helper that sets `output.UNIQUE_EMAIL` to `e2e+<epoch>@test.com`. Used by `register.yaml` to make the happy-path registration idempotent across multiple test runs.

### testID additions (mobile/app/(tabs)/(profile)/index.tsx)

- `testID="screen-profile"` on the root View — allows the logout flow to assert the profile screen is visible after navigation.
- `testID="btn-sign-out"` on the Sign Out Pressable — allows the logout flow to trigger sign-out without relying on text content.

---

## Design Decisions

- **Tab navigation via text label**: `tapOn: "Profile"` uses the tab label text. `tabBarTestID` is not a valid Expo Router prop (confirmed in 9.0 handoff), so text-based taps are the established pattern.
- **Validation test proxy assertion**: Zod/RHF field-level errors are plain `<Text>` nodes without testIDs. Rather than adding per-field error testIDs (out of scope for this task), we assert `screen-register` remains visible — confirming no navigation occurred and therefore validation fired.
- **Unique email for register happy path**: Using `runScript` with `Date.now()` makes the test idempotent. A fixed email would pass on run 1 and fail on run 2 with a 409.

---

## Verification

```bash
# TypeScript clean
cd mobile && npx tsc --noEmit       # → 0 errors

# All 91 existing Jest tests still pass
cd mobile && npx jest               # → 91 passed

# Auth flows (requires simulator + backend + seed data)
cd mobile && maestro test e2e/flows/auth/
make test-mobile-e2e-auth
```

## Known Limitations

- **Flows require live environment**: All auth flows need the iOS Simulator running with an installed dev-client build, the backend running (`cd backend && make dev`), and seeded data (Phase 8.0). They cannot run headlessly without infrastructure.
- **Validation flow granularity**: Field-level error text (e.g., "Name is required") is not assertable by testID. If per-field assertion becomes important in a future task, add `testID="error-name"`, `testID="error-email"`, etc. to the Input component's error Text node.

## Next Tasks (can run in parallel after 9.1)

- **9.2** — Discovery flows (feed, search, map)
- **9.5** depends on 9.1 (needs auth helpers) + 9.3 (listing detail)
- **9.7** depends on 9.1 (messaging)
