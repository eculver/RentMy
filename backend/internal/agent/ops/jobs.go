package ops

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/riverqueue/river"
)

// HealthCheckArgs is the River job payload for the OpsAgent periodic health check.
type HealthCheckArgs struct{}

// Kind implements river.JobArgs.
func (HealthCheckArgs) Kind() string { return "ops_health_check" }

// HealthCheckWorker calls Agent.Run on every execution.
type HealthCheckWorker struct {
	river.WorkerDefaults[HealthCheckArgs]
	agent *Agent
}

// NewHealthCheckWorker creates the worker.
func NewHealthCheckWorker(agent *Agent) *HealthCheckWorker {
	return &HealthCheckWorker{agent: agent}
}

// Work runs the full OpsAgent health-check cycle.
func (w *HealthCheckWorker) Work(ctx context.Context, _ *river.Job[HealthCheckArgs]) error {
	slog.Info("ops: health check job executing")
	if err := w.agent.Run(ctx); err != nil {
		return fmt.Errorf("ops: health check job: %w", err)
	}
	return nil
}
