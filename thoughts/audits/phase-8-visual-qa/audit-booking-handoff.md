# Audit: Booking + Handoff (Check-in / Check-out)
**Task:** 8.7  
**Date:** 2026-04-11  
**Method:** Static code analysis (no bookings could be seeded — Stripe API keys are placeholders, so POST /api/v1/bookings fails at runtime; see Appendix limitations in phase-8-visual-qa.md)

---

## Screens audited

| Screen | File |
|--------|------|
| Booking request | `mobile/app/(tabs)/(feed)/booking-request.tsx` |
| Booking status | `mobile/app/(tabs)/(feed)/booking-status.tsx` |
| Active rental | `mobile/app/(tabs)/(feed)/active-rental.tsx` |
| Check-in | `mobile/app/(tabs)/(feed)/check-in.tsx` → `components/screens/CheckInScreen.native.tsx` |
| Check-out | `mobile/app/(tabs)/(feed)/check-out.tsx` → `components/screens/CheckOutScreen.native.tsx` |

## Components audited

| Component | File |
|-----------|------|
| `BookingCard` | `mobile/components/booking/BookingCard.tsx` |
| `IncomingRequest` | `mobile/components/booking/IncomingRequest.tsx` |
| `CancelConfirmation` | `mobile/components/booking/CancelConfirmation.tsx` |
| `GPSStatus` | `mobile/components/handoff/GPSStatus.tsx` |
| `PINDisplay` | `mobile/components/handoff/PINDisplay.tsx` |
| `PINEntry` | `mobile/components/handoff/PINEntry.tsx` |
| `PhotoGrid` | `mobile/components/handoff/PhotoGrid.tsx` |
| `AngleEnforcedCamera` | `mobile/components/camera/AngleEnforcedCamera.native.tsx` |

---

## Bugs Found

### BUG-BH-1: Cost breakdown shows $0 when user picks start date first (no end interaction)
**Severity:** Medium  
**File:** `mobile/app/(tabs)/(feed)/booking-request.tsx:163-169`

When the `DurationPicker` fires `onChangeStart` and no `scheduledEnd` exists yet, the handler calls `setSchedule(s, autoEnd)` (auto-sets end = start + 1hr) but does NOT call `setAmounts`. As a result, `checkoutStore.rentalFee` and `checkoutStore.holdAmount` remain 0. The `CostBreakdown` component renders (both `scheduledStart` and `scheduledEnd` are now set) but displays $0.00 for all line items.

The amounts only update when the user explicitly interacts with the end date picker, which triggers `onChangeEnd` → `handleScheduleChange` → `setAmounts`.

**Steps to reproduce:** Open booking-request screen → tap start date and pick a value → observe CostBreakdown shows $0.00 without touching end date.

**Fix:** In the `else` branch of `onChangeStart`, compute and call `setAmounts` with the auto-end date.

```typescript
// booking-request.tsx onChangeStart handler — else branch fix
else {
  const autoEnd = new Date(s.getTime() + 3_600_000);
  setSchedule(s, autoEnd);
  const hold = holdEstimate?.holdAmount ?? 0;
  const fee = estimateRentalFee(pricePerHour, pricePerDay, s, autoEnd);
  setAmounts(hold, fee);
}
```

---

### BUG-BH-2: `IncomingRequest` and `BookingCard` have hardcoded `mx-4` causing double-padding
**Severity:** Low  
**Files:**  
- `mobile/components/booking/IncomingRequest.tsx:93`  
- `mobile/components/booking/BookingCard.tsx:61`

Both components hardcode `mx-4` (16px horizontal margin). `booking-status.tsx` renders `IncomingRequest` inside a `ScrollView` with `paddingHorizontal: 16`. This results in the card sitting 32px from each edge instead of 16px — visually narrower than the rest of the screen content.

`BookingCard` has the same issue and is likely used in booking-list contexts with existing padding.

**Fix:** Remove hardcoded `mx-4` and `mb-3` from both components; let the parent control spacing.

---

### BUG-BH-3: Maps navigation URL uses listing ID as a search query (not coordinates)
**Severity:** Medium  
**Files:**  
- `mobile/app/(tabs)/(feed)/booking-status.tsx:292-298`  
- `mobile/app/(tabs)/(feed)/active-rental.tsx:97-103`

Both screens construct the Apple Maps deep link as:
```
maps://maps.apple.com/?q=RentMy+pickup+{booking.listingId}
```

This searches Maps for the literal string "RentMy pickup [ulid]" which will return no results. The listing's GPS coordinates are not included in the `Booking` interface (only `listingId` is available).

**Fix options (choose one):**
1. Add `listingLat`/`listingLng` to the `Booking` interface and populate from the backend JOIN.
2. Make a separate `GET /api/v1/listings/{id}` fetch to get coordinates before opening the URL.
3. Use a generic fallback URL (`maps://` with no query) that opens Maps to the user's current location until coordinates are wired.

Recommended: Option 1 — add `listingAddress` to the booking API response and use `?address={listingAddress}` in the Maps URL.

---

### BUG-BH-4: "Report an issue" in active-rental.tsx shows stale placeholder alert
**Severity:** Medium  
**File:** `mobile/app/(tabs)/(feed)/active-rental.tsx:107-114`

`handleReportIssue` shows an `Alert.alert` saying "Issue reporting and dispute flow will be available in a future update." However, the dispute screen exists at `mobile/app/(tabs)/(rentals)/dispute.tsx` and was implemented in earlier phases.

