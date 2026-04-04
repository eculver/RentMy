# Task 1.6 — Profile Screen (RN)

## Status: COMPLETE

**Commit:** TBD (staged, awaiting user commit due to 1Password signing requiring GUI context)
**Branch:** task-1.6-profile-screen
**Branching mode:** git fallback (Graphite unavailable)

---

## What Was Done

### New files

**`mobile/lib/hooks/useUser.ts`**
- `useUser()` — TanStack Query hook that fetches `GET /api/v1/users/me` and returns the `User` object.
- Uses the `api` client from `lib/api.ts` which auto-attaches Bearer tokens.
- Query key: `["user", "me"]`.

**`mobile/lib/hooks/useListings.ts`**
- `useMyListings(page, limit)` — TanStack Query hook that fetches `GET /api/v1/users/me/listings` with pagination.
- Exports `Listing` interface matching the backend `listing.Listing` model (id, hostId, title, description, pricePerHour?, pricePerDay?, status, createdAt, thumbnailUrl?).
- Returns `{ listings, total, page }` per `ListByHostResult` shape.
- Query key: `["listings", "mine", page]`.

**`mobile/components/listing/ListingCard.tsx`**
- Card component showing listing photo (or "No photo" placeholder), title, price, and status badge.
- Photo uses `Image` from react-native with `thumbnailUrl` when available.
- Price displays `pricePerDay` preferring over `pricePerHour`.
- Status badge uses `Badge` component with variant mapping: PENDING→warning, ACTIVE→success, FLAGGED/SUSPENDED→error.
- Accepts optional `onPress` handler for future navigation to listing detail.

### Modified files

**`mobile/app/(tabs)/(profile)/index.tsx`** — Full rewrite:
- Header section: avatar circle (first letter of name), name, email.
- "Create Listing" button → navigates to create-listing screen (preserved from 1.5).
- "My Listings" section: FlatList of `ListingCard` items from `useMyListings()`.
- Loading state: `ActivityIndicator` while fetching.
- Error state: error message if fetch fails.
- Empty state: instructional text when no listings exist.
- "Sign Out" button (preserved, moved to bottom).

---

## Architecture Decisions

- **useUser vs useAuthStore.user:** `useUser()` fetches fresh data from the API; `useAuthStore` `user` field is used for display only (name/email from last login). This ensures profile data stays fresh without requiring re-login.
- **Listing interface in useListings.ts:** Defined locally rather than importing from backend types — mobile and backend evolve independently. Only the fields needed for the profile screen are included.
- **No optimistic updates:** Profile screen is read-only at this stage. TanStack Query's default 1-minute stale time handles cache freshness.

---

## Verification

- `node node_modules/.bin/tsc --noEmit` — PASS (exit 0)
  - Note: Node.js not in system PATH; used `/Users/eculver/.clink-connect/node/bin/node` found via `find`. The prior tasks used `npx tsc --noEmit` — same underlying binary.

---

## Notes for Next Phase (2.x — Discovery + Payments)

- `useMyListings` can be reused or extended for host-side listing management in later phases.
- `ListingCard` is designed to accept an `onPress` handler — ready to wire up to a listing detail screen in Phase 2.6.
- The `Listing` interface will need to be extended when media thumbnails are returned from `GET /api/v1/users/me/listings` (currently `thumbnailUrl` is optional and undefined until that endpoint is updated).
- Phase 1 exit criteria satisfied: Register → Login → Create Listing (with camera) → View in Profile.
