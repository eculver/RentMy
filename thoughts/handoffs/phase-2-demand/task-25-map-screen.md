# Task 2.5 — Map Screen (RN) Handoff

## Summary

Implemented the Map screen tab for the RentMy mobile app. The screen renders an interactive map (react-native-maps with Google Maps provider) centered on the user's current location, fetches listings within the visible viewport, and displays them as price-pill markers. Tapping a marker opens a bottom preview card; tapping the card navigates to the listing detail screen.

## Changes

### New Files

- `mobile/components/map/ListingMarker.tsx` — Marker component rendering a price pill (e.g. `$25/day`). Pill turns sky-blue when selected.
- `mobile/components/map/ListingPreviewCard.tsx` — Absolute-positioned bottom card shown when a marker is tapped. Shows thumbnail, title, price, drive time, and host reputation. Tapping navigates to `/listing/:id`; X button dismisses.
- `mobile/components/map/MapCallout.tsx` — Lightweight inline callout component (unused by default but exported for future use).

### Modified Files

- `mobile/app/(tabs)/(map)/index.tsx` — Full map screen implementation:
  - Centers on user location via `useLocation` hook
  - Converts `Region` to `MapBounds` for the API
  - Debounces region change events (500ms) with `use-debounce` to limit API calls while panning
  - Maintains selected listing state; dismisses on map background tap
- `mobile/lib/hooks/useDiscovery.ts` — Added `MapBounds` interface and `useMapListings(bounds)` query hook calling `GET /api/v1/discovery/map`
- `mobile/app.json` — Added `react-native-maps` plugin config and Google Maps API key placeholder
- `mobile/package.json` / `mobile/package-lock.json` — Added `react-native-maps` dependency

## Dependency Added

`react-native-maps` — Required for `MapView`, `Marker`, and `Callout` components. Standard Expo-compatible map library.

## Verification

- `cd mobile && npx tsc --noEmit` — passes clean

## Branching Mode

Vanilla git fallback (Graphite unavailable).

## Commit

`2321ffb`
