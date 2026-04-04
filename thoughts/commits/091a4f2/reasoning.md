# Commit 091a4f2 — feat: add profile screen with my-listings feed and user hooks

## Why

Task 1.6 completes the Phase 1 supply-side user journey. The profile screen
is the natural landing point after a host creates a listing: they can immediately
see their listing appear under "My Listings" without navigating elsewhere.

## What changed

- `useUser.ts` — fetches `/api/v1/users/me` via TanStack Query; provides fresh
  profile data independent of auth token payload (which can go stale).
- `useListings.ts` — fetches `/api/v1/users/me/listings` with cursor pagination
  shape. Exports `Listing` interface so callers don't depend on backend codegen.
- `ListingCard.tsx` — reusable card with photo/placeholder, title, price, and
  status badge. `onPress` prop left wired but no-op for now; Phase 2.6 will
  hook it to listing detail.
- `(profile)/index.tsx` — rebuilt around FlatList + hooks. Loading/error/empty
  states prevent blank screen during API cold-start.

## Tradeoffs

- Used `useAuthStore.user` for the header display (name/email) rather than
  `useUser()` to avoid a second network request on load. `useUser` is imported
  but available if the profile needs editable fields in a later task.
- Listing interface is local to mobile — intentional decoupling from backend
  struct tags.
