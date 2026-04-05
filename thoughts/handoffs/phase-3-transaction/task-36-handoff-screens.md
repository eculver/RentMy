# Task 3.6 — Handoff screens (RN)

## Status: COMPLETE

**Commit:** eb43ac1
**Branch:** task-3.6-handoff-screens
**Branching mode:** git fallback (Graphite unavailable)

---

## What Was Done

### New screens

**`mobile/app/(tabs)/(feed)/check-in.tsx`**
- Check-in screen for both host and renter sides
- Renter flow: GPS verify → enter PIN from host → take ≥3 angle-enforced photos → Complete Check-in
- Host flow: GPS verify → PIN display (shown from `PINDisplay`) → take ≥3 photos → Complete Check-in
- Calls `POST /api/v1/bookings/:id/check-in` with `mediaIds: []` (photo upload via MediaService is a future task)
- On success → navigates back to `booking-status.tsx`

**`mobile/app/(tabs)/(feed)/check-out.tsx`**
- Check-out / return screen for both parties
- GPS verify → take ≥3 angle-enforced return photos → Complete Return
- No PIN required at check-out (GPS + photos only)
- Calls `POST /api/v1/bookings/:id/check-out`

**`mobile/app/(tabs)/(feed)/active-rental.tsx`**
- Active rental dashboard screen shown while booking is in ACTIVE state
- Live countdown timer ticking down to `scheduledEnd`
- Late return warning banner appears when past `scheduledEnd`
- "Navigate to Return" button opens Maps app via `expo-linking`
- "Report Issue" stub button (dispute flow is Phase 5)
- "Back to booking" button navigates to `booking-status.tsx`

### New components

**`mobile/components/handoff/GPSStatus.tsx`**
- Proximity indicator: polls `useLocation` hook, shows distance in meters
- Green checkmark badge when `verified=true`, red X when >100m or error
- "Verify location" button calls `onVerify` callback
- Displays spinner while verifying

**`mobile/components/handoff/PINEntry.tsx`**
- 4-digit PIN entry with one TextInput per digit, auto-advance on entry
- Submit button calls `onSubmit(pin: string)` callback
- Shows loading spinner and success/error states
- Backspace handled via `onKeyPress`

**`mobile/components/handoff/PINDisplay.tsx`**
- Fetches PIN from `GET /api/v1/bookings/:id/proximity` and displays host's CHECK_IN PIN in large monospace digits
- "Resend PIN via SMS" button calls `POST /api/v1/proximity/sms-fallback`
- Refreshes automatically when transactionId changes

**`mobile/components/handoff/PhotoGrid.tsx`**
- Grid display of captured photos with angle indicator badges
- Shows `{n}/{min}` count; green when minimum met
- "Remove" tap removes photo from capture list
- Displays placeholder tiles for missing photos up to minimum

### New hooks

**`mobile/lib/hooks/useProximity.ts`**
- Central state machine for handoff flow: GPS + PIN + photos
- `verifyGPS()` — calls `POST /api/v1/proximity/verify` with current location; sets `gpsVerified`
- `verifyPIN(pin)` — calls `POST /api/v1/proximity/pin`; sets `pinVerified`
- `addPhoto(uri)` / `removePhoto(uri)` — manages `photos[]` array
- `canComplete` — `gpsVerified && (isRenter ? pinVerified : true) && photos.length >= MIN_PHOTOS`
- `proofType` param: `"CHECK_IN"` or `"CHECK_OUT"`

### Modified files

**`mobile/app/(tabs)/(feed)/booking-status.tsx`**
- ACCEPTED state: added "Navigate to Pickup" button (opens Maps with host location via `expo-linking`)
- "Start Check-in" button navigates to `check-in.tsx` with `transactionId`
- ACTIVE state: "Start Check-out" → `check-out.tsx`; "View Rental" → `active-rental.tsx`

---

## Architecture Decisions

- **`useProximity` owns all handoff state** — GPS, PIN, and photo state are co-located since they must all be complete before the CTA enables. No Zustand store needed; this is transient UI state.
- **`AngleEnforcedCamera` reused from Phase 1** — the same gyroscope-based angle enforcement used for listing creation is applied at check-in and check-out. No duplication.
- **Photo upload is deferred** — `mediaIds: []` is sent to the check-in/out endpoints. The server-side `CheckHandoffComplete` currently enforces GPS + PIN only. Photo enforcement is client-side until Phase 4 or 5 adds a photo diff pipeline (task 5.1).
- **`active-rental.tsx` is separate from `booking-status.tsx`** — the active state has unique UX (countdown timer, navigate button, late return warning) that would overload the booking-status screen.

---

## Pending Work (for later tasks)

- **Photo upload to MediaService**: `mediaIds` is currently `[]`. Task 5.1 (photo diff pipeline) will wire actual upload.
- **Dispute flow from "Report Issue"**: stub button in active-rental.tsx navigates nowhere. Phase 5.2 (DisputeAgent) will provide the flow.
- **Pusher auth endpoint** (`/api/v1/pusher/auth`): still missing from backend. `usePusher` silently fails on private channels.

---

## Verification

```
cd mobile && npx tsc --noEmit → exit 0 (no type errors)
```

Manual checklist:
- booking-status ACCEPTED → "Navigate to Pickup" opens Maps ✓
- "Start Check-in" → check-in.tsx renders with GPS/PIN/photo steps ✓
- GPS verify button posts to /api/v1/proximity/verify ✓
- Renter PIN entry: submit calls /api/v1/proximity/pin ✓
- Host PIN display: fetches from /bookings/:id/proximity ✓
- Camera button opens AngleEnforcedCamera, gyroscope enforcement active ✓
- "Complete Check-in" disabled until GPS + PIN + 3 photos satisfied ✓
- booking-status ACTIVE → "Start Check-out" → check-out.tsx ✓
- active-rental.tsx countdown timer ticks correctly ✓
- Late return warning banner appears past scheduledEnd ✓
