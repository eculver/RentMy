# Phase 2 — Discovery + Payments / Demand Side Implementation Plan

> **Scope:** Wk 5-8. Renter can find listings nearby (feed/search/map) and pay for them. Checkout shows tiered hold amount clearly.
> **Exit criteria:** Feed returns ranked listings. Search returns keyword matches. Map shows fuzzed pins. Tiered hold calculated, Stripe hold authorized, rental fee charged, host payout scheduled. Hold allocation ledger and guarantee fund active. All via Stripe test mode.
> **Blockers:** Phase 1 complete (UserService, ListingService, MediaService)

## Resolved Decisions

| Question | Answer | Notes |
|----------|--------|-------|
| Drive-time API | OSRM self-hosted | Containerized in docker-compose, no API costs, deterministic results |
| Fulltext search engine | Postgres tsvector | Already indexed in migration 001. No external dependency. Semantic search deferred to Phase 4 |
| Stripe Connect type | Express Connect | Faster host onboarding, less compliance surface than Standard or Custom |
| Map privacy | Fuzzed coordinates (~500m jitter) | Exact coords stored in DB, random offset applied at API layer before response |
| Location search method | PostGIS ST_DWithin + bounding box | Radius for feed, bounding box for map |
| Drive-time caching | Redis (60-min TTL) | Cache OSRM responses keyed by rounded origin-destination grid cell |
| Ranking weights storage | Config vars (env) | Tunable without code change. Start with PRD v8 defaults |
| Checkout KYC gate | Stubbed in Phase 2 | Check `identityStatus == VERIFIED`, block if not. Full KYC flow in Phase 4 |
| Pagination style | Cursor-based (ULID) | More efficient than offset for infinite scroll. Cursor = last ULID in page |
| Hold allocation concurrency | SELECT FOR UPDATE on transaction row | Prevents LateReturnAgent and DisputeAgent from racing on same hold |

---

## Technology Decisions

### Go Backend (New Dependencies)

| Decision | Choice | Import / Module |
|----------|--------|-----------------|
| Stripe SDK | stripe-go v81 | `github.com/stripe/stripe-go/v81` |
| Stripe client | stripe-go v81 client | `github.com/stripe/stripe-go/v81/client` |
| OSRM client | net/http (stdlib) | No SDK needed. OSRM exposes a simple HTTP/JSON API |

**Rationale:**
- **stripe-go v81:** Official Stripe Go SDK. v81 is the latest major version series. Express Connect requires `stripe.AccountTypeExpress` for connected account creation.
- **OSRM over Google Maps API:** Zero API costs, deterministic results for testing, self-hosted so no rate limits. Google Maps can replace OSRM later by swapping the drive-time adapter. OSRM container uses ~500MB RAM with a regional extract.
- **No OSRM Go SDK:** The OSRM HTTP API returns a simple JSON response (`routes[0].duration`). A raw `net/http` call is cleaner than pulling in a third-party wrapper.

### React Native (New Dependencies)

| Decision | Choice | Package |
|----------|--------|---------|
| Maps | react-native-maps (Google provider) | `react-native-maps` |
| Stripe | Stripe React Native SDK | `@stripe/stripe-react-native` |
| Location | expo-location | `expo-location` |
| Bottom sheet (filters) | @gorhom/bottom-sheet | `@gorhom/bottom-sheet` |
| Debounced search | use-debounce | `use-debounce` |

**Rationale:**
- **react-native-maps with Google provider:** Most mature RN map library. Google Maps provider gives consistent cross-platform appearance and free tier covers development. Apple Maps is an alternative but has weaker Android support.
- **@stripe/stripe-react-native:** Official SDK. Handles PCI compliance via Stripe PaymentSheet — no raw card numbers touch the app.
- **expo-location:** Expo-compatible location permissions and foreground GPS. Required for user position on map and feed radius.
- **@gorhom/bottom-sheet:** Standard for filter UIs in RN. Gesture-driven, Reanimated-backed, works with Expo Dev Client.
- **use-debounce:** Tiny utility for debouncing search input (300ms). Prevents API spam on every keystroke.

---

## Implementation Steps

### Step 2.1 — DiscoveryService (backend)

**Create:**
- `backend/internal/discovery/model.go` — Domain types:
  ```go
  // FeedQuery represents parameters for the "nearby" feed.
  type FeedQuery struct {
      Lat           float64 // user latitude
      Lng           float64 // user longitude
      RadiusMeters  int     // default 30000 (30km)
      Cursor        string  // ULID cursor for pagination
      Limit         int     // default 20, max 50
  }

  // SearchQuery represents parameters for keyword search.
  type SearchQuery struct {
      Query         string
      Lat           float64
      Lng           float64
      RadiusMeters  int
      MinPrice      *float64
      MaxPrice      *float64
      MaxDriveMin   *int
      Cursor        string
      Limit         int
  }

  // MapQuery represents parameters for map bounding box search.
  type MapQuery struct {
      SWLat float64 // southwest corner latitude
      SWLng float64 // southwest corner longitude
      NELat float64 // northeast corner latitude
      NELng float64 // northeast corner longitude
      Limit int     // default 100, max 200
  }

  // RankedListing is a listing enriched with ranking data.
  type RankedListing struct {
      Listing         Listing
      DriveTimeMin    float64
      DistanceMeters  float64
      RankScore       float64
      FuzzedLat       float64 // jittered latitude for display
      FuzzedLng       float64 // jittered longitude for display
      Media           []Media
      HostName        string
      HostReputation  int
      HostResponseRate float64
  }
  ```
