# Commit 5152587 — Task 9.2: Discovery E2E Flows

## What
Get all 3 discovery E2E flows passing: browse-feed, search-listings, map-view.

## Why
Phase 9 requires comprehensive E2E coverage. Discovery is the core user experience — feed browsing, search with filters, and map exploration. These flows test the full stack: API endpoints, location services, data rendering, and navigation.

## Key decisions
- **Apple Maps over Google Maps:** Switched MapView from `PROVIDER_GOOGLE` to default (Apple Maps) because Google Maps SDK wasn't configured and doesn't work in simulator without API keys. Apple Maps works natively.
- **Native back button on listing detail:** Replaced custom absolute-positioned back button with a native Stack navigator header. Maestro couldn't reliably tap absolute-positioned elements, and the native back button provides better UX anyway.
- **Coordinate taps for BottomSheet:** `@gorhom/bottom-sheet` renders outside the React Native accessibility tree, making elements invisible to Maestro. Used coordinate-based taps as the only reliable approach.
- **Seed script with listing creation:** Extended setup.sh to create keyword-rich listings via the API and activate them via SQL through `docker exec`. This ensures search tests ("camera", "tent") have matching data.
- **Keyboard.dismiss() on filter open:** Fixed keyboard covering the filter sheet by programmatically dismissing it when the filter button is tapped.
