# Commit 2b7f0a9 — feat: add Maestro E2E auth flows (task 9.1)

## Why

Task 9.1 requires full Maestro E2E coverage for the three authentication user
journeys: register, login, and logout. The login happy-path was already present
from task 9.0; this commit adds the remaining five scenarios and the supporting
infrastructure.

## What changed

### mobile/app/(tabs)/(profile)/index.tsx
Added `testID="screen-profile"` to the root View and `testID="btn-sign-out"` to
the Sign Out Pressable. These IDs are required by the logout flow to navigate to
the profile tab and trigger sign-out.

### mobile/e2e/flows/auth/register.yaml
Happy-path registration. Uses `runScript` to generate a timestamp-based unique
email (`e2e+<epoch>@test.com`) so the test is idempotent across runs — a fixed
email would fail on the second run with a 409.

### mobile/e2e/flows/auth/register-duplicate-email.yaml
Error path: submits with a seeded account email (bob@test.com). Backend returns
409 → register.tsx sets `apiError` → `error-message` testID appears.

### mobile/e2e/flows/auth/register-validation.yaml
Client-side validation: submits empty form, then submits with an invalid email
format. In both cases Zod prevents submission and the form stays on screen.
Inline field errors rendered by the Input component are not assertable by
testID (they're plain Text nodes), so we assert `screen-register` remains
visible as the proxy for "no navigation occurred".

### mobile/e2e/flows/auth/login-wrong-password.yaml
Error path: correct email, wrong password. Backend returns 401 → login.tsx sets
`apiError` → `error-message` testID appears.

### mobile/e2e/flows/auth/logout.yaml
Signs in as renter, navigates to Profile tab via `tapOn: "Profile"` (text-based
tap, same pattern as navigate-to-tab.yaml), taps `btn-sign-out`, asserts
`screen-login` appears and `screen-feed`/`screen-profile` are gone.

### mobile/e2e/scripts/gen-unique-email.js
Minimal Maestro runScript JavaScript. Sets `output.UNIQUE_EMAIL` to a
timestamp-suffixed address. Maestro exposes this as `${UNIQUE_EMAIL}` in the
calling flow.

## Decisions

- **Tab navigation via text**: `tapOn: "Profile"` (label text) rather than a
  testID, consistent with the pattern established in task 9.0 and the 9.0
  handoff note that `tabBarTestID` is not supported by Expo Router's Tabs.Screen.
- **Validation test assertions**: Zod/RHF inline field errors are plain `<Text>`
  nodes without testIDs. Rather than adding testIDs for each field's error text
  (scope creep), we assert `screen-register` remains visible — sufficient to
  confirm no navigation occurred and therefore validation fired correctly.
- **runScript path**: relative to the `mobile/` directory (where maestro is
  invoked from the Makefile), so `e2e/scripts/gen-unique-email.js`.
