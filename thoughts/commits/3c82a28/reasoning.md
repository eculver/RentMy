# Commit 3c82a28 — feat: add Maestro E2E discovery flows (task 9.2)

## What changed

Three Maestro flow YAMLs under `mobile/e2e/flows/discovery/`:
- `browse-feed.yaml` — login → feed loads → card visible → scroll → pull-to-refresh → tap card → detail → back
- `search-listings.yaml` — login → search tab → type query → results → clear → filter sheet → apply
- `map-view.yaml` — login → map tab → setLocation → map-container visible (marker tap skipped; see note)

testID additions across five component files:
- `screen-search`, `search-input`, `btn-filter`, `search-results` — search screen
- `screen-map`, `map-container` — map screen (native)
- `map-callout` — ListingPreviewCard (map overlay card)
- `filter-sheet`, `btn-apply-filters`, `btn-reset-filters` — FilterSheet
- `screen-listing-detail`, `btn-back` — listing detail screen

## Why

Task 9.2 requires E2E coverage of the three discovery paths. The testIDs are the hooks Maestro uses to locate and assert on elements. Without them, flows can only use text-based taps (fragile to copy changes).

## Key decisions

**Map marker tap omitted**: react-native-maps `Marker` renders as a native iOS MapKit element outside React Native's accessibility tree. Maestro cannot tap it reliably. The map test asserts `map-container` is visible (confirms MapView rendered and location resolved). The `map-callout` testID is still added to `ListingPreviewCard` so future gray-box tests can assert on it once a workaround is available.

**`clearText` on search-input**: Used to clear the debounced search query. This directly mutates the TextInput value, which triggers `onChangeText` → debounce → `setQuery("")` → idle state. Simpler than backspacing character by character.

**Pull-to-refresh via `swipeDown` on `feed-list`**: Maestro's `swipeDown` with an element target triggers the RefreshControl on the FlatList without swiping the whole screen.
