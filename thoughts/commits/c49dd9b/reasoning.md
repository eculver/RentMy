# Commit c49dd9b — feat: add Maestro E2E profile flows (task 9.4)

## Why this commit

Task 9.4 adds the three profile-area E2E flows: view-profile, referrals, and sign-out.

The profile and referrals screens were missing testIDs that Maestro needs to locate elements by ID rather than by display text (which is brittle and locale-sensitive). Adding testIDs to `profile-name`, `profile-email`, `btn-invite-friends`, `profile-listings-empty`, `screen-referrals`, `referral-code`, and `btn-share-referral` makes these flows resilient to UI text changes.

## Files changed

- `mobile/app/(tabs)/(profile)/index.tsx` — Added `testID` props to name/email texts, Invite Friends button, and empty-state text.
- `mobile/app/(tabs)/(profile)/referrals.tsx` — Added `testID` props to root view, referral code text, and Share button.
- `mobile/e2e/flows/profile/view-profile.yaml` — Asserts profile header (name, email), action buttons, and empty listings state for bob@test.com.
- `mobile/e2e/flows/profile/referrals.yaml` — Navigates from profile to referrals screen, asserts code and share button load.
- `mobile/e2e/flows/profile/sign-out.yaml` — Signs out from profile tab, asserts redirect to login and auth gate active.
