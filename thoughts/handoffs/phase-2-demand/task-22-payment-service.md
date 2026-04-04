# Task 2.2 — PaymentService (backend)

## Status: COMPLETE

**Commit:** 97160ff
**Branch:** task-2.2-payment-service
**Branching mode:** git fallback (Graphite unavailable)

---

## What Was Done

### New files

**`backend/internal/payment/model.go`**
- `PaymentAdapter` interface — `AuthorizeHold`, `CaptureHold`, `ReleaseHold`, `ChargeRentalFee`, `PayoutHost`, `Refund`, `CreateConnectedAccount`, `CreateCustomer`, `CreateSetupIntent`
- `HoldAllocation` — JSONB struct tracking how the authorized hold has been used
- `CaptureReason` — typed constant for late_fee vs damage captures
- `BookingInput`, `BookingResult` — request/response for CreateBooking
- `GuaranteeFundEntry`, `GuaranteeFundHealth` — guarantee fund types
- `ListingSnapshot` — minimal listing fields fetched for booking computation
- `Transaction` — domain representation of a booking transaction
- `OnboardHostResult`, `SetupPaymentResult`, `HoldEstimateResult` — handler response types
- Sentinel errors: `ErrListingNotFound`, `ErrTransactionNotFound`, `ErrNoPaymentMethod`, `ErrNoStripeAccount`, `ErrInsufficientHold`, `ErrInvalidCapture`

**`backend/internal/payment/hold.go`**
- `TieredHold(itemValueCents int64) int64` — PRD §7 tier table:
  - ≤$500: 100% of value
  - $501–$2,000: $500 + 25% of overage
  - $2,001–$5,000: $875 + 15% of overage
  - >$5,000: $1,325 hard ceiling
- `GuaranteeGap(itemValue, holdAmount int64) int64` — uncovered portion covered by fund
- `RentalFee(perHour, perDay int64, duration time.Duration) int64` — prefers daily rate when duration ≥24h
- `PlatformFee(rentalFee int64, takeRateBPS int) int64` — BPS-based fee
- `HostPayout(rentalFee, platformFee int64) int64`
- `GuaranteeFundContribution(platformFee int64, guaranteeRateBPS int) int64`
- `itemValue(snap ListingSnapshot) int64` — prefers HostDeclaredValue over EstimatedValue
- `listingPriceCents(snap ListingSnapshot) (perHour, perDay int64)` — converts float64 dollars to cents

**`backend/internal/payment/stripe.go`**
- `StripeAdapter` — implements `PaymentAdapter` using stripe-go v81 `client.API`
- `NewStripeAdapter(apiKey string, opts ...Option) *StripeAdapter` — functional options pattern
- `AuthorizeHold` — `PaymentIntent` with `CaptureMethod: manual` + `Confirm: true`
- `CaptureHold` — `PaymentIntents.Capture` with `AmountToCapture` for partial holds
- `ReleaseHold` — `PaymentIntents.Cancel`
- `ChargeRentalFee` — `PaymentIntent` with `CaptureMethod: automatic` + `Confirm: true`
- `PayoutHost` — `Transfers.New` to connected account
- `Refund` — `Refunds.New` against a charge ID
- `CreateConnectedAccount` — Express account + `AccountLinks.New` for onboarding URL
- `CreateCustomer` — `Customers.New`
- `CreateSetupIntent` — `SetupIntents.New` for saving payment method on file

**`backend/internal/payment/repository.go`**
- `GetListingForBooking(ctx, listingID)` — fetches active listing pricing/value snapshot
- `GetStripeCustomerID` / `StoreStripeCustomerID` — renter's Stripe customer ID
- `GetStripeAccountID` / `StoreStripeAccountID` — host's Stripe connected account ID
- `GetUserEmailAndName` — for Stripe account/customer creation
- `CreateTransaction(ctx, pgx.Tx, Transaction)` — INSERT with all payment fields; stores dollar amounts as NUMERIC(10,2) converted from cents
- `GetTransaction(ctx, id)` — SELECT with reverse cent conversion via `ROUND(col * 100)::bigint`
- `UpdateHoldAllocation(ctx, pgx.Tx, ...)` — SELECT FOR UPDATE locking + UPDATE
- `UpdateTransactionStatus` — status field update
- `UpdateTransactionStripeTransferID` — records transfer ID after payout
- `InsertGuaranteeFundEntry(ctx, pgx.Tx, ...)` — computes running balance from last entry
- `GetGuaranteeFundBalance` / `GetTotalOutstandingGuaranteeGaps`
- `GetHostTransactionCount` — completed tx count for payout delay logic
- `GetHostReputationScore` — for high-risk host detection
- `GetRenterTransactions` — paginated bookings list
- `BeginTx` — starts a pgx transaction

