# Commit dce24cd — Feed screen (task 2.3)

## Why

Task 2.3 is the first renter-facing screen in Phase 2. The DiscoveryService backend (task 2.1) is live; this commit wires the mobile UI to it so renters can see ranked nearby listings.

## What changed and why each decision was made

**`useLocation` + `locationStore`:** expo-location needs a permission request on first mount. Caching the result in Zustand means navigating away and back doesn't re-prompt GPS. The hook returns `{ lat, lng, loading, error }` so the feed screen can gate rendering on location readiness without prop drilling.

**`useDiscovery`:** All four discovery hooks (`useFeed`, `useSearch`, `useMapListings`, `useHoldEstimate`) live in one file so tasks 2.4–2.6 can import from a single source without duplication. TanStack Query v5 requires `InfiniteData<FeedResponse>` as the third generic for `useInfiniteQuery` to avoid a type error when accessing `.pages`.

**`ListingFeedCard`:** Full-width card (not the existing `ListingCard` row style) because feed browsing calls for a photo-forward layout. Reputation is stored 0–1000 (PRD §8) so `score / 200` converts to a 0–5 star display. Verified-host badge at `reputation >= 500` (top half of the scale).

**Feed screen skeleton:** Three skeleton cards on initial load instead of a full-screen spinner reduce perceived latency. `isLoading` (not `isFetching`) guards them so they only appear once, not on every pagination fetch.
