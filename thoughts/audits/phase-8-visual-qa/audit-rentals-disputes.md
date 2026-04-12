# Audit: Rentals + Disputes + Ratings (Task 8.11)

**Date:** 2026-04-11  
**Method:** Static code audit (iOS Simulator not available in this session)  
**Screens audited:**
- `mobile/app/(tabs)/(rentals)/index.tsx`
- `mobile/app/(tabs)/(rentals)/dispute.tsx`
- `mobile/app/(tabs)/(rentals)/dispute-status.tsx`
- `mobile/app/(tabs)/(rentals)/rate.tsx`
- `mobile/app/(tabs)/(rentals)/return-confirmation.tsx`

**Components audited:**
- `mobile/components/rental/DisputeTimeline.tsx`
- `mobile/components/rental/HoldStatusCard.tsx`
- `mobile/components/rental/PhotoDiffResult.tsx`
- `mobile/components/rating/RatingBubbles.tsx`
- `mobile/components/rating/RatingPrompt.tsx`

**Backend reviewed:**
- `backend/internal/dispute/model.go`, `handler.go`
- `backend/internal/rating/model.go`, `handler.go`
- `mobile/lib/hooks/useDispute.ts`, `useRatings.ts`, `useBookings.ts`, `useBooking.ts`

---

## Summary

9 bugs found: 3 critical protocol mismatches between backend and frontend, 2 broken navigation flows, 2 incomplete implementations, and 2 cosmetic issues.

---

## Bugs

### BUG-1 [CRITICAL] — Rating JSON field name mismatch: backend PascalCase vs frontend camelCase

**File:** `backend/internal/rating/model.go:101-109`

The `Rating` struct has no JSON tags. Go's `encoding/json` serializes unexported-prefix fields as PascalCase (e.g., `ID`, `FromUserID`, `TransactionID`), but the frontend `Rating` interface expects camelCase (`id`, `fromUserId`, `transactionId`).

**Impact:** 
- `useTransactionRatings` returns objects where `fromUserId` is `undefined` (the actual field is `FromUserID`).
- The `hasRated` check in `index.tsx:158` always returns `false`:
  ```tsx
  const hasRated = (ratings ?? []).some((r) => r.fromUserId === currentUserId);
  ```
  The "Rate" button persists even after the user has submitted a rating, causing duplicate rating attempts that hit the `ErrAlreadyRated` 409.

**Fix for 8.12:** Add JSON tags to `backend/internal/rating/model.go`:
```go
type Rating struct {
    ID            string    `json:"id"`
    TransactionID string    `json:"transactionId"`
    FromUserID    string    `json:"fromUserId"`
    ToUserID      string    `json:"toUserId"`
    Bubbles       []Bubble  `json:"bubbles"`
    CreatedAt     time.Time `json:"createdAt"`
}
```

---

### BUG-2 [CRITICAL] — Dispute status values: backend/frontend mismatch

**Files:** `backend/internal/dispute/model.go:23-32`, `mobile/lib/hooks/useDispute.ts:4-9`

The backend `Status` constants don't match the frontend `DisputeStatus` union type:

| Backend               | Frontend              | Match? |
|-----------------------|-----------------------|--------|
| `PENDING`             | `PENDING`             | ✓      |
| `GATHERING`           | `EVIDENCE_GATHERING`  | ✗      |
| `ANALYZING`           | —                     | ✗      |
| `AUTO_RESOLVED`       | —                     | ✗      |
| `AUDIT_QUEUED`        | —                     | ✗      |
| `HUMAN_REVIEW`        | `UNDER_REVIEW`        | ✗      |
| `RESOLVED`            | `RESOLVED`            | ✓      |
| `INCONCLUSIVE`        | —                     | ✗      |
| —                     | `CLOSED`              | ✗      |

