// Base module for TypeScript resolution. At runtime, Metro selects
// AngleEnforcedCamera.native.tsx or AngleEnforcedCamera.web.tsx
// based on the target platform.
export { default, type CapturedPhoto } from "./AngleEnforcedCamera.web";

// Sentinel path used in E2E mode — signals that no real file upload should occur.
// Exported here so non-native consumers (tests, CreateListingScreen) can reference it
// without a platform-specific import.
export const E2E_FIXTURE_PATH = "e2e://fixture-photo.jpg";
