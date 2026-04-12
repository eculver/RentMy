# Audit: Search + Map Screens
**Task:** 8.5  
**Date:** 2026-04-11  
**Method:** Static code analysis of search and map screens, components, hooks, and backend service  

---

## Scope

**Screens audited:**
- `mobile/app/(tabs)/(search)/index.tsx`
- `mobile/app/(tabs)/(map)/index.tsx` (delegates to `MapScreen`)

**Components audited:**
- `mobile/components/screens/MapScreen.native.tsx`
- `mobile/components/search/FilterSheet.tsx`
- `mobile/components/map/ListingMarker.native.tsx`
- `mobile/components/map/ListingPreviewCard.tsx`
- `mobile/components/map/MapCallout.native.tsx`

**Hooks/stores audited:**
- `mobile/lib/hooks/useDiscovery.ts` (`useSearch`, `useMapListings`)
- `mobile/lib/stores/searchStore.ts`

**Backend audited:**
- `backend/internal/discovery/handler.go`
- `backend/internal/discovery/service.go`
- `backend/internal/discovery/repository.go`

---

## Findings

### B1 — MAJOR: Map shows no markers on initial load

**File:** `mobile/components/screens/MapScreen.native.tsx`  
**Lines:** 24, 31–33

`bounds` is initialized to `null`:
```typescript
const [bounds, setBounds] = useState<MapBounds | null>(null);
```
`useMapListings(null)` is disabled (`enabled: bounds !== null`), so no API call is made until bounds are set.

`onRegionChangeComplete` is the only setter for bounds, but `react-native-maps` does **not** fire `onRegionChangeComplete` for the initial `initialRegion` render — only after user interaction (pan or pinch). The map renders the `initialRegion` but fires no callback, leaving `bounds` null indefinitely until the user physically moves the map.

**Impact:** Users see a completely empty map on every first open. Markers never appear unless the user pans or zooms. This makes the map feature appear broken.

**Fix:** Initialize bounds from lat/lng on mount:
```typescript
useEffect(() => {
  if (lat !== null && lng !== null && bounds === null) {
    setBounds(regionToBounds({
      latitude: lat,
      longitude: lng,
      latitudeDelta: INITIAL_DELTA,
      longitudeDelta: INITIAL_DELTA,
    }));
  }
}, [lat, lng]); // eslint-disable-line react-hooks/exhaustive-deps
```

---

### B2 — MEDIUM: Map location error state missing Retry button

**File:** `mobile/components/screens/MapScreen.native.tsx`  
**Lines:** 52–63

The error state renders a static message but no retry action:
```typescript
if (locationError || lat === null || lng === null) {
  return (
    <View ...>
      <Text>Location unavailable</Text>
      <Text>{locationError ?? "Enable location access to browse the map."}</Text>
    </View>
  );
}
```

`useLocation` now exposes `retry: () => void` (added in task 8.4). The feed screen was updated to show a Retry button, but the map screen was missed.

**Impact:** Users with a temporary location error (GPS timeout, brief permission glitch) have no recovery path — they must close and reopen the app.

**Fix:** Destructure `retry` from `useLocation()` and add a Retry `Pressable` below the error text, matching the pattern used in `(feed)/index.tsx`.

---

### B3 — MEDIUM: Search screen shows "No results" while location is still loading

**File:** `mobile/app/(tabs)/(search)/index.tsx`  
**Lines:** 57, 65–73, 140–177

The search screen reads `lat` and `lng` from `useLocation()` but does not check the `loading` state. In TanStack Query v5, when `enabled: false`, `isLoading === false` (because `fetchStatus === 'idle'`). So:

1. User opens search screen before location has been acquired
2. User types a query
3. `lat` and `lng` are `null` → `useSearch` is disabled (`enabled: false`)
4. `isLoading === false`, `listings === []`
5. Screen falls into the FlatList branch → renders `<EmptyResults query="..." />`
6. User sees "No results for 'bike'" even though no search was ever run

**Impact:** Misleading UX — the user believes there are no results when the real issue is that location hasn't loaded yet.

