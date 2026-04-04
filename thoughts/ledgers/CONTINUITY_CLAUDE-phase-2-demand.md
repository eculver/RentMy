# Phase 2 — Discovery + Payments (Demand) Continuity Ledger

## Phase Exit Criteria (from plan)

- [x] Feed returns ranked listings (task 2.1)
- [x] Search returns keyword matches (task 2.1)
- [x] Map shows fuzzed pins (task 2.1)
- [x] Tiered hold calculated (task 2.2)
- [x] Stripe hold authorized (task 2.2 — adapter wired, real Stripe keys needed)
- [x] Rental fee charged (task 2.2)
- [x] Host payout scheduled (task 2.2 — River job)
- [x] Hold allocation ledger (task 2.2)
- [x] Guarantee fund active (task 2.2)
- [x] Feed screen (RN) — task 2.3
- [x] Search screen (RN) — task 2.4
- [x] Map screen (RN) — task 2.5
- [x] Listing detail screen (RN) — task 2.6
- [x] Checkout screen (RN) — task 2.7

---

## Task Log

### Task 2.1 — DiscoveryService (backend)
**Commit:** 152cf31 | **Branch:** task-2.1-discovery-service

Implemented ranked listing feed, fulltext search, and map bounding box endpoints.
OSRM drive-time client with Redis cache. Deterministic location fuzzing (FNV64a seed).
PRD §13 ranking formula: 0.35*avail + 0.30*proximity + 0.20*reputation + 0.15*reliability.
15/15 tests passing.

### Task 2.2 — PaymentService (backend)
**Commit:** 97160ff | **Branch:** task-2.2-payment-service

Implemented:
- `PaymentAdapter` interface with Stripe Express Connect implementation (stripe-go v81)
- `TieredHold` — PRD §7 four-tier structure with $1,325 hard ceiling
- `RentalFee`, `PlatformFee`, `HostPayout`, `GuaranteeFundContribution` — BPS-based math
- `CreateBooking` — hold authorization + rental fee charge + DB transaction
- `ReleaseHold`, `CaptureFromHold` (SELECT FOR UPDATE concurrency control)
- `ScheduleHostPayout` — River job with 48h delay for new/high-risk hosts
- `OnboardHost` / `SetupRenterPayment` — Stripe Express onboarding
- Migration 003: stripe_customer_id, stripe_account_id, stripe_payment_intent_id, stripe_charge_id, stripe_transfer_id
- 20/20 tests passing; full test suite green

---

## Key Decisions Made in Phase 2

| Area | Decision |
|------|----------|
| Drive-time | OSRM self-hosted, opt-in Docker profile, graceful fallback to 0 |
| Fulltext search | Postgres tsvector + ILIKE fallback |
| Map privacy | FNV64a deterministic ~500m jitter (same pin every reload) |
| Payment SDK | stripe-go v81 with client.API (non-global, testable) |
| Money types | int64 cents everywhere; NUMERIC(10,2) at DB boundary only |
| Hold concurrency | SELECT FOR UPDATE on transaction row for CaptureFromHold |
| Payout delay | <3 txns or rep_score < 200 → 48h; established hosts → immediate |
| Stripe in dev | Placeholder keys; real calls fail gracefully (no panic) |

---

## What Next Tasks Need to Know

### Task 2.3 — Feed screen (RN)
- API: `GET /api/v1/discovery/feed?lat=&lng=&radius=&cursor=&limit=`
- Response shape: `{listings: RankedListing[], count: int}`
- Auth required (JWT Bearer)
- Key fields: title, pricePerDay/pricePerHour (dollars), thumbnailUrl, distanceMeters, driveTimeMin

### Task 2.4 — Search screen (RN)
- API: `GET /api/v1/discovery/search?q=&lat=&lng=&radius=&min_price=&max_price=&max_drive_min=&cursor=&limit=`

### Task 2.5 — Map screen (RN)
- API: `GET /api/v1/discovery/map?sw_lat=&sw_lng=&ne_lat=&ne_lng=&limit=`
- Fuzzed coords in `lat`/`lng` fields of each `RankedListing`

### Task 2.6 — Listing detail screen (RN) [COMPLETED]
**Commit:** 53fcce0 | **Branch:** task-2.6-listing-detail-screen

Implemented:
- `useListing(id)` hook — fetches `GET /api/v1/listings/:id`
- `PhotoGallery` — horizontal swipeable FlatList with pagination dots; accepts `string[]` of photo URLs
- `HostInfoCard` — avatar (initials), star rating from 0–1000 reputation score, member since, verified badge
- `HoldExplainer` — amber card showing tiered hold amount, authorization explanation, RentMy Protection row when guaranteeGap > 0
- `AvailabilityCalendar` — parses raw availability JSON; shows "Available anytime" for empty slots, formatted time ranges otherwise
- `mobile/app/(tabs)/(feed)/listing/[id].tsx` — full detail screen with back button, photo gallery, all sections, fixed "Rent Now" / "Edit Listing" CTA
- Updated navigation in feed, search, and map preview card to pass `hostName`, `hostReputation`, `thumbnailUrl`, `driveTimeMin` as route params (avoids needing a host profile API endpoint)
- `npx tsc --noEmit` → exit 0

**Key decisions:**
- Host info via route params (no host profile API yet) — easy to upgrade when Phase 3+ adds one
- No calendar library — read-only time slots rendered as a plain list
- HoldExplainer gated on `!isHost` — irrelevant and confusing to show to the listing owner

### Task 2.7 — Checkout screen (RN) [COMPLETED]
**Commit:** c202cc4 | **Branch:** task-2.7-checkout-screen

Implemented:
- `checkoutStore` (Zustand) — scheduledStart/End, paymentMethodId, holdAmount, rentalFee, totalImpact
- `DurationPicker` — +/- 1-hour step controls, 7-day ceiling enforced, duration label computed
- `CostBreakdown` — rental fee, hold (expandable info), total card impact line items; amounts from cents
- `PaymentMethodSelector` — calls `POST /api/v1/payments/setup`, opens Stripe PaymentSheet
- `checkout.tsx` — KYC gate (stubbed for Phase 4), duration picker, cost breakdown, payment method, "Confirm Booking" → `POST /api/v1/bookings`
- `confirmation.tsx` — success checkmark, booking summary, "Message Host" + "View My Bookings" CTAs
- Wrapped app root in `<StripeProvider>` with `EXPO_PUBLIC_STRIPE_PUBLISHABLE_KEY`
- Installed `@stripe/stripe-react-native` via `npx expo install`
- `npx tsc --noEmit` → exit 0

**Key decisions:**
- Step controls (not calendar library) — avoids 200KB dep for a read-only Phase 2 duration selector
- Client-side rental fee estimate from listing pricing × duration (backend recalculates on booking)
- `router.replace` to confirmation so back button doesn't return to completed checkout
- KYC gate written but inert in Phase 2 (field not in User type yet)

### Task 3.1 — BookingService (backend)
- `ScheduleHostPayout` in PaymentService must be called when a booking transitions to COMPLETED
- `CaptureFromHold` is called by LateReturnAgent and DisputeAgent (Phase 5)
- `ReleaseHold` is called when item is returned in good condition

### Stripe Webhook (deferred)
- A webhook handler at `POST /api/v1/payments/webhook` should be added in Phase 3/4
- Events needed: `payment_intent.succeeded`, `payment_intent.payment_failed`, `account.updated`
- Config field `StripeWebhookSecret` is already wired in config.go