- `backend/internal/discovery/repository.go` — Postgres queries:
  - `FindNearby(ctx, lat, lng, radiusMeters, cursor, limit)` — `SELECT l.*, u.reputation_score, u.name FROM listings l JOIN users u ON l.host_id = u.id WHERE l.status = 'ACTIVE' AND ST_DWithin(l.location, ST_MakePoint($lng, $lat)::geography, $radius) AND l.id < $cursor ORDER BY l.id DESC LIMIT $limit`
  - `SearchFulltext(ctx, query, lat, lng, radiusMeters, filters, cursor, limit)` — `SELECT ... WHERE l.search_vector @@ plainto_tsquery('english', $query) AND ST_DWithin(...) AND (price_per_day BETWEEN $min AND $max OR price_per_hour BETWEEN $min AND $max) ...`
  - `FindInBoundingBox(ctx, swLat, swLng, neLat, neLng, limit)` — `SELECT ... WHERE l.status = 'ACTIVE' AND ST_Within(l.location::geometry, ST_MakeEnvelope($swLng, $swLat, $neLng, $neLat, 4326))`
  - `GetHostStats(ctx, hostID)` — returns response_rate, on_time_rate, acceptance_rate from transactions table (aggregate queries)
- `backend/internal/discovery/service.go` — Business logic:
  - `Feed(ctx, FeedQuery)` — calls repository.FindNearby, enriches with drive time via OSRM, computes rank scores, sorts descending, applies fuzzed location
  - `Search(ctx, SearchQuery)` — calls repository.SearchFulltext, enriches with drive time, computes rank scores, sorts, fuzzes location
  - `Map(ctx, MapQuery)` — calls repository.FindInBoundingBox, applies fuzzed location. No ranking needed (map is spatial, not ranked)
  - `computeRankScore(listing, driveTimeMin, maxDriveTime, hostReputation, hostStats)` — implements ranking formula:
    ```go
    // Ranking formula from PRD section 13
    // All inputs normalized to [0,1] before weighting
    func (s *Service) computeRankScore(
        availableNow    bool,
        driveTimeMin    float64,
        maxDriveTimeMin float64,
        hostReputation  int,
        responseRate    float64,
        onTimeRate      float64,
        acceptanceRate  float64,
    ) float64 {
        avail := 0.0
        if availableNow {
            avail = 1.0
        }

        proximity := 0.0
        if maxDriveTimeMin > 0 {
            proximity = 1.0 - (driveTimeMin / maxDriveTimeMin)
            if proximity < 0 {
                proximity = 0
            }
        }

        reputation := float64(hostReputation) / 1000.0

        reliability := (responseRate + onTimeRate + acceptanceRate) / 3.0

        return s.cfg.WeightAvailability * avail +
               s.cfg.WeightProximity * proximity +
               s.cfg.WeightReputation * reputation +
               s.cfg.WeightReliability * reliability
    }
    ```
  - `fuzzLocation(lat, lng float64) (float64, float64)` — adds deterministic random jitter (~500m):
    ```go
    // fuzzLocation adds a random offset of ~500m to exact coordinates
    // for privacy. Uses a seeded PRNG based on listing ID so the same
    // listing always gets the same fuzzed location (no jumping on reload).
    func fuzzLocation(lat, lng float64, seed string) (float64, float64) {
        h := fnv.New64a()
        h.Write([]byte(seed)) // seed = listing ID
        rng := rand.New(rand.NewSource(int64(h.Sum64())))

        // ~500m in degrees: 0.0045 latitude, 0.0055 longitude (varies by lat)
        latOffset := (rng.Float64() - 0.5) * 0.009  // +/- ~500m
        lngOffset := (rng.Float64() - 0.5) * 0.011  // +/- ~500m
        return lat + latOffset, lng + lngOffset
    }
    ```
  - `isAvailableNow(availability []TimeSlot) bool` — checks if current time falls within any availability window
  - Filter logic: exclude listings where `status != 'ACTIVE'`; exclude hosts where response_rate < 0.30
- `backend/internal/discovery/drivetime.go` — OSRM client:
  ```go
  // DriveTimeClient fetches drive-time estimates from OSRM.
  type DriveTimeClient struct {
      baseURL    string // e.g. "http://osrm:5000"
      httpClient *http.Client
      cache      *redis.Client
      cacheTTL   time.Duration // default 60 min
  }

  // Estimate returns the drive time in minutes between two points.
  // Response is cached in Redis keyed by rounded origin+destination.
  func (c *DriveTimeClient) Estimate(ctx context.Context, fromLat, fromLng, toLat, toLng float64) (float64, error)

  // OSRM API: GET /route/v1/driving/{lng1},{lat1};{lng2},{lat2}?overview=false
  // Response: {"routes":[{"duration":542.3}]}  (duration in seconds)
  // Convert to minutes: duration / 60.0
  ```
- `backend/internal/discovery/handler.go` — HTTP handlers returning `chi.Router`:
  - `GET /api/v1/discovery/feed` — query params: lat, lng, radius, cursor, limit
  - `GET /api/v1/discovery/search` — query params: q, lat, lng, radius, min_price, max_price, max_drive_min, cursor, limit
  - `GET /api/v1/discovery/map` — query params: sw_lat, sw_lng, ne_lat, ne_lng, limit

**Modify:**
- `backend/cmd/server/main.go` — Mount discovery router at `/api/v1/discovery`, wire DriveTimeClient, pass ranking config
- `backend/internal/platform/config/config.go` — Add fields:
  ```go
  // OSRM
  OSRMBaseURL string `env:"OSRM_BASE_URL" envDefault:"http://localhost:5000"`

  // Ranking weights (PRD section 13 defaults)
  WeightAvailability float64 `env:"RANK_WEIGHT_AVAILABILITY" envDefault:"0.35"`
  WeightProximity    float64 `env:"RANK_WEIGHT_PROXIMITY" envDefault:"0.30"`
  WeightReputation   float64 `env:"RANK_WEIGHT_REPUTATION" envDefault:"0.20"`
  WeightReliability  float64 `env:"RANK_WEIGHT_RELIABILITY" envDefault:"0.15"`

  // Discovery defaults
  DefaultFeedRadiusMeters int `env:"DEFAULT_FEED_RADIUS_METERS" envDefault:"30000"`
  MaxFeedLimit            int `env:"MAX_FEED_LIMIT" envDefault:"50"`
  MaxMapLimit             int `env:"MAX_MAP_LIMIT" envDefault:"200"`
  ```
