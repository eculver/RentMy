package ops

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Brett2thered/RentMy/backend/internal/platform/ulid"
)

// Agent orchestrates the full OpsAgent health-check cycle:
// 1. Collect all metrics
// 2. Build and store a HealthSnapshot
// 3. Detect anomalies against rolling history
// 4. Evaluate configured alert rules
// 5. Route triggered alerts
type Agent struct {
	repo      *Repository
	collector *MetricsCollector
	router    *AlertRouter
}

// New creates an Agent with all dependencies wired.
func New(repo *Repository, collector *MetricsCollector, router *AlertRouter) *Agent {
	return &Agent{repo: repo, collector: collector, router: router}
}

// Run executes a full health-check cycle.  It is called by the River periodic
// job every 15 minutes.
func (a *Agent) Run(ctx context.Context) error {
	slog.Info("ops: health check starting")

	biz, err := a.collector.CollectBusinessMetrics(ctx)
	if err != nil {
		return fmt.Errorf("ops: collect business metrics: %w", err)
	}

	trust, err := a.collector.CollectTrustMetrics(ctx)
	if err != nil {
		return fmt.Errorf("ops: collect trust metrics: %w", err)
	}

	supply, err := a.collector.CollectSupplyMetrics(ctx)
	if err != nil {
		return fmt.Errorf("ops: collect supply metrics: %w", err)
	}

	demand, err := a.collector.CollectDemandMetrics(ctx)
	if err != nil {
		return fmt.Errorf("ops: collect demand metrics: %w", err)
	}

	snap := HealthSnapshot{
		ID:         ulid.New(),
		Business:   biz,
		Trust:      trust,
		Supply:     supply,
		Demand:     demand,
		CapturedAt: time.Now().UTC(),
	}

	// Detect anomalies using the last 7 days of history.
	history, err := a.repo.GetSnapshotHistoryRaw(ctx, 7*24*time.Hour, 672) // up to 672 snapshots (15-min intervals × 7 days)
	if err != nil {
		slog.Warn("ops: could not fetch history for anomaly detection", "error", err)
	} else {
		snap.Anomalies = DetectAnomalies(snap, history)
		if len(snap.Anomalies) > 0 {
			slog.Info("ops: anomalies detected", "count", len(snap.Anomalies))
		}
	}

	if err := a.repo.InsertHealthSnapshot(ctx, snap); err != nil {
		return fmt.Errorf("ops: store snapshot: %w", err)
	}

	// Evaluate alert rules.
	rules, err := a.repo.ListAlertRules(ctx)
	if err != nil {
		return fmt.Errorf("ops: list alert rules: %w", err)
	}

	triggered := EvaluateRules(snap, rules)
	values := extractMetricValues(snap)

	for _, rule := range triggered {
		cur := values[rule.MetricName]
		if err := a.router.RouteAlert(ctx, rule, cur); err != nil {
			slog.Warn("ops: route alert failed", "rule", rule.MetricName, "error", err)
		}
	}

	slog.Info("ops: health check complete",
		"snapshot_id", snap.ID,
		"anomalies", len(snap.Anomalies),
		"alerts_fired", len(triggered),
	)
	return nil
}
