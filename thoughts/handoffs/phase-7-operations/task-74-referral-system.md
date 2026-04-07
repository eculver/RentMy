# Task 7.4 Handoff — Referral System (backend + RN)

**Commit:** e195c3d  
**Branch:** task-7.4-referral-system  
**Date:** 2026-04-07  
**Status:** Completed

---

## What was built

### Backend

**Migration** `backend/migrations/015_referrals.sql`  
Three tables: `referral_codes` (one per user), `referrals` (referrer → referee relationships), `referral_payouts` (individual $20 Stripe transfer records).

**Package** `backend/internal/referral/`

| File | Purpose |
|------|---------|
| `model.go` | Domain types: `ReferralCode`, `Referral`, `ReferralPayout`, `ReferralStats` |
| `repository.go` | All Postgres queries incl. fraud signal helpers (`SharedDeviceFingerprint`, `SharedWiFiBSSID`) |
| `service.go` | `GenerateCode`, `ApplyReferralCode`, `OnFirstRentalCompleted`, `EvaluateAndPayout`, `ProcessPayout` |
| `jobs.go` | Two River workers: `ReferralPayoutJobWorker` (evaluate/payout), `ReferralStripeTransferWorker` (Stripe transfer) |
| `handler.go` | HTTP endpoints: `POST/GET /referrals/code`, `POST /referrals/apply`, `GET /referrals/mine`, `GET /ops/referrals`, `GET /ops/referrals/stats` |
| `service_test.go` | 10 unit tests (code generation, apply, fraud blocks, first-rental hook) |

**Modifications**

- `backend/app/server.go` — referral service wired pre/post River; workers registered; handler mounted at `/api/v1`
- `backend/internal/booking/service.go` — `CheckOut` calls `referralSvc.OnFirstRentalCompleted(renterID)` after completion
- `backend/internal/user/service.go` — `Register` calls `referralSvc.ApplyCode(userID, code)` when `RegisterInput.ReferralCode` is set
- `backend/internal/user/model.go` — Added `ReferralCode *string` to `RegisterInput`

**Fraud guards in `EvaluateAndPayout`:**
1. Shared device fingerprint → `FRAUDULENT`
2. Same WiFi BSSID + shared device → `FRAUDULENT` (same WiFi alone → monitor)
3. Referrer has >5 PAID payouts in last 30 days → leave as `FIRST_RENTAL_COMPLETED` for manual review

### Mobile

| File | Purpose |
|------|---------|
| `mobile/lib/hooks/useReferrals.ts` | `useReferralCode()` (auto-generate), `useMyReferrals()`, `useApplyReferralCode()` |
| `mobile/components/referral/ReferralCard.tsx` | Single referral row: status badge, payout amount |
| `mobile/app/(tabs)/(profile)/referrals.tsx` | Full referral screen: code display, copy/share, FlatList of referrals, empty state |
| `mobile/app/(auth)/register.tsx` | Added optional referral code field; pre-filled from deep link `?ref=CODE` |
| `mobile/app/(tabs)/(profile)/index.tsx` | "Invite Friends — Earn $20" button linking to referrals screen |
| `mobile/lib/auth.ts` | `register()` now accepts optional `referralCode` param |
| `mobile/__tests__/screens/referrals.test.tsx` | 7 screen tests (render, code display, copy feedback, empty state, auto-generate) |

---

## Interfaces (for future consumers)

The referral service exposes narrow interfaces to break circular imports:

- `user.ReferralApplier` — `ApplyCode(ctx, refereeID, code) error` (satisfied by `*referral.Service`)
- `booking.referralSvc` — `OnFirstRentalCompleted(ctx, userID)` (satisfied by `*referral.Service`)

---

## Dependencies added

None — uses existing pgxpool, River, Stripe adapter, and payment repository.

---

## Branching mode

Used **Graphite** (`gt create task-7.4-referral-system`).

---

## Verification

```
go vet ./...                              ✓
go build ./cmd/server                    ✓
go test ./internal/... -count=1          ✓ (all 10 referral unit tests pass)
npx tsc --noEmit                         ✓
npx jest --no-coverage                   ✓ (91/91 tests pass)
```
