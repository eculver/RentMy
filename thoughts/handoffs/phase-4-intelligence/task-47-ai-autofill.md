# Task 4.7 — Wire AI Autofill into Listing Creation (RN)

## Summary

Wired the AppraisalAgent API into the listing creation flow. After a listing is created and media attached, the app polls for the AI appraisal result, shows a skeleton loading overlay, and populates the form with AI-suggested title, description, pricing, and tags. Each AI-filled field carries a dismissible "AI" badge that disappears on host edit. If the host declares a day price exceeding 2x the AI's estimated daily value, a modal prompts for justification and calls the override review API.

## Branching Mode

Git fallback (vanilla git). Branch: `task-4.7-ai-autofill`. Commit: `3a4527b`.

## What Was Built

### New Files

| File | Purpose |
|------|---------|
| `mobile/lib/hooks/useAppraisal.ts` | TanStack Query hooks: `useAppraisal(listingId)` polls every 2s while status is PENDING, stops on COMPLETE/FAILED. `useOverride(listingId)` mutation calling `POST /api/v1/listings/:id/override`. |
| `mobile/components/listing/AIAutofillOverlay.tsx` | Full-screen overlay (absolute positioned, z-10) shown while appraisal runs. Uses `Animated.loop` shimmer effect on skeleton bars for title, description, pricing, and tags. Shows error state if appraisal failed. |
| `mobile/components/listing/ValueOverridePrompt.tsx` | Bottom-sheet modal. Displays AI estimate vs host value side-by-side, accepts justification text, calls `useOverride`, shows approve/reject result inline with color coding. |

### Modified Files

**`mobile/components/listing/ListingForm.tsx`**

- Added `AISuggestions` interface (exported): `title`, `description`, `pricePerDay`, `pricePerHour`, `tags`, `estimatedValueCents`
- Added `aiSuggestions` and `onPricePerDayChange` props
- `useEffect` applies AI suggestions via `setValue` and records which fields were AI-filled in a `Set<string>`
- `AIBadge` inline component: sky-colored "AI" pill shown next to the label of any AI-filled field
- Editing a field calls `markEdited(field)` which removes it from the AI-filled set (badge disappears)
- `onPricePerDayChange` callback fires on every keystroke for the price/day field

**`mobile/app/(tabs)/(profile)/create-listing.tsx`**

- Added `listingId` state: set after successful listing creation to start appraisal polling
- `useAppraisal(listingId)` hook drives the overlay visibility: shows while status is PENDING or while loading
- `resolvedSuggestions` maps `AppraisalResult` → `AISuggestions` (cents → dollars conversion)
- `handlePricePerDayChange` checks if new value > 2× AI daily value estimate (using a `÷30` item-value heuristic) and sets `overrideVisible`
- Three render paths after listing creation:
  1. Appraisal pending → `AIAutofillOverlay isLoading={true}`
  2. Appraisal failed → `AIAutofillOverlay` error state + "Continue without AI suggestions" link
  3. Appraisal complete → `router.back()` (listing is ready, form was pre-filled before submit)
- `ValueOverridePrompt` rendered conditionally when `overrideVisible && listingId && estimatedValueCents`

## Key Design Decisions

- **Poll via `refetchInterval`**: TanStack Query's function form of `refetchInterval` stops polling once status is no longer PENDING. No manual cleanup needed.
- **Listing-first flow**: The listing is created with the host's manually-entered values first, then appraisal runs async. The appraisal result is used to pre-fill the form on the _next_ edit, not to override the initial submit. This matches the `COALESCE` behavior on the backend (AI only fills empty fields).
- **Override threshold is price/day not item value**: The plan specified ">2x AI estimate", and the AppraisalAgent returns `estimatedValueCents` (item replacement value), not a daily rental price. Used a `÷30` heuristic (item value / 30 ≈ reasonable daily rate) as the comparison baseline. This is a conservative proxy — hosts of high-value items won't be blocked.
- **No separate tags field in the form**: Tags are display-only (read from AI, shown as pills). The backend accepts them via the appraisal pipeline, not from the form submission.
- **`AIAutofillOverlay` is absolute-positioned**: It overlays the `ListingForm` rather than replacing it, so the form is pre-rendered beneath (no flash on reveal).

## Verification

```
cd mobile && npx tsc --noEmit  # no errors
```

Manual test checklist (simulator):
1. Create listing → capture photos → tap Done → form appears with AI overlay shimmer
2. Overlay disappears once appraisal completes → fields populate with AI badge
3. Edit title → AI badge on title disappears
4. Change price/day to 3× AI estimate → `ValueOverridePrompt` slides up
5. Submit justification → approve/reject shown inline
6. Second listing with same flow → no KYC prompt (already verified, no interference with task 4.6)
