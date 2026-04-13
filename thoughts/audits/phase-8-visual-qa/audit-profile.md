# Audit: Profile + Referrals + KYC (Task 8.13)

**Date:** 2026-04-11
**Method:** Static code audit (iOS Simulator not available in this session)
**Screens audited:**
- `mobile/app/(tabs)/(profile)/index.tsx`
- `mobile/app/(tabs)/(profile)/referrals.tsx`
- `mobile/app/(tabs)/(profile)/verify.tsx` (re-exports `VerifyScreen`)
- `mobile/app/(tabs)/(profile)/create-listing.tsx` (re-exports `CreateListingScreen`)

**Components audited:**
- `mobile/components/screens/VerifyScreen.native.tsx`
- `mobile/components/screens/CreateListingScreen.native.tsx`
- `mobile/components/referral/ReferralCard.tsx`
- `mobile/components/listing/ListingForm.tsx`
- `mobile/components/listing/AIAutofillOverlay.tsx`
- `mobile/components/listing/ValueOverridePrompt.tsx`

**Hooks reviewed:**
- `mobile/lib/hooks/useReferrals.ts`
- `mobile/lib/hooks/useVerification.ts`
- `mobile/lib/hooks/useAppraisal.ts`
- `mobile/lib/hooks/useListings.ts`

---

## Summary

6 bugs found: 2 critical logic errors in the create-listing flow, 2 high-severity issues (missing navigation + deprecated API), 1 medium NativeWind antipattern, and 1 low cosmetic/type issue.

---

## Bugs

### BUG-1 [CRITICAL] — `router.back()` called during render in CreateListingScreen

**File:** `mobile/components/screens/CreateListingScreen.native.tsx:209-211`

```tsx
// Appraisal complete — navigate back (listing is ready)
if (appraisalComplete && !overrideVisible) {
  router.back();
  return null;
}
```

Calling `router.back()` directly in the render function is a React violation. React does not allow triggering state/navigation side effects during rendering — this causes "Cannot update during an existing state transition" warnings and can trigger render loops if the navigation state update causes a re-render before unmount completes.

**Impact:** Unstable navigation behavior; React may warn or skip the navigation entirely depending on timing. In strict mode, the component double-renders, which could fire `router.back()` twice.

**Fix for 8.14:** Move navigation into a `useEffect`:
```tsx
useEffect(() => {
  if (appraisalComplete && !overrideVisible) {
    router.back();
  }
}, [appraisalComplete, overrideVisible, router]);
```

---

### BUG-2 [CRITICAL] — `ValueOverridePrompt` permanently unreachable

**File:** `mobile/components/screens/CreateListingScreen.native.tsx:153-163, 209-212`

The override threshold check in `handlePricePerDayChange` early-returns if `resolvedSuggestions?.estimatedValueCents` is falsy:

```tsx
const handlePricePerDayChange = (dollars: number) => {
  setPendingPriceDollars(dollars);
  if (!resolvedSuggestions?.estimatedValueCents) return; // exits here every time
  // threshold check — never reached before form submission
};
```

`resolvedSuggestions` derives from `appraisalComplete`:
```tsx
const resolvedSuggestions: AISuggestions | null = appraisalComplete
  ? { title: appraisal.itemName, ..., estimatedValueCents: appraisal.estimatedValueCents }
  : aiSuggestions; // aiSuggestions is always null (never set in component)
```

The sequence of events:
1. User types price in form → `handlePricePerDayChange` fires → `resolvedSuggestions` is `null` (appraisal not done yet) → early return → `overrideVisible` stays `false`
2. User submits form → listing created → appraisal polling starts
3. Appraisal completes → `appraisalComplete = true` → `resolvedSuggestions` now has `estimatedValueCents`
4. **Before the user can trigger `handlePricePerDayChange` again**, the render immediately hits `if (appraisalComplete && !overrideVisible)` → `router.back()` fires → prompt never shown