**`backend/internal/payment/service.go`**
- `Config` — `TakeRateBPS`, `GuaranteeRateBPS`, `DamageReserveRate`, `PayoutDelayNewHostH`
- `CreateBooking(ctx, BookingInput)` — full orchestration:
  1. Fetch listing snapshot
  2. Compute hold, rental fee, platform fee, host payout, guarantee gap, damage reserve
  3. Resolve renter's Stripe customer ID (fails if not set up)
  4. Authorize hold (manual-capture PaymentIntent)
  5. Charge rental fee (auto-capture); releases hold on failure
  6. Atomically create transaction row + guarantee fund contribution in one DB transaction
- `ReleaseHold` — releases Stripe hold + updates allocation in DB
- `CaptureFromHold(ctx, txnID, amount, reason)` — SELECT FOR UPDATE, validates remaining, captures, updates allocation
- `ScheduleHostPayout(ctx, transactionID)` — enqueues `PayoutJobArgs` River job with delay based on host history/reputation
- `payoutDelay(ctx, hostID)` — <3 transactions or reputation_score < 200 → 48h delay; otherwise 0
- `GetGuaranteeFundHealth` — balance, outstanding gaps, reserve ratio
- `OnboardHost` / `SetupRenterPayment` — Stripe account/customer setup with DB persistence
- `GetHoldEstimate` — returns item value + hold + gap for listing detail screen
- `GetTransaction` / `GetRenterBookings` — read-side methods

**`backend/internal/payment/payout_job.go`**
- `PayoutJobArgs` — `TransactionID`, `HostAccountID`, `Amount`, `Currency`; `Kind()` = "host_payout"
- `PayoutJobWorker` — calls `PayoutHost`, records transfer ID; logs reconciliation warning if DB update fails (transfer already succeeded)

**`backend/internal/payment/handler.go`**
- `GET /api/v1/listings/{id}/hold-estimate` — public, no auth
- `POST /api/v1/bookings` — requires auth; decodes JSON with listingId, paymentMethodId, scheduledStart, scheduledEnd (RFC3339)
- `GET /api/v1/bookings/{id}` — requires auth
- `GET /api/v1/users/me/bookings` — paginated (limit, offset query params)
- `POST /api/v1/payments/onboard` — host Stripe Express onboarding
- `POST /api/v1/payments/setup` — renter payment method setup, returns clientSecret

**`backend/internal/payment/hold_test.go`** (15 tests, all passing)
- `TieredHold`: all tier boundaries including zero, $500 ceiling, $2k ceiling, $5k ceiling, hard ceiling
- `GuaranteeGap`: equal hold, hold > value (no gap), partial, high-value item
- `RentalFee`: hourly, multi-hour, sub-hour rounding, daily, multi-day, fallbacks
- `PlatformFee`, `HostPayout`, `GuaranteeFundContribution`: BPS math

**`backend/internal/payment/service_test.go`** (5 tests, all passing)
- `mockAdapter` — full mock implementing `PaymentAdapter`
- `TestAmountCalculationChain` — $300 item, 2-day rental, verifies full chain
- `TestAmountCalculationHighValueItem` — $10k item, hard ceiling, hourly fallback
- `TestCaptureReasonValidation` — invalid reason returns `ErrInvalidCapture` before any DB access
- `TestItemValuePreference` — host declared > estimated; fallback; zero

