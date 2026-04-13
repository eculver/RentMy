# Task 9.1 — E2E: Authentication Flows

## Status: COMPLETED

## Verification Output

### `maestro test mobile/e2e/flows/auth/`
```
Waiting for flows to complete...
[Passed] Auth - Logout (42s)
[Passed] Auth - Register (happy path) (37s)
[Passed] Auth - Login (happy path) (37s)
[Passed] Auth - Register (validation: empty fields) (23s)
[Passed] Auth - Register (error: duplicate email) (26s)
[Passed] Auth - Login (error: wrong password) (27s)

6/6 Flows Passed in 3m 12s
```

## Bugs Found and Fixed

### Bug 1: Missing testIDs on register screen, profile screen, and error messages
- **Files:** `mobile/app/(auth)/register.tsx`, `mobile/app/(auth)/login.tsx`, `mobile/app/(tabs)/(profile)/index.tsx`
- **Description:** The register screen had no testIDs on any elements. The login screen was missing `error-message` and `btn-create-account` testIDs. The profile screen was missing `screen-profile` and `btn-sign-out`.
- **Fix:** Added `testID` props: `screen-register`, `input-name`, `input-email`, `input-password`, `btn-register`, `error-message` on register; `error-message`, `btn-create-account` on login; `screen-profile`, `btn-sign-out` on profile.

### Bug 2: Tab bar showing 15+ tabs instead of 6
- **Files:** `mobile/app/(tabs)/(feed)/_layout.tsx`, `mobile/app/(tabs)/(map)/_layout.tsx`, `mobile/app/(tabs)/(messages)/_layout.tsx`, `mobile/app/(tabs)/(search)/_layout.tsx` (all new)
- **Description:** Tab groups `(feed)`, `(map)`, `(messages)`, `(search)` lacked `_layout.tsx` files with Stack navigators. Without a layout, Expo Router registered each individual screen file as a separate tab, resulting in 15+ truncated tabs in the tab bar.
- **Fix:** Added `_layout.tsx` with a `<Stack>` navigator to each group. Now the tab bar correctly shows 6 tabs: Feed, Search, Map, Rentals, Messages, Profile.

### Bug 3: iOS Strong Password AutoFill intercepting password input on register form
- **Files:** `mobile/e2e/flows/auth/register.yaml`, `mobile/e2e/flows/auth/register-duplicate-email.yaml`
- **Description:** The register form's password field uses `autoComplete="new-password"`, which triggers iOS's Strong Password AutoFill. Maestro's `inputText` reports COMPLETED but the text goes into iOS's autofill suggestion UI instead of the actual TextInput. The field appears empty after typing.
- **Fix:** Added `eraseText: 30` before `inputText` on the password field to clear any autofill content. Then dismissed the keyboard by tapping the subtitle text ("Join RentMy to start renting") instead of using `hideKeyboard` (which is unreliable on secureTextEntry fields).

### Bug 4: Maestro `runScript` output variables not resolving in Maestro 2.4.0
- **Files:** `mobile/e2e/flows/auth/register.yaml`, `mobile/e2e/scripts/gen-unique-email.js`
- **Description:** The `runScript` command with `output.VARIABLE` set in a .js file didn't propagate the variable correctly. `${UNIQUE_EMAIL}` resolved to "undefined" when used in `inputText`.
- **Fix:** Replaced `runScript` with `evalScript` for inline JavaScript: `evalScript: ${output.UNIQUE_EMAIL = 'e2e+' + Date.now() + '@test.com'}`. Referenced as `${output.UNIQUE_EMAIL}` in subsequent commands.

### Bug 5: Maestro `tapOn: "Profile"` can't find iOS tab bar items by text
- **Files:** `mobile/e2e/flows/auth/logout.yaml`
- **Description:** iOS tab bar items render their label as `accessibilityText` (e.g., "Profile, tab, 6 of 6") with no `text` attribute. Maestro's `tapOn: "Profile"` text selector couldn't match.
- **Fix:** Changed to `tapOn: text: "Profile, tab.*"` which matches the full accessibility text via regex.

### Bug 6: Auth flow YAML files missing Expo Dev Client handling and clearKeychain
- **Files:** All 5 non-login auth YAML files
- **Description:** Only `login.yaml` (from task 9.0) had the Expo Dev Client handler (tap dev server, dismiss "Continue", close dev tools panel) and `clearKeychain`. The other 5 flows would fail immediately on app launch.
- **Fix:** Added the full Expo Dev Client handling sequence and `clearKeychain` to all auth flows.

## Branching Mode
Graphite mode. Branch: `task-9.1-e2e-auth-flows`.

## Notes for Next Tasks
- Tab navigation in Maestro: use `tapOn: text: "TabName, tab.*"` to match iOS accessibility text for tab bar items.
- Register form password: always use `eraseText: 30` before `inputText` on the password field to clear iOS Strong Password autofill.
- The `evalScript` + `${output.VARIABLE}` pattern is the reliable way to generate dynamic data. Do NOT use `runScript` with .js file output variables — they don't propagate in Maestro 2.4.0.
- Keyboard dismissal: `hideKeyboard` is unreliable on forms with `secureTextEntry` + `autoComplete="new-password"`. Tap a non-interactive text element instead.
- The `gen-unique-email.js` script was simplified to a single-line version. It still works with `runScript` if needed, but `evalScript` inline is preferred.
