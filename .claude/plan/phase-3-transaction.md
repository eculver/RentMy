# Phase 3 — Core Transaction Loop Implementation Plan

> **Scope:** Wk 7-10. Full booking lifecycle: request, accept, handoff, return. Minimum viable transaction.
> **Exit criteria:** Book, handoff, return loop works end-to-end. Complete state machine enforced. Fraud velocity rules block bad bookings. 7-day ceiling enforced. Angle-enforced photos at check-in/check-out. Push notifications fire. In-app messaging works. Phase 4 is unblocked.
> **Blockers:** Phase 2 complete (DiscoveryService, PaymentService, Stripe Connect)

## Resolved Decisions

| Question | Answer | Notes |
|----------|--------|-------|
| State machine implementation | Explicit allowed-transitions map in Go | `map[Status][]Status` — no library. Simple, testable, zero dependencies |
| Push notification provider | Expo Push Notifications | Handles iOS+Android token routing. Go SDK: `exponent-server-sdk-golang` |
| SMS provider | Twilio | PIN fallback delivery. Go SDK: `twilio-go` |
| PIN generation | `crypto/rand` for 4-digit code | Cryptographically secure. No `math/rand` |
| PIN validity window | 1 hour from generation | Configurable via env var |
| Chat UI library | Custom FlatList | No Gifted Chat — reduces dependency surface, full control over UX |
| Real-time transport | Pusher (already integrated Phase 0) | `pusher-js` on mobile for message + status events |
| Auto-decline timeout | 2 hours (configurable) | River delayed job. Platform-level tunable |
| GPS proximity threshold | 100 meters | PRD section 12. Haversine distance calculation |
| Notification storage | Postgres table + in-app read/unread | Push is fire-and-forget; Postgres is source of truth |
| Quiet hours enforcement | Server-side (NotificationService) | Check user preferences before dispatching push |
| Late fee damage reserve | 40% of hold amount protected | LateReturnAgent cannot capture beyond `hold * 0.6` |

---

## Technology Decisions

### Go Backend (New Dependencies)

| Decision | Choice | Import / Module |
|----------|--------|-----------------|
| Expo Push SDK | oliveroneill/exponent-server-sdk-golang | `github.com/oliveroneill/exponent-server-sdk-golang/sdk` |
| Twilio SDK | twilio/twilio-go | `github.com/twilio/twilio-go` |
| Haversine distance | Manual implementation | ~15 lines of Go using `math` stdlib — no library needed |
| PIN generation | crypto/rand (stdlib) | `crypto/rand` + `math/big` for uniform random digit |

**Rationale:**
- **exponent-server-sdk-golang over raw HTTP:** Handles Expo push receipt checking, chunked delivery (100 per batch), and error categorization (DeviceNotRegistered, InvalidCredentials, etc.). Thin wrapper — inspectable source.
- **twilio-go over raw HTTP:** Official SDK with built-in retry, request signing, and response parsing. SMS is safety-critical (handoff fallback) — don't roll your own.
- **Haversine over PostGIS ST_Distance:** Proximity checks are point-to-point distance between two known coordinates. Haversine is 15 lines, zero dependencies, and avoids a database round-trip for a pure math operation. PostGIS is for spatial queries (Phase 2 discovery), not pairwise distance.

### React Native (New Dependencies)

| Decision | Choice | Package |
|----------|--------|---------|
| Real-time client | Pusher JS | `pusher-js` |
| Push notifications | Expo Notifications | `expo-notifications` |
| Deep linking (maps) | Expo Linking | `expo-linking` (already available via Expo) |

---

## Booking State Machine

Every booking has exactly one status. Only the transitions below are valid — all others are rejected at the service layer with a `400 Invalid Transition` error.

### Transition Table

| Current State | Allowed Next States | Trigger | Guard Conditions |
|---------------|-------------------|---------|------------------|
| `REQUESTED` | `ACCEPTED` | Host accepts | Host is the listing owner |
| `REQUESTED` | `DECLINED` | Host declines | Host is the listing owner |
| `REQUESTED` | `AUTO_DECLINED` | River timeout job fires | Host has not responded within timeout window |
| `REQUESTED` | `CANCELLED` | Renter cancels | Renter is the booking requester; cancellation before host response |
| `ACCEPTED` | `ACTIVE` | Both parties complete check-in | GPS proximity verified + PIN accepted + check-in photos from BOTH parties |
| `ACCEPTED` | `CANCELLED` | Either party cancels | Cancellation fee calculated per policy (PRD section 18) |
| `ACTIVE` | `COMPLETED` | Both parties complete check-out | GPS proximity verified + return photos from BOTH parties |
| `ACTIVE` | `DISPUTED` | Either party reports issue | Dispute report submitted by either party |
| `DISPUTED` | `COMPLETED` | Dispute resolved | DisputeAgent or human reviewer issued final decision; all financial operations executed |

### Terminal States

`COMPLETED`, `DECLINED`, `AUTO_DECLINED`, `CANCELLED` — no transitions out.

### Go Implementation

