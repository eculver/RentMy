# Phase 3 — Core Transaction Loop: Continuity Ledger

## Phase Goal

Full booking lifecycle: request → accept → check-in handoff → rental → return handoff → completion. Fraud velocity rules block bad bookings. Push notifications fire. In-app messaging works.

---

## Task Status

| Task | Name | Status | Commit | Notes |
|------|------|--------|--------|-------|
| 3.1 | BookingService (backend) | ✅ completed | b3a66c2 | State machine, fraud, cancellation, auto-decline |
| 3.2 | ProximityService (backend) | ✅ completed | 12dc136 | GPS + PIN handoff, Twilio SMS fallback |
| 3.3 | NotificationService (backend) | ✅ completed | c7a4b09 | Expo push, preferences, quiet hours |
| 3.4 | MessagingService (backend) | ✅ completed | fd737f5 | Pusher real-time chat, booking status events |
| 3.5 | Booking flow (RN) | pending | — | Host accept/decline, renter cancel screens |
| 3.6 | Handoff screens (RN) | pending | — | Check-in/out with angle-enforced photos |
| 3.7 | Messaging screen (RN) | pending | — | Real-time chat UI |

---

## Completed Work Summary

### Task 3.1 — BookingService

Created `backend/internal/booking/` package with full booking lifecycle management.

**Key facts for future sessions:**
- Booking creation moved from `payment.Handler` to `booking.Handler`. PaymentHandler now only has `holdEstimate`, `onboardHost`, `setupPayment`.
- `POST /api/v1/bookings` now goes through `BookingService.CreateBooking` → fraud checks → `PaymentService.CreateBooking` → auto-decline job
- State transitions use SELECT FOR UPDATE locking. Only one state change can succeed for a given booking at a time.
- Auto-decline job kind: `"booking_auto_decline"`. Default timeout: 2h (configurable via `AUTO_DECLINE_TIMEOUT_HOURS`).
- Fraud results include `PayoutDelayed` flag (currently not persisted — future task should add `payout_delayed` column to transactions).
- `booking.Repository.SumHostDamageClaimed` reads JSONB `hold_allocation->>'capturedForDamage'` in cents.
- No new migrations were needed for 3.1 — all required columns exist in the schema.

**Open wiring needed in future tasks:**
- `PaymentService.ScheduleHostPayout` should be called on ACTIVE→COMPLETED transition (done via task 3.2 CheckOut flow)
- NotificationService calls stub-commented in `service.go` (task 3.3)
- `payout_delayed` flag from fraud check not yet persisted (task 3.3 or later)

### Task 3.2 — ProximityService

Created `backend/internal/proximity/` package with GPS + PIN handoff verification.

**Key facts for future sessions:**
- Migration 004 adds `user_id` and `pin_expires_at` to `proximity_proofs`.
- Each handoff event has per-user records: `(transaction_id, proof_type, user_id)`. Host CHECK_IN record stores the PIN; renter's record is marked verified after GPS + PIN pass.
- `BookingService.Accept` now calls `proximitySvc.GenerateCheckInPIN` (best-effort after commit — failure does not roll back acceptance).
- `BookingService.CheckIn/CheckOut` gate on `proximitySvc.CheckHandoffComplete` before status transition.
- `proximitySvc` is injected as an interface into `BookingService` — allows test stubbing without a real DB.
- Twilio client is nil in dev/test when env vars (`TWILIO_ACCOUNT_SID`, `TWILIO_AUTH_TOKEN`, `TWILIO_FROM_NUMBER`) are absent.
- Config defaults: `GPS_THRESHOLD_METERS=100`, `PIN_VALIDITY_MINUTES=30`.

**Open wiring for future tasks:**
- Photo requirement for `CheckHandoffComplete` is enforced client-side (handoff screens, task 3.6). Server only checks GPS + PIN today.
- `PaymentService.ScheduleHostPayout` should be called from `BookingService.CheckOut` after ACTIVE→COMPLETED (not yet wired).

### Task 3.3 — NotificationService

Created `backend/internal/notification/` package with Expo push delivery, preferences, and quiet hours.

**Key facts for future sessions:**
- 15 notification types; 4 mandatory (BOOKING_REQUEST, BOOKING_ACCEPTED, BOOKING_AUTO_DECLINED, CANCELLATION) cannot be disabled.
- Migration 005 adds `notifications` and `push_tokens` tables.
- `users.notification_preferences` JSONB column reused for preference storage.
- Two-phase notificationSvc construction: built with nil riverClient for worker registration, re-created with real riverClient after River starts.
- River workers: PickupApproachingWorker, ReturnApproachingWorker, QuietHoursDeferredWorker.
- BookingService accepts `notificationSvc` interface; fires on CreateBooking, Accept, Cancel, AutoDecline.

### Task 3.4 — MessagingService

Created `backend/internal/messaging/` package with in-transaction chat.

**Key facts for future sessions:**
- No new migration — `messages` table was in `001_initial_schema.sql`.
- Authorization: `GetParties` queries `transactions` table directly (avoids booking import cycle).
- Cursor pagination: `WHERE id > $cursor ORDER BY id ASC LIMIT $n` — ULID sort order = chronological.
- `booking.Service.WithPusher(p)` wires Pusher into booking service post-construction (functional option).
- `booking-status-changed` Pusher events fire on Accept, Decline, Cancel, CheckIn, CheckOut.
- Pusher channel name: `private-transaction-{transactionID}` (see `messaging.TransactionChannel()`).
- `messaging.EventNewMessage` = `"new-message"`, `messaging.EventBookingStatusChanged` = `"booking-status-changed"`.
- `NewServiceFromParts` exported so tests inject stub `repo` without DB.

**Open wiring for future tasks:**
- Task 3.7 (Messaging screen RN) consumes the messaging API + Pusher channel for real-time chat UI.
