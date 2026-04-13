# Task 9.2 — E2E: Discovery Flows (Feed, Search, Map)

**Status:** Completed  
**Branch:** task-9.2-discovery-flows  
**Commit:** 3c82a28

---

## What Was Done

### New Maestro flows (`mobile/e2e/flows/discovery/`)

| File | Scenario |
|------|----------|
| `browse-feed.yaml` | Login → feed loads → card visible → scroll → pull-to-refresh → tap card → detail → back |
| `search-listings.yaml` | Login → search tab → type query → results appear → clear → filter sheet → apply 15-min drive filter |
| `map-view.yaml` | Login → map tab → setLocation → assert map-container visible (marker tap skipped; documented below) |

### testID additions

| Component | testIDs added |
|-----------|--------------|
| `mobile/app/(tabs)/(search)/index.tsx` | `screen-search`, `search-input`, `btn-filter`, `search-results` |
| `mobile/components/screens/MapScreen.native.tsx` | `screen-map`, `map-container` |
| `mobile/components/map/ListingPreviewCard.tsx` | `map-callout` |
| `mobile/components/search/FilterSheet.tsx` | `filter-sheet`, `btn-apply-filters`, `btn-reset-filters` |
| `mobile/app/(tabs)/(feed)/listing/[id].tsx` | `screen-listing-detail`, `btn-back` |

---

## Design Decisions

### Map marker tap skipped
`react-native-maps` `<Marker>` renders as a native iOS MapKit element outside React Native's accessibility tree. Maestro cannot reliably tap it. The test asserts that `map-container` (MapView itself) is visible, which confirms the map rendered and the location resolved. The `map-callout` testID is still wired into `ListingPreviewCard` for future test coverage if a workaround becomes available.

### Tab navigation via text label
Consistent with the pattern established in task 9.1: `tapOn: "Search"` and `tapOn: "Map"` use the tab bar label text. `tabBarTestID` is not a supported prop in Expo Router's `<Tabs>`.

### `clearText` for search reset
Maestro's `clearText` with an element selector clears the TextInput, which fires `onChangeText("")` → debounce → `setQuery("")`. This puts the search screen back to the idle state (no FlatList) cleanly without simulating backspace keystrokes.

### Pull-to-refresh target
`swipeDown` with `element: id: "feed-list"` targets the FlatList's RefreshControl. Without the element selector, `swipeDown` would drag from screen center and might miss the FlatList.

---

## Verification

```bash
# TypeScript clean (0 errors)
cd mobile && ./node_modules/.bin/tsc --noEmit

# All 91 Jest tests pass
cd mobile && ./node_modules/.bin/jest

# Discovery flows (requires simulator + backend + seed data)
cd mobile && maestro test e2e/flows/discovery/
make test-mobile-e2e-discovery
```

## Known Limitations

- **Map marker tap not automated**: Native MapKit markers cannot be driven by Maestro. Manual verification required. See `map-view.yaml` inline comment for steps.
- **Live environment required**: All discovery flows need iOS Simulator with installed dev-client build, backend running (`cd backend && make dev`), and Phase 8.0 seeded data (alice@test.com owns 5 listings).

## Next Tasks (can run in parallel after 9.2)

- **9.3** — Listing detail & creation (depends on 9.0 only; can start now)
- **9.4** — Profile & referral flows (depends on 9.0 only; can start now)
- **9.5** — Booking flows (depends on 9.1 + 9.3)
