# Task 9.3 — E2E: Listing Detail & Creation

**Status:** Completed  
**Branch:** task-9.3-listing-e2e  
**Commit:** b329ed7

---

## What Was Done

### New Maestro flows (`mobile/e2e/flows/listing/`)

| File | Scenario |
|------|----------|
| `view-listing-detail.yaml` | Login as renter → feed → tap listing card → detail screen loads → Rent Now button visible → scroll → back navigation → feed visible again |
| `create-listing.yaml` | Login as host → Profile tab → tap Create Listing → E2E camera bypass → use fixture photo → continue → fill form (title/description/price/lat/lng) → submit → back to profile |

### testID additions

| Component | testIDs added |
|-----------|--------------|
| `mobile/components/listing/ListingForm.tsx` | `input-listing-title`, `input-listing-description`, `input-listing-price-per-day`, `input-listing-lat`, `input-listing-lng`, `btn-create-listing` |
| `mobile/app/(tabs)/(profile)/index.tsx` | `btn-create-listing-nav` |
| `mobile/components/screens/CreateListingScreen.native.tsx` | `screen-create-listing` (both camera and form steps), `error-create-listing` |
| `mobile/app/(tabs)/(feed)/listing/[id].tsx` | `btn-rent-now` |
| `mobile/components/camera/AngleEnforcedCamera.native.tsx` | `camera-e2e-bypass`, `btn-e2e-use-fixture`, `btn-e2e-continue` |

### E2E Camera Bypass

`AngleEnforcedCamera.native.tsx` checks `process.env.EXPO_PUBLIC_E2E_MODE === "true"` at module load. When true, it renders a bypass UI instead of the real camera. The bypass provides a sentinel path (`e2e://fixture-photo.jpg`) via `onCapture`, then `onDone` is triggered by the Continue button.

`CreateListingScreen.native.tsx` skips the `api.post("api/v1/media/upload")` call for any capture whose path equals `E2E_FIXTURE_PATH`. The listing is still created via the real backend API without attached photos (backend allows this).

**Build requirement:** The E2E create-listing flow requires the app to be built with `EXPO_PUBLIC_E2E_MODE=true`:
```bash
EXPO_PUBLIC_E2E_MODE=true npx expo run:ios
```

The `E2E_FIXTURE_PATH` constant is also exported from `AngleEnforcedCamera.tsx` (TypeScript resolution target) so TypeScript resolves the import without errors.

---

## Design Decisions

### Sentinel path over bundled asset
The plan mentioned "pre-uploaded test fixture photos from e2e/fixtures/". Rather than bundling a real JPEG (requires `expo-asset` or `expo-file-system`, neither installed), the bypass uses a sentinel URI (`e2e://fixture-photo.jpg`). The upload loop in `CreateListingScreen` skips any capture with this path. This achieves the same end-to-end behavior (listing created, appraisal triggered) without any new dependencies.

### Post-submission assertion targets Profile screen
The create-listing flow asserts `screen-profile` after submitting because `CreateListingScreen` navigates `router.back()` after the appraisal resolves. The `timeout: 20000` accounts for the real AI appraisal round-trip (backend River job). If the appraisal is still slow in CI, increase this timeout.

### Tab navigation via text label
Consistent with tasks 9.1 and 9.2 — `tapOn: "Profile"` uses the tab bar label text. `tabBarTestID` is not a supported prop in Expo Router's `<Tabs>`.

### `btn-rent-now` only on renter view
The Rent Now button is only rendered when `isHost === false`. The view-detail test logs in as a renter (bob@test.com) who does not own the seeded listings, so this assertion is always valid.

---

## Verification

```bash
# TypeScript clean (0 errors)
cd mobile && ./node_modules/.bin/tsc --noEmit

# All 91 Jest tests pass
cd mobile && ./node_modules/.bin/jest

# Listing flows (requires simulator with E2E_MODE build + backend + seed data)
cd mobile && maestro test e2e/flows/listing/
make test-mobile-e2e-listing
```

## Branching Mode
Graphite mode — used `gt create` and will use `gt submit`.

## Known Limitations

- **create-listing.yaml requires E2E_MODE build**: The flow will get stuck at the camera step if the app is built without `EXPO_PUBLIC_E2E_MODE=true`. Add this to the CI build step when setting up task 9.10.
- **AI appraisal timeout**: The 20-second timeout for `screen-profile` after form submission covers the full appraisal round-trip. If the backend is under load in CI, this may need adjustment.
- **No photos on created listing**: In E2E mode, listings are created without media. This is acceptable for flow testing; it does not affect the appraisal or booking flow tests downstream.

## Next Tasks

- **9.4** — Profile & referral flows (depends on 9.0 only; can start now)
- **9.5** — Booking flows (depends on 9.1 + 9.3; this task unblocks it)
