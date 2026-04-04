# Task 2.3 — Feed Screen (RN)

## Status: COMPLETE

**Branch:** task-2.3-feed-screen
**Branching mode:** git fallback (Graphite unavailable)

---

## What Was Done

### New files

**`mobile/lib/stores/locationStore.ts`**
- Zustand store holding `lat`, `lng`, and `setLocation` action
- Cached position persists across re-renders so `useLocation` skips the permission request if we already have coords

**`mobile/lib/hooks/useLocation.ts`**
- Requests `Accuracy.Balanced` foreground permission via `expo-location`
- Calls `getCurrentPositionAsync` on first mount; sets position in `locationStore`
- Returns `{ lat, lng, loading, error }` — error is set on denied permission or GPS failure
- Uses cancellation flag to avoid state updates after unmount

**`mobile/lib/hooks/useDiscovery.ts`**
- `useFeed(lat, lng)` — `useInfiniteQuery` calling `GET /api/v1/discovery/feed`; cursor = last listing ID in each page
- `useSearch(query, lat, lng, filters)` — `useInfiniteQuery` calling `GET /api/v1/discovery/search`; disabled when query is empty
- `useMapListings(bounds)` — `useQuery` calling `GET /api/v1/discovery/map` with bounding box
- `useHoldEstimate(listingId)` — `useQuery` calling `GET /api/v1/listings/:id/hold-estimate`
- All typed against `RankedListing` (mirrors `discovery.RankedListing` JSON response from backend)
- TanStack Query v5 infinite queries typed with `InfiniteData<FeedResponse>` as the third generic to satisfy `.pages` access

**`mobile/components/listing/ListingFeedCard.tsx`**
- Full-width card: thumbnail image (h-44), title, price badge, drive time with car icon, host name + star rating
- `reputationLabel`: converts 0–1000 PRD reputation score to 0–5 display stars
- "Verified host" badge shown when `hostReputation >= 500`
- Graceful no-photo fallback with `image-outline` icon
- `onPress` callback for navigation

**`mobile/app/(tabs)/(feed)/index.tsx`** (replaced placeholder)
- Renders location loading state → skeleton cards → `FlatList` with `ListingFeedCard` items
- Pull-to-refresh via `RefreshControl`
- Infinite scroll: `onEndReached` (threshold 0.4) calls `fetchNextPage` when `hasNextPage`
- Empty state with emoji illustration and descriptive copy
- Spinner footer during next-page fetch
- Navigation: taps call `router.push('/listing/{id}')` (listing detail screen, implemented in task 2.6)

### Modified files

**`mobile/package.json`** — added `expo-location` (SDK 54 compatible version installed by `npx expo install`)

---

## Architecture Decisions

- **locationStore for caching:** Storing the last known position in Zustand means navigating away and back to the feed doesn't re-request GPS every time. The `useLocation` hook checks the store before calling `requestForegroundPermissionsAsync`.
- **Skeleton instead of spinner:** Three skeleton cards give a better perceived-performance feel than a full-screen spinner. Only shown on initial load (`isLoading`), not on refresh.
- **Cursor = last listing ID:** The backend uses ULID cursor pagination. `getNextPageParam` returns the `id` of the last item in the page, which the backend interprets as "return items with id < cursor, ordered descending." Empty page means no next page.
- **`InfiniteData<FeedResponse>` generic:** TanStack Query v5 changed the default `TData` for `useInfiniteQuery` to `InfiniteData<TQueryFnData>`. Explicitly specifying it as the third generic avoids the implicit `data.pages` type error without using `as`.

---

## Verification

```
cd mobile && npx tsc --noEmit  → exit 0 (no errors)
```

---

## Notes for Next Tasks

- **Task 2.4 (Search screen):** `useSearch` and `useDiscovery`'s `SearchFilters` type are already defined in `useDiscovery.ts`. The search screen imports them directly.
- **Task 2.5 (Map screen):** `useMapListings` and `MapBounds` type are ready in `useDiscovery.ts`. `RankedListing.lat/lng` are already the fuzzed coordinates.
- **Task 2.6 (Listing detail):** `router.push('/listing/${id}')` is wired in the feed card. The route `mobile/app/(tabs)/(feed)/listing/[id].tsx` is expected. `useHoldEstimate` is in `useDiscovery.ts` ready to be called from the detail screen.
- **Location permission on Android:** `expo-location` requires `ACCESS_FINE_LOCATION` permission in `app.json` `plugins` array when building a native binary. Dev client uses EAS-managed permissions.
