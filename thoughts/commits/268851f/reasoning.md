# Commit 268851f — Task 1.5 Listing Creation Flow (RN)

## Why this commit

Task 1.5 delivers the mobile listing creation flow — the core host-side action that allows a user to photograph their item and list it for rent. This was blocked on 1.3 (ListingService API) and 1.4 (auth screens).

## Key decisions

### DeviceMotion over raw Gyroscope
`expo-sensors` provides both `Gyroscope` and `DeviceMotion`. `DeviceMotion` gives fused orientation (alpha/beta/gamma = yaw/pitch/roll) that accounts for gravity, whereas raw gyroscope gives angular velocity. Orientation angles are what we need to enforce the "rotate 30° between shots" rule, so DeviceMotion was the right choice.

### Euclidean angular distance for angle enforcement
Comparing orientations as 3D Euclidean distance in (roll, pitch, yaw) space, with yaw wrap-around correction. This gives a single scalar "how different is this angle from any prior capture" without needing to reason about quaternions.

### Soft block (warn, don't hard-block shutter)
The plan says "soft-blocks shutter if <30deg from any existing photo". The implementation warns (orange indicator + text) but does not disable the shutter button. This preserves the user's ability to capture in edge cases (e.g., flat table item, no good rotation possible) while strongly nudging them toward variety.

### Two-step flow: camera → form
The create-listing screen manages a `step` state. Camera comes first (so the user has photos before filling in metadata), then the form. This mirrors the PRD's intent that photos are the primary signal for the AppraisalAgent downstream.

### Manual lat/lng input
A proper location picker (maps) is deferred. The form exposes raw lat/lng fields for MVP. This is consistent with the phase plan's description and keeps scope contained.

### Stack layout for (profile) group
Previously the profile tab had no stack layout, so `router.push("/(tabs)/(profile)/create-listing")` would not work correctly. Adding `_layout.tsx` with a Stack gives the profile tab its own navigation stack independent of the tab bar.