**Impact:**
- `DisputeTimeline.tsx` has `STATUS_ORDER = ["PENDING", "EVIDENCE_GATHERING", "UNDER_REVIEW", "RESOLVED"]`. The `stepIndex()` function returns `3` (the last slot, "Resolved") for any unrecognized status, so disputes in `GATHERING`, `ANALYZING`, `HUMAN_REVIEW`, etc. all display as "Resolved" in the timeline.
- Auto-refresh logic in `useDispute.ts:75` only stops polling for `RESOLVED` and `CLOSED`. Disputes in `AUTO_RESOLVED`, `AUDIT_QUEUED` states will keep polling every 15s indefinitely.
- The `hasOpenDispute` checks in `return-confirmation.tsx` and `index.tsx` use `d.status !== "RESOLVED" && d.status !== "CLOSED"`. Disputes in `AUTO_RESOLVED` or `AUDIT_QUEUED` are treated as still-open.

**Fix for 8.12:** Align status values. Choose one direction:
- Preferred: update frontend `DisputeStatus` to match backend exactly, then update `DisputeTimeline` `STATUS_ORDER` and all status checks.
- Alternative: rename backend status constants to match frontend naming.

---

### BUG-3 [CRITICAL] — Dispute JSON field name mismatches: backend vs frontend interface

**Files:** `backend/internal/dispute/model.go:43-61`, `mobile/lib/hooks/useDispute.ts:21-37`

Multiple field name mismatches between the Go struct JSON tags and the TypeScript `Dispute` interface:

| Backend JSON key        | Frontend interface field   | Impact                                   |
|-------------------------|----------------------------|------------------------------------------|
| `route`                 | `escalationRoute`          | Escalation route always null in UI       |
| `chargeAmount`          | `damageChargeCents`        | Damage charge never shown                |
| `confidence`            | `agentConfidence`          | Confidence % never shown                 |
| `agentDecisionId` (UUID)| `agentDecision` (verdict string) | Displays a UUID instead of verdict  |
| `reviewerId`            | `resolvedBy`               | Reviewer attribution always null         |
| `evidence` (RawMessage) | `evidenceRefs` (string[])  | Type mismatch — evidence never rendered  |

**Fix for 8.12:** Align field names in either the backend JSON tags or the frontend TypeScript interface. Additionally, the backend should expose the verdict string (from the agent decision record) directly on the dispute response rather than the agent decision UUID, or the frontend should query the agent decision separately.

---

### BUG-4 [HIGH] — `check-out` navigates to `booking-status` instead of `return-confirmation`

**File:** `mobile/components/screens/CheckOutScreen.native.tsx:55-58`

After a successful check-out API call, the screen navigates to `/(tabs)/(feed)/booking-status`. The `return-confirmation` screen exists at `/(tabs)/(rentals)/return-confirmation` but is only reachable via push notification (`mobile/lib/notifications.ts:80`), not from any in-app navigation.

```tsx
// Current (wrong):
router.replace({
  pathname: "/(tabs)/(feed)/booking-status" as never,
  params: { transactionId },
});

// Should be:
router.replace({
  pathname: "/(tabs)/(rentals)/return-confirmation" as never,
  params: { transactionId },
});
```

**Impact:** Users never see the return summary, hold breakdown, photo diff status, or post-return dispute/rate CTAs unless they receive a push notification.

---

### BUG-5 [HIGH] — DISPUTED booking navigates to `booking-status` instead of `dispute-status`

**File:** `mobile/app/(tabs)/(rentals)/index.tsx:229-240`

In `handleBookingPress`, `DISPUTED` bookings fall through to the `else` branch and navigate to `/(tabs)/(feed)/booking-status`. They should navigate to `/(tabs)/(rentals)/dispute-status`.

```tsx
// Current:
function handleBookingPress(booking: Booking) {
  if (booking.status === "ACTIVE") {
    router.push(/* active-rental */);
  } else {
    router.push(/* booking-status  ← wrong for DISPUTED */);
  }
}
```

**Fix for 8.12:** Add an explicit DISPUTED branch:
```tsx
} else if (booking.status === "DISPUTED") {
  router.push({
    pathname: "/(tabs)/(rentals)/dispute-status" as never,
    params: { transactionId: booking.id },
  });
}
```

