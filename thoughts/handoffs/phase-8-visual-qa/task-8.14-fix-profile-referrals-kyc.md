# Task 8.14 Handoff — Fix: Profile + Referrals + KYC Bugs

**Status:** completed
**Branch:** task-8.14-fix-profile-referrals-kyc
**Commit:** 8924686
**Date:** 2026-04-11

---

## What Was Done

Fixed all 6 bugs identified in the Task 8.13 static code audit of the profile area.

---

## Bugs Fixed

| # | Severity | File | Fix |
|---|----------|------|-----|
| BUG-1 | CRITICAL | `CreateListingScreen.native.tsx` | Moved `router.back()` from render body into `useEffect` |
| BUG-2 | CRITICAL | `CreateListingScreen.native.tsx` | Moved override threshold check into `useEffect` triggered by `appraisalComplete` |
| BUG-3 | HIGH | `(profile)/index.tsx` | Added "Verify Identity" Pressable guarded by `identityStatus !== "VERIFIED"` |
| BUG-4 | HIGH | `(profile)/referrals.tsx` | Replaced `react-native` `Clipboard` with `expo-clipboard` |
| BUG-5 | MEDIUM | `ReferralCard.tsx` | Split `statusColor` into `statusBg` + `statusText` maps |
| BUG-6 | LOW | `AIAutofillOverlay.tsx` | Changed `ShimmerBar` `width` prop type to `` `${number}%` ``, removed unsafe cast |

---

## BUG-1 + BUG-2 Detail

The two critical bugs in `CreateListingScreen` are fixed with a single `useEffect`. A `overrideChecked` ref ensures the check runs exactly once when `appraisalComplete` first becomes true:

```
appraisalComplete becomes true
  → !overrideChecked.current → set to true
  → if price > 2x AI estimate: setOverrideVisible(true), return
  → else: router.back()

overrideVisible later becomes false (user dismissed prompt)
  → overrideChecked.current is true → else if (!overrideVisible): router.back()
```

`handlePricePerDayChange` is simplified to only track `pendingPriceDollars` — the threshold check is no longer needed there since it can never fire with a non-null `estimatedValueCents` during form-fill time.

---

## BUG-4: Package installed

```bash
npx expo install expo-clipboard
```

`expo-clipboard` was not previously in `package.json`. Now installed and `Clipboard.setStringAsync` is used (async, non-crashing).

---

## Verification

- `npx tsc --noEmit` — no TypeScript errors
- `npx jest` — 91/91 tests pass (11 test suites)
- `go vet ./...` — clean
- `go build -o /dev/null ./cmd/server` — clean

---

## Branching Mode

Graphite (`gt create`) — Graphite mode used for this session.
