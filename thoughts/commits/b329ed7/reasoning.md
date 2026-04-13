# Commit b329ed7 — feat: add Maestro E2E listing flows (task 9.3)

## What changed

### New E2E flows
- `mobile/e2e/flows/listing/view-listing-detail.yaml` — Logs in as renter, taps a listing card from the feed, asserts detail screen loads with "Rent Now" CTA, scrolls, navigates back.
- `mobile/e2e/flows/listing/create-listing.yaml` — Logs in as host, navigates to Profile → Create Listing, uses the E2E camera bypass to provide a fixture photo, fills the listing form, submits, and asserts navigation back to Profile.

### E2E camera bypass (`AngleEnforcedCamera.native.tsx`)
`react-native-vision-camera` cannot be automated by Maestro — it renders native hardware UI outside React Native's accessibility tree. The bypass:
- Checks `process.env.EXPO_PUBLIC_E2E_MODE === "true"` at module load time.
- Renders a fully testable UI (`camera-e2e-bypass`, `btn-e2e-use-fixture`, `btn-e2e-continue`) instead of the real camera.
- Calls `onCapture` with a sentinel path (`e2e://fixture-photo.jpg`) when the fixture button is tapped.

### Upload skip in CreateListingScreen
`handleFormSubmit` now skips file uploads for captures whose path equals `E2E_FIXTURE_PATH`. The sentinel check is a one-line guard — no other logic is changed. The listing is still created via the real API; it just has no attached media in E2E mode, which the backend allows.

### testIDs added
| Component | testIDs |
|-----------|---------|
| `ListingForm.tsx` | `input-listing-title`, `input-listing-description`, `input-listing-price-per-day`, `input-listing-lat`, `input-listing-lng`, `btn-create-listing` |
| `ProfileScreen` (index.tsx) | `btn-create-listing-nav` |
| `CreateListingScreen.native.tsx` | `screen-create-listing` (camera step + form step), `error-create-listing` |
| `listing/[id].tsx` | `btn-rent-now` |

### E2E_FIXTURE_PATH export
Exported from both `AngleEnforcedCamera.native.tsx` (runtime) and `AngleEnforcedCamera.tsx` (TypeScript resolution target) so `CreateListingScreen` can import without a platform-specific path.

## Why this approach

The plan (phase-9-mobile-e2e.md step 9.3) explicitly calls for an `E2E_MODE` camera bypass rather than trying to drive the native camera view. This is the industry-standard pattern for E2E testing of camera-gated flows. The sentinel path approach keeps the bypass isolated to a single guard in `handleFormSubmit` and avoids any mock/stub infrastructure.
