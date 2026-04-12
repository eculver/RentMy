# Task 8.13 Handoff — Audit: Profile + Referrals + KYC

**Status:** completed
**Branch:** task-8.13-audit-profile-referrals-kyc
**Date:** 2026-04-11

---

## What Was Done

Static code audit of the profile area screens and their underlying components. No iOS Simulator was available; bugs were identified through code reading.

**Screens audited:**
- `mobile/app/(tabs)/(profile)/index.tsx`
- `mobile/app/(tabs)/(profile)/referrals.tsx`
- `mobile/app/(tabs)/(profile)/verify.tsx` (delegates to VerifyScreen.native.tsx)
- `mobile/app/(tabs)/(profile)/create-listing.tsx` (delegates to CreateListingScreen.native.tsx)

**Components audited:**
- `mobile/components/screens/VerifyScreen.native.tsx`
- `mobile/components/screens/CreateListingScreen.native.tsx`
- `mobile/components/referral/ReferralCard.tsx`
- `mobile/components/listing/ListingForm.tsx`
- `mobile/components/listing/AIAutofillOverlay.tsx`
- `mobile/components/listing/ValueOverridePrompt.tsx`

**Deliverable:** `thoughts/audits/phase-8-visual-qa/audit-profile.md`

---

## Bug Summary

| # | Severity | Description | File |
|---|----------|-------------|------|
| BUG-1 | CRITICAL | `router.back()` called during render (React violation) | `CreateListingScreen.native.tsx:209-211` |
| BUG-2 | CRITICAL | `ValueOverridePrompt` permanently unreachable — override flow broken | `CreateListingScreen.native.tsx` |
| BUG-3 | HIGH | No "Verify Identity" entry point in profile screen | `(profile)/index.tsx` |
| BUG-4 | HIGH | `Clipboard` imported from deprecated `react-native` package; crashes on copy | `(profile)/referrals.tsx:10` |
| BUG-5 | MEDIUM | NativeWind class-splitting antipattern may cause missing styles in prod | `ReferralCard.tsx:44-45` |
| BUG-6 | LOW | `ShimmerBar` `width` prop has unsafe type assertion | `AIAutofillOverlay.tsx:34` |

---

## Key Findings

### CreateListingScreen flow is fundamentally broken

The two critical bugs interact: BUG-1 (render-phase navigation) combined with BUG-2 (broken override check timing) mean that:
1. The `ValueOverridePrompt` (2x price override safety gate) **never appears**
2. Navigation in the render phase can cause React warnings / double-navigation

The root cause: `handlePricePerDayChange` checks override threshold against `resolvedSuggestions?.estimatedValueCents`, which is only populated from `appraisalComplete`. But when `appraisalComplete` first becomes true, `overrideVisible` is still false → `router.back()` fires before the override check can ever run.

**Fix approach for 8.14:**
- Move `router.back()` into a `useEffect` guarded by `appraisalComplete && !overrideVisible`
- Move override threshold check into a `useEffect` that runs when `appraisalComplete` becomes true, comparing `pendingPriceDollars` (captured from form submit) against the AI estimate

### KYC screen is completely unreachable from the UI

The verify screen is a full, working implementation but zero navigation paths lead there from within the app. The profile screen must gain a "Verify Identity" CTA conditioned on `identityStatus !== "VERIFIED"`.

### Clipboard crash

`Clipboard.setString` from `react-native` is `undefined` in Expo SDK 54 (RN 0.76). The Copy Code button crashes. Fix: `npx expo install expo-clipboard` and switch to `Clipboard.setStringAsync`.

---

## What 8.14 Should Fix

1. **BUG-1**: Move `router.back()` from render to `useEffect` in `CreateListingScreen`
2. **BUG-2**: Move override threshold check to `useEffect` triggered by `appraisalComplete`
3. **BUG-3**: Add "Verify Identity" Pressable to profile screen with `identityStatus` guard
4. **BUG-4**: Replace `Clipboard` from `react-native` with `expo-clipboard` in referrals screen
5. **BUG-5**: Split `ReferralCard.statusColor` into `statusBg` + `statusText` maps
6. **BUG-6**: Change `ShimmerBar` `width` prop type to `` `${number}%` `` (remove unsafe cast)

---

## Branching Mode

Graphite (`gt create`) — succeeded, Graphite mode used for this session.
