# Task 3.2 — ProximityService Handoff

**Status:** Completed  
**Commit:** 12dc136  
**Branch:** task-3.2-proximity-service  
**Date:** 2026-04-04

---

## What Was Built

ProximityService implementing PRD §12 (GPS + PIN proximity handoff) for check-in and check-out.

### New files

| File | Purpose |
|------|---------|
| `backend/migrations/004_proximity_user_pin_expiry.sql` | Adds `user_id` and `pin_expires_at` to `proximity_proofs` |
| `backend/internal/proximity/model.go` | ProximityProof domain type, ProofType/Method enums, input types, ProximityStatus |
| `backend/internal/proximity/pin.go` | `GeneratePIN` (crypto/rand 4-digit zero-padded), `ValidatePIN` (match + expiry) |
| `backend/internal/proximity/pin_test.go` | PIN generation/validation tests |
| `backend/internal/proximity/distance.go` | Haversine great-circle distance, `IsWithinThreshold` |
| `backend/internal/proximity/distance_test.go` | Known-coordinate distance tests, threshold boundary tests |
| `backend/internal/proximity/repository.go` | Insert, FindByTransactionID, FindByTransactionAndType, UpdateVerified, UpsertPIN, GetListingLocation, GetTransactionParties, GetUserPhone |
| `backend/internal/proximity/twilio.go` | `TwilioClient.SendSMS` — plain HTTP, no external SDK |
| `backend/internal/proximity/service.go` | GenerateCheckInPIN, VerifyGPS, VerifyPIN, CheckHandoffComplete, SMSFallback, GetStatus |
| `backend/internal/proximity/handler.go` | POST /proximity/verify, POST /proximity/pin, POST /proximity/sms-fallback, GET /bookings/:id/proximity |

### Modified files

| File | Change |
|------|--------|
| `backend/internal/platform/config/config.go` | Added `GPSThresholdMeters`, `PINValidityMinutes`, `TwilioAccountSID/AuthToken/FromNumber` |
| `backend/internal/booking/model.go` | Added `ErrHandoffIncomplete` sentinel |
| `backend/internal/booking/service.go` | Accept now calls `proximitySvc.GenerateCheckInPIN`; added CheckIn/CheckOut methods; `proximitySvc` interface injected via constructor |
| `backend/internal/booking/handler.go` | Added POST /bookings/:id/check-in and /check-out endpoints |
| `backend/cmd/server/main.go` | Wired proximity repository, service, handler; nil Twilio client when env vars absent |

---

## Key Design Decisions

**User-scoped proof records:** Each `(transaction_id, proof_type, user_id)` triple gets its own `proximity_proofs` row. The host's CHECK_IN record stores the PIN + expiry; the renter's CHECK_IN record is marked verified after GPS + PIN pass.

**PIN flow:** Host accepts → `GenerateCheckInPIN` creates host's proof record with PIN. Renter calls VerifyGPS (creates renter proof), then VerifyPIN (reads PIN from host record, marks renter verified).

**Handoff gate:** `CheckHandoffComplete` counts verified records per type. BookingService.CheckIn/CheckOut call this before allowing ACCEPTED→ACTIVE or ACTIVE→COMPLETED transitions.

**Twilio SMS:** Plain HTTP client, no external SDK. Client is nil in dev/test when env vars are absent; `SMSFallback` returns an error if nil (safe for endpoints).

**proximitySvc interface in BookingService:** The `proximitySvc` interface breaks the potential import cycle and allows testing without a real DB.

---

## Verification

```
go vet ./...          ✓ no issues
go build ./cmd/server ✓ clean
go test ./... -count=1

ok  backend/internal/booking    0.373s
ok  backend/internal/discovery  0.206s
ok  backend/internal/listing    0.540s
ok  backend/internal/media      0.835s
ok  backend/internal/payment    0.855s
ok  backend/internal/proximity  0.863s (11 tests: PIN + Haversine + threshold)
ok  backend/internal/user       2.700s
```

---

## Branching

Used vanilla git (Graphite unavailable). Branch `task-3.2-proximity-service` stacks on `task-3.1-booking-service`.

---

## Fix Applied

Task 3.1 `BookingService` was fully implemented last session (commit `b3a66c2`) but `progress.json` was not updated in commit `390eae1`. Fixed in this session: status set to `"completed"` with correct commitSha.

---

## Next Tasks Unblocked

- **3.5** Booking flow (RN) — depends on 3.1 (now verified complete) and 3.2
- **3.6** Handoff screens (RN) — depends on 3.2 (GPS/PIN API) and 3.5
- **3.3** NotificationService — no dependencies
- **3.4** MessagingService — no dependencies
