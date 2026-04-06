package guaranteefund

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/riverqueue/river"
)

// FundHealthCheckJobArgs is the payload for the hourly fund health check.
type FundHealthCheckJobArgs struct{}

// Kind implements river.JobArgs.
func (FundHealthCheckJobArgs) Kind() string { return "guarantee_fund_health_check" }

// FundHealthCheckWorker runs the hourly reserve ratio check and fires alerts.
type FundHealthCheckWorker struct {
	river.WorkerDefaults[FundHealthCheckJobArgs]
	svc *Service
}

// NewFundHealthCheckWorker creates the worker.
func NewFundHealthCheckWorker(svc *Service) *FundHealthCheckWorker {
	return &FundHealthCheckWorker{svc: svc}
}

// Work checks the fund health and logs alerts at degraded thresholds.
func (w *FundHealthCheckWorker) Work(ctx context.Context, _ *river.Job[FundHealthCheckJobArgs]) error {
	slog.Info("guaranteefund: health check job started")

	health, err := w.svc.GetFundHealth(ctx)
	if err != nil {
		return fmt.Errorf("guaranteefund: health check: %w", err)
	}

	switch health.Action {
	case ReserveActionAlert:
		slog.Warn("guaranteefund: reserve ratio LOW — recommend increasing guaranteeRate by 5%",
			"balance", health.Balance,
			"outstandingGaps", health.OutstandingGaps,
			"reserveRatio", health.ReserveRatio,
		)
	case ReserveActionRestrictHigh:
		slog.Error("guaranteefund: reserve ratio CRITICAL — restricting high-value listings (>$2000)",
			"balance", health.Balance,
			"outstandingGaps", health.OutstandingGaps,
			"reserveRatio", health.ReserveRatio,
		)
	case ReserveActionRestrictAllGap:
		slog.Error("guaranteefund: reserve ratio EMERGENCY — restricting all gap bookings (>$500)",
			"balance", health.Balance,
			"outstandingGaps", health.OutstandingGaps,
			"reserveRatio", health.ReserveRatio,
		)
	default:
		slog.Info("guaranteefund: health check OK",
			"balance", health.Balance,
			"reserveRatio", health.ReserveRatio,
			"lossRatio", health.LossRatio,
		)
	}

	return nil
}

// LossRatioCheckJobArgs is the payload for the daily loss ratio check.
type LossRatioCheckJobArgs struct{}

// Kind implements river.JobArgs.
func (LossRatioCheckJobArgs) Kind() string { return "guarantee_fund_loss_ratio_check" }

// LossRatioCheckWorker runs the daily loss ratio analysis.
type LossRatioCheckWorker struct {
	river.WorkerDefaults[LossRatioCheckJobArgs]
	svc *Service
}

// NewLossRatioCheckWorker creates the worker.
func NewLossRatioCheckWorker(svc *Service) *LossRatioCheckWorker {
	return &LossRatioCheckWorker{svc: svc}
}

// Work checks the rolling 90-day loss ratio and logs alerts when above target.
func (w *LossRatioCheckWorker) Work(ctx context.Context, _ *river.Job[LossRatioCheckJobArgs]) error {
	slog.Info("guaranteefund: loss ratio check job started")

	health, err := w.svc.GetFundHealth(ctx)
	if err != nil {
		return fmt.Errorf("guaranteefund: loss ratio check: %w", err)
	}

	if health.LossRatio > w.svc.cfg.LossRatioTarget {
		slog.Warn("guaranteefund: loss ratio exceeds target — recommend increasing guaranteeRate, raising hold tiers, or tightening high-value listing requirements",
			"lossRatio", health.LossRatio,
			"target", w.svc.cfg.LossRatioTarget,
			"balance", health.Balance,
		)
	} else {
		slog.Info("guaranteefund: loss ratio OK",
			"lossRatio", health.LossRatio,
			"target", w.svc.cfg.LossRatioTarget,
		)
	}

	return nil
}
