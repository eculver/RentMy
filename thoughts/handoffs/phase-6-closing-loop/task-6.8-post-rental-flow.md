# Task 6.8 — Post-Rental Flow (RN)

**Status:** Completed  
**Commit:** 31e3fbb053bd98f0542cf31641294197d5d1cdc4  
**Date:** 2026-04-06  
**Branch mode:** Graphite (gt create task-6.8-post-rental-flow)

---

## What was implemented

### New `(rentals)` tab

- **`mobile/app/(tabs)/(rentals)/_layout.tsx`** — Stack layout with no header (each screen provides its own).
- **`mobile/app/(tabs)/(rentals)/index.tsx`** — Rentals list screen showing all bookings (renter + host perspectives, deduplicated). Active rentals show a "Return" shortcut; completed rentals show a "Rate" pill if the current user hasn't rated yet. RatingPrompt modal opens inline.
- **`mobile/app/(tabs)/_layout.tsx`** — Added `(rentals)` tab between Map and Messages, using `receipt-outline` icon.

### New screens

| File | Purpose |
|------|---------|
| `return-confirmation.tsx` | Post-checkout summary: transaction details, HoldStatusCard, photo diff notice, links to rate/dispute |
| `rate.tsx` | Full-page rating UI with RatingBubbles, role-aware bubble set, success state, skip |
| `dispute.tsx` | Dispute filing: reason selector (DAMAGE / MISSING_ITEM / OTHER), description text input (min 10 chars), evidence note, submit |
| `dispute-status.tsx` | Dispute tracking: DisputeTimeline, INCONCLUSIVE re-prompt banner, decision details when resolved |

### New components (`mobile/components/rental/`)

| File | Purpose |
|------|---------|
| `HoldStatusCard.tsx` | Bar visualization of hold allocation buckets (late, damage, reserve, released) |
| `PhotoDiffResult.tsx` | Side-by-side check-in/out photo pairs with ClassificationBadge |
| `DisputeTimeline.tsx` | Vertical 4-step timeline: filed → evidence → under review → resolved |

### New hooks (`mobile/lib/hooks/`)

| File | Purpose |
|------|---------|
| `useDispute.ts` | `useTransactionDisputes`, `useDispute`, `useFileDispute`. Open disputes poll every 15 s via `refetchInterval`. |
| `usePostRental.ts` | Orchestrates completion detection, rating state, Pusher event subscription (hold-released, damage-detected). Returns `step`, `showRatingPrompt`, `hasOpenDispute`, etc. |

### New lib file

- **`mobile/lib/notifications.ts`** — `NotificationType` union, typed payload interfaces for each post-rental event, `NOTIFICATION_ROUTES` record, `getNotificationParams` helper.

### Tests

- **`mobile/__tests__/components/rental.test.tsx`** — 18 tests covering HoldStatusCard (5), PhotoDiffResult (3), DisputeTimeline (5), notifications module (4). All pass.

---

## Decisions and rationale

- **Rentals tab separate from Feed** — The plan explicitly required `(rentals)` as its own tab. Keeps active feed (discovery) distinct from rental management.
- **Polling vs Pusher for disputes** — Used both: `refetchInterval` (15 s) as a fallback for dispute status, Pusher events for immediate hold-release and damage-detect signals. Ensures updates arrive even if the WebSocket drops.
- **HoldStatusCard zeros** — The backend doesn't yet expose a per-line-item hold breakdown. The card renders gracefully with zero values; a future task can wire in the real data.
- **No new dependencies** — All libraries (TanStack Query, Pusher, NativeWind, Ionicons) already present.

---

## Verification results

```
cd mobile && npx tsc --noEmit    ✅ clean
cd mobile && npx jest            ✅ 84/84 pass (18 new + 66 existing)
```

---

## Notes for downstream tasks

- `usePostRental` is ready to consume `hold-released` events once the backend emits them (no backend change needed, event name matches the Pusher channel pattern).
- `PhotoDiffResult` expects pairs from the CV pipeline result; Task 6.1 photo-diff output can be surfaced here by passing the diff pairs from the transaction API.
- The `NOTIFICATION_ROUTES` table in `notifications.ts` is the single source of truth for deep-linking from push notifications — update it if new notification types are added.
