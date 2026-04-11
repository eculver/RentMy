# Commit 6757a98 — Reasoning

## Why this commit

Task 1.4 wires the mobile auth UI to the real UserService API (implemented in Task 1.1). The dev-only one-tap login and "Coming Soon" register placeholder are replaced with production-ready forms.

## Key design choices

**Refresh token storage:** The original auth store only held an access token. We need the refresh token to survive app restarts (user should stay logged in for 7 days). SecureStore was the natural extension — same mechanism, new key.

**No circular import:** auth.ts making API calls needs its own ky instance, not the `api` singleton. The `api` singleton imports from auth.ts (to read the token and call logout). If auth.ts imported from api.ts, we'd have a cycle. The bare `authApi = ky.create(...)` in auth.ts breaks the cycle cleanly.

**Refresh-before-logout on 401:** The access token TTL is 15 minutes. Without silent refresh, users on slow connections or long sessions would get logged out constantly. The afterResponse hook in api.ts now tries refreshTokens() first; only if that fails does it call logout().

**Zod for form validation:** Zod is already a dependency (v4.3.6) and @hookform/resolvers is installed. Using them here avoids hand-rolled validation and gives typed form data via `z.infer<typeof schema>`.

**Password rules on register:** The backend requires `Test1234!` style passwords (uppercase + digit). The Zod schema enforces this at the UI layer so users see clear error messages before the API call rather than a generic 400.
