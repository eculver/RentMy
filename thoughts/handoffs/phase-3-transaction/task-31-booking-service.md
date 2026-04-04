# Task 3.1 — BookingService (backend)

## Status: COMPLETE

**Commit:** b3a66c2
**Branch:** task-3.1-booking-service
**Branching mode:** git fallback (Graphite unavailable)

---

## What Was Done

### New package: `backend/internal/booking/`

**`model.go`**
- `Booking` domain type — booking state view of the `transactions` table (no financial fields; those stay in payment domain)
- `Status` constants: REQUESTED, ACCEPTED, DECLINED, AUTO_DECLINED, ACTIVE, COMPLETED, DISPUTED, CANCELLED
- `MaxDuration = 7 * 24h` ceiling constant (PRD §7)
- Input types: `CreateInput`, `AcceptInput`, `DeclineInput`, `CancelInput`
- Sentinel errors: `ErrBookingNotFound`, `ErrInvalidTransition`, `ErrNotAuthorized`, `ErrListingNotAvailable`, `ErrFraudBlocked`, `ErrDurationExceedsLimit`

**`statemachine.go`**
- `AllowedTransitions` map — the complete valid transition set
- `CanTransition(from, to)` — predicate
- `ValidateTransition(from, to)` — returns formatted error wrapping `ErrInvalidTransition`

**`statemachine_test.go`** (3 tests)
- All valid transitions: 9 cases pass
- All invalid transitions (including terminal states): 14 cases pass

**`fraud.go`**
- `FraudVelocityConfig` — tunable thresholds (NewAccountThresholdDays, FirstNTransactions, PayoutDelay, DamageClaimCapCents, DamageClaimWindowDays)
- `FraudResult` — Blocked bool, BlockReason string, PayoutDelayed bool
- `CheckFraudVelocity(ctx, repo, renterID, hostID, cfg)` — runs all three PRD §9 rules:
  1. New-to-new lockout: both parties < 30 days old → blocked
  2. Damage claim cap: host's damage within first 60 days exceeds cap → blocked
  3. First-N payout delay: either party has < 3 completed transactions → PayoutDelayed=true

**`fraud_test.go`** (5 tests)
- New-to-new blocked, one-established allowed
- First-N flagged for payout delay
- Damage cap blocked
- Both established, no delay

**`cancellation.go`**
- `CancellerRole` (RENTER / HOST)
- `HostCancelConfig` (LateFeeBPS, VeryLateFeeBPS)
- `CalculateCancellationFee(role, rentalFeeCents, scheduledStart, now, hostCfg)`:
  - Renter: >2h=0%, 1-2h=25%, <1h=50%, after start=100%
  - Host: >2h=0%, <2h=LateFeeBPS%, after start=VeryLateFeeBPS%

**`cancellation_test.go`** (3 test functions, 8 sub-tests)
- All renter timing brackets
- All host timing brackets
- Exact 2h boundary (no fee, uses fixed reference time to avoid race)

**`repository.go`**
- `FindByID`, `FindByRenterID`, `FindByHostID` — read the `transactions` table into `Booking` type
- `UpdateStatus(ctx, tx, id, status)` — SELECT FOR UPDATE + validate transition + UPDATE; concurrent-safe
- `UpdateCancellation(ctx, tx, id, status, cancelledBy, feeCents)` — same locking pattern + writes cancelled_by and cancellation_fee
- `BeginTx` — starts a pgx transaction
- `GetUserCreatedAt` — for fraud check
- `CountCompletedByUser` — for fraud check (both renter and host)
- `SumHostDamageClaimed` — sums `hold_allocation->>'capturedForDamage'` JSONB field across transactions
- `IsListingAvailable` — checks for overlapping REQUESTED/ACCEPTED/ACTIVE bookings
- `GetListingHostID` — fetches host_id for an active listing (used by service for fraud check)

**`service.go`**
- `Config` — all tunable parameters (AutoDeclineTimeout, fraud thresholds, cancel fee BPS)
- `CreateBooking` — validates 7-day ceiling, checks listing availability, runs fraud checks, delegates to `payment.Service.CreateBooking`, enqueues auto-decline job
- `Accept(AcceptInput)` — verifies requester is host, transitions REQUESTED→ACCEPTED
- `Decline(DeclineInput)` — verifies requester is host, transitions REQUESTED→DECLINED
- `Cancel(CancelInput)` — identifies renter vs host role, calculates fee, transitions to CANCELLED
- `GetBooking(id, requesterID)` — authorization check (renter or host only)
- `ListByRenter`, `ListByHost` — delegate to repository

