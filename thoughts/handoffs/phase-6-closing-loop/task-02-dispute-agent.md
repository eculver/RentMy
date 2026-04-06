# Task 6.2 — DisputeAgent with Escalation Gate

## Summary

Implemented the full DisputeAgent service with escalation gate routing, evidence gathering, LLM-based damage assessment, hold allocation capture, and human review workflow.

## What was built

### New package: `backend/internal/dispute/`

| File | Purpose |
|------|---------|
| `model.go` | Domain types: Dispute, EvidencePackage, FileDisputeInput, ResolveInput, AgentDecisionOutput |
| `repository.go` | Full CRUD + evidence gathering from existing tables (media, messages, proximity_proofs, transactions) |
| `escalation.go` | Escalation gate routing logic implementing PRD §20 routing table |
| `hold.go` | HoldService for damage captures, hold release, and overflow handling (card charge + guarantee fund) |
| `prompt.go` | DisputeAgent LLM prompt template (v1) with structured JSON output |
| `service.go` | Core business logic: FileDispute, GatherEvidence, RunDisputeAgent, RouteAndExecute, ResolveByHuman |
| `jobs.go` | River workers: DisputeResolutionWorker, RePromptExpiryWorker, SLAMonitorWorker |
| `handler.go` | HTTP handlers mounted on chi router |

### New migration: `backend/migrations/011_disputes.sql`

Creates the `disputes` table with indexes on transaction_id, status, sla_deadline, and reporter_id.

### Modified files

| File | Change |
|------|--------|
| `backend/internal/platform/config/config.go` | Added `DisputeSLAActiveHours`, `DisputeSLAPostReturnHours` |
| `backend/internal/payment/service.go` | Added `UpdateTransactionStatus`, `ChargeForDamageOverflow`, `ClaimGuaranteeFund` |
| `backend/app/server.go` | Wired dispute service, registered River workers, mounted handler |
| `backend/tests/integration/helpers_test.go` | Added disputes to CleanupDB truncation, added SLA config |

### API Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/api/v1/transactions/:id/disputes` | File a dispute |
| GET | `/api/v1/transactions/:id/disputes` | Get disputes for a transaction |
| GET | `/api/v1/disputes/:id` | Get dispute by ID |
| GET | `/api/v1/admin/disputes/review-queue` | Human review queue (paginated) |
| POST | `/api/v1/admin/disputes/:id/resolve` | Human reviewer approves/overrides |
| POST | `/api/v1/admin/disputes/:id/request-evidence` | Request more evidence |

### Escalation Gate Routing

| Condition | Route |
|-----------|-------|
| Confidence >= 0.85 AND charge <= $200 | AUTO_RESOLVE |
| Confidence >= 0.85 AND charge $201-$1000 | AUTO_RESOLVE_AUDIT |
| Confidence >= 0.85 AND charge > $1000 | HUMAN_REVIEW |
| Confidence < 0.85 | HUMAN_REVIEW |
| Photo diff INCONCLUSIVE | HUMAN_REVIEW |
| Fraud flags active | HUMAN_REVIEW (overrides all) |

## Tests

- **Unit tests** (14 cases): `escalation_test.go` — all routing table permutations + edge cases
- **Unit tests** (2 cases): `prompt_test.go` — prompt construction verification
- **Integration tests** (6 cases): `dispute_api_test.go` — file dispute, duplicate rejection, get by transaction, get by ID, not found, review queue

## Dependencies used

- Existing: pgx, chi, River, agent/decision, agent/router, payment
- No new external dependencies added

## Branching

- Branch: `task-6.2-dispute-agent`
- Mode: Graphite

## Notes for downstream tasks

- Task 6.3 (LateReturnAgent) can now escalate to DisputeAgent via `dispute.Service.FileDispute`
- Task 6.7 (Outcome Linking) can link outcomes to dispute agent decisions via `agent_decision_id` on disputes
- The `HoldService.CaptureAndEscalate` handles overflow to guarantee fund, used by both dispute and late return
- SLA monitoring runs as a periodic River job but does not yet send notifications (depends on OpsAgent in Phase 7)
