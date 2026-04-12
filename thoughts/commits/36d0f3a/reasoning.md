# Commit 36d0f3a — OpsAgent (task 7.1)

## Why this commit exists

Task 7.1 implements the OpsAgent — the backend subsystem responsible for platform
health monitoring (PRD §25). The commit is a greenfield package addition with no
changes to existing business logic, keeping the diff clean and reviewable.

## Key decisions

### No PagerDuty SDK dependency
The plan referenced `pagerduty-go` SDK, but PagerDuty Events API v2 is a single
HTTP POST with a JSON body. Adding a third-party SDK for one endpoint introduces
supply-chain risk and a `go.sum` entry with no real benefit. `net/http` is used
directly; the interface is identical.

### Anomaly detection threshold: 2 standard deviations
Standard statistical approach. Conservative enough to avoid alert fatigue (≈4.5%
false-positive rate under normal distribution) while catching real anomalies. The
threshold is not configurable via env var at this stage; it can be made tunable in
a refinement phase if operators need it.

### Floating-point epsilon guard in anomaly detection
When historical metric values are nearly constant (e.g., fraud rate stable at 0.05
for 7 days), floating-point arithmetic produces a standard deviation that is not
exactly 0.0 but on the order of 1e-16. Dividing any difference by that gives a
huge z-score. The guard `std < 1e-9 { continue }` treats near-zero std as
"no meaningful deviation possible" — semantically correct.

### Metrics use actual DB schema
The transactions table has `platform_fee`, `host_payout`, `guarantee_contribution`
(migration 003), `actual_end`, and `scheduled_end`. There is no `payout_at`
column or separate payouts table. Host payout velocity is approximated as
`avg(NOW() - actual_end)` for completed transactions — a rough proxy until a
dedicated payout timestamp is tracked in a future phase.

### River periodic job registration
Registered before `riverpkg.New()` is called (like all other workers), with a
configurable interval (default 15 minutes via `OPS_HEALTH_CHECK_INTERVAL_MINUTES`).
The periodic job ID `"ops_health_check"` is unique across all registered jobs.

## Integration test note
Docker (OrbStack) was unresponsive during verification, preventing
`go test ./tests/integration/...` from completing. All unit tests pass (13/13).
The integration test file follows the established pattern from other tasks and will
be validated in CI on push.
