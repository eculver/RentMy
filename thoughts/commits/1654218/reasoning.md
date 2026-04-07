# Commit 1654218 — fix: add base module stubs for platform-split components

## Why

TypeScript's `moduleResolution: "bundler"` does not automatically resolve React Native platform extensions (`.native.tsx`, `.web.tsx`). When a file imports `../camera/AngleEnforcedCamera`, TS looks for `AngleEnforcedCamera.ts(x)` but not `.native.tsx` or `.web.tsx`. This caused 15 `TS2307` errors across the codebase.

## Approach

Created base `.tsx` files that re-export from the `.web.tsx` variant. This is the standard pattern for Expo projects with platform splits — the base file serves as the TypeScript resolution target, while Metro's platform-aware bundler still picks `.native.tsx` or `.web.tsx` at runtime depending on the target platform.

## Alternatives Considered

- **TypeScript path mapping**: Would require complex `paths` entries in tsconfig for each component. Not maintainable.
- **Declaration files (`.d.ts`)**: Would need manual type definitions that drift from the actual implementations.
- **Re-export from `.native.tsx`**: Would cause TypeScript to try resolving native-only imports (react-native-maps, vision-camera) on web, which would fail.

Re-exporting from `.web.tsx` is correct because the web stubs are simple placeholders that don't import native-only packages.