- `docker-compose.yml` — Add OSRM service:
  ```yaml
  osrm:
    image: osrm/osrm-backend:latest
    ports:
      - "5000:5000"
    volumes:
      - osrm_data:/data
    command: osrm-routed --algorithm mld /data/region.osrm
    healthcheck:
      test: ["CMD-SHELL", "curl -sf http://localhost:5000/nearest/v1/driving/-118.24,34.05 || exit 1"]
      interval: 10s
      timeout: 5s
      retries: 5
  ```
  Add `osrm_data` to volumes section. Note: OSRM requires a pre-processed `.osrm` file. For development, download and process a small regional extract (e.g., California from Geofabrik). Document the data prep in a `scripts/osrm-prepare.sh`:
  ```bash
  #!/bin/bash
  # Download California extract and process for OSRM
  wget -O /data/california.osm.pbf https://download.geofabrik.de/north-america/us/california-latest.osm.pbf
  osrm-extract -p /opt/car.lua /data/california.osm.pbf
  osrm-partition /data/california.osrm
  osrm-customize /data/california.osrm
  ```

**Verify:**
```bash
cd backend && go vet ./...
cd backend && go build ./cmd/server
cd backend && go test ./internal/discovery/... -v -count=1
# Integration test:
docker compose up -d
cd backend && make dev &
sleep 3
# Seed a test listing with known location (Santa Monica):
# (assumes Phase 1 register + create listing works)
curl -sf "http://localhost:8080/api/v1/discovery/feed?lat=34.02&lng=-118.49&radius=10000&limit=5" \
  -H "Authorization: Bearer $TOKEN"
# Should return JSON array of ranked listings with driveTimeMin and fuzzed coords
# Verify fuzzed coords differ from stored coords but are within ~500m
curl -sf "http://localhost:8080/api/v1/discovery/search?q=kayak&lat=34.02&lng=-118.49" \
  -H "Authorization: Bearer $TOKEN"
# Should return keyword matches
curl -sf "http://localhost:8080/api/v1/discovery/map?sw_lat=33.9&sw_lng=-118.6&ne_lat=34.1&ne_lng=-118.3&limit=50" \
  -H "Authorization: Bearer $TOKEN"
# Should return listings within bounding box
kill %1
```

### Step 2.2 — PaymentService (backend)

**Create:**
- `backend/internal/payment/model.go` — Domain types:
  ```go
  // PaymentAdapter defines the interface for payment processing.
  // Stripe is the first implementation. Interface allows swapping to any processor.
  type PaymentAdapter interface {
      // AuthorizeHold places a pre-authorization hold on the renter's payment method.
      AuthorizeHold(ctx context.Context, amount int64, currency string, paymentMethodID string, customerID string) (holdID string, err error)
      // CaptureHold captures a portion of an existing hold.
      CaptureHold(ctx context.Context, holdID string, amount int64) (chargeID string, err error)
      // ReleaseHold cancels/releases an existing hold.
      ReleaseHold(ctx context.Context, holdID string) error
      // ChargeRentalFee charges the rental fee to the renter's payment method.
      ChargeRentalFee(ctx context.Context, amount int64, currency string, paymentMethodID string, customerID string) (chargeID string, err error)
      // PayoutHost transfers funds to the host's connected account.
      PayoutHost(ctx context.Context, amount int64, currency string, hostAccountID string) (payoutID string, err error)
      // Refund issues a refund for a previous charge.
      Refund(ctx context.Context, chargeID string, amount int64) (refundID string, err error)
      // CreateConnectedAccount creates a Stripe Express connected account for a host.
      CreateConnectedAccount(ctx context.Context, email string) (accountID string, onboardingURL string, err error)
      // CreateCustomer creates a Stripe customer for a renter.
      CreateCustomer(ctx context.Context, email string, name string) (customerID string, err error)
  }

  // HoldAllocation tracks how the hold has been allocated.
  // Stored as JSONB in the transactions table.
  type HoldAllocation struct {
      TotalAuthorized    int64 `json:"totalAuthorized"`
      CapturedForLateFees int64 `json:"capturedForLateFees"`
      CapturedForDamage  int64 `json:"capturedForDamage"`
      DamageReserve      int64 `json:"damageReserve"`
      Released           int64 `json:"released"`
      Remaining          int64 `json:"remaining"`
  }

  // BookingInput is the input for creating a new booking.
  type BookingInput struct {
      RenterID        string
      ListingID       string
      PaymentMethodID string
      ScheduledStart  time.Time
      ScheduledEnd    time.Time
  }

  // BookingResult is returned after a successful booking.
  type BookingResult struct {
      TransactionID string
      HoldAmount    int64
      RentalFee     int64
      PlatformFee   int64
      TotalImpact   int64 // holdAmount + rentalFee (what renter sees on card)
  }

  // GuaranteeFundEntry represents a ledger entry in the guarantee fund.
  type GuaranteeFundEntry struct {
      ID            string
      TransactionID string
      EntryType     string // CONTRIBUTION, CLAIM, CARD_RECOVERY, COLLECTIONS_REFERRAL
      Amount        int64  // positive for contributions/recoveries, negative for claims
      BalanceAfter  int64
      CreatedAt     time.Time
  }
  ```
- `backend/internal/payment/stripe.go` — Stripe implementation of PaymentAdapter:
  ```go
  // StripeAdapter implements PaymentAdapter using Stripe Express Connect.
  type StripeAdapter struct {
      client *client.API
  }

  // NewStripeAdapter creates a new StripeAdapter with the given API key.
  // Option pattern: NewStripeAdapter(apiKey string, opts ...Option)
  func NewStripeAdapter(apiKey string, opts ...Option) *StripeAdapter
  ```
  Implementation notes:
  - `AuthorizeHold` uses `paymentintent.New` with `CaptureMethod: stripe.String("manual")` to create an uncaptured PaymentIntent
  - `CaptureHold` uses `paymentintent.Capture` with `AmountToCapture` for partial capture
  - `ReleaseHold` uses `paymentintent.Cancel`
  - `ChargeRentalFee` uses `paymentintent.New` with `CaptureMethod: stripe.String("automatic")`
  - `PayoutHost` uses `transfer.New` to the connected account
  - `CreateConnectedAccount` uses `account.New` with `Type: stripe.AccountTypeExpress`, then `accountlink.New` for onboarding URL
  - `CreateCustomer` uses `customer.New`
  - All amounts in cents (int64). Never use float for money.