```go
// internal/booking/statemachine.go

type Status string

const (
    StatusRequested    Status = "REQUESTED"
    StatusAccepted     Status = "ACCEPTED"
    StatusDeclined     Status = "DECLINED"
    StatusAutoDeclined Status = "AUTO_DECLINED"
    StatusActive       Status = "ACTIVE"
    StatusCompleted    Status = "COMPLETED"
    StatusDisputed     Status = "DISPUTED"
    StatusCancelled    Status = "CANCELLED"
)

// AllowedTransitions defines every valid state change.
// Any transition not in this map is rejected.
var AllowedTransitions = map[Status][]Status{
    StatusRequested: {StatusAccepted, StatusDeclined, StatusAutoDeclined, StatusCancelled},
    StatusAccepted:  {StatusActive, StatusCancelled},
    StatusActive:    {StatusCompleted, StatusDisputed},
    StatusDisputed:  {StatusCompleted},
    // Terminal states have no outgoing transitions:
    // StatusCompleted, StatusDeclined, StatusAutoDeclined, StatusCancelled
}

func CanTransition(from, to Status) bool {
    allowed, ok := AllowedTransitions[from]
    if !ok {
        return false
    }
    for _, s := range allowed {
        if s == to {
            return true
        }
    }
    return false
}
```

---

## Fraud Velocity Rules (PRD Section 9)

These are deterministic rule checks enforced before booking confirmation — not AI agent decisions. They run in the BookingService as middleware/preconditions.

### Rules

| Rule | Check | Action |
|------|-------|--------|
| New-to-new lockout | Both renter AND host accounts are < 30 days old | Block booking. Error: "One party must have an established account (>30 days)" |
| First-3 payout delay | Either party has < 3 completed transactions | Flag transaction for 48-hour payout delay (stored on transaction record) |
| Damage claim cap | Host has received > $X in damage claims within first 60 days of account creation | Block booking. Error: "Account under review — damage claim threshold exceeded" |

### Implementation

```go
// internal/booking/fraud.go

type FraudVelocityConfig struct {
    NewAccountThresholdDays   int           // default: 30
    FirstNTransactions        int           // default: 3
    PayoutDelayDuration       time.Duration // default: 48h
    DamageClaimCapAmount      float64       // tunable, e.g., 500.00
    DamageClaimCapWindowDays  int           // default: 60
}

// CheckFraudVelocity runs all velocity rules before booking confirmation.
// Returns a FraudResult with block/delay decisions.
func (s *Service) CheckFraudVelocity(ctx context.Context, renter, host User, cfg FraudVelocityConfig) (FraudResult, error)
```

---

## Cancellation Fee Calculation (PRD Section 18)

### Renter Cancels

| Timing | Fee |
|--------|-----|
| > 2 hours before scheduled pickup | No fee |
| 1-2 hours before scheduled pickup | 25% of rental fee |
| < 1 hour before scheduled pickup | 50% of rental fee |
| After host has confirmed and is waiting (ACCEPTED, past scheduled start) | 100% of rental fee |

### Host Cancels

| Timing | Fee | Additional |
|--------|-----|------------|
| > 2 hours before scheduled pickup | No fee | Warning tracked on host record |
| < 2 hours before scheduled pickup | Tunable % of rental fee | Ranking hit |
| After renter is en route (past scheduled start) | Higher fee (tunable) | Ranking hit + suspension risk |
| Repeated cancellations | Progressive fee increase | Progressive suspension |

---

## PIN Generation and Proximity Verification Flow

### PIN Generation (on booking acceptance)

1. Host accepts booking -> BookingService transitions to `ACCEPTED`
2. BookingService calls ProximityService.GeneratePIN(transactionID)
3. ProximityService generates 4-digit PIN using `crypto/rand`:
   ```go
   func GeneratePIN() (string, error) {
       n, err := rand.Int(rand.Reader, big.NewInt(10000))
       if err != nil {
           return "", err
       }
       return fmt.Sprintf("%04d", n.Int64()), nil
   }
   ```
4. PIN stored in `proximity_proofs` table with `proof_type = CHECK_IN`, `verified = false`
5. PIN sent to HOST only (via push notification + in-app display)
6. PIN expires after 1 hour (configurable). If expired, host can regenerate

### Check-in Flow

1. Both parties arrive at pickup location
2. Renter opens check-in screen -> app sends GPS coordinates to `POST /api/v1/proximity/verify`
3. Server calculates Haversine distance between renter GPS and host GPS (or listing location)
4. If distance <= 100m -> GPS verified, stored in proximity proof
5. Host verbally tells renter the 4-digit PIN
6. Renter enters PIN on check-in screen -> `POST /api/v1/proximity/pin`
7. Server validates PIN matches + not expired
8. Both parties capture check-in photos (angle-enforced, min 3 photos, >= 30 degrees apart)
9. Photos uploaded via MediaService, linked to transaction as `CHECK_IN` media
10. When ALL conditions met (GPS + PIN + photos from both parties): BookingService transitions `ACCEPTED -> ACTIVE`, sets `actual_start`

### Check-out Flow

1. Both parties meet for return
2. GPS proximity verification repeated (same 100m threshold)
3. Both parties capture return photos (same angle enforcement as check-in)
4. Photos uploaded as `CHECK_OUT` media
5. When ALL conditions met (GPS + return photos from both parties): BookingService transitions `ACTIVE -> COMPLETED`, sets `actual_end`

### SMS Fallback

If either party's app is unreachable during handoff:
1. Host requests SMS fallback -> `POST /api/v1/proximity/sms-fallback`
2. ProximityService sends PIN to renter's phone via Twilio SMS
3. Renter confirms PIN verbally to host
4. Host enters confirmation on their device
5. Photos captured and uploaded once connectivity restores
6. ProximityProof record created with `method = SMS_FALLBACK`

---

## Implementation Steps

### Step 3.1 — BookingService (backend)

