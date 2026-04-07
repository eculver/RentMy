# Task 6.9 — Guarantee Fund Refinement

## Summary

Refactored guarantee fund operations to eliminate code duplication between `payment` and `guaranteefund` packages, wired River periodic job scheduling, enhanced claim escalation logic, and added comprehensive tests.

## Changes

### 1. Code Consolidation (payment → guaranteefund)

**Removed from `payment` package:**
- `GuaranteeFundEntry` and `GuaranteeFundHealth` types from `model.go`
- `InsertGuaranteeFundEntry`, `GetGuaranteeFundBalance`, `GetTotalOutstandingGuaranteeGaps` from `repository.go`
- `GetGuaranteeFundHealth`, `ClaimGuaranteeFund` from `service.go`

**Added to `payment.Service`:**
- `guaranteeFundSvc *guaranteefund.Service` dependency (injected via constructor)
- `CreateBooking` now delegates contribution to `guaranteefund.Service.Contribute`

### 2. River Periodic Jobs

- Updated `platform/river.New()` to accept variadic `*river.PeriodicJob` args
- `app/server.go` now configures:
  - `FundHealthCheckJob` — runs every 1 hour
  - `LossRatioCheckJob` — runs every 24 hours

### 3. Claim Escalation

- `guaranteefund.Service.Claim()` now returns `ClaimResult{Requested, Claimed, Shortfall}`
- `dispute/hold.go:CaptureAndEscalate` uses `ClaimResult.Shortfall` to trigger `RecordCollectionsReferral` for uncovered amounts
- `dispute.HoldService` now depends on both `payment.Service` and `guaranteefund.Service`

### 4. Repository Interface

- Added `RepositoryInterface` to `guaranteefund/service.go` to enable mock-based testing
- `Service.repo` field changed from `*Repository` to `RepositoryInterface`

### 5. Tests

**Unit tests (11 new, 22 total):**
- `TestContribute_InsertsEntryAndUpdatesBalance`
- `TestContribute_SkipsZeroAmount`, `TestContribute_SkipsNegativeAmount`
- `TestClaim_FullAmount_WhenSufficientBalance`
- `TestClaim_PartialAmount_WhenInsufficientBalance`
- `TestClaim_FailsWhenFundEmpty`, `TestClaim_FailsWhenBalanceNegative`
- `TestGetFundHealth_CalculatesAllMetrics`, `TestGetFundHealth_ZeroGaps`, `TestGetFundHealth_AlertState`
- `TestDoubleEntryIntegrity`

**Integration tests (2 new):**
- `TestGuaranteeFundAdminEndpoints` — health + entries endpoints return correct shapes
- `TestGuaranteeFundLedgerIntegrity` — ledger balance tracks through contributions and claims

## Dependencies Added

None — all imports were already in go.mod.

## Branching

Graphite mode. Branch: `task-6.9-guarantee-fund-refinement`

## Verification

- `go vet ./...` — pass
- `go build ./cmd/server` — pass
- `go test ./internal/guaranteefund/...` — 22/22 pass
- `go test ./internal/payment/...` — 11/11 pass
- `go test ./tests/integration/... -run GuaranteeFund` — 2/2 pass
