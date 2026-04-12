# Audit: Auth Flow (Login + Register)

Date: 2026-04-11

## Screenshots

- Bootstrap screenshot: `/tmp/rentmy-bootstrap-2.png` — App launched to (feed)/index instead of login screen
- Location error: `/tmp/rentmy-bootstrap-3.png` — "Location unavailable" on feed (blocking, but separate from auth)

## Bugs Found

- [BUG-001] P0 — Auth gate bypassed on fresh install: app navigates to (feed)/index instead of (auth)/login when no token exists. Root layout uses conditional `Stack.Screen` rendering which Expo Router does not enforce as a navigation guard. Both `(tabs)` and `(auth)` are file-system routes and the router resolves `(tabs)` as the default. Fix: use `Redirect` component from expo-router when `!isAuthenticated`. — `mobile/app/_layout.tsx:27-31`

- [BUG-002] P2 — Tab bar labels truncated/cut off at bottom of screen. Tab icons and partial labels visible but text is clipped. May be a safe area inset issue. — `mobile/app/(tabs)/_layout.tsx`

- [BUG-003] P2 — Route name "(feed)/index" shown as header title instead of "Feed" or app name. The root `Stack` has `headerShown: false` but the `(tabs)` layout has `headerShown: true`, showing raw route group names. — `mobile/app/(tabs)/_layout.tsx:21`

## Console Errors

- None observed during initial load (no red screen errors)

## Notes

- Login and register screens could not be visually tested because the auth gate is bypassed — the app never navigates to them
- After fixing BUG-001, we should re-screenshot the login/register screens to check their layout
- The login screen code (`login.tsx`) looks structurally sound — Zod validation, proper error handling, form controllers
- The register screen includes referral code pre-fill from deep link params — good
