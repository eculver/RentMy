# Task 7.1 — OpsAgent (backend)

## Summary

Implements the OpsAgent: platform health monitoring, anomaly detection, and alert
routing. The agent runs every 15 minutes as a River periodic job, collects metrics
from four categories, stores a `HealthSnapshot`, compares against rolling 7-day
history to flag anomalies, and dispatches triggered alerts to Slack and/or PagerDuty.

## Files Created

| File | Purpose |
|------|---------|
| `backend/migrations/013_ops_agent.sql` | DB schema: ops_health_snapshots, ops_alert_rules, ops_alerts |
| `backend/internal/agent/ops/model.go` | Domain types: HealthSnapshot, AlertRule, Alert, MetricValue, etc. |
| `backend/internal/agent/ops/errors.go` | Sentinel errors: ErrNoSnapshot, ErrRuleNotFound, ErrAlertNotFound |
| `backend/internal/agent/ops/repository.go` | Postgres queries for all three ops tables |
| `backend/internal/agent/ops/metrics.go` | MetricsCollector (raw SQL aggregations) + DetectAnomalies |
| `backend/internal/agent/ops/alerts.go` | EvaluateRules, AlertRouter.RouteAlert |
| `backend/internal/agent/ops/slack.go` | SlackSender: POST to webhook, retry on 5xx |
| `backend/internal/agent/ops/pagerduty.go` | PagerDutySender: PagerDuty Events API v2 via net/http |
| `backend/internal/agent/ops/agent.go` | Agent.Run: orchestrates full health-check cycle |
| `backend/internal/agent/ops/jobs.go` | HealthCheckWorker: River worker + args |
| `backend/internal/agent/ops/handler.go` | HTTP handlers mounted at /api/v1/ops/... |
| `backend/internal/agent/ops/alerts_test.go` | Unit tests: EvaluateRules (6 cases) |
| `backend/internal/agent/ops/metrics_test.go` | Unit tests: DetectAnomalies + meanStdDev (7 cases) |
| `backend/tests/integration/ops_api_test.go` | Integration tests: 7 endpoint tests |

## Files Modified

| File | Change |
|------|--------|
| `backend/internal/platform/config/config.go` | Added SlackWebhookURL, PagerDutyRoutingKey, OpsHealthCheckIntervalM |
| `backend/app/server.go` | Wired OpsAgent: repository, collector, Slack/PD senders, alert router, agent, worker, handler, periodic job |
| `backend/tests/integration/helpers_test.go` | Added ops tables to CleanupDB TRUNCATE list |

## HTTP API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/ops/metrics/current` | Latest health snapshot |
| GET | `/api/v1/ops/metrics/history?duration=7d\|30d\|90d` | Snapshot history |
| GET | `/api/v1/ops/alerts` | Paginated alerts (filters: severity, acknowledged, page, limit) |
| PUT | `/api/v1/ops/alerts/:alertId/acknowledge` | Acknowledge a fired alert |
| GET | `/api/v1/ops/alerts/rules` | List all alert rules |
| PUT | `/api/v1/ops/alerts/rules/:ruleId` | Update rule threshold/severity/channel/enabled |

All endpoints require a valid JWT (standard auth middleware).

## Environment Variables Added

| Variable | Default | Purpose |
|----------|---------|---------|
| `SLACK_WEBHOOK_URL` | `""` | Slack incoming webhook; empty = no Slack alerts |
| `PAGERDUTY_ROUTING_KEY` | `""` | PagerDuty Events API v2 routing key; empty = no PD alerts |
| `OPS_HEALTH_CHECK_INTERVAL_MINUTES` | `15` | River periodic job interval in minutes |

## Alert Routing Logic

- `INFO` / `WARNING` → Slack only
- `CRITICAL` → Slack + PagerDuty (if PD key configured)
- PagerDuty dedup key: `{rule_id}-{metric_name}-{hour}` (prevents duplicate pages in same hour)

## Anomaly Detection

Uses 2-sigma z-score against the rolling 7-day snapshot history. A metric is flagged
as anomalous when `|current - rolling_mean| / rolling_std > 2.0`. Metrics where
`std < 1e-9` (effectively constant history) are skipped to avoid floating-point
artifacts.

## Design Notes

- **No PagerDuty SDK**: Used `net/http` directly — PD Events API v2 is a single
  POST with JSON body; no SDK needed.
- **Payout velocity approximation**: `avg(NOW() - actual_end)` for completed
  transactions since no `payout_at` column exists in the DB (Stripe owns that).
- **Alert rules seeded at runtime**: Rules are stored in the DB; ops staff upsert
  them via the API or migration. No hardcoded defaults at startup.

## Verification

| Check | Result |
|-------|--------|
| `go vet ./...` | PASS |
| `go build ./cmd/server` | PASS |
| `go test ./internal/agent/ops/... -v` | PASS (13/13 tests) |
| `go test $(non-integration packages) -count=1` | PASS (24 packages) |
| Integration tests | SKIPPED — Docker unresponsive during session; CI will validate |

## Branch Mode

Graphite (`gt create task-7.1-ops-agent`)

## Dependencies Added

None (PagerDuty via net/http instead of pagerduty-go SDK)

## Next Tasks

- 7.2: FraudAgent (depends on 7.1 — same branch stack)
- 7.3: Ops dashboard web (depends on 7.1 + 7.2)
