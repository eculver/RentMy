# Task 3.3 — NotificationService Handoff

**Status:** Completed  
**Commit:** c7a4b09  
**Branch:** task-3.3-notification-service  
**Date:** 2026-04-04

---

## What Was Built

NotificationService implementing PRD §16 (notification types, push delivery, preferences, quiet hours).

### New files

| File | Purpose |
|------|---------|
| `backend/migrations/005_notifications.sql` | `notifications` table + `push_tokens` table |
| `backend/internal/notification/model.go` | Notification domain type, Type enum (15 types), Preferences, PushToken, IsMandatory |
| `backend/internal/notification/preferences.go` | IsTypeDisabled (mandatory types always enabled), IsQuietHours, QuietHoursEndTime |
| `backend/internal/notification/preferences_test.go` | IsTypeDisabled tests, quiet hours wrapping/non-wrapping, boundary conditions |
| `backend/internal/notification/repository.go` | Insert, FindByUserID (paginated), MarkRead, MarkAllRead, CountUnread, InsertPushToken, GetPushTokens, DeletePushToken, GetUserPreferences, UpdateUserPreferences |
| `backend/internal/notification/push.go` | PushClient wrapping Expo SDK (PublishMultiple), DeviceNotRegistered stale-token detection |
| `backend/internal/notification/service.go` | Notify (preference check → store → push/defer), notifyDirect, RegisterPushToken, GetNotifications, MarkRead/All, CountUnread, GetPreferences, UpdatePreferences |
| `backend/internal/notification/scheduled_jobs.go` | PickupApproachingWorker, ReturnApproachingWorker, QuietHoursDeferredWorker + schedule helpers |
| `backend/internal/notification/handler.go` | 7 HTTP endpoints: list, read, read-all, unread-count, preferences, update preferences, register-token |
| `backend/internal/notification/service_test.go` | stubRepo in-memory impl + tests: store, disabled type skipped, mandatory cannot be disabled, mark read, not found, mark all, count unread |

### Modified files

| File | Change |
|------|--------|
| `backend/internal/platform/config/config.go` | Added `ExpoPushAccessToken`, `PickupReminderMinutes`, `ReturnReminderMinutes` |
| `backend/internal/booking/service.go` | Added `notificationSvc` interface + field; `NewService` accepts it; CreateBooking notifies host; Accept notifies renter + schedules reminders; Cancel notifies other party |
| `backend/internal/booking/autodecline_job.go` | `notificationSvc` injected; Work notifies both parties on auto-decline |
| `backend/cmd/server/main.go` | Build notificationSvc pre-River (for workers), re-create with riverClient post-River; register 3 notification River workers; mount notificationHandler |

---

## API Endpoints

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| GET | `/api/v1/notifications` | Required | Paginated list (limit, offset params) |
| POST | `/api/v1/notifications/:id/read` | Required | Mark single notification read |
| POST | `/api/v1/notifications/read-all` | Required | Mark all read |
| GET | `/api/v1/notifications/unread-count` | Required | `{"count": N}` |
| GET | `/api/v1/notifications/preferences` | Required | User's preference object |
| PUT | `/api/v1/notifications/preferences` | Required | Update preferences |
| POST | `/api/v1/notifications/register-token` | Required | Save Expo push token |

---

## Key Design Decisions

**Mandatory notification types:** BOOKING_REQUEST, BOOKING_ACCEPTED, BOOKING_AUTO_DECLINED, CANCELLATION cannot be disabled by user preference. `IsTypeDisabled()` hard-returns `false` for these regardless of the user's `DisabledTypes` list.

**Two-phase NotificationService construction:** Built first with `nil` riverClient so River workers can be registered. Re-created with real `riverClient` after River starts so BookingService can schedule pickup/return reminder jobs.

**Existing JSONB column reused:** `users.notification_preferences` already existed in the initial schema — we marshal/unmarshal `Preferences` struct into it directly.

**Per-token message construction:** Each `PushMessage` targets exactly one token so that `DeviceNotRegistered` errors map cleanly back to the stale token for deletion.

---

## Dependency Added

`github.com/oliveroneill/exponent-server-sdk-golang v0.0.0-20210823140141-d050598be512`

**Rationale:** Expo SDK handles `DeviceNotRegistered` error categorization, token validation (`NewExponentPushToken` rejects non-Expo tokens), and chunked delivery via `PublishMultiple`. Rolling raw HTTP would require duplicating this logic.

---

## Verification

```
go vet ./...          ✓ no issues
go build ./cmd/server ✓ clean
go test ./... -count=1

ok  backend/internal/booking      0.466s
ok  backend/internal/discovery    0.614s
ok  backend/internal/listing      0.794s
ok  backend/internal/media        1.075s
ok  backend/internal/notification 0.293s (15 tests: preference, quiet hours, service logic)
ok  backend/internal/payment      1.100s
ok  backend/internal/proximity    1.115s
ok  backend/internal/user         2.928s
```

---

## Branching

Used vanilla git (Graphite unavailable). Branch `task-3.3-notification-service` stacks on `task-3.2-proximity-service`.

---

## Next Tasks Unblocked

- **3.4** MessagingService — no dependencies (was already unblocked)
- **3.5** Booking flow (RN) — depends on 3.1 (complete)
- **3.6** Handoff screens (RN) — depends on 3.2 and 3.5
- **3.7** Messaging screen (RN) — depends on 3.4