---

### BUG-6 [MEDIUM] — `return-confirmation` hardcodes zero hold allocation

**File:** `mobile/app/(tabs)/(rentals)/return-confirmation.tsx:86-92`

```tsx
// Hardcoded placeholder:
const holdAllocation = {
  authorizedCents: 0,
  capturedLateCents: 0,
  capturedDamageCents: 0,
  damageReserveCents: 0,
  releasedCents: 0,
};
```

The comment acknowledges this ("In a real implementation these would come from the transaction/hold API"), but the feature is incomplete. `HoldStatusCard` always renders "Total authorized: $0.00" and "Hold release is pending — check back shortly".

**Fix for 8.12:** Add hold allocation fields to the `Booking` type (or a separate API endpoint) and populate the `holdAllocation` from real data. If the backend doesn't yet expose these fields, the `HoldStatusCard` should be hidden until data is available, not shown with all-zeros.

---

### BUG-7 [MEDIUM] — `PhotoDiffResult` component exists but is never rendered in any rentals screen

**Files:** `mobile/components/rental/PhotoDiffResult.tsx`, `mobile/app/(tabs)/(rentals)/dispute-status.tsx`, `mobile/app/(tabs)/(rentals)/return-confirmation.tsx`

`PhotoDiffResult` is a well-implemented component (tested in `__tests__/components/rental.test.tsx`) that shows side-by-side check-in/check-out photo pairs with damage classification badges. However, it is imported and used nowhere in the rentals screens.

- `dispute-status.tsx` shows a text description of the agent decision but not the photo comparison.
- `return-confirmation.tsx` shows a "Photo comparison in progress" banner but never renders the actual diff when it's available.

**Fix for 8.12:** Integrate `PhotoDiffResult` into `dispute-status.tsx` (show photos when dispute has evidence) and/or `return-confirmation.tsx` (show when photo comparison is available).

---

### BUG-8 [LOW] — INCONCLUSIVE re-prompt "Upload photos" is a stub

**File:** `mobile/app/(tabs)/(rentals)/dispute-status.tsx:66-71`

```tsx
const handleReprompt = () => {
  Alert.alert(
    "Upload additional photos",
    "Please take new photos...",
    [{ text: "OK" }],
  );
};
```

This shows only an informational alert. It does not navigate to a camera/photo upload screen and does not call any API endpoint.

**Fix for 8.12:** Navigate to the check-out camera screen in re-prompt mode, or implement a dedicated evidence re-upload flow.

---

### BUG-9 [LOW] — `HoldStatusCard` `textColor` prop is dead code

**File:** `mobile/components/rental/HoldStatusCard.tsx:50-52`

```tsx
{/* suppress unused textColor — reserved for future label styling */}
{textColor === "" && null}
```

The `textColor` prop is accepted by `BarSegment` but never applied. The workaround comment hints this was intentional, but it adds confusion and a dead prop to the component interface.

**Fix for 8.12:** Remove the `textColor` prop from `BarSegment` entirely if it's not used, or apply it to the label text.

---

## Screens Not Navigable Without Seeded Bookings

Due to the seeding limitation documented in the Phase 8 appendix (no bookings because Stripe payment method is required), the following screens cannot be visually tested against live data:
- All five rentals screens
- Both dispute screens
- Rate screen

The bugs above were found through static analysis. Visual confirmation should be done in 8.12 after fixing bugs or after seeding bookings via a test endpoint that bypasses Stripe.

---

## Non-Bugs (Design Decisions)

- `RatingPrompt` modal used in rentals list vs. `rate.tsx` full screen — both exist intentionally; the modal is for quick rating from the list, the screen is for deep-link / notification-driven rating.
- `primary-600` NativeWind classes in `RatingBubbles` and `RatingPrompt` — `primary` is defined in `tailwind.config.js` as sky-600 equivalent. No issue.
- `DisputeTimeline` uses "← current" inline text indicator — intentional visual design.