`ValueOverridePrompt` can never appear because `overrideVisible` cannot become `true` before `router.back()` fires.

**Impact:** The 2x override guard (a core trust/safety feature) is completely bypassed. Hosts can list items at any price relative to AI estimate without ever being prompted for justification.

**Fix for 8.14:** Move the threshold check to a `useEffect` that runs when `appraisalComplete` first becomes true, comparing against `pendingPriceDollars` (which was captured from the form submission):

```tsx
useEffect(() => {
  if (!appraisalComplete || !resolvedSuggestions?.estimatedValueCents) return;
  if (pendingPriceDollars == null) return;

  const aiDailyValueDollars =
    resolvedSuggestions.estimatedValueCents / 100 / DAILY_PRICE_DIVISOR;
  if (pendingPriceDollars > aiDailyValueDollars * OVERRIDE_THRESHOLD_MULTIPLIER) {
    setOverrideVisible(true);
  }
}, [appraisalComplete, resolvedSuggestions, pendingPriceDollars]);
```

This runs once when appraisal completes, compares the submitted price to the AI estimate, and shows the prompt before the navigation `useEffect` (BUG-1 fix) fires.

---

### BUG-3 [HIGH] — Profile screen has no "Verify Identity" entry point

**File:** `mobile/app/(tabs)/(profile)/index.tsx`

The KYC verification screen exists at `mobile/app/(tabs)/(profile)/verify.tsx` (backed by `VerifyScreen.native.tsx`), but the profile screen provides no button, link, or banner to navigate there. The profile screen offers:

- "Create Listing" → `/(tabs)/(profile)/create-listing`
- "Invite Friends — Earn $20" → `/(tabs)/(profile)/referrals`
- Sign Out

There is no path to `/(tabs)/(profile)/verify`.

**Impact:** Identity verification is inaccessible to users through the normal app UI. Renters who need KYC to book an item have no in-app path to complete it — they would only see error messages when trying to book, with no actionable CTA.

**Fix for 8.14:** Add a "Verify Identity" row to the profile screen, conditionally shown based on the user's `identityStatus`:

```tsx
const identityStatus = useAuthStore((s) => s.user?.identityStatus);

{identityStatus !== "VERIFIED" && (
  <Pressable
    className="mt-3 w-full border border-amber-400 py-3 rounded-xl items-center"
    onPress={() => router.push("/(tabs)/(profile)/verify")}
  >
    <Text className="text-amber-600 font-semibold">Verify Identity</Text>
  </Pressable>
)}
```

---

### BUG-4 [HIGH] — `Clipboard` imported from deprecated `react-native` package

**File:** `mobile/app/(tabs)/(profile)/referrals.tsx:10`

```tsx
import {
  ...
  Clipboard,
} from "react-native";
```

`Clipboard` was deprecated in React Native 0.59 and is no longer exported from `react-native` in modern Expo SDK versions. This project uses Expo SDK 54 (React Native 0.76), where `Clipboard` from `react-native` is `undefined`.

`handleCopy` calls `Clipboard.setString(codeData.code)` — this will throw a runtime `TypeError: Cannot read properties of undefined (reading 'setString')` when the user taps "Copy Code".

**Impact:** Copy Code button crashes the app with a TypeError on all modern Expo SDK versions.

**Fix for 8.14:** Install `expo-clipboard` and update the import:

```bash
cd mobile && npx expo install expo-clipboard
```

```tsx
import * as Clipboard from "expo-clipboard";
// ...
const handleCopy = async () => {
  if (!codeData?.code) return;
  await Clipboard.setStringAsync(codeData.code);
  setCopied(true);
  setTimeout(() => setCopied(false), 2000);
};
```

---

### BUG-5 [MEDIUM] — NativeWind class-splitting antipattern in `ReferralCard`

**File:** `mobile/components/referral/ReferralCard.tsx:44-45`

