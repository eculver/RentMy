# Phase 6 — Returns, Disputes, Trust — Progress Ledger

## Status: In Progress

## Tasks

| Task | Name | Status | Branch | Commit |
|------|------|--------|--------|--------|
| 6.1 | Photo diff pipeline | Completed | task-6.1-photo-diff-pipeline | 0c5d226 |
| 6.2 | DisputeAgent with escalation gate | Completed | task-6.2-dispute-agent | 35a58c6 |
| 6.3 | LateReturnAgent | Completed | task-6.3-late-return-agent | 6d867cd |
| 6.4 | Rating system (backend + RN) | Pending | — | — |
| 6.5 | Reputation score recalculation | Pending | — | — |
| 6.6 | Guarantee fund accounting | Pending | — | — |
| 6.7 | Outcome linking (Agent Learning Framework) | Pending | — | — |
| 6.8 | Post-rental flow (RN) | Pending | — | — |

## Notes

### Task 6.1 — Photo Diff Pipeline
- Two-stage pipeline: Python CV sidecar (Stage 1) + LLM via model router (Stage 2)
- cv-service runs in SAM 2 passthrough mode until model weights are provisioned
- Graceful degradation built in: cv-service failure → raw images, model router unavailable → INCONCLUSIVE
- All existing tests continue to pass

### Task 6.2 — DisputeAgent with Escalation Gate
- Full dispute service with escalation gate routing (PRD §20 routing table)
- Evidence gathering from existing tables (media, messages, proximity, photo diff, agreements)
- LLM-based damage assessment via model router (TaskEvidenceAnalysis)
- Hold capture via payment service, overflow handling (card + guarantee fund)
- Human review workflow with SLA monitoring (River periodic job)
- Added `UpdateTransactionStatus`, `ChargeForDamageOverflow`, `ClaimGuaranteeFund` to payment service
- 14 unit tests for escalation gate, 6 integration tests for API endpoints
- All existing tests continue to pass

### Task 6.3 — LateReturnAgent
- Two-stage architecture: deterministic hourly charging + LLM-based escalation
- Damage reserve cap enforced: late fees capped at 60% of hold (configurable), 40% reserved for DisputeAgent
- River jobs: LateReturnCheckJob (fires at scheduled_end, re-enqueues hourly), LateReturnEscalationJob (fires at 4h+ threshold)
- Double rate for conflicting bookings per PRD §19
- ScheduleLateReturnCheck() integrated into booking.Accept()
- Conservative escalation: LLM defaults to WARNING on failure, FLAGGED_FOR_REVIEW is rare
- 4 test groups covering hourly rate calculation, fee cap enforcement, damage reserve invariant, status mapping
- All existing tests continue to pass
