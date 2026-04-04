# Task 1.5 — Listing Creation Flow (RN)

## Status: COMPLETE

**Commit:** 268851f
**Branch:** task-1.5-listing-creation-flow
**Branching mode:** git fallback (Graphite unavailable)

---

## What Was Done

### New files

**`mobile/lib/hooks/useGyroscope.ts`**
- `useGyroscope(updateIntervalMs)` — subscribes to `expo-sensors` `DeviceMotion`, converts alpha/beta/gamma to yaw/pitch/roll in degrees, returns `Orientation` object updated at the given interval.
- `angularDistance(a, b)` — Euclidean distance in (roll, pitch, yaw) space with yaw wrap-around correction. Used by the camera to determine if the current pose is sufficiently different from prior captures.

**`mobile/components/camera/AngleEnforcedCamera.tsx`**
- Uses `react-native-vision-camera` for camera access and photo capture.
- Calls `useGyroscope(100)` to track current orientation.
- Compares current orientation against all prior `captures` using `angularDistance`; if any prior capture is within 30°, sets `tooClose = true`.
- Indicator pill at top of frame: green ("New angle — ready to capture") or orange ("Rotate device ≥30° for variety").
- Shutter border also turns orange when too close; text warning shown below the shutter.
- Shutter is not hard-disabled when too close (soft block only) — user can still capture, just warned.
- "Continue" button appears after first capture; pressing it calls `onDone` to advance to the form step.
- Props: `captures`, `onCapture`, `onDone`, `maxPhotos` (default 6).
- Camera permission handling: shows permission prompt if not granted.

**`mobile/components/listing/ListingForm.tsx`**
- `react-hook-form` + `zodResolver` with Zod schema.
- Fields: `title` (min 3), `description` (min 10), `pricePerDay` (required positive number), `pricePerHour` (optional positive number), `maxDuration` (pill selector), `lat`/`lng` (decimal input).
- Duration selector: 4 options (1d / 2d / 3d / 7d). Selected option highlighted in sky-600. Drives `maxDuration` field.
- Exports `ListingFormData` type consumed by `create-listing.tsx`.
- Uses `Input` and `Button` from `components/ui/`.

**`mobile/app/(tabs)/(profile)/create-listing.tsx`**
- Two-step flow: `step === "camera"` renders `AngleEnforcedCamera`; `step === "form"` renders `ListingForm`.
- On form submit:
  1. Uploads each captured photo as multipart to `POST /api/v1/media/upload` with orientation JSON. Collects `mediaId[]`.
  2. Creates listing via `POST /api/v1/listings`.
  3. Attaches media via `POST /api/v1/listings/:id/media`.
  4. Navigates back on success.
- API error mapping: 400 → invalid data, 413 → photo too large, else → generic.

**`mobile/app/(tabs)/(profile)/_layout.tsx`**
- Stack layout for the `(profile)` route group.
- `index` screen: `headerShown: false`.
- `create-listing` screen: `title: "Create Listing"` (native header with back button).

### Modified files

**`mobile/app/(tabs)/(profile)/index.tsx`**
- Added "Create Listing" `Pressable` that navigates to `/(tabs)/(profile)/create-listing`.
- Displays user name and email from `useAuthStore`.
- Added Sign Out button.

**`mobile/package.json`** — added `react-native-vision-camera` and `expo-sensors`.

---

## API Integration

| Step | Endpoint | Notes |
|------|----------|-------|
| Upload photo | `POST /api/v1/media/upload` | Multipart: `image` file + `orientation` JSON string |
| Create listing | `POST /api/v1/listings` | JSON body matching ListingService model |
| Attach media | `POST /api/v1/listings/:id/media` | `{ mediaIds: string[] }` |

---

## Dependencies Added

| Package | Reason |
|---------|--------|
| `react-native-vision-camera` | Camera access and photo capture (no gallery, camera-only per plan) |
| `expo-sensors` | DeviceMotion for fused orientation (roll/pitch/yaw) |

---

## Verification

- `npx tsc --noEmit` — PASS (zero errors)

---

## Notes for Next Task (1.6 — Profile screen RN)

- `useAuthStore` exposes `user` with `name`, `email` (and other fields from `User` type)
- `api` client auto-attaches Bearer tokens
- `GET /api/v1/users/me/listings` is live — paginated with `?page=1&limit=20`
- `GET /api/v1/listings/:id` returns listing + media[]
- The `(profile)` Stack layout is in place — `index` is the profile screen, additional screens can be added as `Stack.Screen` entries in `_layout.tsx`
