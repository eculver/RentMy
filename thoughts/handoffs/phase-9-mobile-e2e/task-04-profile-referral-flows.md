# Task 9.4 — E2E: Profile & Referral Flows

## Summary

All 3 profile E2E flows pass against the real app: view profile, sign out, and referrals.

## What Changed

### App Code — Missing testIDs added

**`mobile/app/(tabs)/(profile)/index.tsx`:**
- `profile-name` on user name Text
- `profile-email` on user email Text
- `btn-invite-friends` on the "Invite Friends — Earn $20" Pressable
- `profile-listings-empty` on the empty state Text

**`mobile/app/(tabs)/(profile)/referrals.tsx`:**
- `screen-referrals` on the root View
- `referral-code` on the referral code Text
- `btn-share-referral` on the Share Pressable

**`mobile/app/(tabs)/(profile)/_layout.tsx`:**
- Added `referrals` route to the Stack navigator (was missing — Expo Router's automatic routing worked but without an explicit Stack.Screen entry, the screen rendered without a proper header)

### Flow YAML Fixes

All 3 flows in `mobile/e2e/flows/profile/` were updated to follow Maestro 2.4.0 patterns established in tasks 9.1–9.3:

1. **Tab navigation:** Changed `tapOn: "Profile"` → `tapOn: text: "Profile, tab.*"` (iOS accessibility text pattern)
2. **Assertions with timeouts:** Changed `assertVisible: { id, timeout }` → `extendedWaitUntil: { visible: { id }, timeout }` (the `assertVisible` + `timeout` combo is invalid in Maestro 2.4.0)
3. **Negative assertions:** Changed `assertNotVisible: { id }` → `extendedWaitUntil: { notVisible: { id }, timeout }` (bare `assertNotVisible` is invalid in 2.4.0)

## Bugs Found and Fixed

**No app bugs found.** The profile and referrals screens worked correctly once testIDs were added. The referral code API (GET → 404 fallback to POST auto-generate) works as designed. The sign-out flow correctly clears auth state and redirects to login.

The only issues were:
1. Missing testIDs (added above)
2. Invalid Maestro 2.4.0 syntax in the pre-existing flow templates (fixed above)
3. Missing `referrals` route in profile Stack layout (added above)

## Flakiness Note

One suite run (1 of 3) saw a transient `kAXErrorInvalidUIElement` failure from the iOS XCTest framework during the Expo Dev Client screen transition. This is an iOS Simulator issue, not an app bug — the view hierarchy becomes momentarily invalid during rapid screen transitions. The subsequent 2 runs passed cleanly. This is a known Maestro/iOS limitation and doesn't warrant an app-side fix.

## Test Output

### Individual flow runs (all passed):
```
> Flow Profile - View profile screen (happy path)
Run ../../helpers/login-as-renter.yaml... COMPLETED
Tap on "Profile, tab.*"... COMPLETED
Assert that id: screen-profile is visible... COMPLETED
Assert that id: profile-name is visible... COMPLETED
Assert that id: profile-email is visible... COMPLETED
Assert that id: btn-create-listing-nav is visible... COMPLETED
Assert that id: btn-invite-friends is visible... COMPLETED
Assert that id: profile-listings-empty is visible... COMPLETED
Assert that id: btn-sign-out is visible... COMPLETED
```

```
> Flow Profile - Sign out
Run ../../helpers/login-as-renter.yaml... COMPLETED
Tap on "Profile, tab.*"... COMPLETED
Assert that id: screen-profile is visible... COMPLETED
Tap on id: btn-sign-out... COMPLETED
Assert that id: screen-login is visible... COMPLETED
Assert that id: screen-feed is not visible... COMPLETED
Assert that id: screen-profile is not visible... COMPLETED
```

```
> Flow Profile - Referrals screen (happy path)
Run ../../helpers/login-as-renter.yaml... COMPLETED
Tap on "Profile, tab.*"... COMPLETED
Assert that id: screen-profile is visible... COMPLETED
Tap on id: btn-invite-friends... COMPLETED
Assert that id: screen-referrals is visible... COMPLETED
Assert that id: referral-code is visible... COMPLETED
Assert that id: btn-share-referral is visible... COMPLETED
```

### Suite run (3/3 passed):
```
Waiting for flows to complete...
[Passed] Profile - Sign out (36s)
[Passed] Profile - Referrals screen (happy path) (34s)
[Passed] Profile - View profile screen (happy path) (31s)

3/3 Flows Passed in 1m 41s
```

### Regression suites (all passed):
- Auth: 6/6 Flows Passed
- Discovery: 3/3 Flows Passed
- Listing: 2/2 Flows Passed

## No Overrides

This task required zero app-side overrides. All profile and referral functionality works with the real app and real backend. No `__DEV__` bypasses, no test-only endpoints, no environment switches.

## Graphite Mode

Branch created with `gt create task-9.4-profile-referral-flows`.
