# Commit 00c487a — Booking flow (RN)

## Why this commit

Task 3.5 adds the React Native booking flow that connects the renter and host
on both sides of the booking lifecycle. The backend BookingService (task 3.1)
exposes all the endpoints; this commit wires the mobile UI to those endpoints.

## Key decisions

### Two booking entry points
The plan calls for a "Rent Now" button on the feed card linking to a quick
booking request screen (`booking-request.tsx`), separate from the full
checkout.tsx used via the listing detail page. This preserves the existing
date-selection checkout flow while adding a direct fast path from the feed.

### booking-status.tsx replaces confirmation.tsx as the post-booking screen
The static `confirmation.tsx` showed a one-time success message with no live
data. `booking-status.tsx` polls the booking endpoint and subscribes to Pusher,
so both renter and host see real-time transitions (REQUESTED → ACCEPTED, etc.).
`checkout.tsx` was updated to redirect there directly. `confirmation.tsx` is
retained for now but is no longer on the primary booking path.

### usePusher uses require() with a lazy pattern
`pusher-js/react-native` uses `require()` in its implementation and has
non-standard module internals. The hook captures the constructor via
`require('pusher-js/react-native').default` to avoid TypeScript module
resolution issues while keeping the hook synchronous. TypeScript is satisfied
via typed constructors; the `any` types are scoped to the constructor call only.

### Private channel auth wired but endpoint is pending
`usePusher` configures `channelAuthorization` pointing to
`/api/v1/pusher/auth`. That endpoint is not yet implemented in the backend.
Until it is, private channels will fail to authenticate silently — the UI
degrades gracefully (no real-time updates, but polling via pull-to-refresh
still works). The auth endpoint should be added in a backend task alongside
or before task 3.7.

### Auto-decline countdown in IncomingRequest
The IncomingRequest component uses a `setInterval` to count down the seconds
remaining before the booking auto-declines. It derives the deadline from
`booking.createdAt + autoDeclineSeconds` (default 2 hours, matching the
backend's `AutoDeclineTimeoutH` config). No server round-trip needed.
