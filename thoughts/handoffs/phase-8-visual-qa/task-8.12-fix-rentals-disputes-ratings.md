# Task 8.12 — Fix: Rentals + Disputes + Ratings Bugs

**Date:** 2026-04-11  
**Branch:** `task-8.12-fix-rentals-disputes-ratings`  
**Commit:** `5c3b4eb`  
**Status:** Completed  
**Mode:** Graphite

---

## What Was Fixed

9 bugs from the 8.11 static audit, ranging from critical protocol mismatches to cosmetic dead code.

### BUG-1 [CRITICAL] — Rating JSON field name mismatch

**File:** `backend/internal/rating/model.go`

Added `json:"..."` tags to the `Rating` struct so `encoding/json` serializes to camelCase (`id`, `transactionId`, `fromUserId`, `toUserId`, `bubbles`, `createdAt`) matching the frontend `Rating` interface in `useRatings.ts`. Without tags, Go serialized PascalCase fields which the frontend couldn't read — `hasRated` was always `false`, causing the "Rate" button to persist after rating.

### BUG-2 [CRITICAL] — Dispute status mismatch: backend vs frontend

**Files:** `mobile/lib/hooks/useDispute.ts`, `mobile/components/rental/DisputeTimeline.tsx`, `mobile/app/(tabs)/(rentals)/dispute-status.tsx`, `mobile/app/(tabs)/(rentals)/return-confirmation.tsx`

Updated `DisputeStatus` type to match all 8 backend constants: `PENDING | GATHERING | ANALYZING | AUTO_RESOLVED | AUDIT_QUEUED | HUMAN_REVIEW | RESOLVED | INCONCLUSIVE`. Removed the stale `EVIDENCE_GATHERING`, `UNDER_REVIEW`, `CLOSED` values.

Introduced two helper sets:
- `TERMINAL_STATUSES` — statuses where polling should stop (`RESOLVED | AUTO_RESOLVED | AUDIT_QUEUED | INCONCLUSIVE`)
- `CLOSED_STATUSES` — statuses where the dispute is effectively closed (`RESOLVED | AUTO_RESOLVED | AUDIT_QUEUED`)

Updated `DisputeTimeline` to map 8 backend statuses onto 4 visual steps via a `STATUS_STEP` lookup table (removing the old `STATUS_ORDER.indexOf` fallback that caused all unknown statuses to display as "Resolved").

### BUG-3 [CRITICAL] — Dispute JSON field name mismatches

**Files:** `backend/internal/dispute/model.go`, `mobile/lib/hooks/useDispute.ts`

Updated backend JSON tags on `Dispute` struct to match frontend interface field names:
- `Route` → `json:"escalationRoute,omitempty"`
- `ChargeAmount` → `json:"damageChargeCents,omitempty"`
- `Confidence` → `json:"agentConfidence,omitempty"`
- `ReviewerID` → `json:"resolvedBy,omitempty"`

Renamed `agentDecision: string | null` (was displaying a UUID) to `agentDecisionId: string | null` in the frontend interface. The `isInconclusive` check now correctly uses `dispute.status === "INCONCLUSIVE"` instead of `dispute.agentDecision === "INCONCLUSIVE"`. Added `evidence: DisputeEvidence | null` with typed structure for extracting `photoDiffResult`, `photoDiffConfidence`, `checkInMedia`, `checkOutMedia`.

### BUG-4 [HIGH] — CheckOut navigated to booking-status instead of return-confirmation

**File:** `mobile/components/screens/CheckOutScreen.native.tsx:55`

Changed `router.replace` destination from `/(tabs)/(feed)/booking-status` to `/(tabs)/(rentals)/return-confirmation`. Users now see the return summary, hold status, and post-return CTAs immediately after check-out.

### BUG-5 [HIGH] — DISPUTED bookings navigated to booking-status instead of dispute-status

**File:** `mobile/app/(tabs)/(rentals)/index.tsx:229`

Added explicit `DISPUTED` branch in `handleBookingPress` that routes to `/(tabs)/(rentals)/dispute-status` with `transactionId`. Previously DISPUTED fell through to the `else` branch and landed on the generic booking-status screen.

### BUG-6 [MEDIUM] — HoldStatusCard showed $0.00 when backend data is unavailable

**File:** `mobile/app/(tabs)/(rentals)/return-confirmation.tsx`

Introduced `hasHoldData = holdAllocation.authorizedCents > 0` guard. The `HoldStatusCard` is now conditionally rendered only when real hold data is available, preventing misleading "Total authorized: $0.00" and "Hold release is pending" messages when the backend hasn't returned hold data yet.

### BUG-7 [MEDIUM] — PhotoDiffResult was never rendered

**File:** `mobile/app/(tabs)/(rentals)/dispute-status.tsx`

Integrated `PhotoDiffResult` into `DisputeDetail`. When `dispute.evidence` contains `photoDiffResult`, the component shows the overall classification badge and any matched check-in/check-out photo pairs (paired by index from `checkInMedia` and `checkOutMedia` arrays in the evidence package).

### BUG-8 [LOW] — INCONCLUSIVE reprompt was a dead Alert stub

**File:** `mobile/app/(tabs)/(rentals)/dispute-status.tsx`

Replaced the informational `Alert.alert(...)` stub with `router.push` to `/(tabs)/(feed)/check-out` with the transaction ID. The check-out camera screen is the best available re-upload surface until a dedicated evidence re-upload flow is built.

### BUG-9 [LOW] — Dead `textColor` prop in HoldStatusCard BarSegment

**File:** `mobile/components/rental/HoldStatusCard.tsx`

Removed `textColor: string` from `BarSegmentProps` and the dead `{textColor === "" && null}` no-op. All four `BarSegment` call sites were updated to omit the unused prop.

---

## Test Impact

- Updated `mobile/__tests__/components/rental.test.tsx` to use the new status values (`GATHERING`, `HUMAN_REVIEW` instead of `EVIDENCE_GATHERING`, `UNDER_REVIEW`).
- Updated `mobile/lib/hooks/usePostRental.ts` to use `CLOSED_STATUSES` from `useDispute.ts` (removed the `d.status !== "CLOSED"` comparison which TypeScript correctly flagged as impossible after the type update).

## Verification

All passed:
- `cd mobile && npx tsc --noEmit` ✓
- `cd mobile && npx jest` — 91/91 tests ✓
- `cd backend && go vet ./...` ✓
- `cd backend && go build -o /dev/null ./cmd/server` ✓

---

## Next Tasks Ready

- **8.13** — Audit: Profile + Referrals + KYC (deps: 8.0 ✓)
- **8.15** — Audit: Ops Dashboard + Backend Ops Tools (deps: 8.0 ✓)
- **8.17** — Documentation Cleanup (no deps)
- **8.18** — Compilation & Test Suite Health Check (no deps)
