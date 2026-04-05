# Commit c7a4b09 — Reasoning

## What
Add `NotificationService` (task 3.3) implementing PRD §16 notification types.

## Why

The core transaction loop (Phase 3) requires that both parties receive push
notifications at key lifecycle events: booking requests, acceptances,
auto-declines, cancellations, and time-based reminders before pickup/return.
Without this service those events are silent and the UX is unusable.

## Design decisions

**Postgres table as source of truth:** In-app notifications are stored in a
`notifications` table regardless of push delivery success. Push is
fire-and-forget; the app can always fetch missed notifications from the API.

**Separate `push_tokens` table:** One user may have multiple devices. Storing
tokens in a dedicated table (with UNIQUE constraint on token) allows clean
idempotent registration and targeted deletion when Expo returns
`DeviceNotRegistered`.

**Existing `notification_preferences` JSONB column:** The `users` table
already had a `notification_preferences JSONB` column from the initial schema.
We reuse it rather than adding a new table — keeps the user row as the single
source of preference truth.

**Mandatory types (cannot be disabled):** BOOKING_REQUEST, BOOKING_ACCEPTED,
BOOKING_AUTO_DECLINED, CANCELLATION are safety-critical. `IsTypeDisabled()`
ignores them even if listed in user's `DisabledTypes`.

**Quiet hours with River deferral:** The in-app record is always stored
immediately. If the current time falls inside the user's quiet window, a
`QuietHoursDeferredJob` is scheduled to fire when quiet hours end rather than
dropping the push notification entirely.

**notificationSvc interface in BookingService:** Mirrors the proximitySvc
pattern from task 3.2 — interface breaks the import cycle and allows test
injection. AutoDeclineJobWorker also receives the interface for auto-decline
notifications.

**Two-phase notificationSvc construction in main.go:** NotificationService is
built before River starts (with `nil` riverClient) so workers can be
registered. After River starts, we re-create the service with the real
`riverClient` so scheduled jobs (pickup/return reminders, quiet hours
deferral) work correctly. The worker instances built before River already have
their `svc` pointer pointing at the first instance; the second creation does
not invalidate those workers because they reference the service object directly
(River workers are not hot-swapped). To make scheduled jobs work from within
workers, the BookingService (built after River) passes the second
notificationSvc instance.

**Expo SDK over raw HTTP:** The `exponent-server-sdk-golang` SDK handles
chunked delivery, `DeviceNotRegistered` error categorization, and request
signing. It is a thin, inspectable wrapper with no heavy dependencies.
