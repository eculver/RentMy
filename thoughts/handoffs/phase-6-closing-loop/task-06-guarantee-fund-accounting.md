# Task 6.6 — Guarantee Fund Accounting

## Summary

Created a dedicated `guaranteefund` package that encapsulates the platform guarantee fund ledger, reserve ratio monitoring, loss ratio tracking, and admin API endpoints.

## What Was Done

### Created: `backend/internal/guaranteefund/`

- **model.go** — `Entry` domain type with typed `EntryType` enum (`CONTRIBUTION`, `CLAIM`, `CARD_RECOVERY`, `COLLECTIONS_REFERRAL`), `FundHealth` struct (balance, outstandingGaps, reserveRatio, lossRatio, action), `ReserveAction` enum with four levels per PRD §7
- **repository.go** — `InsertEntry` (double-entry with running balance_after), `GetCurrentBalance`, `GetOutstandingGaps`, `GetRolling90DayClaims`, `GetRolling90DayContributions`, `GetEntries` (paginated ledger)
- **service.go** — `Contribute`, `Claim` (fund cannot go negative, caps at available balance), `RecordCardRecovery`, `RecordCollectionsReferral`, `GetFundHealth` (full health with reserve ratio + loss ratio + recommended action), `CheckReserveRatio` (four-tier threshold logic)
- **jobs.go** — `FundHealthCheckWorker` (hourly health check, logs alerts at degraded thresholds), `LossRatioCheckWorker` (daily loss ratio analysis, warns when above 0.6 target)
- **handler.go** — `GET /api/v1/admin/guarantee-fund/health` (fund health dashboard), `GET /api/v1/admin/guarantee-fund/entries` (paginated ledger entries)
- **service_test.go** — 20 unit tests covering all reserve ratio thresholds (table-driven), boundary conditions, and enum values

### Modified

- **platform/config/config.go** — Added `ReserveRatioNormal` (0.15), `ReserveRatioAlert` (0.10), `ReserveRatioRestrictHigh` (0.05), `LossRatioTarget` (0.6) config fields
- **app/server.go** — Wired guarantee fund service (pre-river + full), registered `FundHealthCheckWorker` and `LossRatioCheckWorker`, mounted handler on API router

## Design Decisions

1. **Separate package from payment**: The plan calls for a dedicated `guaranteefund` package. The existing payment service already has basic fund operations (`ClaimGuaranteeFund`, `InsertGuaranteeFundEntry`). The new package provides the monitoring, health check, and admin API layer. The payment service's existing methods continue to work for the transactional path (booking contributions and claims during dispute resolution).

2. **Reserve ratio thresholds from PRD §7**: Normal (≥15%), Alert (10-15%), Restrict High Value (5-10%), Restrict All Gap (<5%).

3. **Loss ratio**: Rolling 90-day window as specified in PRD. Claims/contributions queried from the existing `guarantee_fund_entries` table.

4. **No database migration needed**: The `guarantee_fund_entries` table was already created in the initial schema (001_initial_schema.sql).

## Verification

- `go vet ./...` — clean
- `go build ./cmd/server` — clean
- `go test ./... -count=1` — all 23 packages pass
- `go test ./internal/guaranteefund/... -v -count=1` — 20/20 tests pass

## Branch Mode

Graphite (`gt create`)

## Dependencies Added

None — uses existing project dependencies (chi, pgx, river, testify).
