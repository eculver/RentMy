# Task 3.4 — MessagingService Handoff

**Status:** Completed  
**Commit:** fd737f5  
**Branch:** task-3.4-messaging-service  
**Date:** 2026-04-04

---

## What Was Built

MessagingService implementing PRD §6 (messages table): in-transaction chat
between renters and hosts, with Pusher real-time delivery and push notifications.

### New files

| File | Purpose |
|------|---------|
| `backend/internal/messaging/model.go` | Message domain type, SendMessageInput, Parties, sentinel errors, MaxContentLength |
| `backend/internal/messaging/pusher.go` | TransactionChannel() naming helper, EventNewMessage and EventBookingStatusChanged constants |
| `backend/internal/messaging/repository.go` | Insert (RETURNING), FindByTransactionID (cursor-based ASC pagination), GetParties (auth check against transactions table) |
| `backend/internal/messaging/service.go` | SendMessage (validate → insert → Pusher → push notification), GetMessages, repo/pusherClient/notificationSvc interfaces, NewServiceFromParts for test injection |
| `backend/internal/messaging/handler.go` | POST /api/v1/bookings/:id/messages (201), GET /api/v1/bookings/:id/messages?cursor&limit (200) |
| `backend/internal/messaging/service_test.go` | 10-case unit test suite: send success, host→renter direction, empty content, too long, not a party, transaction not found, pagination, get-messages auth |

### Modified files

| File | Change |
|------|--------|
| `backend/internal/booking/service.go` | Added `pusherSvc` interface + `WithPusher()` functional option; added `triggerStatusChanged()` helper; fire `booking-status-changed` events on Accept, Decline, Cancel, CheckIn, CheckOut |
| `backend/cmd/server/main.go` | Import messaging package; construct messagingRepo, messagingSvc, messagingHandler; mount messagingHandler; call `bookingSvc.WithPusher(pusherClient)` |

---

## API Endpoints

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| POST | `/api/v1/bookings/:id/messages` | Required | Send a message; returns 201 with message object |
| GET | `/api/v1/bookings/:id/messages` | Required | Get messages (cursor pagination, oldest first) |

### GET query params

| Param | Default | Notes |
|-------|---------|-------|
| `cursor` | "" (first page) | Exclusive lower bound (ULID of last seen message) |
| `limit` | 50 | Max messages per page |

### Response shape

```json
{
  "messages": [...],
  "nextCursor": "01J..." // omitted when no further pages
}
```

---

## Key Design Decisions

**No migration needed.** The `messages` table was already created in `001_initial_schema.sql`.

**Authorization via transactions table.** Rather than importing the booking package (creating a cycle), the messaging repository queries `SELECT renter_id, host_id FROM transactions WHERE id = $1` directly. This keeps the import graph acyclic.

**Cursor pagination on ULID.** ULIDs sort lexicographically by creation time, so `WHERE id > $cursor ORDER BY id ASC LIMIT $n` gives stable, consistent pages without offset drift.

**pusherSvc via WithPusher() functional option.** The booking service already had five constructor params. Adding Pusher as an optional post-construction call (functional option pattern) avoids breaking the existing constructor signature and keeps the zero-value safe (nil pusherSvc = no events).

**NewServiceFromParts exported for tests.** The service internally uses a `repo` interface. `NewServiceFromParts` accepts that interface so tests can inject in-memory stubs without any DB.

**Pusher and push are best-effort.** Failures are logged via `slog.Warn` but never returned to the caller. A failed Pusher trigger doesn't fail the message send.

---

## Verification

```
go vet ./...            ✓ no issues
go build ./cmd/server   ✓ clean
go test ./... -count=1

ok  backend/internal/booking      0.199s
ok  backend/internal/discovery    0.359s
ok  backend/internal/listing      0.530s
ok  backend/internal/media        0.968s
ok  backend/internal/messaging    0.689s  (10 tests)
ok  backend/internal/notification 0.984s
ok  backend/internal/payment      1.104s
ok  backend/internal/proximity    1.148s
ok  backend/internal/user         2.759s
```

---

## Branching

Used vanilla git (Graphite unavailable). Branch `task-3.4-messaging-service` stacks on `task-3.3-notification-service`.

---

## Next Tasks Unblocked

- **3.5** Booking flow (RN) — depends on 3.1 (complete)
- **3.6** Handoff screens (RN) — depends on 3.2 and 3.5
- **3.7** Messaging screen (RN) — depends on 3.4 (now complete)
