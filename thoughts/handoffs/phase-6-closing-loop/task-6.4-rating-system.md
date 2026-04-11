# Task 6.4 — Rating System (backend + RN)

**Status:** Completed  
**Commit:** 356ec39bf92ede62f87a9c614976d4f52e04dc75  
**Date:** 2026-04-06  
**Branch mode:** Graphite (gt create task-6.4-rating-system)

---

## What was implemented

### Backend (`backend/internal/rating/`)

- **model.go** — `Bubble` type with full constant sets for renter-rates-host and host-rates-renter. Validation helpers `ValidateBubblesForRenter` and `ValidateBubblesForHost`. Sentinel errors for all domain failure modes.
- **helpers.go** — `marshalBubbles`/`unmarshalBubbles` for JSONB serialisation, `isUniqueViolation` matching Postgres 23505.
- **repository.go** — `FindTransactionForRating` (lightweight status/renter/host projection), `HasUserRated`, `Insert` (UNIQUE constraint maps to `ErrAlreadyRated`), `FindByTransactionID`, `FindByToUserID` (paginated), `BubbleSummary` (JSON unnest aggregation).
- **service.go** — `SubmitRating` validates COMPLETED status, correct bubble set by role, inserts rating, then emits one `positive_rating` signal per bubble via `riskSvc.EmitSignal()`. Signal emit failures are non-fatal (logged, do not fail the request). Also `GetRatingsForTransaction`, `GetRatingsForUser`, `GetRatingBubbleSummary`.
- **handler.go** — Four authenticated endpoints:
  - `POST /api/v1/transactions/:id/ratings` → 201/400/403/409/422
  - `GET /api/v1/transactions/:id/ratings` → 200
  - `GET /api/v1/users/:id/ratings` → 200 (paginated)
  - `GET /api/v1/users/:id/ratings/summary` → 200 (bubble counts)
- **service_test.go** — 8 unit tests using fake repo/risk agent; all pass.

### Integration tests (`backend/tests/integration/rating_api_test.go`)

7 integration tests covering: renter-rates-host, host-rates-renter, duplicate 409, not-completed 422, wrong-bubble 400, get-transaction-ratings, get-user-ratings, get-summary.

### Wire-up (`backend/app/server.go`)

Added `ratingRepo`, `ratingSvc`, `ratingHandler` and mounted the router after `lateReturnHandler`.

### Mobile

- **`mobile/lib/hooks/useRatings.ts`** — `RatingBubble` type, `BUBBLE_LABELS`, `RENTER_BUBBLES`, `HOST_BUBBLES`, `useTransactionRatings`, `useUserRatings`, `useUserRatingsSummary`, `useSubmitRating`.
- **`mobile/components/rating/RatingBubbles.tsx`** — Tappable pill tags. `readOnly` prop for display-only mode (used on profile screen).
- **`mobile/components/rating/RatingPrompt.tsx`** — Post-rental modal. Determines correct bubble set from renterID. Submits via `useSubmitRating`, shows success/error feedback, auto-dismisses after 800ms on success.
- **`mobile/app/(tabs)/(profile)/index.tsx`** — Fetches bubble summary via `useUserRatingsSummary`; renders `RatingBubbles` in read-only mode with count labels below when the user has received ratings.
- **`mobile/__tests__/components/rating.test.tsx`** — 7 component tests.

---

## Decisions and rationale

- **Reputation via EmitSignal (synchronous)** — The plan said "enqueue reputation recalculation job". The existing `riskSvc.EmitSignal` already inserts the signal and calls `RecalculateReputation` synchronously. No separate River job was needed — this is consistent with how dispute, late return, and other agents integrate with the risk service.
- **Signal emit failures non-fatal** — A rating submission should not fail because reputation recalculation had a transient DB error. The signal is still inserted (or deduped); the score will be corrected when the next signal fires or on the monthly recalculation job.
- **No new dependencies** — All libraries already present in the repo.

---

## Verification results

```
go vet ./...                          ✅ clean
go build ./cmd/server                 ✅ clean
go test ./internal/rating/... -v      ✅ 8/8 pass
go test ./... (non-integration)       ✅ all pass
go test ./tests/integration/...       ✅ all pass (7 new + all existing)
npx tsc --noEmit                      ✅ clean
npx jest                              ✅ 66/66 pass (7 new)
```
