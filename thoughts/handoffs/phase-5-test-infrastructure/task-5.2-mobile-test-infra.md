# Task 5.2 — Mobile Test Infrastructure

## Summary

Set up Jest + React Native Testing Library (RNTL) + MSW test infrastructure for the mobile app.

## What Was Done

### New files
- `mobile/jest.config.js` — Jest config using `jest-expo` preset, with `transformIgnorePatterns` extended to include `msw`, `ky`, and other ESM-only packages
- `mobile/__tests__/setup.ts` — Global test setup: mocks `expo-secure-store` and `expo-location` (native modules that don't exist in Node), starts/resets/stops MSW server
- `mobile/__tests__/mocks/server.ts` — MSW `setupServer` with baseline handlers for `/auth/login`, `/auth/register`, `/auth/refresh`, and `/discovery/feed`
- `mobile/__tests__/components/smoke.test.tsx` — Smoke test verifying RNTL renders a React Native component correctly
- `mobile/__tests__/lib/api.test.ts` — Tests for `lib/api.ts`: Authorization header injection (authenticated vs unauthenticated), 401 handling (refresh call, logout on failed refresh)

### Dependencies added (`mobile/package.json`)
- `jest` (devDependency, pinned to 29)
- `jest-expo`
- `@testing-library/react-native`
- `@testing-library/jest-native`
- `msw` (Mock Service Worker, node adapter)

## Key Decisions

- **MSW over fetch mocks**: MSW intercepts at the network layer, so `ky`'s retry/hooks run realistically
- **`jest-expo` preset**: Handles React Native transforms without manual Babel config duplication
- **`transformIgnorePatterns`**: Extended to include `msw`, `@mswjs`, `ky` because they ship as ESM and need transpilation by Babel in Jest
- **Functional `setState` in Zustand tests**: `useAuthStore.setState(s => ({ ...s, ... }))` avoids TypeScript errors from partial state casts
- **Graphite mode**: Used `gt` for branch management (branch was pre-created by previous session)

## Verification

```
cd mobile && npx tsc --noEmit   # passes (0 errors)
cd mobile && npx jest           # 2 suites, 5 tests, all pass
```

## Next Task

5.3 — Phase 1-2 backend integration tests (depends on 5.1, which is complete)
