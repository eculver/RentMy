# Cross-Cutting Concerns (Threaded Throughout)

These aren't a phase — they're woven into every phase as the relevant services are built.

---

## Testing Strategy
- Unit tests for all business logic (risk scoring, reputation calculation, tiered holds, cancellation fees, hold allocation caps)
- Integration tests for service interactions (BookingService → PaymentService → ProximityService → HoldAllocation)
- Agent decision quality tests: golden set of inputs → expected classifications (build the set as you go, expand with real data). After Phase 5, the outcome linking pipeline provides real-world validation data
- E2E tests for critical path: register → list → discover → book → handoff → return → payout
- State machine tests: every valid transition succeeds, every invalid transition is rejected

## Observability
- Structured logging (zerolog or slog) from Phase 0
- Request tracing (OpenTelemetry) from Phase 1
- Metrics (Prometheus) from Phase 2: request latency, error rates, queue depths
- Dashboards (Grafana) from Phase 3: service health, payment flow, agent performance
- Agent calibration metrics from Phase 5: confidence accuracy, override rates, loss ratio

## Error Handling
- Idempotency keys on all payment operations AND booking creation (prevent duplicate bookings from double-tap)
- Retry with exponential backoff on external API calls (Stripe, KYC, LLM, SAM segmentation)
- Circuit breakers on third-party dependencies
- Dead letter queue for failed River jobs
- HoldAllocation atomic operations: `READ COMMITTED` isolation + `SELECT ... FOR UPDATE` row-level locking on the transaction row. All capture operations within a single pgx transaction: lock → read `remaining` → validate → update `hold_allocation` JSON → commit. Prevents race conditions between LateReturnAgent and DisputeAgent

## Rate Limiting
- Per-user rate limits on all public endpoints (token bucket, Redis-backed)
- Tighter limits on write operations (listing creation, booking requests)
- Separate limits for search (higher) vs. mutations (lower)