**Create:**
- `backend/internal/booking/model.go` — Booking domain type, CreateBookingInput, AcceptInput, CancelInput, status constants
- `backend/internal/booking/statemachine.go` — AllowedTransitions map, CanTransition function, transition validation with error messages
- `backend/internal/booking/fraud.go` — FraudVelocityConfig, CheckFraudVelocity (new-to-new lockout, first-3 delay, damage cap), FraudResult type
- `backend/internal/booking/cancellation.go` — CalculateCancellationFee (renter policy, host policy per PRD section 18)
- `backend/internal/booking/repository.go` — Insert, FindByID, FindByRenterID, FindByHostID, FindByListingID, UpdateStatus (with row-level locking for concurrent transitions), UpdateCancellation
- `backend/internal/booking/service.go` — Business logic: CreateBooking (validate listing active, validate 7-day ceiling, run fraud velocity checks, check listing availability), Accept (transition REQUESTED->ACCEPTED, trigger PaymentService hold+charge, schedule auto-decline cancellation), Decline, Cancel (calculate fee, apply), GetBooking, ListByUser
- `backend/internal/booking/handler.go` — HTTP handlers returning `chi.Router`:
  - `POST /api/v1/bookings`
  - `GET /api/v1/bookings/:id`
  - `GET /api/v1/users/me/bookings`
  - `POST /api/v1/bookings/:id/accept`
  - `POST /api/v1/bookings/:id/decline`
  - `POST /api/v1/bookings/:id/cancel`
- `backend/internal/booking/autodecline_job.go` — River job: AutoDeclineArgs{TransactionID}, fires after configurable timeout, transitions REQUESTED->AUTO_DECLINED if still in REQUESTED state, notifies renter

**Modify:**
- `backend/cmd/server/main.go` — Mount booking router, register AutoDecline River worker
- `backend/internal/platform/config/config.go` — Add `AutoDeclineTimeout`, `FraudNewAccountDays`, `FraudFirstNTransactions`, `FraudPayoutDelay`, `FraudDamageClaimCap`, `FraudDamageClaimWindowDays`, `HostCancelFeePercent`, `DamageReserveRate`

**Verify:**
```bash
cd backend && go vet ./...
cd backend && go build ./cmd/server
cd backend && go test ./internal/booking/... -v -count=1
# State machine tests: every valid transition succeeds, every invalid rejected
# Fraud velocity tests: new-to-new blocked, first-3 flagged, damage cap enforced
# Cancellation fee tests: all timing brackets produce correct fees
# 7-day ceiling test: bookings >168h rejected
# Integration test:
docker compose up -d
cd backend && make dev &
sleep 3
# Create booking
curl -sf -X POST http://localhost:8080/api/v1/bookings \
  -H "Authorization: Bearer $RENTER_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"listingId":"<id>","scheduledStart":"...","scheduledEnd":"..."}'
# Accept booking
curl -sf -X POST http://localhost:8080/api/v1/bookings/<id>/accept \
  -H "Authorization: Bearer $HOST_TOKEN"
# Should return 200 with status=ACCEPTED
kill %1
```

### Step 3.2 — ProximityService (backend)

**Create:**
- `backend/internal/proximity/model.go` — ProximityProof domain type, VerifyGPSInput, VerifyPINInput, ProofType (CHECK_IN, CHECK_OUT), Method (GPS, BLE, SMS_FALLBACK)
- `backend/internal/proximity/pin.go` — GeneratePIN (crypto/rand 4-digit), ValidatePIN (check match + expiry)
- `backend/internal/proximity/distance.go` — Haversine function (two lat/lng pairs -> meters), IsWithinThreshold (distance <= configurable threshold)
- `backend/internal/proximity/repository.go` — Insert, FindByTransactionID, FindByTransactionAndType, UpdateVerified
- `backend/internal/proximity/service.go` — GenerateCheckInPIN (called on booking accept, stores proof record), VerifyGPS (calculate distance, update proof), VerifyPIN (validate match + not expired), CheckHandoffComplete (both parties GPS verified + PIN + photos), SMSFallback (send PIN via Twilio)
- `backend/internal/proximity/handler.go` — HTTP handlers:
  - `POST /api/v1/proximity/verify` (GPS coordinates)
  - `POST /api/v1/proximity/pin` (PIN entry)
  - `POST /api/v1/proximity/sms-fallback` (trigger SMS delivery)
  - `GET /api/v1/bookings/:id/proximity` (get proximity status for a booking)
- `backend/internal/proximity/twilio.go` — Twilio SMS client wrapper: SendSMS(to, body), configured with account SID + auth token

**Modify:**
- `backend/cmd/server/main.go` — Mount proximity router
- `backend/internal/platform/config/config.go` — Add `GPSThresholdMeters`, `PINValidityDuration`, `TwilioAccountSID`, `TwilioAuthToken`, `TwilioFromNumber`
- `backend/internal/booking/service.go` — Accept method calls ProximityService.GenerateCheckInPIN; CheckIn/CheckOut methods verify handoff completeness before transitioning

**Verify:**
```bash
cd backend && go test ./internal/proximity/... -v -count=1
# PIN generation: always 4 digits, zero-padded, cryptographically random
# Haversine: known coordinate pairs produce correct distances
# PIN expiry: expired PINs rejected
# GPS threshold: 99m passes, 101m fails at 100m threshold
# Integration test:
curl -sf -X POST http://localhost:8080/api/v1/proximity/verify \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"transactionId":"<id>","lat":33.770,"lng":-118.190}'
# Should return 200 with verified=true if within threshold
```

### Step 3.3 — NotificationService (backend)

