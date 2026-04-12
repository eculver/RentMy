# Handoff: Task 8.6 — Fix: Search + Map Bugs

**Status:** Completed  
**Branch:** `task-8.6-fix-search-map-bugs`  
**Commit:** `5cbb9a8`  
**Date:** 2026-04-11  
**Mode:** Graphite (gt)

---

## What Was Done

Implemented fixes for all 5 bugs documented in the 8.5 audit (`thoughts/audits/phase-8-visual-qa/audit-search-map.md`).

---

## Fixes Implemented

### B1 — MAJOR: Map markers now appear on initial load

**File:** `mobile/components/screens/MapScreen.native.tsx`

Added `useEffect` that initializes `bounds` from `lat`/`lng` when location is first available and `bounds` is still `null`. Previously, `bounds` could only be set via `onRegionChangeComplete`, which `react-native-maps` does not fire for the initial `initialRegion` render.

```typescript
useEffect(() => {
  if (lat !== null && lng !== null && bounds === null) {
    setBounds(regionToBounds({
      latitude: lat, longitude: lng,
      latitudeDelta: INITIAL_DELTA, longitudeDelta: INITIAL_DELTA,
    }));
  }
}, [lat, lng]);
```

### B2 — MEDIUM: Map error state has Retry button

**File:** `mobile/components/screens/MapScreen.native.tsx`

Destructured `retry` from `useLocation()`. Added `Pressable` Retry button in the error state, matching the pattern from `(feed)/index.tsx`.

### B3 — MEDIUM: Search no longer shows "No results" during GPS acquire

**File:** `mobile/app/(tabs)/(search)/index.tsx`

Destructured `loading: locationLoading` from `useLocation()`. Added a `locationLoading || lat === null || lng === null` guard before the `isLoading` check in the results section, showing a spinner + "Waiting for location…" message instead of the empty-results component.

### B4 — MINOR: Search TextInput is now controlled

**File:** `mobile/app/(tabs)/(search)/index.tsx`

Added local `inputValue` state, bound `value={inputValue}` to the TextInput. `handleInputChange` updates both local state and the debounced store setter. The field now reflects the stored query on navigation back.

### B5 — MINOR: Pagination cursor aligned with DB sort order

**Files:** `backend/internal/discovery/handler.go`, `mobile/lib/hooks/useDiscovery.ts`

- Backend: Added `minID(listings)` helper that returns the minimum ULID among returned listings (= last in `ORDER BY id DESC`). The `feed` and `search` handlers now include `nextCursor` in the JSON response.
- Frontend: `FeedResponse` interface gets `nextCursor?: string`. Both `useFeed` and `useSearch` use `lastPage.nextCursor` as the next page cursor, falling back to the last item's ID for backwards compatibility.

---

## Files Changed

| File | Change |
|------|--------|
| `mobile/components/screens/MapScreen.native.tsx` | B1 (bounds init useEffect), B2 (Retry button), added `Pressable`+`useEffect` imports |
| `mobile/app/(tabs)/(search)/index.tsx` | B3 (location guard), B4 (controlled input), added `useState` |
| `mobile/lib/hooks/useDiscovery.ts` | B5 (nextCursor in FeedResponse, updated getNextPageParam) |
| `backend/internal/discovery/handler.go` | B5 (minID helper, nextCursor in feed + search responses) |
| `thoughts/commits/5cbb9a8/reasoning.md` | Commit reasoning |

---

## Verification

All verification commands passed:
- `go vet ./...` — clean
- `go build -o /dev/null ./cmd/server` — clean
- `npx tsc --noEmit` — only pre-existing profile screen errors (unchanged)
- `npx jest` — 91/91 tests pass
- Discovery unit tests — all pass

---

## Pre-existing TypeScript Errors (Not Introduced by This Task)

`app/(tabs)/(profile)/index.tsx` has 2 TS2345 errors from router path types. These were present before this task (from a previous session) and are out of scope.

---

## Next Task

**8.7 — Audit: Booking + Handoff** — visually inspect booking and handoff screens.
