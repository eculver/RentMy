# Commit Reasoning: 5cbb9a8

**Task:** 8.6 — Fix: Search + Map Bugs  
**Branch:** `task-8.6-fix-search-map-bugs`  
**Date:** 2026-04-11

## Why these changes

Five bugs were documented in the 8.5 audit (static code analysis). This commit implements all five fixes.

### B1 — MapScreen bounds initialization (MAJOR)

`react-native-maps` does not fire `onRegionChangeComplete` for the initial `initialRegion` render. The component relied on that callback to set `bounds`, which in turn enabled the `useMapListings` query. Result: blank map on first open until user pans.

Fix: `useEffect` that sets `bounds` from `lat`/`lng` when they first become available and `bounds` is still null. This runs once after GPS acquires position and triggers the initial marker fetch.

### B2 — Map error Retry button (MEDIUM)

`useLocation` added `retry: () => void` in task 8.4. The feed screen was updated to use it, but `MapScreen.native.tsx` was missed. Users with a transient GPS error had no recovery path.

Fix: destructure `retry` from `useLocation()`, render a `Pressable` Retry button in the location error state — same pattern as `(feed)/index.tsx`.

### B3 — Search "No results" flash (MEDIUM)

TanStack Query v5 changed semantics: `isLoading === false` when `enabled === false` (because `fetchStatus === 'idle'`). The search screen didn't check `locationLoading`, so if a user typed a query before GPS was ready, the query was disabled but `isLoading` was false → the FlatList rendered with empty data → `<EmptyResults>` showed "No results for 'X'".

Fix: add a `locationLoading || lat === null || lng === null` guard before the `isLoading` check, showing a spinner + "Waiting for location…" message instead.

### B4 — Uncontrolled TextInput (MINOR)

The search input had `onChangeText` but no `value` prop. The debounced store setter updated `query` in Zustand, but this was never fed back to the visible input. On navigation back, the field appeared empty while results rendered for the stored query.

Fix: local `inputValue` state initialized from `query`, bound via `value={inputValue}`. A `handleInputChange` callback updates both local state and the debounced store setter.

### B5 — Pagination cursor misalignment (MINOR)

The DB queries use `ORDER BY l.id DESC` with cursor `id < $4`. After fetching, the service re-ranks results by `rankScore` before returning to the handler. The frontend used `last item in ranked list` as the next cursor.

Example of the bug: DB returns IDs [100, 90, 80, 70, 60] (desc). After ranking: [90, 60, 100, 70, 80]. Frontend cursor = 80. Next page: `id < 80` → returns 70 and 60 again (duplicates), missing nothing new.

The correct cursor is the minimum ID from the returned set (= last in DB-desc order). Fix: added `minID(listings)` helper in `handler.go` that scans the enriched result for the smallest ID. Backend now returns `nextCursor` in the JSON. Frontend uses `lastPage.nextCursor` with fallback to old behavior for compatibility.
