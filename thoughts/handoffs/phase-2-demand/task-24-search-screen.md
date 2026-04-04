# Task 2.4 — Search Screen (RN)

## Status: COMPLETE

**Commit:** dc0755a
**Branch:** task-2.4-search-screen
**Branching mode:** git fallback (Graphite unavailable)

---

## What Was Done

### New files

**`mobile/lib/stores/searchStore.ts`**
- Zustand store holding `query: string` and `filters: SearchFilters`
- Actions: `setQuery`, `setFilters`, `resetFilters`
- Persists search state across navigations within the session (e.g. backing out and returning to search)

**`mobile/components/search/FilterSheet.tsx`**
- `@gorhom/bottom-sheet` `BottomSheet` component exposed via `forwardRef<BottomSheet>`
- Local state mirrors current `filters` prop on open so changes are not applied until "Apply filters" is tapped
- Drive time: 4 pill buttons (5 / 15 / 30 / 60 min), toggleable — tap same value to deselect
- Price range: two `TextInput` fields (min / max), decimal-pad keyboard, empty = no filter
- "Apply filters": builds `SearchFilters` from local state, calls `onApply`, closes sheet
- "Reset all": clears local state, calls `onReset`, closes sheet
- `BottomSheetBackdrop` with `disappearsOnIndex={-1}` for dismiss-on-tap-outside behavior
- `enablePanDownToClose` for swipe-to-dismiss

### Modified files

**`mobile/app/(tabs)/(search)/index.tsx`** (replaced placeholder)
- Search bar: `TextInput` with `Ionicons` search icon, debounced 300ms via `useDebouncedCallback` from `use-debounce`
- Filter button: badge showing active filter count when > 0, tinted blue with count pill
- Results: `FlatList` with `ListingFeedCard` (same component as feed) and infinite scroll (`onEndReached` threshold 0.4)
- Three states: idle (no query) → `EmptyIdle`, loading → `ActivityIndicator`, empty results → `EmptyResults`
- `EmptyIdle`: "Search for anything nearby" — shown when query is blank
- `EmptyResults`: `No results for "{query}"` — shown when search returns 0 items
- Navigation: `router.push('/listing/${item.id}')` on card tap (connects to listing detail, task 2.6)

**`mobile/app/_layout.tsx`**
- Wrapped app in `GestureHandlerRootView` from `react-native-gesture-handler`
- Required by `@gorhom/bottom-sheet` v5 for gesture recognition. Wraps `QueryClientProvider`.

### Dependencies added

| Package | Version | Reason |
|---------|---------|--------|
| `@gorhom/bottom-sheet` | ^5.2.8 | Filter sheet UI |
| `react-native-gesture-handler` | ~2.28.0 | Required peer dep for bottom-sheet gestures |
| `use-debounce` | ^10.1.1 | 300ms debounce on search input |

---

## Architecture Decisions

- **`forwardRef` for FilterSheet:** The search screen needs to call `filterSheetRef.current?.expand()` and `close()` imperatively. `forwardRef` forwards the `BottomSheet` ref directly so the parent controls open/close state without lifting sheet visibility into React state (avoids re-render of the whole screen on open).
- **Local state in FilterSheet, not store:** The filter sheet has pending (unapplied) state while open. Keeping it local prevents the search query from firing on every slider/toggle change before the user taps "Apply". Only on apply does the store get updated, triggering a single TanStack Query refetch.
- **Debounced query vs. store query:** `useDebouncedCallback` updates the store after 300ms idle. The `useSearch` hook is `enabled` only when `query.length > 0`. This means no API calls fire for empty input and the user sees the idle state immediately when clearing the search bar.
- **`GestureHandlerRootView` at root:** Placed at the outermost layout so it wraps all screens, including auth screens. This is the recommended placement and avoids issues with nested gesture handlers in modals.
- **Reuse `ListingFeedCard`:** No duplicate card component. Search results and feed share the same card for visual consistency.

---

## Verification

```
cd mobile && npx tsc --noEmit  → exit 0 (no errors)
```

---

## Notes for Next Tasks

- **Task 2.5 (Map screen):** `useMapListings` and `MapBounds` are already defined in `useDiscovery.ts`. The `_layout.tsx` already has `GestureHandlerRootView`, so map gestures will work out of the box.
- **Task 2.6 (Listing detail):** `router.push('/listing/${item.id}')` is wired in both the feed card (task 2.3) and search results (this task). The route `mobile/app/(tabs)/(feed)/listing/[id].tsx` needs to be created.
- **Filter expansion:** `SearchFilters` in `useDiscovery.ts` currently has `maxDriveMin`, `minPrice`, `maxPrice`. A `durationType` filter (hourly/daily) can be added to the store and `FilterSheet` when needed without changing the backend query — the `useSearch` hook can include it as an additional `searchParams` field once the backend supports it.
