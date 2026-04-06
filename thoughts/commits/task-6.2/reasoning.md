# Commit Reasoning — Task 6.2: DisputeAgent with Escalation Gate

## Why this approach

The DisputeAgent is the core dispute resolution pipeline. It must:
1. Gather evidence from multiple existing tables (no data duplication)
2. Call an LLM for damage assessment
3. Route decisions through a configurable escalation gate
4. Execute hold captures or queue for human review
5. Support human override of agent decisions

### Key design decisions

**Escalation gate as pure function:** `RouteDecision` takes confidence, charge amount, photo diff result, and fraud flags — no dependencies. This makes it trivially testable and easy to tune thresholds.

**Evidence gathering reads from existing tables:** The plan requires assembling evidence from transactions, media, messages, proximity_proofs, and photo_diff — all via read-only queries. No denormalization.

**HoldService delegates to PaymentService:** Rather than duplicating Stripe capture logic, HoldService is a thin wrapper over payment.CaptureFromHold with overflow handling (card charge → guarantee fund fallback).

**Pre-river worker pattern:** Same pattern used by notification/verification services. Workers are created before River starts with nil dependencies, then replaced with full dependencies after River client exists.

**Payment service extensions:** Added `UpdateTransactionStatus`, `ChargeForDamageOverflow`, and `ClaimGuaranteeFund` to payment.Service. These were needed for dispute resolution but are general-purpose enough for LateReturnAgent (task 6.3) and future phases.
