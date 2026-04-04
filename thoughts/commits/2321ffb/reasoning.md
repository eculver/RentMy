# Commit 2321ffb — Reasoning

## Why this commit

Task 2.5 requires a Map screen that lets users visually explore nearby listings on a map. The map-based discovery UX complements the Feed (scroll) and Search screens by giving a spatial view of available items.

## Key design decisions

1. **Debounced region changes (500ms)** — Panning fires many events per second. Debouncing prevents API flood while still feeling responsive.

2. **Bounds-based query (`useMapListings`)** — Rather than sending user lat/lng, we send the visible viewport corners so the backend can return only listings actually visible on screen.

3. **Price-pill markers over callouts** — Airbnb-style price pills are more scannable than standard pin markers. The `ListingMarker` component uses `StyleSheet` (not NativeWind) because `react-native-maps` custom marker children don't support NativeWind className resolution.

4. **Bottom preview card instead of inline callout** — A full-width bottom card gives enough room to show thumbnail + metadata without the map feeling cluttered. The `MapCallout.tsx` component is included as an alternative if the product direction changes.

5. **`tracksViewChanges={false}`** — Prevents markers from re-rendering on every map frame, which is a known performance issue with custom marker views on Android.