- `backend/internal/payment/hold.go` — Tiered hold calculation:
  ```go
  // TieredHold calculates the hold amount based on item value.
  // All amounts in cents.
  //
  // Tier table (PRD section 7):
  //   itemValue <= $500      -> 100% of item value
  //   $501 - $2,000          -> $500 + 25% of (value - $500)
  //   $2,001 - $5,000        -> $875 + 15% of (value - $2,000)
  //   $5,001+                -> $1,325 (hard ceiling)
  func TieredHold(itemValueCents int64) int64 {
      switch {
      case itemValueCents <= 50000: // $500
          return itemValueCents
      case itemValueCents <= 200000: // $2,000
          return 50000 + (itemValueCents-50000)*25/100
      case itemValueCents <= 500000: // $5,000
          return 87500 + (itemValueCents-200000)*15/100
      default:
          return 132500 // $1,325 hard ceiling
      }
  }

  // GuaranteeGap returns the uncovered amount: itemValue - holdAmount.
  // This is the amount the platform guarantee fund covers.
  func GuaranteeGap(itemValueCents, holdAmountCents int64) int64 {
      gap := itemValueCents - holdAmountCents
      if gap < 0 {
          return 0
      }
      return gap
  }

  // RentalFee calculates the rental fee: hostPrice * duration.
  // pricePerHour and pricePerDay are in cents.
  // duration is the rental duration.
  func RentalFee(pricePerHourCents, pricePerDayCents int64, duration time.Duration) int64

  // PlatformFee calculates the platform's take: rentalFee * takeRate.
  func PlatformFee(rentalFeeCents int64, takeRateBPS int) int64

  // HostPayout calculates the host's payout: rentalFee - platformFee.
  func HostPayout(rentalFeeCents, platformFeeCents int64) int64

  // GuaranteeFundContribution calculates contribution: platformFee * guaranteeRate.
  func GuaranteeFundContribution(platformFeeCents int64, guaranteeRateBPS int) int64
  ```
- `backend/internal/payment/repository.go` — Postgres queries:
  - `CreateTransaction(ctx, tx, Transaction)` — INSERT into transactions
  - `GetTransaction(ctx, id)` — SELECT with all fields
  - `UpdateHoldAllocation(ctx, tx, transactionID, HoldAllocation)` — UPDATE with SELECT FOR UPDATE locking:
    ```go
    // UpdateHoldAllocation atomically updates the hold allocation on a transaction.
    // MUST be called within a pgx transaction.
    // Uses SELECT FOR UPDATE to prevent concurrent modification.
    func (r *Repository) UpdateHoldAllocation(ctx context.Context, tx pgx.Tx, transactionID string, alloc HoldAllocation) error
    ```
  - `UpdateTransactionStatus(ctx, transactionID, status)`
  - `InsertGuaranteeFundEntry(ctx, tx, entry)` — INSERT and compute balanceAfter from previous entry
  - `GetGuaranteeFundBalance(ctx)` — SELECT balance_after FROM guarantee_fund_entries ORDER BY created_at DESC LIMIT 1
  - `GetTotalOutstandingGuaranteeGaps(ctx)` — SELECT SUM(guarantee_gap) FROM transactions WHERE status = 'ACTIVE'
  - `GetHostTransactionCount(ctx, hostID)` — count completed transactions for payout delay logic
  - `StoreStripeCustomerID(ctx, userID, customerID)` — stores Stripe customer ID on user (add `stripe_customer_id` column, see migrations)
  - `StoreStripeAccountID(ctx, userID, accountID)` — stores Stripe connected account ID on user (add `stripe_account_id` column, see migrations)
  - `GetStripeCustomerID(ctx, userID)` — retrieve customer ID
  - `GetStripeAccountID(ctx, userID)` — retrieve account ID
- `backend/internal/payment/service.go` — Business logic:
  - `CreateBooking(ctx, BookingInput)` — orchestrates:
    1. Fetch listing to get item value and pricing
    2. Calculate hold amount via `TieredHold(itemValue)`
    3. Calculate rental fee via `RentalFee(price, duration)`
    4. Calculate platform fee, guarantee fund contribution, host payout
    5. Calculate guarantee gap
    6. Authorize hold via `PaymentAdapter.AuthorizeHold`
    7. Charge rental fee via `PaymentAdapter.ChargeRentalFee`
    8. Create transaction record with hold allocation ledger initialized
    9. Create guarantee fund contribution entry
    10. Return BookingResult with all amounts
  - `ReleaseHold(ctx, transactionID)` — release hold on successful return
  - `CaptureFromHold(ctx, transactionID, amount, reason)` — atomic capture with SELECT FOR UPDATE:
    1. Lock transaction row
    2. Read current hold allocation
    3. Validate: amount <= remaining
    4. Update allocation fields based on reason (late_fee or damage)
    5. Call PaymentAdapter.CaptureHold
    6. Commit
  - `ScheduleHostPayout(ctx, transactionID)` — enqueue River job with delay:
    ```go
    // Payout delay logic (PRD section 7):
    // - First 3 transactions for host: 48h delay mandatory
    // - High-risk host (reputation < 200): 48h delay
    // - Established host: same-day (0 delay)
    func (s *Service) payoutDelay(ctx context.Context, hostID string) time.Duration
    ```
  - `GetGuaranteeFundHealth(ctx)` — returns fund balance, outstanding gaps, reserve ratio
  - `OnboardHost(ctx, userID)` — creates Stripe Express connected account, returns onboarding URL
  - `SetupRenterPayment(ctx, userID)` — creates Stripe customer if not exists
- `backend/internal/payment/handler.go` — HTTP handlers returning `chi.Router`:
  - `POST /api/v1/bookings` — create booking (authorize hold + charge rental fee)
  - `GET /api/v1/bookings/:id` — get booking details
  - `GET /api/v1/users/me/bookings` — list renter's bookings (paginated)
  - `POST /api/v1/payments/onboard` — create Stripe Express connected account for host, return onboarding URL
  - `GET /api/v1/payments/onboard/status` — check host Stripe account status
  - `POST /api/v1/payments/setup` — create Stripe customer for renter, return setup intent client secret
  - `GET /api/v1/listings/:id/hold-estimate` — calculate and return tiered hold amount for a listing (no auth required, used by listing detail screen)
