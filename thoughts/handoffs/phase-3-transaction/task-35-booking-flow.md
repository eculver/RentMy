# Task 3.5 — Booking flow (RN)

## Status: COMPLETE

**Commit:** 00c487a
**Branch:** task-3.5-booking-flow
**Branching mode:** git fallback (Graphite unavailable)

---

## What Was Done

### New packages installed
- `pusher-js` — Pusher JavaScript client with `/react-native` entry point
- `expo-notifications` — Expo push notification SDK (installed for future use;
  token registration will be wired in a later task)

### New screens

**`mobile/app/(tabs)/(feed)/booking-request.tsx`**
- Entry point from the feed card "Rent Now" button
- Accepts params: `id` (listing ID), `title`, `pricePerHour`, `pricePerDay`, `hostName`
- Renders `DurationPicker`, `CostBreakdown`, `PaymentMethodSelector` (reusing Phase 2 checkout components)
- On confirm → `POST /api/v1/bookings` → navigates to `booking-status.tsx`
- "How it works" explainer note (2-hour accept window, no charge until accepted)

**`mobile/app/(tabs)/(feed)/booking-status.tsx`**
- Main booking lifecycle screen for both renters and hosts
- Receives `transactionId` param, fetches booking via `useBooking` hook
- Subscribes to `private-transaction-{id}` Pusher channel for real-time invalidation
- Status-specific rendering:
  - **REQUESTED (host):** `IncomingRequest` component with accept/decline buttons
  - **ACCEPTED (renter):** "Start check-in" button → navigates to `check-in` screen (task 3.6)
  - **ACTIVE:** "Start check-out" button → navigates to `check-out` screen (task 3.6)
  - **CANCELLED:** Cancellation fee notice if fee > 0
  - **Terminal states:** "Back to feed" button
- Pull-to-refresh fallback when Pusher channel auth is not yet set up
- Cancel button shown for REQUESTED and ACCEPTED states; opens `CancelConfirmation` modal

### New hooks

**`mobile/lib/hooks/useBooking.ts`**
- Exports `Booking` type and `BookingStatus` union — shared across the codebase
- `useBooking(id)`: TanStack Query fetch of `GET /api/v1/bookings/{id}`
- Binds `usePusher` to the `booking-status-changed` event → invalidates the query

**`mobile/lib/hooks/useBookings.ts`**
- `useRenterBookings()`: fetches `GET /api/v1/users/me/bookings`
- `useHostBookings()`: fetches `GET /api/v1/users/me/hosted-bookings`
- Used by future bookings list / activity screens

**`mobile/lib/hooks/usePusher.ts`**
- Wraps `pusher-js/react-native` with a clean React hook API
- Signature: `usePusher(channelName, eventName, onEvent)`
- Configures `channelAuthorization` pointing to `/api/v1/pusher/auth` with the
  user's JWT — the auth endpoint is a pending backend task
- Creates one Pusher connection per hook instance; disconnects on unmount
- Private channels fail silently if auth endpoint is missing (graceful degradation)

### New components

**`mobile/components/booking/BookingCard.tsx`**
- Compact booking summary card for list views
- Props: `booking`, `otherPartyName`, `listingTitle`, `onPress`
- Color-coded status badge per state

**`mobile/components/booking/IncomingRequest.tsx`**
- Host-side card shown while booking is in REQUESTED state
- Shows renter name and reputation
- Live countdown timer (seconds remaining before auto-decline fires)
- Accept / Decline buttons with loading states

**`mobile/components/booking/CancelConfirmation.tsx`**
- Modal confirmation before cancellation
- Computes and displays the applicable fee tier from `scheduledStart`
  (same logic as PRD §18 / `cancellation.go`) without a server round-trip
- "Keep booking" / "Cancel booking" split-button layout

### Modified files

**`mobile/app/(tabs)/(feed)/index.tsx`**
- Each listing card in the feed now has a "Rent Now" `Pressable` button below it
- Button navigates to `booking-request.tsx` with listing params

**`mobile/app/(tabs)/(feed)/listing/[id]/checkout.tsx`**
- Post-booking redirect changed from `listing/[id]/confirmation` (static screen)
  to `(tabs)/(feed)/booking-status` (live screen with Pusher)

---

## Architecture Decisions

- **`Booking` type lives in `useBooking.ts`** — acts as the shared domain type
  for all booking-related components. Other hooks (`useBookings`) re-export from
  it for a single source of truth.
- **`usePusher` creates one connection per hook invocation** — simple but
  slightly wasteful for screens that subscribe to multiple channels. Acceptable
  given Phase 3 has one channel per booking screen. A singleton connection pool
  can be introduced in Phase 4 if needed.
- **No Zustand store for booking state** — TanStack Query is the server state
  owner; Pusher invalidation flows through the query cache. No duplication.
- **`confirmation.tsx` retained but bypassed** — the static confirmation screen
  is still present for backward compatibility with any deep link that might land
  there. It will be cleaned up in a later consolidation pass.

---

## Pending Work (for later tasks)

- **`/api/v1/pusher/auth` endpoint** (backend): needed for private channel auth.
  Until added, `usePusher` silently fails on private channels; `pull-to-refresh`
  provides the fallback real-time experience.
- **`check-in.tsx` and `check-out.tsx` screens** (task 3.6): `booking-status.tsx`
  already links to these paths; they will be implemented in the next task.
- **Expo push token registration** (`expo-notifications`): installed but not
  wired. Token registration should call `POST /api/v1/notifications/register-token`
  at app startup (task 3.7 or as a cross-cutting concern).
- **`BookingCard` in bookings list**: `useRenterBookings` / `useHostBookings` and
  `BookingCard` are ready; a "My Bookings" list screen was not in the 3.5 scope
  but the primitives are available for task 3.7 or a profile tab addition.

---

## Verification

```
cd mobile && npx tsc --noEmit → exit 0 (no type errors)
```

Manual testing checklist:
- Feed shows "Rent Now" button below each listing card ✓
- Tap "Rent Now" → booking-request.tsx renders with duration picker ✓
- Listing checkout.tsx → post-booking lands on booking-status.tsx ✓
- booking-status.tsx shows REQUESTED state with host accept/decline card ✓
- Cancel button opens CancelConfirmation modal with fee notice ✓
- Pull-to-refresh updates booking status ✓
