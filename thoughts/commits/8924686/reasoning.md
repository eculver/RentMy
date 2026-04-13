# Commit 8924686 — Reasoning

**Task:** 8.14 — Fix: Profile + Referrals + KYC Bugs

## Why this commit exists

Six bugs were identified in the 8.13 static code audit of the profile area. This commit fixes all six, restoring correct behavior to create-listing, the profile screen, and referrals.

## Bug-by-bug decisions

### BUG-1 + BUG-2: CreateListingScreen — render-phase navigation + unreachable ValueOverridePrompt

The original code called `router.back()` directly in the render body when `appraisalComplete && !overrideVisible`. React prohibits side effects (including navigation) during render. The fix moves this into a `useEffect`.

The override threshold check in `handlePricePerDayChange` could never fire because `resolvedSuggestions?.estimatedValueCents` is `null` at form-fill time (before appraisal runs), and the user cannot change the price while the appraisal overlay is showing. After appraisal completes, `handlePricePerDayChange` is never called again.

Fix: a single `useEffect` handles both concerns. A `overrideChecked` ref guards the check so it runs exactly once when `appraisalComplete` first becomes true — either showing the override prompt or navigating back. When the override is dismissed (setting `overrideVisible = false`), the effect re-runs and navigates back.

### BUG-3: Profile screen — no Verify Identity entry point

`VerifyScreen.native.tsx` is a complete implementation with no navigation path from the UI. Added an amber-bordered "Verify Identity" Pressable below the referrals button, conditioned on `user?.identityStatus !== "VERIFIED"`. When `identityStatus` is absent (legacy users), the button shows — safer than hiding it.

### BUG-4: Clipboard crash — deprecated react-native API

`Clipboard` from `react-native` is `undefined` in Expo SDK 54 (RN 0.76). Replaced with `expo-clipboard` (installed via `npx expo install expo-clipboard`), using the async `Clipboard.setStringAsync` API.

### BUG-5: ReferralCard NativeWind class-splitting

NativeWind's JIT compiler scans source for complete class strings. Splitting `"bg-blue-100 text-blue-700"` at runtime with `.split(" ")` means neither class appears as a complete string in source — both may be pruned from the production CSS bundle. Replaced with two separate maps (`statusBg`, `statusText`) keyed on `Referral["status"]`.

### BUG-6: ShimmerBar unsafe type assertion

`width as \`${number}%\`` suppresses a legitimate type error instead of fixing it. Narrowing the prop type to `` `${number}%` `` lets TypeScript verify all call sites (`"85%"`, `"100%"`, etc.) at compile time, and removes the cast.
