# Commit 97160ff — PaymentService

## Why this commit

Task 2.2 implements the full payment backend required for renters to book items. Without this, the demand side of the marketplace has no way to exchange money: no holds, no rental fee charges, no host payouts, and no guarantee fund.

## Key choices

**Stripe Express Connect over Standard/Custom:** Less KYC compliance surface for hosts; faster onboarding. Stripe manages KYC for Express accounts. The `CreateConnectedAccount` implementation uses `AccountTypeExpress` + an AccountLink for the onboarding URL — the host completes Stripe's hosted onboarding flow outside the app.

**`client.API` vs global `stripe.Key`:** stripe-go supports both patterns. The `client.API.Init()` approach avoids setting a package-level global, which is important for testability and for avoiding race conditions if multiple tests run in parallel with different API keys.

**All amounts in int64 cents:** Float64 for money causes rounding errors. The PRD's tier calculations (e.g., "$500 + 25% of overage") are exact with integer arithmetic. Conversion to/from NUMERIC(10,2) happens only at the DB boundary via multiplication/division by 100.

**`paymentRepo` built before River client starts:** River requires all workers to be registered in the `Workers` struct before `river.NewClient()` is called. The payout worker needs `paymentRepo` and `stripeAdapter`. Rather than constructing the whole service graph early, just the dependencies of the worker are built upfront.

**CreateBooking error handling:** If the DB write fails after Stripe charges have been made, the user has been charged but no transaction record exists. This is logged as an error. Full idempotency would require a Stripe idempotency key and a reconciliation job — deferred to a later phase when the platform has real transaction volume.
