# Task 9.4 — E2E: Profile & Referral Flows

**Status:** Completed  
**Branch:** task-9.4-profile-e2e  
**Commit:** c49dd9b

---

## What Was Done

### New Maestro flows (`mobile/e2e/flows/profile/`)

| File | Scenario |
|------|----------|
| `view-profile.yaml` | Login as renter → Profile tab → assert name/email/buttons visible → assert empty listings state (bob@test.com has no listings) |
| `referrals.yaml` | Login as renter → Profile → tap Invite Friends → Referrals screen loads → referral code visible → Share button visible |
| `sign-out.yaml` | Login as renter → Profile → tap Sign Out → redirected to login screen → auth gate active (feed/profile not visible) |

### testID additions

| Component | testIDs added |
|-----------|--------------|
| `mobile/app/(tabs)/(profile)/index.tsx` | `profile-name`, `profile-email`, `btn-invite-friends`, `profile-listings-empty` |
| `mobile/app/(tabs)/(profile)/referrals.tsx` | `screen-referrals`, `referral-code`, `btn-share-referral` |

---

## Design Decisions

### bob@test.com empty state
The view-profile flow logs in as the seeded renter `bob@test.com` who has no listings. This gives a deterministic empty state assertion (`profile-listings-empty`) without depending on the ordering of a FlatList. The host account (`alice@test.com`) would show listings, but the FlatList rendering makes assertion more complex; the empty state test is sufficient to verify the My Listings section renders correctly.

### sign-out.yaml vs auth/logout.yaml
The existing `auth/logout.yaml` flow (from task 9.1) covers the same sign-out action. The new `profile/sign-out.yaml` intentionally navigates via the Profile tab before signing out — this exercises the profile-specific path (navigating to the Profile tab, confirming the screen loads, then signing out). The auth flow tests sign-out as a direct action from wherever the user lands after login. Both are valuable.

### Referral code timeout
The `referral-code` element has a `timeout: 8000` because it requires a backend API call (`GET /api/v1/users/me/referrals`) to resolve. The code card shows an `ActivityIndicator` while loading. Maestro's implicit wait handles this naturally with the timeout.

---

## Verification

```bash
# TypeScript clean (0 errors)
cd mobile && ./node_modules/.bin/tsc --noEmit

# All 91 Jest tests pass
cd mobile && ./node_modules/.bin/jest

# Profile flows (requires simulator + backend + seed data)
cd mobile && maestro test e2e/flows/profile/
make test-mobile-e2e-profile
```

## Branching Mode
Graphite mode — used `gt create` and will use `gt submit`.

## Next Tasks

- **9.5** — Booking request & status flows (depends on 9.1 + 9.3; both complete)
- **9.7** — Messaging flows (depends on 9.1; complete)
