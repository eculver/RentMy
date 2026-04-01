# Task 07: React Native Scaffold — Handoff

## What Was Done

Replaced the `create-expo-app --template tabs` boilerplate with a clean, feature-ready RentMy scaffold.

## Files Deleted (template boilerplate)

- `app/(tabs)/index.tsx`
- `app/(tabs)/two.tsx`
- `app/+html.tsx`
- `app/modal.tsx`
- `components/__tests__/StyledText-test.js`
- `components/EditScreenInfo.tsx`
- `components/ExternalLink.tsx`
- `components/StyledText.tsx`
- `components/Themed.tsx`
- `components/useClientOnlyValue.ts`
- `components/useClientOnlyValue.web.ts`
- `components/useColorScheme.ts`
- `components/useColorScheme.web.ts`
- `constants/Colors.ts` (entire `constants/` directory removed)

## Files Created

### NativeWind / Build Config

- `tailwind.config.js` — Tailwind config with NativeWind preset and `primary` color scale (sky blue)
- `global.css` — Tailwind directives entry point
- `nativewind-env.d.ts` — TypeScript type reference for NativeWind
- `metro.config.js` — Metro bundler wired to NativeWind CSS input
- `babel.config.js` — Babel preset with `jsxImportSource: "nativewind"` and `nativewind/babel`

### lib/

- `lib/auth.ts` — Zustand store for auth state; persists token + user to `expo-secure-store`
- `lib/api.ts` — `ky` client with `EXPO_PUBLIC_API_URL` prefix, auto-attaches Bearer token, auto-calls `logout()` on 401
- `lib/query.ts` — TanStack Query `QueryClient` with 60s stale time, 2 query retries, 0 mutation retries

### app/ (routing)

- `app/_layout.tsx` — Root layout: imports `global.css`, wraps in `QueryClientProvider`, gates routing on `isAuthenticated`
- `app/(auth)/_layout.tsx` — Auth stack (login, register)
- `app/(auth)/login.tsx` — Dev-mode sign-in (one-tap, hardcoded token for development)
- `app/(auth)/register.tsx` — Register placeholder screen
- `app/(tabs)/_layout.tsx` — Five-tab bar using Ionicons: Feed, Search, Map, Messages, Profile
- `app/(tabs)/(feed)/index.tsx`
- `app/(tabs)/(search)/index.tsx`
- `app/(tabs)/(map)/index.tsx`
- `app/(tabs)/(messages)/index.tsx`
- `app/(tabs)/(profile)/index.tsx` — Shows user name/email; Sign Out button calls `useAuthStore.logout()`
- `app/+not-found.tsx` — Fixed: removed `@/components/Themed` import, now uses RN core `Text`/`View`

### components/ui/

- `Button.tsx` — Primary / secondary / ghost variants with loading spinner
- `Input.tsx` — Labeled text input with error state
- `Card.tsx` — White rounded card with subtle border/shadow
- `Avatar.tsx` — Shows image URI or initials fallback; sm/md/lg sizes
- `Badge.tsx` — Colored pill badge: success / warning / error / info

## Verification

`npx tsc --noEmit` passes with zero errors.

## Notes / Next Steps

- The dev login button in `login.tsx` hardcodes `"dev-token"` — replace with real API call when auth endpoint is ready.
- `lib/api.ts` reads `EXPO_PUBLIC_API_URL` from env; set this in `.env.local` for local dev (e.g. `http://localhost:8080`).
- Each tab group is a nested route group `(feed)`, `(search)`, etc. — ready to add nested screens without touching `_layout.tsx`.
- `components/__tests__/` directory is empty — add component tests here as UI primitives are fleshed out.
