# Task 4.6 — Wire KYC into Booking Flow (RN) Handoff

## Summary

Integrated Stripe Identity verification into the mobile booking flow. New users hitting checkout are now gated behind identity verification. The verification flow uses the native `@stripe/stripe-identity-react-native` SDK (v0.6.0) which presents a full-screen native sheet for document + selfie capture.

## Branching Mode

Git fallback (vanilla git). Branch: `task-4.6-kyc-booking-flow`. Commit: `c95c3ae`.

## New Dependency

| Package | Version | Rationale |
|---------|---------|-----------|
| `@stripe/stripe-identity-react-native` | `^0.6.0` | Native Stripe Identity SDK for iOS/Android. Same vendor as payments (Stripe). No second SDK relationship needed. |

## What Was Built

### `mobile/lib/auth.ts` (modified)

- Added `IdentityStatus` type: `"PENDING" | "VERIFIED" | "REJECTED" | "ESCALATED"`
- Added `identityStatus?: IdentityStatus` to `User` interface
- Added `setIdentityStatus(status)` action that updates both Zustand state and SecureStore so status survives app restarts

### `mobile/lib/hooks/useVerification.ts` (new)

| Export | Purpose |
|--------|---------|
| `useStartVerification()` | Mutation → `POST /api/v1/verification/start` → returns `{sessionId, sessionUrl, ephemeralKeySecret}` |
| `useVerificationStatus(enabled)` | Query → `GET /api/v1/verification/status` — polls every 3s when `enabled: true` |

### `mobile/app/(tabs)/(profile)/verify.tsx` (new)

Full-screen KYC screen. States:

| State | Display |
|-------|---------|
| `idle` | Explains verification requirements, "Start Verification" CTA |
| `starting` | Loading indicator while calling backend + loading Stripe sheet |
| `processing` | Polling backend status every 3s after Stripe sheet completes |
| `verified` | Success screen with "Continue" button (calls `router.back()`) |
| `rejected` | Error screen with "Try Again" button |
| `error` | Generic error (FlowFailed, escalated, network error) with "Try Again" |

Uses `useStripeIdentity(fetchOptions)` where `fetchOptions` returns `{sessionId, ephemeralKeySecret, brandLogo}` from Stripe Identity SDK.

### `mobile/components/verification/KYCGate.tsx` (new)

Render-prop gate that reads `user.identityStatus` from auth store:
- `VERIFIED` → renders `children`
- `REJECTED` → rejection message + "Retry Verification" → navigates to verify screen
- `ESCALATED` → "under review" message
- `PENDING` / undefined → "verification required" prompt + "Verify Identity" → navigates to verify screen

### `mobile/app/(tabs)/(feed)/listing/[id]/checkout.tsx` (modified)

- Removed inline KYC status check (was a stub that cast `user as unknown`)
- Extracted booking form to `CheckoutContent` component
- Wrapped `CheckoutContent` with `<KYCGate>` at the default export level
- Removed the now-unused `identityStatus`/`isVerified` variables

### `mobile/app/(tabs)/(profile)/_layout.tsx` (modified)

Registered `verify` screen in the profile Stack (`headerShown: false` — the screen owns its own header).

### Backend (targeted fix, `internal/agent/verification/`)

| File | Change |
|------|--------|
| `model.go` | `StartVerificationResult`: added `EphemeralKeySecret string json:"ephemeralKeySecret,omitempty"` |
| `service.go` | `StripeSessionResult`: added `EphemeralKeySecret` field; passed through in `StartVerification` return |
| `stripe.go` | Maps `session.ClientSecret` → `StripeSessionResult.EphemeralKeySecret` |

**Why**: The `@stripe/stripe-identity-react-native` SDK v0.6.0 requires `sessionId + ephemeralKeySecret` (not `clientSecret`). The backend previously only returned `sessionId + sessionUrl`. Adding `ephemeralKeySecret` was the minimum backend change to unblock the native SDK.

## KYC Decision Flow (Mobile)

```
User taps "Checkout"
│
└── KYCGate checks user.identityStatus
    ├── VERIFIED  → CheckoutContent rendered normally
    ├── REJECTED  → Rejection screen → "Retry" → verify.tsx
    ├── ESCALATED → Under review screen (no action)
    └── PENDING / nil → "Verify" screen → verify.tsx
          │
          └── verify.tsx
                │
                ├── POST /api/v1/verification/start
                │     └── returns {sessionId, ephemeralKeySecret}
                │
                ├── useStripeIdentity presents native sheet
                │
                └── sheet dismissed
                      ├── FlowCompleted → poll GET /api/v1/verification/status every 3s
                      │     ├── VERIFIED  → setIdentityStatus("VERIFIED") → success screen
                      │     ├── REJECTED  → setIdentityStatus("REJECTED") → rejected screen
                      │     └── ESCALATED → error screen (under review)
                      ├── FlowCanceled → return to idle
                      └── FlowFailed   → error screen
```

## Verification

All passed:
```
cd mobile && npx tsc --noEmit          # no errors
cd backend && go vet ./...              # no issues
cd backend && go build ./cmd/server    # clean build
cd backend && go test ./...            # all packages green
```