**Create:**
- `backend/internal/notification/model.go` — Notification domain type (id, userID, type, title, body, data JSON, read bool, createdAt), NotificationType enum (BOOKING_REQUEST, BOOKING_ACCEPTED, BOOKING_AUTO_DECLINED, CANCELLATION, PICKUP_APPROACHING, PROXIMITY_VERIFIED, RETURN_APPROACHING, LATE_RETURN_WARNING, LATE_RETURN_ESCALATION, DISPUTE_OPENED, DISPUTE_RESOLVED, PAYOUT_SENT, NEW_MESSAGE, KYC_STATUS, LISTING_FLAGGED), UserPreferences type
- `backend/internal/notification/repository.go` — Insert, FindByUserID (paginated, newest first), MarkRead, MarkAllRead, CountUnread
- `backend/internal/notification/preferences.go` — GetPreferences, UpdatePreferences, IsTypeDisabled (booking notifications always return enabled regardless of user setting), IsQuietHours (compare current time to user's quiet hours window)
- `backend/internal/notification/push.go` — Expo push client wrapper: SendPush(token, title, body, data), BatchSend, HandleReceipts (DeviceNotRegistered -> remove token from user). Uses `exponent-server-sdk-golang`
- `backend/internal/notification/service.go` — Notify(userID, type, title, body, data): check preferences, check quiet hours (defer if in quiet hours via River delayed job), store in Postgres, send push if enabled, send SMS if fallback enabled and push fails
- `backend/internal/notification/handler.go` — HTTP handlers:
  - `GET /api/v1/notifications` (paginated, for current user)
  - `POST /api/v1/notifications/:id/read`
  - `POST /api/v1/notifications/read-all`
  - `GET /api/v1/notifications/unread-count`
  - `GET /api/v1/notifications/preferences`
  - `PUT /api/v1/notifications/preferences`
  - `POST /api/v1/notifications/register-token` (save Expo push token)
- `backend/internal/notification/scheduled_jobs.go` — River jobs: PickupApproachingJob (fires 30min before scheduled_start), ReturnApproachingJob (fires 30min before scheduled_end), QuietHoursDeferredJob (fires when quiet hours end)

**Modify:**
- `backend/cmd/server/main.go` — Mount notification router, register River workers for scheduled notification jobs
- `backend/internal/platform/config/config.go` — Add `ExpoPushAccessToken`, `PickupReminderMinutes`, `ReturnReminderMinutes`
- `backend/internal/booking/service.go` — After booking creation: notify host (BOOKING_REQUEST). After accept: notify renter (BOOKING_ACCEPTED), schedule PickupApproachingJob + ReturnApproachingJob. After auto-decline: notify both (BOOKING_AUTO_DECLINED). After cancel: notify other party (CANCELLATION)

**Verify:**
```bash
cd backend && go test ./internal/notification/... -v -count=1
# Preference tests: booking notifications cannot be disabled
# Quiet hours: notifications deferred during quiet window, delivered after
# Push token registration and deregistration on DeviceNotRegistered
# Scheduled job tests: pickup/return reminders fire at correct times
# Integration test:
curl -sf -X POST http://localhost:8080/api/v1/notifications/register-token \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"token":"ExponentPushToken[xxxx]"}'
# Should return 200
curl -sf http://localhost:8080/api/v1/notifications/unread-count \
  -H "Authorization: Bearer $TOKEN"
# Should return {"count": 0}
```

### Step 3.4 — MessagingService (backend)

**Create:**
- `backend/internal/messaging/model.go` — Message domain type, SendMessageInput
- `backend/internal/messaging/repository.go` — Insert, FindByTransactionID (paginated, chronological), CountByTransactionID
- `backend/internal/messaging/service.go` — SendMessage (validate sender is renter or host on the transaction, insert, trigger Pusher event on channel `private-transaction-{id}`, trigger push notification to recipient), GetMessages (paginated)
- `backend/internal/messaging/handler.go` — HTTP handlers:
  - `POST /api/v1/bookings/:id/messages`
  - `GET /api/v1/bookings/:id/messages` (paginated: `?cursor=<ulid>&limit=50`)
- `backend/internal/messaging/pusher.go` — Pusher channel helper: channel naming (`private-transaction-{transactionID}`), event types (`new-message`, `booking-status-changed`)

**Modify:**
- `backend/cmd/server/main.go` — Mount messaging router
- `backend/internal/booking/service.go` — On status change, trigger Pusher event (`booking-status-changed`) on the transaction channel

**Verify:**
```bash
cd backend && go test ./internal/messaging/... -v -count=1
# Authorization: only booking parties can send/read messages
# Pagination: cursor-based pagination returns correct pages
# Pusher: mock Pusher client receives trigger calls
# Integration test:
curl -sf -X POST http://localhost:8080/api/v1/bookings/<id>/messages \
  -H "Authorization: Bearer $RENTER_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"content":"On my way!"}'
# Should return 201 with message object
curl -sf http://localhost:8080/api/v1/bookings/<id>/messages \
  -H "Authorization: Bearer $HOST_TOKEN"
# Should return paginated messages including "On my way!"
```

### Step 3.5 — Booking flow (RN)

**Create:**
- `mobile/app/(tabs)/(feed)/booking-request.tsx` — Booking request confirmation screen: shows listing summary, rental fee, hold amount, duration, total card impact. "Confirm Booking" button calls `POST /api/v1/bookings`
- `mobile/app/(tabs)/(feed)/booking-status.tsx` — Booking status screen: shows current state (color-coded badge), next steps text, action buttons per state (cancel if REQUESTED/ACCEPTED, navigate to check-in if ACCEPTED, navigate to check-out if ACTIVE). Real-time status updates via Pusher subscription on `private-transaction-{id}` channel
- `mobile/components/booking/IncomingRequest.tsx` — Host incoming request card: listing photo, renter name+reputation, rental period, accept/decline buttons with auto-decline countdown timer
- `mobile/components/booking/BookingCard.tsx` — Booking summary card for lists: status badge, listing thumbnail, other party name, date range
- `mobile/components/booking/CancelConfirmation.tsx` — Cancel confirmation modal: shows calculated fee before confirming
- `mobile/lib/hooks/useBooking.ts` — TanStack Query hook for single booking (with Pusher real-time invalidation)
- `mobile/lib/hooks/useBookings.ts` — TanStack Query hook for user's bookings list
- `mobile/lib/hooks/usePusher.ts` — Pusher client hook: connect, subscribe to channel, bind events, auto-disconnect on unmount

**Modify:**
- `mobile/app/(tabs)/(feed)/index.tsx` — Listing card gains "Rent Now" button linking to booking-request screen

**Install:**
```bash
cd mobile && npx expo install expo-notifications pusher-js
```

**Verify:**
```bash
cd mobile && npx tsc --noEmit
# Manual: Host receives push notification for new request
# Manual: Accept/decline updates renter screen in real time via Pusher
# Manual: Auto-decline countdown visible and fires correctly
```

### Step 3.6 — Handoff screens (RN)

**Create:**
- `mobile/app/(tabs)/(feed)/check-in.tsx` — Check-in screen: GPS status indicator (green checkmark when verified, red X when >100m), PIN entry field (renter side) / PIN display (host side), camera capture button, captured photos grid, "Complete Check-in" button (disabled until all conditions met: GPS + PIN + min 3 photos)
- `mobile/app/(tabs)/(feed)/check-out.tsx` — Check-out screen: GPS status indicator, camera capture button, captured photos grid, "Complete Return" button (disabled until GPS verified + min 3 return photos)
- `mobile/app/(tabs)/(feed)/active-rental.tsx` — Active rental screen: countdown timer showing time remaining until scheduled_end, listing details, "Navigate to Return" button (deep link to Maps), "Report Issue" button (opens dispute flow), late return warning banner (appears when past scheduled_end)
- `mobile/components/handoff/GPSStatus.tsx` — GPS proximity indicator component: polls device location, shows distance to target, green/red status
- `mobile/components/handoff/PINEntry.tsx` — 4-digit PIN entry with auto-advance between digits
- `mobile/components/handoff/PINDisplay.tsx` — Large 4-digit PIN display for host to show renter
- `mobile/components/handoff/PhotoGrid.tsx` — Grid of captured check-in/check-out photos with angle indicator badges
- `mobile/lib/hooks/useLocation.ts` — Hook wrapping expo-location for continuous GPS tracking during handoff
- `mobile/lib/hooks/useProximity.ts` — Hook that combines GPS + PIN + photo state, calls proximity API endpoints

**Modify:**
- `mobile/app/(tabs)/(feed)/booking-status.tsx` — "Navigate to Pickup" button: uses `expo-linking` to open Maps app with host location (revealed only after ACCEPTED state). "Start Check-in" button links to check-in screen. "Start Check-out" button links to check-out screen
- `mobile/components/camera/AngleEnforcedCamera.tsx` — (from Phase 1) Reused for check-in and check-out photo capture with same gyroscope angle enforcement (>= 30 degrees between shots)

**Install:**
```bash
cd mobile && npx expo install expo-location
```

**Verify:**
```bash
cd mobile && npx tsc --noEmit
# Manual: GPS indicator updates in real time as user moves
# Manual: PIN entry accepts correct PIN, rejects wrong
# Manual: Camera enforces angle diversity at check-in and check-out
# Manual: Check-in completes -> rental starts -> active rental screen shows timer
# Manual: Check-out completes -> rental ends -> completion screen
```

### Step 3.7 — Messaging screen (RN)

**Create:**
- `mobile/app/(tabs)/(messages)/conversation.tsx` — Chat screen: FlatList with inverted scroll (newest at bottom), message bubbles (sender vs receiver styling), text input bar with send button, auto-scroll on new message, load-more on scroll to top (cursor pagination)
- `mobile/components/messaging/MessageBubble.tsx` — Message bubble component: sender name, message text, timestamp, left-aligned (other party) vs right-aligned (current user) styling
- `mobile/components/messaging/MessageInput.tsx` — Text input with send button, disabled state while sending, character limit
- `mobile/components/messaging/ConversationList.tsx` — List of active conversations: last message preview, unread indicator, other party avatar + name
- `mobile/lib/hooks/useMessages.ts` — TanStack Query hook for messages (cursor-based pagination), with Pusher real-time subscription to append new messages via query cache update
- `mobile/lib/hooks/useConversations.ts` — TanStack Query hook for user's conversations (derived from bookings with messages)

**Modify:**
- `mobile/app/(tabs)/(messages)/index.tsx` — Replace placeholder with ConversationList, tap -> navigate to conversation screen
- `mobile/app/(tabs)/_layout.tsx` — Messages tab badge showing unread count (from notification unread-count endpoint)

**Verify:**
```bash
cd mobile && npx tsc --noEmit
# Manual: Send message -> appears in conversation for both parties in real time
# Manual: Push notification tap -> opens correct conversation
# Manual: Scroll to top loads older messages
# Manual: Unread badge on messages tab updates correctly
```

---

## API Endpoints

| Method | Path | Auth | Request Body | Response | Errors |
|--------|------|------|-------------|----------|--------|
| POST | `/api/v1/bookings` | Yes | `{listingId, scheduledStart, scheduledEnd}` | `{booking}` | 400 validation, 400 >7-day ceiling, 403 fraud velocity block, 404 listing, 409 listing unavailable |
| GET | `/api/v1/bookings/:id` | Yes | — | `{booking, listing, otherParty}` | 403 not a party, 404 |
| GET | `/api/v1/users/me/bookings` | Yes | `?status=ACTIVE&page=1&limit=20` | `{bookings[], total, page}` | — |
| POST | `/api/v1/bookings/:id/accept` | Yes | — | `{booking}` | 403 not host, 400 invalid transition |
| POST | `/api/v1/bookings/:id/decline` | Yes | — | `{booking}` | 403 not host, 400 invalid transition |
| POST | `/api/v1/bookings/:id/cancel` | Yes | — | `{booking, cancellationFee}` | 403 not a party, 400 invalid transition |
| POST | `/api/v1/bookings/:id/check-in` | Yes | `{mediaIds[]}` | `{booking, proximityProof}` | 400 conditions not met, 403, 404 |
| POST | `/api/v1/bookings/:id/check-out` | Yes | `{mediaIds[]}` | `{booking, proximityProof}` | 400 conditions not met, 403, 404 |
| POST | `/api/v1/proximity/verify` | Yes | `{transactionId, lat, lng}` | `{verified, distance, threshold}` | 400 validation, 404 transaction |
| POST | `/api/v1/proximity/pin` | Yes | `{transactionId, pin}` | `{verified}` | 400 invalid/expired PIN, 404 |
| POST | `/api/v1/proximity/sms-fallback` | Yes | `{transactionId}` | `{sent: true}` | 400 no phone, 404 |
| GET | `/api/v1/bookings/:id/proximity` | Yes | — | `{checkIn: ProximityStatus, checkOut: ProximityStatus}` | 403 not a party, 404 |
| POST | `/api/v1/bookings/:id/messages` | Yes | `{content}` | `{message}` | 400 empty, 403 not a party, 404 |
| GET | `/api/v1/bookings/:id/messages` | Yes | `?cursor=<ulid>&limit=50` | `{messages[], nextCursor}` | 403 not a party, 404 |
| GET | `/api/v1/notifications` | Yes | `?page=1&limit=20` | `{notifications[], total, page}` | — |
| POST | `/api/v1/notifications/:id/read` | Yes | — | `{notification}` | 404 |
| POST | `/api/v1/notifications/read-all` | Yes | — | `{count}` | — |
| GET | `/api/v1/notifications/unread-count` | Yes | — | `{count}` | — |
| GET | `/api/v1/notifications/preferences` | Yes | — | `{preferences}` | — |
| PUT | `/api/v1/notifications/preferences` | Yes | `{types: {}, quietHours: {}}` | `{preferences}` | 400 validation |
| POST | `/api/v1/notifications/register-token` | Yes | `{token}` | `{registered: true}` | 400 invalid token |

---

## Database Migrations

### Migration 003 — Notifications and push tokens

```sql
-- notifications (in-app notification storage)
CREATE TABLE notifications (
    id              TEXT PRIMARY KEY,           -- ULID
    user_id         TEXT NOT NULL REFERENCES users(id),
    type            TEXT NOT NULL
                    CHECK (type IN (
                        'BOOKING_REQUEST', 'BOOKING_ACCEPTED', 'BOOKING_AUTO_DECLINED',
                        'CANCELLATION', 'PICKUP_APPROACHING', 'PROXIMITY_VERIFIED',
                        'RETURN_APPROACHING', 'LATE_RETURN_WARNING', 'LATE_RETURN_ESCALATION',
                        'DISPUTE_OPENED', 'DISPUTE_RESOLVED', 'PAYOUT_SENT',
                        'NEW_MESSAGE', 'KYC_STATUS', 'LISTING_FLAGGED'
                    )),
    title           TEXT NOT NULL,
    body            TEXT NOT NULL,
    data            JSONB NOT NULL DEFAULT '{}',  -- arbitrary payload (transactionId, etc.)
    read            BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_notifications_user_id ON notifications(user_id);
CREATE INDEX idx_notifications_user_unread ON notifications(user_id) WHERE read = FALSE;

-- push_tokens (Expo push tokens per user per device)
CREATE TABLE push_tokens (
    id              TEXT PRIMARY KEY,           -- ULID
    user_id         TEXT NOT NULL REFERENCES users(id),
    token           TEXT NOT NULL UNIQUE,       -- ExponentPushToken[xxxx]
    platform        TEXT NOT NULL CHECK (platform IN ('ios', 'android')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_push_tokens_user_id ON push_tokens(user_id);
```

### Migration 004 — Transaction indexes and columns for fraud velocity

```sql
-- Add payout_delayed flag for first-3-transaction fraud velocity rule
ALTER TABLE transactions ADD COLUMN payout_delayed BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE transactions ADD COLUMN payout_delayed_until TIMESTAMPTZ;

-- Index for fraud velocity: count completed transactions per user
CREATE INDEX idx_transactions_renter_completed
    ON transactions(renter_id) WHERE status = 'COMPLETED';
CREATE INDEX idx_transactions_host_completed
    ON transactions(host_id) WHERE status = 'COMPLETED';

-- Index for damage claim cap: sum damage claims per host within window
-- (queries filter by host_id + created_at + photo_diff_result)
CREATE INDEX idx_transactions_host_damage
    ON transactions(host_id, created_at)
    WHERE photo_diff_result IN ('COSMETIC_DAMAGE', 'FUNCTIONAL_DAMAGE', 'MISSING_ITEM');
```

---

## Risks and Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| Race condition on state transitions | Two concurrent requests could both pass CanTransition check | Use `SELECT ... FOR UPDATE` row lock in UpdateStatus. Single pgx transaction wraps read-check-write |
| Expo push token expiry | Silent notification failures | Handle `DeviceNotRegistered` receipt error: delete stale token, prompt user to re-register on next app open |
| GPS spoofing on proximity check | Fake handoff verification | v1: accept GPS at face value (adequate for honest users). v1.5: BLE as secondary signal. GPS spoof detection deferred to fraud agent (Phase 6) |
| Twilio SMS delivery failures | Handoff blocked when app unreachable | Retry SMS up to 3 times with exponential backoff. If all fail, allow manual confirmation by both parties with timestamp reconciliation |
| River job missed (auto-decline) | Host never responds, renter stuck | River is Postgres-backed with at-least-once delivery. If server restarts, job re-queues. Add monitoring: alert if REQUESTED bookings exceed timeout + 5min |
| Pusher message ordering | Chat messages appear out of order | Messages have ULID (monotonic). Client sorts by ULID, not arrival order. Pusher is best-effort real-time; TanStack Query refetch is source of truth |
| PIN brute-force (10,000 combinations) | Attacker guesses PIN | Rate limit: 5 attempts per PIN per transaction. After 5 failures, PIN invalidated, host must regenerate. Log failed attempts for fraud signal |
| Late-fee / damage-reserve contention | LateReturnAgent and DisputeAgent race for hold funds | Hold allocation uses `SELECT FOR UPDATE` row lock. LateReturnAgent caps at `hold * (1 - damageReserveRate)`. All captures are atomic pgx transactions |
| Notification spam during active rental | User disables notifications, misses critical alerts | Booking notifications (request, accept, cancel, late return) cannot be disabled per PRD. Quiet hours defer but never drop these types |

---

## Testing Strategy

### State Machine Tests (CRITICAL — complete coverage)

Every valid transition must succeed. Every invalid transition must be rejected. This is the single most important test suite in Phase 3.

**Valid transitions (9 tests — all must pass):**

| Test | From | To | Expected |
|------|------|----|----------|
| Host accepts request | REQUESTED | ACCEPTED | Success |
| Host declines request | REQUESTED | DECLINED | Success |
| Auto-decline fires | REQUESTED | AUTO_DECLINED | Success |
| Renter cancels request | REQUESTED | CANCELLED | Success |
| Check-in completes | ACCEPTED | ACTIVE | Success |
| Either party cancels accepted | ACCEPTED | CANCELLED | Success |
| Check-out completes | ACTIVE | COMPLETED | Success |
| Dispute opened | ACTIVE | DISPUTED | Success |
| Dispute resolved | DISPUTED | COMPLETED | Success |

**Invalid transitions (exhaustive — all must return error):**

| Test | From | To | Expected |
|------|------|----|----------|
| REQUESTED -> ACTIVE | REQUESTED | ACTIVE | Error: invalid transition |
| REQUESTED -> COMPLETED | REQUESTED | COMPLETED | Error: invalid transition |
| REQUESTED -> DISPUTED | REQUESTED | DISPUTED | Error: invalid transition |
| ACCEPTED -> ACCEPTED | ACCEPTED | ACCEPTED | Error: invalid transition |
| ACCEPTED -> DECLINED | ACCEPTED | DECLINED | Error: invalid transition |
| ACCEPTED -> AUTO_DECLINED | ACCEPTED | AUTO_DECLINED | Error: invalid transition |
| ACCEPTED -> COMPLETED | ACCEPTED | COMPLETED | Error: invalid transition |
| ACCEPTED -> DISPUTED | ACCEPTED | DISPUTED | Error: invalid transition |
| ACTIVE -> REQUESTED | ACTIVE | REQUESTED | Error: invalid transition |
| ACTIVE -> ACCEPTED | ACTIVE | ACCEPTED | Error: invalid transition |
| ACTIVE -> CANCELLED | ACTIVE | CANCELLED | Error: invalid transition |
| DISPUTED -> REQUESTED | DISPUTED | REQUESTED | Error: invalid transition |
| DISPUTED -> CANCELLED | DISPUTED | CANCELLED | Error: invalid transition |
| DISPUTED -> DISPUTED | DISPUTED | DISPUTED | Error: invalid transition |
| COMPLETED -> (any) | COMPLETED | any | Error: terminal state |
| DECLINED -> (any) | DECLINED | any | Error: terminal state |
| AUTO_DECLINED -> (any) | AUTO_DECLINED | any | Error: terminal state |
| CANCELLED -> (any) | CANCELLED | any | Error: terminal state |

### Guard Condition Tests

| Test | Description | Expected |
|------|-------------|----------|
| ACCEPTED->ACTIVE without GPS | Attempt check-in without proximity proof | Error: GPS proximity required |
| ACCEPTED->ACTIVE without PIN | Attempt check-in without PIN verification | Error: PIN verification required |
| ACCEPTED->ACTIVE without renter photos | Check-in missing renter's photos | Error: check-in photos required from both parties |
| ACCEPTED->ACTIVE without host photos | Check-in missing host's photos | Error: check-in photos required from both parties |
| ACCEPTED->ACTIVE all conditions met | GPS + PIN + both parties' photos | Success: transition to ACTIVE |
| ACTIVE->COMPLETED without GPS | Attempt check-out without proximity | Error: GPS proximity required |
| ACTIVE->COMPLETED without renter photos | Check-out missing renter's return photos | Error: return photos required from both parties |
| ACTIVE->COMPLETED without host photos | Check-out missing host's return photos | Error: return photos required from both parties |
| ACTIVE->COMPLETED all conditions met | GPS + both parties' return photos | Success: transition to COMPLETED |

### Fraud Velocity Tests

| Test | Description | Expected |
|------|-------------|----------|
| New-to-new: both < 30 days | Renter (5 days old) books from host (10 days old) | Blocked: new-to-new lockout |
| New-to-new: renter new, host established | Renter (5 days) books from host (60 days) | Allowed |
| New-to-new: both established | Both > 30 days | Allowed |
| First-3: renter has 0 completed | Renter's first booking | Flagged: 48h payout delay |
| First-3: renter has 2 completed | Renter's third booking | Flagged: 48h payout delay |
| First-3: renter has 3 completed | Renter's fourth booking | No delay |
| Damage cap: host at limit | Host has $500+ damage claims in first 60 days | Blocked |
| Damage cap: host under limit | Host has $200 damage claims in first 60 days | Allowed |
| Damage cap: host past window | Host has $600 claims but account > 60 days old | Allowed (window expired) |

### Cancellation Fee Tests

| Test | Description | Expected Fee |
|------|-------------|-------------|
| Renter cancels > 2h before | scheduledStart is 3h away | $0 |
| Renter cancels 1.5h before | scheduledStart is 1.5h away | 25% of rental fee |
| Renter cancels 30min before | scheduledStart is 30min away | 50% of rental fee |
| Renter cancels after host waiting | Past scheduledStart, status ACCEPTED | 100% of rental fee |
| Host cancels > 2h before | scheduledStart is 3h away | $0 (warning tracked) |
| Host cancels < 2h before | scheduledStart is 1h away | Configured % of rental fee |

### Proximity Tests

| Test | Description | Expected |
|------|-------------|----------|
| GPS within threshold | 50m apart (threshold 100m) | Verified: true |
| GPS outside threshold | 150m apart (threshold 100m) | Verified: false |
| GPS at boundary | 100m exactly | Verified: true (inclusive) |
| PIN correct | Matching 4-digit PIN | Verified: true |
| PIN incorrect | Wrong PIN | Verified: false |
| PIN expired | Correct PIN but >1h old | Error: PIN expired |
| PIN brute force | 6th attempt on same PIN | Error: PIN invalidated |
| Haversine accuracy | Known coordinate pairs | Distances match within 1m |
| SMS fallback | Twilio API called with correct number | SMS sent, method=SMS_FALLBACK |

### Notification Tests

| Test | Description | Expected |
|------|-------------|----------|
| Booking request | New booking created | Host receives push + in-app |
| Booking accepted | Host accepts | Renter receives push + in-app |
| Auto-declined | Timeout fires | Both parties receive push + in-app |
| Booking notifications undisableable | User disables BOOKING_REQUEST in preferences | Notification still delivered |
| Quiet hours deferred | Notification sent during quiet hours | Stored in DB, push deferred via River job |
| Quiet hours bypass for booking | Booking notification during quiet hours | Push delivered immediately (booking overrides quiet hours) |
| Stale token cleanup | Push returns DeviceNotRegistered | Token deleted from push_tokens |
| Pickup approaching | 30min before scheduledStart | Both parties receive push |
| Return approaching | 30min before scheduledEnd | Renter receives push |

### Messaging Tests

| Test | Description | Expected |
|------|-------------|----------|
| Send message as renter | Renter sends to booking conversation | 201 created, Pusher event fired |
| Send message as host | Host sends to booking conversation | 201 created, Pusher event fired |
| Send message as outsider | Non-party attempts to send | 403 forbidden |
| Read messages as outsider | Non-party attempts to read | 403 forbidden |
| Cursor pagination | Fetch with cursor | Returns next page, correct order |
| Empty conversation | No messages yet | Returns empty array |

### Integration Tests (End-to-End)

| Test | Flow | Steps |
|------|------|-------|
| Happy path | Full booking lifecycle | Register two users -> create listing -> create booking -> accept -> verify GPS -> enter PIN -> upload check-in photos -> rental active -> verify GPS -> upload return photos -> rental completed |
| Auto-decline | Timeout flow | Create booking -> wait for River job -> verify status=AUTO_DECLINED -> verify renter notification |
| Cancellation | Renter cancels after accept | Create booking -> accept -> cancel -> verify fee charged -> verify both notified |
| Fraud block | New-to-new | Create two users (both new) -> attempt booking -> verify 403 with fraud velocity message |

### RN Tests

- TypeScript strict mode passes (`npx tsc --noEmit`)
- Manual: full handoff ceremony on two iOS simulators (renter + host)
- Manual: push notification tap navigates to correct screen
- Manual: messaging real-time with Pusher
- Manual: auto-decline countdown visible and correct

---

## Implementation Order

| Step | What | Day | Depends On |
|------|------|-----|------------|
| 3.1 | BookingService (state machine, fraud velocity, cancellation, auto-decline) | Day 1-3 | Phase 2 complete (PaymentService) |
| 3.2 | ProximityService (PIN, GPS, SMS fallback) | Day 3-5 | 3.1 (needs booking to exist) |
| 3.3 | NotificationService (push, in-app, preferences, scheduled jobs) | Day 4-6 | 3.1 (triggers from booking events) |
| 3.4 | MessagingService (messages, Pusher events) | Day 5-7 | 3.1 (scoped to transactions) |
| 3.5 | Booking flow (RN) | Day 5-7 | 3.1, 3.3 (needs API + push) |
| 3.6 | Handoff screens (RN) | Day 7-9 | 3.2, 3.5 (needs proximity API + booking screens) |
| 3.7 | Messaging screen (RN) | Day 8-10 | 3.4, 3.5 (needs messaging API + Pusher hook) |

Steps 3.3 and 3.4 are independent of each other — can be parallelized.
Steps 3.5 and 3.3/3.4 backend work can overlap (mobile vs backend).
Step 3.6 depends on both 3.2 (proximity API) and 3.5 (booking screens for navigation).
Step 3.7 depends on 3.4 (messaging API) but can start once Pusher hook from 3.5 is done.
