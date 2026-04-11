# Phase 3 — Core Transaction Loop: Continuity Ledger

## Phase Goal

Full booking lifecycle: request → accept → check-in handoff → rental → return handoff → completion. Fraud velocity rules block bad bookings. Push notifications fire. In-app messaging works.

---

## Task Status

| Task | Name | Status | Commit | Notes |
|------|------|--------|--------|-------|
| 3.1 | BookingService (backend) | ✅ completed | b3a66c2 | State machine, fraud, cancellation, auto-decline |
| 3.2 | ProximityService (backend) | pending | — | GPS + PIN handoff verification |
| 3.3 | NotificationService (backend) | pending | — | Expo push + Twilio SMS |
| 3.4 | MessagingService (backend) | pending | — | Pusher real-time chat |
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
- `PaymentService.ScheduleHostPayout` should be called on ACTIVE→COMPLETED transition (task 3.2)
- NotificationService calls stub-commented in `service.go` (task 3.3)
- `payout_delayed` flag from fraud check not yet persisted (task 3.3 or later)
