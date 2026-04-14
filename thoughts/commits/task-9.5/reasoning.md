# Task 9.5 — Commit Reasoning

## What changed

All 5 booking Maestro E2E flows now pass: create-booking-request, view-booking-status,
host-accept-booking, host-decline-booking, and cancel-booking.

## Why these changes were necessary

### Seed script cascade delete (setup.sh)
The `transactions` table has 10 child tables with FK constraints (proximity_proofs,
agent_decisions, media, messages, ratings, etc.). A simple `DELETE FROM transactions`
fails with FK violation. The fix uses a PL/pgSQL block to collect bob's transaction IDs
and delete from all child tables before deleting transactions.

### Stub payment adapter (stub.go)
E2E booking creation requires a payment adapter. Rather than configuring real Stripe
credentials in dev, a stub adapter returns success for all operations when
`StripeSecretKey == "sk_test_placeholder"`. This is the same pattern used for the
camera bypass (__DEV__ check) — no global E2E_MODE flag.

### SafeAreaView -> View (booking-status.tsx, checkout.tsx)
RN 0.81.5 has a rendering bug where SafeAreaView wrapping ScrollView causes nested
content to not render when used inside tab navigators with headerShown: true. The
tabs header already handles safe area insets, so SafeAreaView is redundant anyway.

### Rentals tab booking-status re-export
Booking status needs to be viewable from both the Feed tab (after creating a booking)
and the Rentals tab (when tapping an existing booking). A re-export file in the
rentals group keeps navigation within the correct tab stack.

### Alice account backdating
The fraud velocity check blocks bookings when both renter and host accounts are
newer than 30 days. Backdating alice's created_at by 60 days bypasses this for
E2E tests while keeping the fraud check active for real users.
