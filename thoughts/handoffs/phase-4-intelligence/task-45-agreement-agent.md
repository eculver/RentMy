# Task 4.5 — AgreementAgent Handoff

## Summary

Implemented the AgreementAgent from PRD §14. When a host accepts a booking, an agreement is automatically generated: the base lawyer-reviewed template is loaded, Sonnet generates item-specific clauses, guardrail validation strips any clauses that would weaken liability/arbitration language, and the merged agreement is persisted as an immutable snapshot. Both parties can then accept via HTTP endpoint before the booking goes ACTIVE.

## Branching Mode

Git fallback (vanilla git). Branch: `task-4.5-agreement-agent`. Commit: `e3104da`.

## What Was Built

### `backend/migrations/010_agreements.sql`

Two new tables:
- `agreements` — immutable agreement snapshot per transaction. Stores `full_agreement` JSONB, `custom_clauses` JSONB, `guardrail_warnings` JSONB, template version, model, and prompt version.
- `agreement_acceptances` — per-user acceptance log. UNIQUE index on `(agreement_id, user_id)` prevents double-acceptance. Records IP address and device ID for audit.

### `backend/internal/agent/agreement/`

| File | Purpose |
|------|---------|
| `model.go` | `Clause`, `ClauseCategory`, `Agreement`, `Acceptance`, `AcceptanceStatus`, HTTP response/request types, AI result structs, base template structs |
| `repository.go` | `Insert`, `FindByTransactionID`, `FindByID`, `InsertAcceptance`, `FindAcceptances`, `UpdateAgreementSnapshot`, `GetTransactionParties`, `GetListingForTransaction`, `GetAppraisalCategory`, `GetLastAcceptedAt` |
| `service.go` | `GenerateAgreement` (idempotent), `TriggerAgreement` (satisfies booking interface), `ValidateAcceptance`, `GetAcceptanceStatus`, `BothPartiesAccepted`, `GetAgreement`, `generateClauses` (Sonnet call + guardrail filtering), `loadBaseTemplate`, `mergeClauses` |
| `handler.go` | `GET /api/v1/transactions/:id/agreement`, `POST /api/v1/transactions/:id/agreement/accept`, `GET /api/v1/transactions/:id/agreement/status` |
| `service_test.go` | 5 tests: `TestLoadBaseTemplate`, `TestMergeClauses`, `TestGuardrailCheck` (6 sub-cases), `TestGuardrails`, `TestMergedAgreementIsValidJSON` |

### Changes to Existing Files

**`backend/internal/booking/service.go`**
- Added `agreementSvc` interface (`TriggerAgreement(ctx, transactionID) error`)
- Added `agreementSvc` field (nil-safe)
- Added `WithAgreementAgent(a agreementSvc) *Service` builder method
- `Accept` now calls `TriggerAgreement` after the status commit (best-effort, logs warning on failure)

**`backend/cmd/server/main.go`**
- Imports `backend/internal/agent/agreement`
- Builds `agreementRepo` and `agreementSvc`
- Chains `.WithAgreementAgent(agreementSvc)` onto `bookingSvc`
- Builds `agreementHandler` and mounts on `/api/v1`

## Design Decisions

- **Idempotent generation** — `GenerateAgreement` returns the existing agreement if one already exists for the transaction. Re-calling on accept is safe.
- **Guardrail validation is programmatic (regex), not AI** — Banned patterns cover: `limit.*liability`, `no.*liability`, `waive.*arbitration`, `not.*subject.*arbitration`, `no.*hold`, `refund`, `no.*fee`, `waive.*fee`. This is deterministic and fully auditable. Stripped clauses are logged in `guardrail_warnings` JSONB for review.
- **`TriggerAgreement` adapter method** — The booking service uses a narrow interface to avoid an import cycle. `TriggerAgreement` wraps `GenerateAgreement` and discards the return value.
- **`UpdateAgreementSnapshot` writes to `transactions.agreement_snapshot`** — The `agreement_snapshot` JSONB column already exists on the transactions table (from the initial schema). The agreement package writes to it directly without going through the booking package.
- **Nil-safe model router** — When `ANTHROPIC_API_KEY` is absent, `generateClauses` is skipped. The agreement is still generated with only the base template sections and zero custom clauses (valid and functional).
- **Prompt path resolution via `runtime.Caller`** — Avoids hardcoded absolute paths in production or relative paths that break depending on the working directory. Works in both `go test` and the server binary.
- **No River jobs** — Generation is synchronous (called in the Accept flow). The Sonnet call is covered by the 120s router timeout. If generation fails, it logs a warning and booking acceptance still commits. Agreement can be manually triggered later via the appraise endpoint pattern if needed.

## Guardrail Coverage

| Pattern | Target | Protects |
|---------|--------|----------|
| `limit.*liability` | liability limitation | §5 Liability |
| `no.*liability` | liability negation | §5 Liability |
| `waive.*arbitration` | arbitration waiver | §8 Governing Law |
| `not.*subject.*arbitration` | arbitration opt-out | §8 Governing Law |
| `no.*hold` | security hold removal | §4 Security Hold |
| `refund` | unauthorised refund clauses | §4 Security Hold |
| `no.*fee` | fee waiver | §7 Cancellation, §4 Late Fees |
| `waive.*fee` | fee waiver | §7 Cancellation, §4 Late Fees |

## Verification

All passed:
```
cd backend && go vet ./...              # no issues
cd backend && go build ./cmd/server    # clean build
cd backend && go test ./...            # all packages green
cd backend && go test ./internal/agent/agreement/ -v -count=1  # 5 tests PASS
```
