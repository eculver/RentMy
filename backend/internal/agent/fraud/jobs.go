package fraud

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/riverqueue/river"
)

// PatternScanArgs is the River job payload for the FraudAgent periodic pattern scan.
type PatternScanArgs struct{}

// Kind implements river.JobArgs.
func (PatternScanArgs) Kind() string { return "fraud_pattern_scan" }

// PatternScanWorker calls Agent.RunScheduledScan on every execution.
type PatternScanWorker struct {
	river.WorkerDefaults[PatternScanArgs]
	agent *Agent
}

// NewPatternScanWorker creates the worker.
func NewPatternScanWorker(agent *Agent) *PatternScanWorker {
	return &PatternScanWorker{agent: agent}
}

// Work runs the full cross-platform pattern scan.
func (w *PatternScanWorker) Work(ctx context.Context, _ *river.Job[PatternScanArgs]) error {
	slog.Info("fraud: pattern scan job executing")
	if err := w.agent.RunScheduledScan(ctx); err != nil {
		return fmt.Errorf("fraud: pattern scan job: %w", err)
	}
	return nil
}
