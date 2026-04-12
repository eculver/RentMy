// Base module for TypeScript resolution. At runtime, Metro selects
// AngleEnforcedCamera.native.tsx or AngleEnforcedCamera.web.tsx
// based on the target platform.
export { default, type CapturedPhoto } from "./AngleEnforcedCamera.web";