**`backend/migrations/003_stripe_ids.sql`**
- `users`: `stripe_customer_id TEXT`, `stripe_account_id TEXT`
- `transactions`: `stripe_payment_intent_id TEXT`, `stripe_charge_id TEXT`, `stripe_transfer_id TEXT`, `platform_fee NUMERIC(10,2)`, `host_payout NUMERIC(10,2)`, `guarantee_contribution NUMERIC(10,2)`

### Modified files

**`backend/internal/platform/config/config.go`**
- Added: `StripeSecretKey`, `StripePublishableKey`, `StripeWebhookSecret`
- Added: `TakeRateBPS` (2000), `GuaranteeRateBPS` (1000), `DamageReserveRate` (4000), `PayoutDelayNewHostH` (48)

**`backend/cmd/server/main.go`**
- `stripeAdapter` and `paymentRepo` created before River client starts (so payout worker can be registered)
- `river.AddWorker(workers, payment.NewPayoutJobWorker(paymentRepo, stripeAdapter))` — registered before River start
- `paymentSvc` and `paymentHandler` wired after River client is running
- `paymentHandler.Mount(apiV1, authMW)` mounted at `/api/v1/...`

**`.env.example`**
- Added Stripe env vars with placeholder test values
- Added payment config env vars with PRD defaults

---

## Architecture Decisions

- **`PaymentAdapter` interface:** Decouples business logic from Stripe. Any payment processor can be swapped by implementing the interface. Tests use `mockAdapter`.
- **Stripe client.API (non-global):** stripe-go v81 supports `client.API.Init()` to avoid setting global API key state. This is thread-safe and testable.
- **Cents everywhere:** All monetary amounts are int64 cents internally. Converted to/from Postgres NUMERIC(10,2) only at the persistence boundary. Float is never used for money.
- **CreateBooking atomicity:** Hold authorization and rental fee charge happen via Stripe before the DB transaction. If the DB write fails after Stripe charges succeed, manual reconciliation is needed (logged as error). This is a known trade-off; two-phase commit is not practical across Stripe + Postgres.
- **SELECT FOR UPDATE in CaptureFromHold:** Prevents concurrent LateReturnAgent and DisputeAgent from racing on the same hold. The lock is taken within a pgx transaction.
- **Payout worker registered before River starts:** `paymentRepo` and `stripeAdapter` are built early in `main.go` so they're available when registering workers with the River `Workers` registry, which must happen before `river.NewClient()`.
- **Stripe placeholder keys in dev:** `sk_test_placeholder` satisfies config parsing without connecting to Stripe. Actual Stripe calls fail gracefully in dev (network error, not panic).

---

## Verification

```
cd backend && go vet ./...          → exit 0
cd backend && go build ./cmd/server → exit 0
cd backend && go test ./internal/payment/... -v -count=1 → 20/20 PASS
cd backend && go test ./... -count=1 → all packages PASS
```

---

## Notes for Next Tasks

- **Task 2.3 (Feed screen, RN):** No dependency on PaymentService. Can start immediately.
- **Task 2.4 (Search screen, RN):** No dependency on PaymentService.
- **Task 2.5 (Map screen, RN):** No dependency on PaymentService.
- **Task 2.6 (Listing detail screen, RN):** Calls `GET /api/v1/listings/:id/hold-estimate` (no auth) to display hold amount. Response: `{itemValue, holdAmount, guaranteeGap}` in cents.
- **Task 2.7 (Checkout screen, RN):** Uses `POST /api/v1/payments/setup` to get `clientSecret`, then `POST /api/v1/bookings` to create booking.
- **Real Stripe keys:** Replace `sk_test_placeholder` with actual test-mode keys from the Stripe dashboard for manual integration testing. Add test Stripe keys to `.env.individual` (gitignored).
- **Stripe webhook:** `STRIPE_WEBHOOK_SECRET` is wired in config but a webhook handler is not yet implemented. Phase 3 or 4 should add `POST /api/v1/payments/webhook` to handle `payment_intent.succeeded`, `payment_intent.payment_failed`, `account.updated` events.
- **`ScheduleHostPayout` call site:** Currently only exposed as a service method. Phase 3 BookingService should call it when a booking transitions to `COMPLETED` status.
