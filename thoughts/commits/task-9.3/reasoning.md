# Task 9.3 Commit Reasoning

## Why these changes

The listing E2E flows (view detail + create listing) required fixing real bugs discovered during Maestro testing:

1. **API response mismatch** — The `CreateListingScreen` assumed flat responses (`{ id }`) but the backend wraps them (`{ listing: { id } }`). This caused the listing creation to appear broken even though the backend was working correctly. The media-attach call went to `/api/v1/listings/undefined/media`.

2. **Keyboard dismiss issue** — iOS decimal-pad keyboards have no Done button, so `hideKeyboard` doesn't work. The YAML flow tapped "Location.*" to dismiss, but that text was too close to the lat input and the keyboard stayed up, causing the longitude value to be typed into the lat field.

3. **Overlay z-index** — `AIAutofillOverlay` uses `absolute inset-0 z-10`, covering the "Continue without AI suggestions" button beneath it. Fixed by positioning the button at `z-20`.

## What the prior WIP session did

The WIP commit (10783a9) from the prior session added the camera bypass, testIDs, route refactor, and YAML rewrites. It hit the 200-turn limit before running Maestro. This session resumed from verification, ran the flows, found and fixed the 3 bugs above.
