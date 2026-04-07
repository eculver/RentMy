// Package river provides a River durable job queue backed by Postgres via pgx v5.
package river

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

// New creates and starts a River client with the given workers and optional
// periodic jobs. The caller must call client.Stop(ctx) on shutdown to allow
// in-flight jobs to finish gracefully.
func New(ctx context.Context, pool *pgxpool.Pool, workers *river.Workers, periodicJobs ...*river.PeriodicJob) (*river.Client[pgx.Tx], error) {
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 10},
		},
		Workers:      workers,
		PeriodicJobs: periodicJobs,
		Logger:       slog.Default(),
	})
	if err != nil {
		return nil, fmt.Errorf("create river client: %w", err)
	}

	if err := client.Start(ctx); err != nil {
		return nil, fmt.Errorf("start river client: %w", err)
	}

	slog.Info("river job queue started", "queues", "default", "max_workers", 10)
	return client, nil
}

// RunMigrations runs River's internal schema migrations against the
// given Postgres pool. This must be called before starting the client.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("create river migrator: %w", err)
	}

	res, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return fmt.Errorf("run river migrations: %w", err)
	}

	slog.Info("river migrations complete", "versions_applied", len(res.Versions))
	return nil
}
