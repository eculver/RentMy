# Commit d797d31 — Auth E2E Flows

## What changed
All 6 auth E2E Maestro flows now pass: login, register, logout, wrong-password, register-validation, register-duplicate-email.

## Why
Task 9.1 requires all auth E2E flows to pass against the real app. The flows existed from task 9.0 scaffolding but only login.yaml had been tested and fixed.

## Key decisions

1. **Tab group layouts:** Added `_layout.tsx` Stack navigators to 4 tab groups that were missing them. Without these, Expo Router registered every file as a separate tab (15+ tabs). This was a real app bug, not a test-only issue.

2. **iOS Strong Password autofill workaround:** The register form's password field triggers iOS's Strong Password suggestion because of `autoComplete="new-password"`. Maestro's `inputText` goes into the autofill UI instead of the TextInput. Fixed by adding `eraseText: 30` before `inputText` to clear autofill content. This is a test-side workaround, not an app-side change.

3. **evalScript over runScript:** Maestro 2.4.0's `runScript` doesn't properly propagate `output.VARIABLE` to subsequent steps. Switched to `evalScript` with inline JavaScript and `${output.VARIABLE}` references.

4. **Tab navigation via accessibility text:** iOS tab bar items expose labels as `accessibilityText` ("Profile, tab, 6 of 6"), not as `text`. Changed to regex matching: `tapOn: text: "Profile, tab.*"`.