**Fix:** Replace the Alert with a navigation call:
```typescript
router.push({
  pathname: "/(tabs)/(rentals)/dispute" as never,
  params: { transactionId: booking.id },
});
```

---

### BUG-BH-5: "Message host/renter" button navigates to inbox, not specific conversation
**Severity:** Low  
**File:** `mobile/app/(tabs)/(feed)/booking-status.tsx:339-351`

The "Message host/renter" button navigates to `/(tabs)/(messages)` (the inbox tab) rather than the specific conversation for this booking. The user must manually locate the right conversation in their inbox.

**Fix:** Navigate to the conversation screen with the booking context:
```typescript
router.push({
  pathname: "/(tabs)/(messages)/conversation" as never,
  params: { bookingId: booking.id },
});
```
This requires the conversation screen to accept a `bookingId` param and look up or create the thread.

---

### BUG-BH-6: `IncomingRequest` missing `renterName` and `renterReputation` props
**Severity:** Low  
**File:** `mobile/app/(tabs)/(feed)/booking-status.tsx:222-227`

`IncomingRequest` is rendered without `renterName` or `renterReputation`:
```tsx
<IncomingRequest
  booking={booking}
  onAccept={handleAccept}
  onDecline={handleDecline}
/>
```
The component shows a greyed-out avatar and no renter identity. The host sees the date range but not who is requesting.

**Root cause:** The `Booking` interface (`useBooking.ts`) does not include renter display info — only `renterId`. The backend GET /api/v1/bookings/:id response would need to include `renterName` (or a `renter` sub-object).

**Fix:** Extend the `Booking` interface and backend response to include `renterName` (and optionally `renterReputationScore`), then pass them to `IncomingRequest`.

---

### BUG-BH-7: `isRenter` is `false` during data load — `useProximity` initialized with wrong role
**Severity:** Low  
**Files:**  
- `mobile/components/screens/CheckInScreen.native.tsx:41-44`  
- `mobile/components/screens/CheckOutScreen.native.tsx:37-41`

`isRenter` is derived from `bookingData`, which is `undefined` on first render. `useProximity(id, type, isRenter)` is called before the loading guard, so it receives `isRenter = false` on initial render. The `canComplete` formula differs between host (GPS + photos) and renter (GPS + PIN + photos).

In practice this doesn't cause a functional bug because:
- The loading spinner renders while `isLoading || !bookingData`, preventing user interaction.
- `canComplete` is recalculated correctly once data loads.

But if bookingData loads extremely fast (cached query), there could be a render cycle where the UI briefly computes `canComplete` with the wrong role. The fix is to default to `isRenter = true` (the more restrictive requirement) or disable proximity tracking until the role is known.

---

### BUG-BH-8: `PINDisplay` phone number field has no format validation
**Severity:** Low  
**File:** `mobile/components/handoff/PINDisplay.tsx:33-49`

The "Send PIN via SMS" input accepts any string for the phone number and only validates that the field is non-empty. An incorrectly formatted phone number (e.g., "5551234" without country code) will fail at the API level with a cryptic error.

**Fix:** Add a basic E.164 format check before enabling the Send button:
```typescript
const isValidPhone = /^\+[1-9]\d{6,14}$/.test(phone.trim());
```

---

## TypeScript Status

Running `npx tsc --noEmit` from `mobile/` shows two pre-existing errors in `app/(tabs)/(profile)/index.tsx` (route type mismatches on `create-listing` and `referrals` paths). **None of the booking/handoff files have TypeScript errors.**

---

## Summary

| ID | Description | Severity | File(s) |
|----|-------------|----------|---------|
| BUG-BH-1 | Cost breakdown shows $0 when user picks start date first | Medium | `booking-request.tsx` |
| BUG-BH-2 | `IncomingRequest` / `BookingCard` hardcoded `mx-4` double-pads inside padded container | Low | `IncomingRequest.tsx`, `BookingCard.tsx` |
| BUG-BH-3 | Maps URL uses listing ID not coordinates | Medium | `booking-status.tsx`, `active-rental.tsx` |
| BUG-BH-4 | "Report an issue" shows placeholder alert instead of navigating to dispute screen | Medium | `active-rental.tsx` |
| BUG-BH-5 | Message button goes to inbox, not specific conversation | Low | `booking-status.tsx` |
| BUG-BH-6 | `IncomingRequest` missing renter name/reputation | Low | `booking-status.tsx`, `IncomingRequest.tsx` |
| BUG-BH-7 | `isRenter` false during data load causes wrong initial `canComplete` | Low | `CheckInScreen.native.tsx`, `CheckOutScreen.native.tsx` |
| BUG-BH-8 | PIN SMS field missing E.164 format validation | Low | `PINDisplay.tsx` |

**Total bugs:** 8 (3 medium, 5 low)  
**No critical or high-severity bugs found** — all screens render correctly and complete flows are functional assuming the backend APIs respond as expected.

---

## Limitations

- No live simulator testing possible — no bookings exist in the DB (Stripe placeholder keys block booking creation).
- All bugs found via static code analysis of screens and components.
- Backend proximity endpoints (`/api/v1/proximity/verify`, `/api/v1/proximity/pin`, `/api/v1/proximity/sms-fallback`) not exercised — their response shapes are assumed correct.