**Fix:** Add a location guard before showing search results:
```typescript
const { lat, lng, loading: locationLoading } = useLocation();
// ...
// In the render:
} else if (locationLoading || (lat === null || lng === null)) {
  return (
    <View className="flex-1 items-center justify-center px-8">
      <Text className="text-sm text-gray-500 text-center">
        Waiting for location…
      </Text>
    </View>
  );
}
```

---

### B4 — MINOR: Search TextInput is uncontrolled (no `value` prop)

**File:** `mobile/app/(tabs)/(search)/index.tsx`  
**Lines:** 107–115

```typescript
<TextInput
  onChangeText={debouncedSetQuery}
  placeholder="Search listings…"
  // ← no value prop
/>
```

The input is uncontrolled. The `query` from `useSearchStore` is used for API calls but is never fed back into the input's visible text. If the screen mounts while `query` already has a value (e.g., hot reload, navigation back), the input shows empty while results render for the stale stored query.

**Impact:** Inconsistent visual state — results shown for a query the user can't see.

**Fix:** Introduce a local `inputValue` state, bind `value={inputValue}`, and update both the local state and the debounced store setter on change:
```typescript
const [inputValue, setInputValue] = useState(query);
const handleChange = (text: string) => {
  setInputValue(text);
  debouncedSetQuery(text);
};
```

---

### B5 — MINOR: Pagination cursor is derived from re-ranked results (not DB order)

**Files:** `mobile/lib/hooks/useDiscovery.ts` (lines 100–104, 62–65), `backend/internal/discovery/service.go` (lines 81, 170–176)

`useSearch` and `useFeed` both use cursor-based pagination:
```typescript
getNextPageParam: (lastPage) => {
  if (!lastPage.listings || lastPage.listings.length === 0) return undefined;
  return lastPage.listings[lastPage.listings.length - 1].id;
},
```

The backend DB query uses `AND ($4 = '' OR l.id < $4) ORDER BY l.id DESC` for pagination. However, the service re-sorts results by `rankScore` before returning them to the client. The cursor sent to the next page is the last listing's ID in rank order — not in DB `id` order. This means:

- Page 1 returns 20 listings sorted by rank. Cursor = id of 20th-ranked listing.
- Page 2 queries `l.id < cursor` (DB order), which may skip listings with IDs between the cursor and the DB-ordered last item that simply ranked higher.

**Impact:** Listing gaps in infinite scroll. Some listings may never appear in paginated results.

**Note:** This affects both feed and search. It is a design limitation in the current keyset pagination strategy, not introduced in this phase.

---

## Summary

| ID | Severity | Screen/File | Description |
|----|----------|-------------|-------------|
| B1 | MAJOR | MapScreen.native.tsx | No markers on initial load — bounds never initialized |
| B2 | MEDIUM | MapScreen.native.tsx | Map error state missing Retry button |
| B3 | MEDIUM | (search)/index.tsx | Shows "No results" while location is loading |
| B4 | MINOR | (search)/index.tsx | Search TextInput is uncontrolled |
| B5 | MINOR | useDiscovery.ts + service.go | Pagination cursor derived from re-ranked results |

**Total bugs:** 5  
**Blockers:** 0  
**Majors:** 1 (B1)  
**Mediums:** 2 (B2, B3)  
**Minors:** 2 (B4, B5)

---

## What's Working Well

- `FilterSheet` correctly manages local state and syncs with parent on apply/reset
- Drive-time filter is correctly applied in the service layer after enrich
- `useSearch` and `useFeed` correctly disable when `lat/lng` are null (no spurious API calls)
- Marker deselect on map tap (`onPress={() => setSelectedListing(null)}`) works correctly
- `fuzzLocation` is deterministic per listing ID — markers don't jump on re-render
- `ListingPreviewCard` and `ListingFeedCard` both use the same price/reputation display logic
- Web stubs (`MapScreen.web.tsx`, `ListingMarker.web.tsx`, `MapCallout.web.tsx`) correctly prevent Metro from bundling native-only code
- `MapCallout.native.tsx` is implemented and available if the preview card approach needs to be swapped
- Backend map bounding-box parameter order (`ST_MakeEnvelope(swLng, swLat, neLng, neLat)`) is correct

---

## Next Task

**8.6 — Fix: Search + Map Bugs** — implement fixes for B1–B5 above.