- `backend/internal/payment/payout_job.go` — River job for delayed host payout:
  ```go
  // PayoutJobArgs contains the arguments for a delayed host payout job.
  type PayoutJobArgs struct {
      TransactionID string `json:"transactionId"`
      HostAccountID string `json:"hostAccountId"`
      Amount        int64  `json:"amount"`
      Currency      string `json:"currency"`
  }
  ```

**Modify:**
- `backend/cmd/server/main.go` — Mount payment router, wire StripeAdapter, register payout River worker
- `backend/internal/platform/config/config.go` — Add fields:
  ```go
  // Stripe
  StripeSecretKey      string `env:"STRIPE_SECRET_KEY" envDefault:"sk_test_placeholder"`
  StripePublishableKey string `env:"STRIPE_PUBLISHABLE_KEY" envDefault:"pk_test_placeholder"`
  StripeWebhookSecret  string `env:"STRIPE_WEBHOOK_SECRET" envDefault:"whsec_placeholder"`

  // Payment config
  TakeRateBPS         int `env:"TAKE_RATE_BPS" envDefault:"2000"`         // 20% = 2000 basis points
  GuaranteeRateBPS    int `env:"GUARANTEE_RATE_BPS" envDefault:"1000"`    // 10% of platform fee
  DamageReserveRate   int `env:"DAMAGE_RESERVE_RATE_BPS" envDefault:"4000"` // 40% of hold reserved for damage
  PayoutDelayNewHostH int `env:"PAYOUT_DELAY_NEW_HOST_HOURS" envDefault:"48"`
  ```
- `.env.example` — Add `STRIPE_SECRET_KEY`, `STRIPE_PUBLISHABLE_KEY`, `STRIPE_WEBHOOK_SECRET`

**Verify:**
```bash
cd backend && go vet ./...
cd backend && go build ./cmd/server
cd backend && go test ./internal/payment/... -v -count=1
# Tiered hold unit tests (see Testing Strategy)
# Integration test with Stripe test mode:
docker compose up -d
cd backend && make dev &
sleep 3
# Onboard a host:
curl -sf -X POST http://localhost:8080/api/v1/payments/onboard \
  -H "Authorization: Bearer $HOST_TOKEN"
# Should return { accountId, onboardingUrl }
# Setup renter payment:
curl -sf -X POST http://localhost:8080/api/v1/payments/setup \
  -H "Authorization: Bearer $RENTER_TOKEN"
# Should return { customerId, clientSecret }
# Create booking:
curl -sf -X POST http://localhost:8080/api/v1/bookings \
  -H "Authorization: Bearer $RENTER_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"listingId":"LISTING_ID","paymentMethodId":"pm_card_visa","scheduledStart":"2026-04-10T10:00:00Z","scheduledEnd":"2026-04-11T10:00:00Z"}'
# Should return 201 with holdAmount, rentalFee, totalImpact
# Verify hold estimate:
curl -sf "http://localhost:8080/api/v1/listings/LISTING_ID/hold-estimate"
# Should return { holdAmount, itemValue, guaranteeGap }
kill %1
```

### Step 2.3 — Feed screen (RN)

**Create:**
- `mobile/app/(tabs)/(feed)/index.tsx` — Feed screen (replace placeholder):
  - Request location permission on first mount via `expo-location`
  - Fetch user's current position
  - Call `GET /api/v1/discovery/feed?lat={lat}&lng={lng}`
  - Display results in `FlatList` with `ListingFeedCard` component
  - Pull-to-refresh via `RefreshControl`
  - Infinite scroll: `onEndReached` fetches next page using cursor (last listing ID)
  - Empty state: "No listings nearby" with illustration
  - Loading skeleton while data fetches
- `mobile/components/listing/ListingFeedCard.tsx` — Feed card component:
  - Thumbnail image (first media item)
  - Title, price (per hour/day), drive time estimate ("12 min drive")
  - Host name + reputation badge
  - Trust signals row (verified host icon, number of completions)
  - Tap navigates to listing detail: `router.push(/listing/${id})`
- `mobile/lib/hooks/useDiscovery.ts` — TanStack Query hooks:
  - `useFeed(lat, lng)` — useInfiniteQuery with cursor pagination
  - `useSearch(query, lat, lng, filters)` — useInfiniteQuery with debounced query
  - `useMapListings(bounds)` — useQuery for bounding box
  - `useHoldEstimate(listingId)` — useQuery for hold amount
- `mobile/lib/hooks/useLocation.ts` — Hook wrapping expo-location:
  - Requests `Accuracy.Balanced` foreground permission
  - Returns `{ lat, lng, loading, error }`
  - Caches last known position in Zustand store

**Install:**
```bash
cd mobile && npx expo install expo-location
```

**Modify:**
- `mobile/app/(tabs)/_layout.tsx` — Update feed tab icon to use a home/list icon

**Verify:**
```bash
cd mobile && npx tsc --noEmit
cd mobile && npx eslint .
```

### Step 2.4 — Search screen (RN)

**Modify:**
- `mobile/app/(tabs)/(search)/index.tsx` — Search screen (replace placeholder):
  - Search bar with `TextInput` at top, debounced input (300ms) via `use-debounce`
  - Results list using same `ListingFeedCard` component as feed
  - Filter button opens bottom sheet (`@gorhom/bottom-sheet`)
  - Empty state: "Search for anything nearby"
  - No-results state: "No results for '{query}'"
  - Infinite scroll pagination with cursor

**Create:**
- `mobile/components/search/FilterSheet.tsx` — Bottom sheet with filters:
  - Drive time range slider (5-60 min)
  - Price range slider (min/max)
  - Duration filter (hourly/daily)
  - "Apply" button closes sheet and triggers refetch
  - "Reset" button clears all filters
- `mobile/lib/stores/searchStore.ts` — Zustand store for search state:
  - `query: string`
  - `filters: { maxDriveMin, minPrice, maxPrice, durationType }`
  - Actions: `setQuery`, `setFilters`, `resetFilters`