**`handler.go`**
- `POST /api/v1/bookings` — booking creation (replaces payment handler's route)
- `GET /api/v1/bookings/:id` — get booking (auth: renter or host)
- `GET /api/v1/users/me/bookings` — renter's booking list
- `GET /api/v1/users/me/hosted-bookings` — host's booking list
- `POST /api/v1/bookings/:id/accept` — host accepts → 204
- `POST /api/v1/bookings/:id/decline` — host declines → 204
- `POST /api/v1/bookings/:id/cancel` — either party cancels → 204

**`autodecline_job.go`**
- `AutoDeclineJobArgs{TransactionID}` — River job payload; `Kind()` = "booking_auto_decline"
- `AutoDeclineJobWorker.Work` — checks if still REQUESTED; if so transitions to AUTO_DECLINED; if not, no-op
- `scheduleAutoDecline` — internal helper to enqueue the job with configured delay
- `riverInserter` interface — minimal surface for testability; compile-time check against `*river.Client[pgx.Tx]`

### Modified files

**`backend/internal/payment/handler.go`**
- Removed: `createBooking`, `getBooking`, `listBookings` handlers and their `Mount` routes
- Removed: now-unused `strconv` and `time` imports
- Payment handler now only has: `holdEstimate`, `onboardHost`, `setupPayment`

**`backend/internal/platform/config/config.go`**
- Added: `AutoDeclineTimeoutH` (default 2), `FraudNewAccountDays` (30), `FraudFirstNTransactions` (3), `FraudDamageClaimCapCents` (50000 = $500), `FraudDamageClaimWindowDays` (60), `HostCancelLateBPS` (2500), `HostCancelVeryLateBPS` (5000)

**`backend/cmd/server/main.go`**
- `bookingRepo` created early (before River starts) for auto-decline worker registration
- `booking.NewAutoDeclineJobWorker(bookingRepo)` registered with River
- `bookingSvc` and `bookingHandler` constructed and wired
- `bookingHandler.Mount(apiV1, authMW)` mounted

---

## Architecture Decisions

- **Booking domain owns state transitions; Payment domain owns finances.** The booking service delegates to `payment.Service.CreateBooking` for Stripe operations. No circular dependency: booking → payment, not the other way around.
- **`payment.Handler` loses booking routes.** The booking creation and retrieval endpoints moved to `booking.Handler`. This creates a single authoritative home for booking lifecycle management.
- **SELECT FOR UPDATE for concurrent transitions.** Both `UpdateStatus` and `UpdateCancellation` lock the row before reading current state, preventing two concurrent accept/cancel calls from racing past each other.
- **Auto-decline as a no-op job.** Rather than trying to cancel River jobs when a host acts, the job checks current status. If the host accepted before timeout, the job is a no-op. No River job cancellation complexity needed.
- **`SumHostDamageClaimed` reads JSONB directly.** `hold_allocation->>'capturedForDamage'` avoids a new damage-specific column. Values in JSONB are already in cents (HoldAllocation.CapturedForDamage is int64 marshaled directly).
- **`fraudChecker` interface.** `CheckFraudVelocity` takes an interface rather than `*Repository` for testability. The mock in `fraud_test.go` fully controls all values without a database.
- **7-day ceiling enforced at the service layer.** Checked before payment processing so no Stripe charge is made for invalid bookings.

---

## Verification

```
go vet ./...              → exit 0 (no issues)
go build ./cmd/server     → exit 0 (binary builds)
go test ./internal/booking/... -v -count=1 → 12/12 PASS
go test ./... -count=1    → all packages PASS (booking, payment, discovery, listing, media, user)
```

---

## Notes for Next Tasks

- **Task 3.2 (ProximityService):** BookingService.Accept already transitions status. ProximityService will call `BookingService.Accept` indirectly when both GPS + PIN + photos are verified. Or, ProximityService can directly call `booking.Repository.UpdateStatus` with the booking's transition methods.
- **Task 3.3 (NotificationService):** BookingService should notify host on BOOKING_REQUEST and renter on BOOKING_ACCEPTED/AUTO_DECLINED/CANCELLED. Stub call sites are in service.go (marked with `_ = err` comment).
- **`ScheduleHostPayout` call site:** PaymentService.ScheduleHostPayout should be called when a booking transitions to COMPLETED (from BookingService or ProximityService). Wire this in task 3.2 when ACTIVE→COMPLETED is triggered.
- **Fraud `PayoutDelayed` flag:** Currently computed but not stored or acted on. Task 3.3 or later can add a `payout_delayed BOOLEAN` column to transactions and wire it through.
- **`GET /api/v1/bookings/:id` authorization:** Currently returns only the booking domain fields (Booking type). If the client needs financial fields (rental fee, hold amount), add a combined view or let them call the payment endpoint separately. Phase 3 UX can determine what's needed.
