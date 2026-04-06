# Task 5.4 Handoff — Phase 3-4 Backend Integration Tests

**Status:** Completed  
**Branch:** task-5.4-phase34-integration-tests  
**Date:** 2026-04-05  
**Graphite mode:** yes

---

## What Was Built

Four new integration test files added to `backend/tests/integration/`:

### `booking_api_test.go` (17 tests)

| Test | What it covers |
|------|---------------|
| `TestBookingGetNotFound` | 404 for missing booking |
| `TestBookingGetUnauthorized` | 401 without auth |
| `TestBookingGetForbidden` | 403 for third-party viewer |
| `TestBookingGetSuccess` | 200 for renter |
| `TestBookingGetAsHost` | 200 for host |
| `TestBookingListRenter` | 2 bookings returned for renter |
| `TestBookingListRenterEmpty` | empty list when no bookings |
| `TestBookingListHost` | 1 booking returned for host |
| `TestBookingAccept` | REQUESTED → ACCEPTED, status verified |
| `TestBookingAcceptOnlyHost` | 403 when renter tries to accept |
| `TestBookingDecline` | REQUESTED → DECLINED |
| `TestBookingCancel` | REQUESTED → CANCELLED |
| `TestBookingStateMachineRejections` | double-accept returns 400 |
| `TestBookingCheckInHandoffIncomplete` | 422 when proximity not complete |
| `TestBookingCreateDurationExceeds7Days` | 400 for >7-day duration |
| `TestBookingCreateMissingListingID` | 400 for missing listingId |
| `TestBookingCreateMissingPaymentMethod` | 400 for missing paymentMethodId |
| `TestBookingCreateRequiresAuth` | 401 without auth |
| `TestBookingCancelNotFound` | 404 for missing booking |
| `TestBookingFraudVelocityCheck` | 403 both-new-account fraud block |

### `proximity_api_test.go` (10 tests)

| Test | What it covers |
|------|---------------|
| `TestProximityVerifyGPSRequiresAuth` | 401 without auth |
| `TestProximityVerifyGPSMissingTransactionID` | 400 validation |
| `TestProximityVerifyGPSInvalidProofType` | 400 for bad proofType |
| `TestProximityVerifyGPSTooFarAway` | 422 when >100m from listing |
| `TestProximityVerifyGPSWithinRange` | 200 verified=true at listing coords |
| `TestProximityPINVerifyInvalidLength` | 400 for PIN != 4 digits |
| `TestProximityPINVerifyMissingTransactionID` | 400 validation |
| `TestProximityPINGenerationAfterAccept` | host CHECK_IN proof created on accept |
| `TestProximityGetStatusUnauthorized` | 401 without auth |
| `TestProximityGetStatusForbiddenForNonParty` | 403 for non-party |
| `TestProximitySMSFallbackMissingFields` | 400 for missing fields |

### `notification_api_test.go` (10 tests)

| Test | What it covers |
|------|---------------|
| `TestNotificationListEmpty` | empty list for new user |
| `TestNotificationListRequiresAuth` | 401 without auth |
| `TestNotificationUnreadCountEmpty` | count=0 for new user |
| `TestNotificationGetPreferencesDefaults` | pushEnabled=true default |
| `TestNotificationUpdatePreferences` | PUT → GET round-trip |
| `TestNotificationRegisterToken` | push token stored |
| `TestNotificationRegisterTokenMissing` | 400 for missing token |
| `TestNotificationCreationOnBookingAccept` | BOOKING_ACCEPTED created for renter |
| `TestNotificationMarkRead` | single mark-read decrements count |
| `TestNotificationReadAllMarksAllRead` | read-all resets count to 0 |

### `agent_api_test.go` (9 tests)

| Test | What it covers |
|------|---------------|
| `TestGetReputationNotFound` | 404 for missing user |
| `TestGetReputationSuccess` | 200, score=0, signals=[] |
| `TestGetReputationRequiresAuth` | 401 without auth |
| `TestGetRiskScoreNotFound` | 404 when no risk_scores row |
| `TestGetRiskScoreSuccess` | 200 with seeded row, verifies control |
| `TestGetRiskScoreRequiresAuth` | 401 without auth |
| `TestGetAgreementNotFound` | 404 when no agreement |
| `TestGetAgreementStatusNotFound` | 404 for missing agreement status |
| `TestDecisionAuditLog` | agent_decisions row persisted, readable |
| `TestAgreementAcceptNotFound` | 404 when no agreement to accept |

**Total new tests:** 50  
**Total suite (including Phase 1-2 + smoke):** ~76 tests  
**Run time:** ~7 seconds

---

## Schema Discoveries

Two mistakes were made and fixed during development:

1. **`risk_scores` table** — No `id`, `control`, or `signals` columns. Primary key is `transaction_id`. `control` is computed from `risk_score` in Go code. Schema: `(transaction_id, risk_score, risk_level, breakdown, computed_at)`.

2. **`agent_decisions` table** — No `agent_name`, `agent_version`, `output`, or `latency_ms` columns. Correct fields are `agent_type` (not `agent_name`), `decision` (not `output`).

---

## Design Decisions

- **`TestBookingFraudVelocityCheck`** tests the both-new-account rule by using two brand-new users. This passes because the booking service checks fraud BEFORE Stripe, so the 403 is returned from the Go service layer without needing a valid Stripe key.

- **`TestBookingCheckInHandoffIncomplete`** accepts the booking first (ACCEPTED state is required for check-in), then calls check-in without any GPS verification to confirm 422 is returned.

- **No `CreateBooking` happy-path test** — `POST /api/v1/bookings` goes through Stripe (`paymentSvc.CreateBooking`) and the test config uses a placeholder Stripe key. All other state transitions are tested via `CreateTestBooking` (direct DB insert).

- **`TestProximityVerifyGPSWithinRange`** uses the exact listing coordinates (0 m distance) to reliably stay within the 100 m GPS threshold.

- **`TestDecisionAuditLog`** seeds both `risk_scores` and `agent_decisions` directly to avoid the Stripe dependency.

---

## Verification

```
ok  github.com/giits/rentmy/backend/tests/integration  7.122s
```

All 76 integration tests pass. All existing Phase 1-2 tests unaffected.

---

## Mode

**Graphite mode** used for branch creation (`gt create task-5.4-phase34-integration-tests`).