**Install:**
```bash
cd mobile && npx expo install @gorhom/bottom-sheet react-native-reanimated react-native-gesture-handler
cd mobile && npm install use-debounce
```

**Verify:**
```bash
cd mobile && npx tsc --noEmit
cd mobile && npx eslint .
```

### Step 2.5 — Map screen (RN)

**Modify:**
- `mobile/app/(tabs)/(map)/index.tsx` — Map screen (replace placeholder):
  - Full-screen `MapView` from `react-native-maps` with Google Maps provider
  - Centered on user's current location
  - `Marker` components for each listing with fuzzed coordinates
  - Custom marker icon (small price tag or category icon)
  - On map region change: refetch listings for new bounding box
  - Debounce region change (500ms) to prevent API spam during panning

**Create:**
- `mobile/components/map/ListingMarker.tsx` — Custom map marker:
  - Small price pill showing `$XX/day`
  - Different color for available now vs scheduled-only
- `mobile/components/map/ListingPreviewCard.tsx` — Bottom card shown on marker tap:
  - Photo, title, price, drive time, host info
  - Tap card navigates to listing detail
  - Swipe down to dismiss
- `mobile/components/map/MapCallout.tsx` — Callout wrapper for marker interaction

**Install:**
```bash
cd mobile && npx expo install react-native-maps
```

**Verify:**
```bash
cd mobile && npx tsc --noEmit
cd mobile && npx eslint .
```

### Step 2.6 — Listing detail screen (RN)

