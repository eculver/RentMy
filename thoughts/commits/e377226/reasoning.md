# Commit e377226 — Task 9.4: E2E Profile & Referral Flows

## Why

Task 9.4 required getting all profile-related E2E flows passing. The 3 flows (view-profile, sign-out, referrals) already existed as YAML templates but referenced testIDs that didn't exist in the app code and used invalid Maestro 2.4.0 syntax patterns.

## What

1. **Added 7 testIDs** across profile screen (4) and referrals screen (3) so Maestro can find and assert on these elements.

2. **Registered `referrals` route** in the profile Stack layout. Expo Router's automatic file-based routing handled it implicitly, but without an explicit `Stack.Screen` entry the navigation header was missing.

3. **Fixed Maestro YAML syntax** in all 3 flows to match patterns established in tasks 9.1–9.3:
   - `assertVisible: { id, timeout }` → `extendedWaitUntil: { visible: { id }, timeout }`
   - `assertNotVisible: { id }` → `extendedWaitUntil: { notVisible: { id }, timeout }`
   - `tapOn: "Profile"` → `tapOn: text: "Profile, tab.*"` (iOS accessibility text)

## Decisions

- No app bugs found — the profile/referrals screens worked correctly once testIDs were added
- No overrides needed — all flows exercise the real app against the real backend
- The referral code auto-generation (GET 404 → POST fallback) works as designed