```tsx
<View className={`px-2 py-0.5 rounded-full ${statusColor[referral.status].split(" ")[0]}`}>
  <Text className={`text-xs font-medium ${statusColor[referral.status].split(" ")[1]}`}>
```

The `statusColor` values are combined strings like `"bg-blue-100 text-blue-700"`, which are then split at runtime. NativeWind's Babel/Metro plugin performs static analysis to generate the style registry at build time. When individual class names are derived via `.split(" ")[0]` rather than appearing as whole strings in a `className` prop, the analyzer may not register them correctly — especially under production builds with optimization passes that could alter the string values.

**Impact:** In production builds, status badge colors may not render. Fragile dependency on exact string format and ordering in the `statusColor` map.

**Fix for 8.14:** Use separate lookup maps:

```tsx
const statusBg: Record<Referral["status"], string> = {
  SIGNED_UP: "bg-blue-100",
  FIRST_RENTAL_COMPLETED: "bg-yellow-100",
  PAID: "bg-green-100",
  FRAUDULENT: "bg-red-100",
};

const statusText: Record<Referral["status"], string> = {
  SIGNED_UP: "text-blue-700",
  FIRST_RENTAL_COMPLETED: "text-yellow-700",
  PAID: "text-green-700",
  FRAUDULENT: "text-red-700",
};

// Usage:
<View className={`px-2 py-0.5 rounded-full ${statusBg[referral.status]}`}>
  <Text className={`text-xs font-medium ${statusText[referral.status]}`}>
```

---

### BUG-6 [LOW] — `ShimmerBar` `width` type assertion unsafe

**File:** `mobile/components/listing/AIAutofillOverlay.tsx:34`

```tsx
<Animated.View
  style={{ opacity, width: width as `${number}%` }}
  className="h-4 bg-gray-200 rounded-md mb-2"
/>
```

The `width` prop is typed as `string`, but the cast `as \`${number}%\`` is a TypeScript-only assertion — it performs no runtime validation. Callers could pass non-percentage strings (e.g., `"full"`, `"auto"`) and the cast would not catch it. The correct type for a percentage-only prop is to accept `\`${number}%\`` directly or use a `DimensionValue` from React Native.

**Impact:** Low risk — all current call sites pass valid percentage strings like `"85%"`. TypeScript won't catch a future misuse.

**Fix for 8.14:** Change the prop type:

```tsx
function ShimmerBar({ width, delay }: { width: `${number}%`; delay: number }) {
  // ...
  style={{ opacity, width }}
```

---

## Screens Without Runtime Navigation Issues

These screens are structurally correct and require no routing fixes:

- `referrals.tsx` — loads and displays referrals correctly (apart from BUG-4 Clipboard crash on copy)
- `verify.tsx` — delegates to `VerifyScreen.native.tsx` which has correct Stripe Identity flow and state machine

---

## Non-Bugs (Design Decisions)

- **`handleStart` idempotent return**: When `result.ephemeralKeySecret` is absent, `VerifyScreen` sets state to "processing" and polls — this is intentional behavior for sessions already started in a previous app session. The user cannot replay the Stripe sheet for an existing session; polling is the correct path.

- **`ListingForm` `"" as unknown as number` defaults**: Using empty strings as number defaults for `react-hook-form` is the standard workaround when `z.coerce.number()` is used with `zodResolver`. The `zod` coerce transforms the empty string to `NaN` on submit, which then fails `positive()` validation — as intended.

- **`ProfileScreen` avatar initials**: The profile avatar shows `user.name.charAt(0)`. This is intentional — the profile is the current user, whose name is always available from auth state.

- **Verify screen `ScreenState` `"starting"` vs `"processing"` distinction**: Both show a loading indicator with different labels ("Starting…" vs "Verifying…"). This is intentional UX to distinguish the Stripe sheet launch phase from the backend polling phase.
