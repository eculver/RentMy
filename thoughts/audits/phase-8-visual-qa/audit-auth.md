# Audit: Auth Flow (Login + Register)

Date: 2026-04-11
Auditor: Claude (session 2 — code-level analysis)

---

## Screenshots

- Bootstrap screenshot: `/tmp/rentmy-bootstrap-2.png` — App launched to (feed)/index instead of login screen
- Location error: `/tmp/rentmy-bootstrap-3.png` — "Location unavailable" on feed (blocking, but separate from auth)
- Note: Login/register screens could not be visually tested because BUG-001 bypasses the auth gate. Fix BUG-001 first, then re-screenshot.

---

## Files Audited

- `mobile/app/(auth)/login.tsx`
- `mobile/app/(auth)/register.tsx`
- `mobile/app/(auth)/_layout.tsx`
- `mobile/app/_layout.tsx`
- `mobile/components/ui/Input.tsx`
- `mobile/components/ui/Button.tsx`
- `mobile/lib/auth.ts`
- `backend/internal/user/model.go` (RegisterInput validation)
- `mobile/__tests__/screens/auth.test.tsx`

---

## Test Results

All 15 auth screen tests pass:
- `npx jest __tests__/screens/auth.test.tsx` → 15/15 PASS

TypeScript errors found in other screens (not auth):
- `app/(tabs)/(profile)/index.tsx` — 2 TS errors (route type mismatches — tracked under task 8.13/8.14)

---

## Bugs Found

### BUG-001 — P0: Auth gate bypassed on fresh install
**Location:** `mobile/app/_layout.tsx:27-31`
**Description:** When no auth token exists, the app navigates to `(tabs)` (the feed) instead of `(auth)/login`. The root layout renders `<Stack.Screen name="(tabs)">` or `<Stack.Screen name="(auth)">` conditionally but this does not redirect the router — Expo Router resolves `(tabs)` as the default route regardless.
**Fix:** Replace conditional `Stack.Screen` with explicit `<Redirect href="/(auth)/login" />` when `!isAuthenticated`. See task 8.2.

### BUG-002 — P2: Tab bar labels truncated at bottom of screen
**Location:** `mobile/app/(tabs)/_layout.tsx`
**Description:** Tab icons and partial labels visible but text is clipped. Likely a safe area inset issue — the bottom tab bar does not account for iOS home indicator safe area.
**Fix:** Ensure tab bar style includes `paddingBottom` or uses `useSafeAreaInsets()`. See task 8.2.

### BUG-003 — P2: Raw route group names shown as header title
**Location:** `mobile/app/(tabs)/_layout.tsx:21`
**Description:** Header shows raw route names like "(feed)/index" instead of "Feed". The root Stack has `headerShown: false` but the tabs layout may be showing a nested Stack header.
**Fix:** Set `headerShown: false` consistently in the tabs layout or set explicit titles. See task 8.2.

### BUG-004 — P2: Password strength enforced on frontend but not backend
**Location:** `mobile/app/(auth)/register.tsx:15-19`, `backend/internal/user/model.go:35`
**Description:** The register Zod schema requires uppercase letter + number (beyond `min=8`). The backend `RegisterInput` struct only validates `min=8`. Users can call the API directly and register with passwords that would fail the mobile form.
**Fix:** Add `min=8` + regex validators to backend `RegisterInput.Password` field. Or document that client-side validation is intentionally stronger. See task 8.2.

### BUG-005 — P3: No focused border state on Input component
**Location:** `mobile/components/ui/Input.tsx:13`
**Description:** The `TextInput` always shows `border-gray-300` (or `border-red-500` on error) — there is no focused ring/border change. Standard mobile UX expectation is a highlighted border when a field is active.
**Fix:** Use `isFocused` state with `onFocus`/`onBlur` to toggle a `border-primary-500` class. See task 8.2.

### BUG-006 — P3: Blank flash while loading auth state
**Location:** `mobile/app/_layout.tsx:19-21`
**Description:** `if (isLoading) return null` shows a blank white screen while the auth token is being loaded from SecureStore on app start. A loading spinner or the native splash screen extension would prevent this flash.
**Fix:** Return a loading indicator (`<ActivityIndicator>` centered) instead of `null`. See task 8.2.

### BUG-007 — P3: `router.back()` on Register screen is fragile
**Location:** `mobile/app/(auth)/register.tsx:151`
**Description:** The "Already have an account? Sign in" button uses `router.back()`. If a user deep-links directly to `/register` (e.g., from a referral link), there is no login screen in the navigation history — `router.back()` will pop to an unexpected previous screen or do nothing.
**Fix:** Replace `router.back()` with `router.replace('/(auth)/login')` to always navigate to the login screen. See task 8.2.

---

## Items Confirmed Working

- Zod validation schemas (email, password) are correctly configured on both screens
- Backend returns correct HTTP status codes: 401 on bad login, 409 on duplicate email
- Frontend handles both error codes with user-friendly messages
- JWT tokens stored in SecureStore (secure, not AsyncStorage)
- Refresh token rotation implemented in `auth.ts`
- Referral code pre-fill from deep link `?ref=CODE` param works in register.tsx
- `autoCapitalize`, `autoComplete`, `keyboardType` props all set correctly on inputs
- `KeyboardAvoidingView` + `ScrollView` pattern prevents keyboard from covering form
- All 15 unit tests pass

---

## Priority Order for Task 8.2

1. BUG-001 (P0) — auth gate
2. BUG-004 (P2) — password validation backend
3. BUG-007 (P3) — register back navigation
4. BUG-005 (P3) — input focus state
5. BUG-006 (P3) — loading flash
6. BUG-002, BUG-003 — tab bar issues (may be shared with other audit areas)
