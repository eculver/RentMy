# Task 9.3 — E2E: Listing Detail & Creation

## Status: COMPLETED

## Verification Output

### `maestro test mobile/e2e/flows/listing/`
```
Waiting for flows to complete...
[Passed] Listing - View listing detail (happy path) (39s)
[Passed] Listing - Create listing (__DEV__ camera bypass) (1m 16s)

2/2 Flows Passed in 1m 55s
```

### Auth regression check: `maestro test mobile/e2e/flows/auth/`
```
Waiting for flows to complete...
[Passed] Auth - Logout (35s)
[Passed] Auth - Register (happy path) (30s)
[Passed] Auth - Login (happy path) (29s)
[Passed] Auth - Register (validation: empty fields) (23s)
[Passed] Auth - Register (error: duplicate email) (27s)
[Passed] Auth - Login (error: wrong password) (28s)

6/6 Flows Passed in 2m 52s
```

### Discovery regression check: `maestro test mobile/e2e/flows/discovery/`
```
Waiting for flows to complete...
[Passed] Discovery - Browse feed (happy path) (39s)
[Passed] Discovery - Search listings (47s)
[Passed] Discovery - Map view (33s)

3/3 Flows Passed in 1m 59s
```

## Camera Override Justification

The create-listing flow requires `AngleEnforcedCamera` which needs hardware camera access. The iOS Simulator has no camera hardware, so a `__DEV__`-only bypass was added in `AngleEnforcedCamera.native.tsx`:

- **Gate:** `__DEV__ && !device` — only activates when no camera hardware is detected (simulator). Production builds and real-device dev builds use the real camera.
- **Implementation:** Writes a real 4×4 white JPEG to the cache directory via `expo-file-system`'s `File` class, then passes it through the normal `onCapture` callback. The photo travels through the real media-upload pipeline (FormData upload to `/api/v1/media/upload`).
- **No global switch:** No `E2E_MODE`, `IS_E2E`, or custom env vars. The bypass is scoped to the single component and only active in simulator dev builds.

## Bugs Found and Fixed

### Bug 1: API response wrapper mismatch in CreateListingScreen
- **Files:** `mobile/components/screens/CreateListingScreen.native.tsx`
- **Description:** The frontend expected flat `{ id, title }` responses from `POST /api/v1/listings` and `POST /api/v1/media/upload`, but the backend wraps responses as `{ listing: { id, ... } }` and `{ media: { id, ... } }`. This caused `listing.id` to be `undefined`, and the media-attach call went to `/api/v1/listings/undefined/media` (400 error).
- **Fix:** Updated `MediaResponse` and `ListingResponse` types to match the nested backend response format. Destructure `{ listing }` from the parsed response.

### Bug 2: Lat/lng values concatenated into single field
- **Files:** `mobile/e2e/flows/listing/create-listing.yaml`
- **Description:** After entering latitude, the YAML flow used `tapOn: text: "Location.*"` to dismiss the numeric keyboard. This tap landed too close to the lat input, so the keyboard didn't dismiss and the lat field stayed focused. The subsequent longitude input text was appended to the lat field, causing form validation to fail ("Invalid input").
- **Fix:** Changed keyboard dismiss targets to `tapOn: text: "Listing Details"` — the heading at the top of the form, far from any input field. This reliably dismisses the numeric keyboard.

### Bug 3: "Continue without AI suggestions" button untappable (behind overlay)
- **Files:** `mobile/components/screens/CreateListingScreen.native.tsx`
- **Description:** When appraisal fails, the `AIAutofillOverlay` renders with `absolute inset-0 z-10`, covering the entire screen. The "Continue without AI suggestions" text was rendered below the overlay in the DOM, so taps were intercepted by the overlay and never reached the text. Maestro could see the text (95% opaque white overlay) but couldn't interact with it.
- **Fix:** Moved the continue button to an `absolute bottom-10 z-20` positioned `Pressable`, floating above the overlay. Also switched from `<Text onPress>` to `<Pressable>` for more reliable tap handling.

### Bug 4 (from prior WIP session): Missing testIDs on ListingForm inputs
- **Files:** `mobile/components/listing/ListingForm.tsx`
- **Description:** The listing form inputs (title, description, price, lat, lng) and the submit button had no testIDs.
- **Fix:** Added testIDs: `input-listing-title`, `input-listing-description`, `input-listing-price-per-day`, `input-listing-lat`, `input-listing-lng`, `btn-create-listing`.

### Bug 5 (from prior WIP session): Listing detail refactored to directory route
- **Files:** `mobile/app/(tabs)/(feed)/listing/[id].tsx` → `mobile/app/(tabs)/(feed)/listing/[id]/index.tsx`
- **Description:** The listing detail screen was a single file route. Future checkout routes need to nest under `[id]/`, so the route was refactored to a directory layout with `_layout.tsx` and `index.tsx`.

## Branching Mode
Graphite mode. Branch: `task-9.3-e2e-listing-flows`.

## Notes for Next Tasks
- **Create listing flow creates real listings:** Each E2E run creates a new "E2E Test Power Drill" listing in the database. The seed script's idempotent check is on alice's listing count, but these E2E-created listings accumulate.
- **Appraisal always fails in test env:** No AI service configured, so the appraisal job fails with "no listing photos found" (the media-attach step runs after listing creation, but the appraisal job fires immediately on listing creation). The E2E flow handles this by waiting for and tapping "Continue without AI suggestions".
- **Camera bypass pattern:** The `__DEV__ && !device` pattern in `AngleEnforcedCamera.native.tsx` is reusable for check-in/check-out flows (task 9.6) that also use this camera component.
