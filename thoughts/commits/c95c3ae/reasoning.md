# Commit c95c3ae — Task 4.6: Wire KYC into Booking Flow

## Why this commit

Phase 4 requires all renters to be identity-verified before booking. The VerificationAgent (task 4.2) built the backend KYC flow; this commit wires it into the mobile checkout screen.

## Key decisions

**Native SDK over web URL**: `@stripe/stripe-identity-react-native` launches a native iOS/Android sheet vs. redirecting to `session.URL` (web-hosted flow). The native path is more reliable on mobile (no browser redirect, no deep-link coordination) and provides a better UX.

**KYCGate as a wrapper component**: Rather than duplicating identity checks across every screen that might require verification, a single `KYCGate` component handles all four status paths (`VERIFIED`, `PENDING`, `REJECTED`, `ESCALATED`). Only checkout needs it today; other screens can adopt it without changes to their logic.

**Polling over Pusher for verification status**: Verification status is webhook-driven (Stripe → our backend). The mobile app polls every 3s while the Stripe sheet is open and for a short window after completion. This is simpler than subscribing to a Pusher channel for a one-time event, and 3s latency on KYC confirmation is acceptable.

**Backend change scope**: Added `ephemeralKeySecret` to `StartVerificationResult`. This is the minimum change to unblock the native SDK — the field maps directly from `session.ClientSecret` which Stripe already returns; it was just not being forwarded.
