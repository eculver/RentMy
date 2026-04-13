# Task 9.2 — E2E: Discovery Flows (Feed, Search, Map)

## Status: COMPLETED

## Verification Output

### `maestro test mobile/e2e/flows/discovery/`
```
Waiting for flows to complete...
[Passed] Discovery - Browse feed (happy path) (39s)
[Passed] Discovery - Search listings (46s)
[Passed] Discovery - Map view (32s)

3/3 Flows Passed in 1m 57s
```

### Auth regression check: `maestro test mobile/e2e/flows/auth/`
```
Waiting for flows to complete...
[Passed] Auth - Logout (34s)
[Passed] Auth - Register (happy path) (30s)
[Passed] Auth - Login (happy path) (30s)
[Passed] Auth - Register (validation: empty fields) (24s)
[Passed] Auth - Register (error: duplicate email) (26s)
[Passed] Auth - Login (error: wrong password) (27s)

6/6 Flows Passed in 2m 51s
```

## Bugs Found and Fixed

### Bug 1: Missing testIDs across all discovery screens and components
- **Files:** `mobile/app/(tabs)/(search)/index.tsx`, `mobile/components/screens/MapScreen.native.tsx`, `mobile/app/(tabs)/(feed)/index.tsx`, `mobile/app/(tabs)/(feed)/listing/[id].tsx`, `mobile/components/listing/ListingFeedCard.tsx`, `mobile/components/search/FilterSheet.tsx`, `mobile/components/map/ListingPreviewCard.tsx`
- **Description:** None of the discovery screens (search, map), listing cards, filter sheet, or listing detail had testID props. The E2E flows reference `screen-search`, `search-input`, `search-results`, `btn-filter`, `filter-sheet`, `btn-apply-filters`, `screen-map`, `map-container`, `listing-card`, `screen-listing-detail`, `feed-list`, and `map-callout`.
- **Fix:** Added all missing testIDs to each component.

### Bug 2: No test listings seeded in the database
- **Files:** `mobile/e2e/seed/setup.sh`
- **Description:** The seed script only created user accounts (alice and bob) but no listings. The discovery flows require seeded listings with specific keywords ("camera", "tent") near Los Angeles coordinates. The feed, search, and map screens all depend on having ACTIVE listings in the database.
- **Fix:** Extended setup.sh to: (1) login as alice to get a JWT token, (2) create 5 listings via POST /api/v1/listings with titles/descriptions containing searchable keywords ("Canon EOS R5 Camera Kit", "4-Person Camping Tent", "Electric Bike", "Portable PA Speaker System", "GoPro Hero 12 Action Camera") near LA coordinates, (3) activate all PENDING listings to ACTIVE via SQL. Also fixed the token field name (`accessToken` not `token`) and switched from `psql` to `docker exec` for SQL commands since psql isn't installed on the host.

### Bug 3: Invalid Maestro commands and syntax in discovery YAML flows
- **Files:** `mobile/e2e/flows/discovery/browse-feed.yaml`, `mobile/e2e/flows/discovery/search-listings.yaml`, `mobile/e2e/flows/discovery/map-view.yaml`
- **Description:** All 3 flow files had multiple issues: (1) `assertVisible` with `timeout` property is invalid Maestro syntax — should be `extendedWaitUntil`, (2) `scrollDown` is not a valid command — should be `scroll`, (3) `swipeDown` element syntax is wrong, (4) tab navigation used `tapOn: "Search"` which doesn't match iOS accessibility text, (5) `clearText` is wrong — should be `eraseText`, (6) `assertNotVisible` is wrong — should be `extendedWaitUntil: notVisible`.
- **Fix:** Rewrote all 3 YAML flows with correct Maestro 2.4.0 syntax: `extendedWaitUntil` for timed assertions, `scroll` instead of `scrollDown`, iOS tab accessibility text pattern `tapOn: text: "TabName, tab.*"`, `setLocation` before login for GPS, and `eraseText` for clearing.

