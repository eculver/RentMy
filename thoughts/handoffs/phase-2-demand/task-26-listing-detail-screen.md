# Task 2.6 — Listing detail screen (RN)

## Status: COMPLETE

**Commit:** 53fcce0
**Branch:** task-2.6-listing-detail-screen
**Branching mode:** git fallback (Graphite unavailable)

---

## What Was Done

### New files

**`mobile/lib/hooks/useListing.ts`**
- `ListingDetail` interface — mirrors the backend `listing.Listing` Go struct with TypeScript types
- `useListing(id)` — TanStack Query hook calling `GET /api/v1/listings/:id`, enabled only when id is non-null

**`mobile/components/listing/PhotoGallery.tsx`**
- Horizontal `FlatList`, pagingEnabled, full-width photos
- Pagination dots overlay at the bottom (only rendered when >1 photo)
- Active dot is larger and fully white; inactive dots are translucent
- Graceful empty state: shows placeholder icon when `photos` array is empty
- `onPress` handler forwarded to each photo/placeholder for optional tap action

**`mobile/components/listing/HostInfoCard.tsx`**
- Initial-letter avatar in sky-100 circle (no external avatar service needed in Phase 2)
- Star rating computed from 0–1000 reputation score via `score / 200`
- Blue shield-checkmark badge for hosts with reputation ≥ 500
- Member since date formatted from listing `createdAt` (Phase 3 can swap to a real `user.createdAt` from a host profile endpoint when one exists)

**`mobile/components/listing/HoldExplainer.tsx`**
- Amber card showing hold amount prominently (`Temporary hold: $XX`)
- Body text explains the hold as an authorization, not a charge
- Conditional RentMy Protection row when `guaranteeGap > 0`
- All amounts formatted as dollars from cents via `cents / 100`

**`mobile/components/listing/AvailabilityCalendar.tsx`**
- Parses the raw `availability` JSON field from the listing API
- Empty/null array → "Available anytime — contact host to arrange pickup" green card
- Non-empty array of `{ start, end }` time slots → list with clock icons and locale-formatted date ranges

**`mobile/app/(tabs)/(feed)/listing/[id].tsx`**
- Route: `/listing/[id]` (Expo Router dynamic segment; group folders `(tabs)/(feed)` are invisible in URL)
- Accepts URL params: `id`, `hostName`, `hostReputation`, `thumbnailUrl`, `driveTimeMin` — all optional except `id`
- Back button overlaid on photo gallery (absolute-positioned, visible over any photo)
- Scroll view with `paddingBottom: 120` to clear the fixed CTA bar
- Fetches `useListing(id)` and `useHoldEstimate(id)` in parallel
- Detects host vs. renter by comparing `listing.hostId === user?.id` from Zustand auth store
  - Host sees "Edit Listing" grey button (tap handler stubbed — wired in Phase 3+)
  - Renter sees "Rent Now" sky-blue button → navigates to `/listing/[id]/checkout` (Task 2.7)
- HoldExplainer only rendered for renters (hidden from the host who owns the listing)
- Error state with "Go Back" CTA if listing fetch fails or 404

### Modified files

**`mobile/app/(tabs)/(feed)/index.tsx`**
- Changed `router.push('/listing/${item.id}')` to pass `params: { id, hostName, hostReputation, thumbnailUrl, driveTimeMin }` so the detail screen can show host info without a separate API call

**`mobile/app/(tabs)/(search)/index.tsx`**
- Same navigation update as feed

**`mobile/components/map/ListingPreviewCard.tsx`**
- Same navigation update; map preview card now passes all host params on tap

---

## Architecture Decisions

- **Host info via route params, not a separate API call:** There is no public `GET /api/v1/users/:id` host profile endpoint in Phase 2. The discovery API already includes `hostName` and `hostReputation` in `RankedListing`. Passing these as route params avoids an extra round-trip and keeps the screen responsive. When Phase 3+ adds a host profile endpoint, `HostInfoCard` can be upgraded to fetch live data.
- **`thumbnailUrl` as first (and only) gallery photo in Phase 2:** The backend listing API (`GET /api/v1/listings/:id`) returns a `Listing` struct with no embedded media URLs. Media is managed separately via the MediaService. Passing `thumbnailUrl` from the discovery response gives the detail screen a photo without needing a media list endpoint. The `PhotoGallery` component accepts `string[]`, so adding multi-photo support later only requires passing a richer array.
- **`memberSince` from listing `createdAt` vs. user `createdAt`:** The HostInfoCard shows "Member since" using the listing's `createdAt` as a proxy (the listing was created after the user). This is slightly inaccurate but acceptable for Phase 2. Phase 3 can pass the correct value once a host profile endpoint exists.
- **No calendar library dependency:** An external calendar library (react-native-calendars, etc.) would add ~200KB to the bundle for a feature that is read-only in Phase 2. Instead, `AvailabilityCalendar` parses the raw JSON time slots and formats them as a simple list. A visual calendar can replace this in Phase 5 when availability editing is needed.
- **Hold explainer gated on `isHost`:** Showing a hold amount to the listing owner would be confusing ("why would I hold myself?"). The `HoldExplainer` is hidden for the host; they see the Edit Listing CTA instead.

---

## Verification

```
cd mobile && npx tsc --noEmit  → exit 0 (no errors)
```

---

## Notes for Next Tasks

- **Task 2.7 (Checkout screen, RN):** The "Rent Now" button navigates to `/listing/[id]/checkout`. Create `mobile/app/(tabs)/(feed)/listing/[id]/checkout.tsx`. The checkout screen should read `id` from params, use `useHoldEstimate(id)` for the hold display, and call `POST /api/v1/payments/setup` → `POST /api/v1/bookings`.
- **Host profile endpoint (Phase 3+):** Add `GET /api/v1/users/:id/public` returning `{ name, reputationScore, createdAt, responseRate }`. Update `HostInfoCard` to accept a `hostId` and fetch the profile directly.
- **Multi-photo gallery (Phase 3+):** Add `GET /api/v1/listings/:id/media` returning `[{ id, url, mimeType }]`. Pass the URL array to `PhotoGallery` in the detail screen.
- **Edit listing navigation (Phase 3):** Wire the "Edit Listing" button to navigate to the listing edit form. The create-listing flow in `mobile/app/(tabs)/(profile)/create-listing.tsx` can be adapted.