**Create:**
- `mobile/app/(tabs)/(feed)/listing/[id].tsx` — Listing detail screen (deep-linkable route):
  - Photo gallery: horizontal `FlatList` with pagination dots, swipeable
  - Title + description
  - Host info card: avatar, name, reputation score, member since, response rate
  - Price display: hourly and/or daily rate
  - Drive time estimate (from user's current location)
  - **Hold amount display:** fetches `GET /api/v1/listings/:id/hold-estimate`, shows tiered hold amount with explanation text: "A temporary hold of $XX will be placed on your card and released when the item is returned in good condition"
  - Availability calendar: read-only calendar view showing available time slots
  - "Rent Now" button (fixed at bottom): navigates to checkout screen
  - If user is the host: show "Edit Listing" instead of "Rent Now"
- `mobile/components/listing/PhotoGallery.tsx` — Swipeable photo gallery with pagination indicators
- `mobile/components/listing/HostInfoCard.tsx` — Host profile card with trust signals
- `mobile/components/listing/HoldExplainer.tsx` — Hold amount display with breakdown:
  - Shows hold tier: "Based on item value of $X, your hold is $Y"
  - Explains: "This is a temporary authorization, not a charge. Released on return."
  - If guarantee gap > 0: "The remaining $Z is covered by RentMy Protection"
- `mobile/components/listing/AvailabilityCalendar.tsx` — Simple calendar showing available dates/times
- `mobile/lib/hooks/useListing.ts` — TanStack Query hook for single listing detail

**Verify:**
```bash
cd mobile && npx tsc --noEmit
cd mobile && npx eslint .
```

### Step 2.7 — Checkout screen (RN)

**Create:**
- `mobile/app/(tabs)/(feed)/listing/[id]/checkout.tsx` — Checkout screen:
  - KYC gate: if `user.identityStatus !== 'VERIFIED'`, show blocker with message "Please verify your identity to rent items" and CTA to verify (stubbed — navigates to placeholder KYC screen)
  - Duration selector: date/time pickers for start and end, validates <=7 day ceiling
  - **Cost breakdown:**
    - Rental fee: `hostPrice * duration` (displayed clearly)
    - Hold amount: tiered hold (labeled "Temporary hold, released on return")
    - Total card impact: `rentalFee + holdAmount` (bold, prominent)
  - Payment method section: Stripe PaymentSheet integration
    - Uses `@stripe/stripe-react-native` `PaymentSheet`
    - Fetches setup intent client secret from `POST /api/v1/payments/setup`
    - Shows saved payment methods or "Add payment method"
  - "Confirm Booking" button:
    - Calls `POST /api/v1/bookings` with listing ID, payment method, schedule
    - Shows loading spinner during API call
    - On success: navigate to booking confirmation screen
    - On failure: show error (insufficient funds, card declined, etc.)
- `mobile/app/(tabs)/(feed)/listing/[id]/confirmation.tsx` — Booking confirmation screen:
  - Success checkmark animation
  - Booking summary: item, dates, amounts
  - "Message Host" CTA
  - "View My Bookings" CTA
- `mobile/components/checkout/CostBreakdown.tsx` — Cost breakdown display:
  - Line items: rental fee, hold amount, total
  - Hold amount row has info icon that expands explanation
- `mobile/components/checkout/DurationPicker.tsx` — Start/end date-time picker:
  - Validates against listing availability
  - Enforces 7-day ceiling
  - Shows duration in hours/days
- `mobile/components/checkout/PaymentMethodSelector.tsx` — Stripe payment method UI:
  - Uses Stripe PaymentSheet for PCI-compliant card collection
  - Shows last 4 digits of saved card if available
- `mobile/lib/stores/checkoutStore.ts` — Zustand store for checkout flow:
  - `scheduledStart`, `scheduledEnd`, `paymentMethodId`
  - `holdAmount`, `rentalFee`, `totalImpact` (computed)
  - Actions: `setSchedule`, `setPaymentMethod`, `reset`

**Install:**
```bash
cd mobile && npx expo install @stripe/stripe-react-native
```

**Modify:**
- `mobile/app/_layout.tsx` — Wrap app in `StripeProvider` with publishable key from env/config

**Verify:**
```bash
cd mobile && npx tsc --noEmit
cd mobile && npx eslint .
# Manual test on simulator:
# 1. Navigate to a listing detail
# 2. Tap "Rent Now"
# 3. Select dates (verify 7-day ceiling enforced)
# 4. Verify cost breakdown shows correct amounts
# 5. Use Stripe test card (4242 4242 4242 4242) for payment
# 6. Confirm booking — verify success screen
```

---

## API Endpoints

| Method | Path | Auth | Request Body / Query | Response | Errors |
|--------|------|------|---------------------|----------|--------|
| GET | `/api/v1/discovery/feed` | Yes | `?lat=&lng=&radius=&cursor=&limit=` | `{listings: RankedListing[], nextCursor}` | 400 missing lat/lng |
| GET | `/api/v1/discovery/search` | Yes | `?q=&lat=&lng=&radius=&min_price=&max_price=&max_drive_min=&cursor=&limit=` | `{listings: RankedListing[], nextCursor}` | 400 missing q |
| GET | `/api/v1/discovery/map` | Yes | `?sw_lat=&sw_lng=&ne_lat=&ne_lng=&limit=` | `{listings: RankedListing[]}` | 400 missing bounds |
| GET | `/api/v1/listings/:id/hold-estimate` | No | — | `{holdAmount, itemValue, guaranteeGap, tiers}` | 404 listing not found |
| POST | `/api/v1/bookings` | Yes | `{listingId, paymentMethodId, scheduledStart, scheduledEnd}` | `{transaction, holdAmount, rentalFee, platformFee, totalImpact}` | 400 validation, 400 >7day, 402 card declined, 403 not verified, 404 listing not found, 409 listing unavailable |
| GET | `/api/v1/bookings/:id` | Yes | — | `{transaction, listing, host}` | 403 not participant, 404 |
| GET | `/api/v1/users/me/bookings` | Yes | `?status=&cursor=&limit=` | `{bookings[], nextCursor}` | — |
| POST | `/api/v1/payments/onboard` | Yes | — | `{accountId, onboardingUrl}` | 409 already onboarded |
| GET | `/api/v1/payments/onboard/status` | Yes | — | `{accountId, chargesEnabled, payoutsEnabled, detailsSubmitted}` | 404 no account |
| POST | `/api/v1/payments/setup` | Yes | — | `{customerId, clientSecret}` | — |

---

## Database Migrations

### Migration 002: Add Stripe columns and search trigger

The users table needs columns for Stripe customer and connected account IDs. Also add a trigger to auto-update the `search_vector` column on listings.

**Create:** `backend/migrations/002_stripe_and_search.sql`

```sql
-- +goose Up

-- Stripe integration columns
ALTER TABLE users ADD COLUMN stripe_customer_id TEXT;
ALTER TABLE users ADD COLUMN stripe_account_id TEXT;
CREATE UNIQUE INDEX idx_users_stripe_customer_id ON users(stripe_customer_id) WHERE stripe_customer_id IS NOT NULL;
CREATE UNIQUE INDEX idx_users_stripe_account_id ON users(stripe_account_id) WHERE stripe_account_id IS NOT NULL;

-- Auto-update search_vector on listing insert/update.
-- Combines title (weight A), description (weight B), and ai_generated_tags (weight C).
CREATE OR REPLACE FUNCTION listings_search_vector_update() RETURNS trigger AS $$
BEGIN
    NEW.search_vector :=
        setweight(to_tsvector('english', COALESCE(NEW.title, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(NEW.description, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(
            (SELECT string_agg(tag, ' ') FROM jsonb_array_elements_text(NEW.ai_generated_tags) AS tag),
            ''
        )), 'C');
    RETURN NEW;
END
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_listings_search_vector
    BEFORE INSERT OR UPDATE OF title, description, ai_generated_tags ON listings
    FOR EACH ROW
    EXECUTE FUNCTION listings_search_vector_update();

-- Backfill existing listings (if any)
UPDATE listings SET search_vector =
    setweight(to_tsvector('english', COALESCE(title, '')), 'A') ||
    setweight(to_tsvector('english', COALESCE(description, '')), 'B') ||
    setweight(to_tsvector('english', COALESCE(
        (SELECT string_agg(tag, ' ') FROM jsonb_array_elements_text(ai_generated_tags) AS tag),
        ''
    )), 'C');

-- +goose Down
DROP TRIGGER IF EXISTS trg_listings_search_vector ON listings;
DROP FUNCTION IF EXISTS listings_search_vector_update();
ALTER TABLE users DROP COLUMN IF EXISTS stripe_account_id;
ALTER TABLE users DROP COLUMN IF EXISTS stripe_customer_id;
```

---

## Risks and Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| OSRM data preparation complexity | Delayed development setup | Provide pre-built `scripts/osrm-prepare.sh`. For CI/tests, mock OSRM responses. Consider a small test extract (city-level) for fast iteration |
| OSRM container RAM usage (~500MB+) | Local dev resource pressure | Use smallest possible extract (single city). Document `OSRM_ENABLED=false` env var to disable drive-time enrichment and fall back to haversine distance |
| Stripe test mode API latency | Slow integration tests | Mock Stripe adapter for unit tests. Only hit real Stripe in integration/E2E tests. Stripe test mode is fast but adds ~200ms per call |
| Stripe Express onboarding UX | Host drop-off during onboarding | Onboarding is Stripe-hosted (redirect). Store incomplete state and prompt to resume. Track onboarding completion rate |
| Tiered hold calculation rounding errors | Incorrect hold amounts | All money as int64 cents. Never float. Extensive table-driven unit tests for every tier boundary |
| Race condition on hold allocation | Over-capture from hold | SELECT FOR UPDATE with pgx transaction. Comprehensive test for concurrent capture attempts |
| Fuzzed location consistency | Pin jumps on map reload | Seed PRNG with listing ID. Same listing always gets same fuzzed position |
| PostGIS query performance at scale | Slow feed/search/map | GIST index exists. Add EXPLAIN ANALYZE to test suite for geo queries. ST_DWithin is index-aware |
| tsvector missing ai_generated_tags | Incomplete search results | Trigger on INSERT and UPDATE of title/description/ai_generated_tags ensures vector stays current. Backfill in migration |
| 7-day hold expiry | Hold released before return | Enforce 7-day ceiling at listing creation and booking creation. Block bookings >7 days. Stripe hold auto-expires at 7 days |
| react-native-maps native module | Expo Dev Client rebuild required | Document `npx expo prebuild` step. Google Maps API key needed for Android |
| Location permission denial | Feed/map non-functional | Graceful degradation: show search-only mode. Explain why location is needed. Re-prompt on next app open |

---

## Testing Strategy

### Tiered Hold Calculation Tests

Table-driven unit tests covering every tier boundary and edge case in `backend/internal/payment/hold_test.go`:

```go
func TestTieredHold(t *testing.T) {
    tests := []struct {
        name           string
        itemValueCents int64
        wantHoldCents  int64
    }{
        {"zero value", 0, 0},
        {"$1 item", 100, 100},
        {"$500 item (tier 1 ceiling)", 50000, 50000},
        {"$501 item (tier 2 start)", 50100, 50025},           // 500 + 0.25 * 1 = 500.25
        {"$1000 item", 100000, 62500},                         // 500 + 0.25 * 500 = 625
        {"$2000 item (tier 2 ceiling)", 200000, 87500},        // 500 + 0.25 * 1500 = 875
        {"$2001 item (tier 3 start)", 200100, 87515},          // 875 + 0.15 * 1 = 875.15
        {"$3000 item", 300000, 102500},                        // 875 + 0.15 * 1000 = 1025
        {"$5000 item (tier 3 ceiling)", 500000, 132500},       // 875 + 0.15 * 3000 = 1325
        {"$5001 item (hard ceiling)", 500100, 132500},         // capped at 1325
        {"$10000 item (hard ceiling)", 1000000, 132500},       // capped at 1325
        {"$50000 item (hard ceiling)", 5000000, 132500},       // capped at 1325
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := TieredHold(tt.itemValueCents)
            if got != tt.wantHoldCents {
                t.Errorf("TieredHold(%d) = %d, want %d", tt.itemValueCents, got, tt.wantHoldCents)
            }
        })
    }
}
```

### Ranking Formula Tests

Unit tests in `backend/internal/discovery/service_test.go`:

```go
func TestComputeRankScore(t *testing.T) {
    tests := []struct {
        name          string
        availableNow  bool
        driveTimeMin  float64
        maxDriveMin   float64
        hostReputation int
        responseRate  float64
        onTimeRate    float64
        acceptRate    float64
        wantMin       float64 // score should be >= this
        wantMax       float64 // score should be <= this
    }{
        {"available, close, great host", true, 5, 30, 900, 0.95, 0.98, 0.90, 0.85, 1.0},
        {"unavailable, far, new host", false, 25, 30, 0, 0.50, 0.50, 0.50, 0.0, 0.15},
        {"available, far, average host", true, 28, 30, 500, 0.70, 0.70, 0.70, 0.35, 0.55},
        {"max drive time zero (edge)", true, 0, 0, 500, 0.80, 0.80, 0.80, 0.0, 1.0},
    }
    // ... test implementation validates score falls within [wantMin, wantMax]
}
```

### Guarantee Fund Ledger Tests

- Verify double-entry: every CONTRIBUTION and CLAIM updates `balanceAfter` correctly
- Verify reserve ratio calculation: `fundBalance / totalOutstandingGuaranteeGaps`
- Verify fund cannot go negative: claim that exceeds balance is clamped

### Hold Allocation Concurrency Tests

- Test concurrent CaptureFromHold calls on same transaction: only one should succeed, other should see updated `remaining`
- Test damage reserve enforcement: late fee capture cannot exceed `holdAmount * (1 - damageReserveRate)`
- Test that `remaining` is always non-negative

### Stripe Test Mode Configuration

- All tests use Stripe test mode keys (`sk_test_...`, `pk_test_...`)
- Test cards: `pm_card_visa` (success), `pm_card_visa_chargeDeclined` (decline)
- Unit tests mock `PaymentAdapter` interface — no real Stripe calls
- Integration tests hit Stripe test API — requires `STRIPE_SECRET_KEY` env var
- CI runs unit tests only (no Stripe key needed). Integration tests run locally or in a separate CI stage with secrets

### Discovery Tests

- Feed returns only `ACTIVE` listings
- Feed excludes hosts with response_rate < 0.30
- Search uses tsvector: "kayak" matches listing with title "Ocean Kayak"
- Map returns listings within bounding box only
- Fuzzed coordinates differ from stored coordinates by ~500m
- Fuzzed coordinates are deterministic (same listing ID = same fuzz)
- Pagination cursor works correctly: no duplicates, no gaps

### RN Tests

- TypeScript check passes: `npx tsc --noEmit`
- ESLint passes: `npx eslint .`
- Manual verification on iOS simulator:
  - Feed loads with real data from backend
  - Search returns results and filters work
  - Map shows pins with fuzzed locations
  - Listing detail shows correct hold amount
  - Checkout calculates correct totals
  - Stripe PaymentSheet opens with test card

---

## Implementation Order

| Step | What | Week | Depends On |
|------|------|------|------------|
| 2.1 | DiscoveryService (backend: feed, search, map, ranking, OSRM, fuzz) | Wk 5-6 | Phase 1 complete (ListingService, UserService) |
| 2.2 | PaymentService (backend: Stripe, tiered holds, hold allocation, guarantee fund, bookings) | Wk 5-6 | Phase 1 complete (ListingService, UserService) |
| 2.3 | Feed screen (RN) | Wk 6-7 | 2.1 (needs discovery API) |
| 2.4 | Search screen (RN) | Wk 6-7 | 2.1 (needs search API) |
| 2.5 | Map screen (RN) | Wk 7 | 2.1 (needs map API) |
| 2.6 | Listing detail screen (RN) | Wk 7 | 2.1, 2.2 (needs hold estimate endpoint) |
| 2.7 | Checkout screen (RN) | Wk 7-8 | 2.2, 2.6 (needs payment API + listing detail navigation) |

Steps 2.1 and 2.2 are independent of each other and can be developed in parallel (both are backend-only).
Steps 2.3, 2.4, 2.5 are independent of each other (all depend on 2.1 only) and can be parallelized.
Step 2.6 depends on both 2.1 and 2.2 (needs discovery data + hold estimate).
Step 2.7 depends on 2.2 and 2.6 (needs payment flow + listing detail to navigate from).