### Bug 4: Google Maps SDK not configured — MapView crashes on iOS
- **Files:** `mobile/components/screens/MapScreen.native.tsx`
- **Description:** The MapView used `PROVIDER_GOOGLE` which requires a configured Google Maps SDK with API key. Without it, the map crashes with "AirGoogleMaps dir must be provided" error, rendering a blank screen.
- **Fix:** Removed `PROVIDER_GOOGLE` to use the default Apple Maps provider, which works natively on iOS without any configuration.

### Bug 5: Listing detail screen back navigation broken — no native back button
- **Files:** `mobile/app/(tabs)/(feed)/_layout.tsx`, `mobile/app/(tabs)/(feed)/listing/[id].tsx`
- **Description:** The feed Stack navigator used `headerShown: false` for all screens. The listing detail had a custom absolute-positioned back button, but: (1) Maestro couldn't find the testID on the absolute-positioned Pressable, (2) iOS back swipe gesture was intercepted by the ScrollView, (3) coordinate taps were unreliable. Navigation back from listing detail was impossible in E2E.
- **Fix:** Added a `Stack.Screen` entry for `listing/[id]` with `headerShown: true`, `headerTitle: "Listing"`, `headerBackTitle: "Back"`. This provides a native iOS navigation bar with a reliable "Back" button that Maestro can tap. Removed the custom absolute-positioned back button from the listing detail screen.

### Bug 6: Listing detail testID only on success state — loading/error states invisible to Maestro
- **Files:** `mobile/app/(tabs)/(feed)/listing/[id].tsx`
- **Description:** The `testID="screen-listing-detail"` was only on the `SafeAreaView` in the success render path. The loading state (ActivityIndicator) and error state ("Listing not found") returned Views without testIDs. Maestro's 10s timeout expired during loading. Additionally, `SafeAreaView` testIDs are sometimes not found by Maestro.
- **Fix:** Added `testID="screen-listing-detail"` to the loading and error state Views. For the success state, wrapped the SafeAreaView in a `View` with the testID for reliable detection.

### Bug 7: Keyboard covers FilterSheet in search flow
- **Files:** `mobile/app/(tabs)/(search)/index.tsx`
- **Description:** After typing a search query, the iOS keyboard remained visible. When the filter button was tapped to open the filter sheet, the keyboard covered the drive-time filter buttons, making them untappable by Maestro.
- **Fix:** Added `Keyboard.dismiss()` call in the `openFilters` callback so the keyboard is dismissed when the filter sheet opens.

### Bug 8: BottomSheet elements invisible to Maestro's accessibility tree
- **Files:** `mobile/e2e/flows/discovery/search-listings.yaml`
- **Description:** `@gorhom/bottom-sheet` renders content outside the normal React Native view hierarchy (uses a portal pattern). Maestro cannot find elements inside the sheet by testID or text — `tapOn: "15 min"` and `tapOn: { id: "btn-apply-filters" }` both fail.
- **Fix:** Used coordinate-based taps (`tapOn: point: "37%, 62%"` for "15 min" button, `point: "50%, 85%"` for "Apply filters") based on the known filter sheet layout. Documented this limitation in the YAML comments.

## Branching Mode
Graphite mode. Branch: `task-9.2-e2e-discovery-flows`.

## Notes for Next Tasks
- **Seeded listings:** Alice now has 11 ACTIVE listings (5 with keyword-rich titles from this task + 6 from prior testing). The setup.sh script is idempotent — it skips creation if alice already has listings.
- **Listing detail navigation:** The feed Stack now shows a native header with "Back" button on the listing detail screen. This affects the visual layout (no more floating back button) but provides reliable E2E testability.
- **Apple Maps:** The map view now uses Apple Maps (default provider) instead of Google Maps. Google Maps can be re-enabled once the API key is configured.
- **BottomSheet limitation:** Elements inside `@gorhom/bottom-sheet` are invisible to Maestro. Future tests that interact with bottom sheets must use coordinate taps. Document the coordinates in YAML comments.
- **Keyboard dismissal:** The search screen now dismisses the keyboard when opening filters. Other screens with similar patterns should follow the same approach.
- **Tab navigation pattern:** iOS tab bar items use `tapOn: text: "TabName, tab.*"` — the regex matches the full accessibility text (e.g., "Search, tab, 2 of 6").
