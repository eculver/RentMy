# Handoff: Task 8.2 — Fix: Auth Flow Bugs

**Status:** Completed  
**Commit:** a0a5626  
**Branch:** task-8.2-fix-auth-bugs  
**Branching mode:** Graphite

---

## What Was Done

Fixed all 7 bugs documented in the task 8.1 audit (`thoughts/audits/phase-8-visual-qa/audit-auth.md`).

## Files Changed

| File | Change |
|------|--------|
| `mobile/app/_layout.tsx` | BUG-001: add `<Redirect href="/(auth)/login" />` when `!isAuthenticated`; BUG-006: replace `return null` with `<ActivityIndicator>` during loading |
| `mobile/app/(auth)/register.tsx` | BUG-007: `router.back()` → `router.replace("/(auth)/login")` |
| `mobile/components/ui/Input.tsx` | BUG-005: add `isFocused` state, toggle `border-primary-500` on focus |
| `mobile/app/(tabs)/_layout.tsx` | BUG-002/003: `headerShown: false`; `useSafeAreaInsets` for tab bar `paddingBottom` |
| `backend/internal/user/model.go` | BUG-004: add `containsany=ABCDEFGHIJKLMNOPQRSTUVWXYZ,containsany=0123456789` to `RegisterInput.Password` |
| `backend/internal/user/service_test.go` | Add `TestRegister_ValidationRejectsPasswordWithoutUppercase` and `TestRegister_ValidationRejectsPasswordWithoutNumber` |
| `backend/tests/integration/user_api_test.go` | Update `"supersecret1"` → `"Supersecret1"` to meet new strength rules |

## Bug Resolution Summary

| ID | Priority | Status | Notes |
|----|----------|--------|-------|
| BUG-001 | P0 | Fixed | `<Redirect>` rendered conditionally in root layout |
| BUG-002 | P2 | Fixed | `useSafeAreaInsets` paddingBottom on tab bar |
| BUG-003 | P2 | Fixed | `headerShown: false` in tabs layout |
| BUG-004 | P2 | Fixed | Backend validator parity with mobile Zod schema |
| BUG-005 | P3 | Fixed | Input focus border state via `isFocused` |
| BUG-006 | P3 | Fixed | `ActivityIndicator` replaces blank loading screen |
| BUG-007 | P3 | Fixed | Deep-link safe navigation on register screen |

## Verification

- `cd mobile && npx jest` → 91/91 tests pass
- `cd backend && go vet ./...` → clean
- `cd backend && go build -o /dev/null ./cmd/server` → clean
- `cd backend && go test ./... -count=1` → 27/27 packages pass
- `cd mobile && npx tsc --noEmit` → 2 pre-existing errors in `(profile)/index.tsx` (tracked under tasks 8.13/8.14), no new errors

## Notes for Task 8.3

The pre-existing TypeScript errors in `mobile/app/(tabs)/(profile)/index.tsx` (route type mismatches) are tracked for fixing in tasks 8.13/8.14 (Profile audit/fix). Do not address them in task 8.3.
