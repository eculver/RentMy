// Package main is the entry point for the RentMy API server.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/giits/rentmy/backend/app"
	"github.com/giits/rentmy/backend/internal/platform/config"
	"github.com/giits/rentmy/backend/internal/platform/httpserver"
	"github.com/giits/rentmy/backend/internal/platform/postgres"
	"github.com/giits/rentmy/backend/internal/platform/pusher"
	localredis "github.com/giits/rentmy/backend/internal/platform/redis"
	riverpkg "github.com/giits/rentmy/backend/internal/platform/river"
	locals3 "github.com/giits/rentmy/backend/internal/platform/s3"
	"github.com/giits/rentmy/backend/migrations"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	setupLogger(cfg)

	slog.Info("running database migrations")
	if err := postgres.RunMigrations(cfg.DatabaseURL, migrations.FS, "."); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	ctx := context.Background()
	pool, err := postgres.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connecting to postgres: %w", err)
	}
	defer pool.Close()

	slog.Info("running river migrations")
	if err := riverpkg.RunMigrations(ctx, pool); err != nil {
		return fmt.Errorf("running river migrations: %w", err)
	}

	redisClient, err := localredis.New(ctx, cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("connecting to redis: %w", err)
	}
	defer redisClient.Close()

	s3Client, err := locals3.New(ctx, locals3.Config{
		Endpoint:  cfg.S3Endpoint,
		AccessKey: cfg.S3AccessKey,
		SecretKey: cfg.S3SecretKey,
		Region:    cfg.S3Region,
	})
	if err != nil {
		return fmt.Errorf("creating s3 client: %w", err)
	}

	for _, bucket := range []string{"media-originals", "media-thumbnails"} {
		if err := s3Client.EnsureBucket(ctx, bucket); err != nil {
			return fmt.Errorf("ensuring bucket %s: %w", bucket, err)
		}
	}

	pusherClient, err := pusher.New(pusher.Config{
		AppID:   cfg.PusherAppID,
		Key:     cfg.PusherKey,
		Secret:  cfg.PusherSecret,
		Host:    cfg.PusherHost,
		Cluster: cfg.PusherCluster,
	})
	if err != nil {
		return fmt.Errorf("creating pusher client: %w", err)
	}

	srv, err := app.New(ctx, app.Deps{
		Pool:   pool,
		Redis:  redisClient,
		S3:     s3Client,
		Pusher: pusherClient,
		Config: cfg,
	})
	if err != nil {
		return fmt.Errorf("building application: %w", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		slog.Info("stopping river job queue")
		if err := srv.Stop(stopCtx); err != nil {
			slog.Error("river shutdown error", "error", err)
		}
		slog.Info("river job queue stopped")
	}()

	addr := fmt.Sprintf(":%d", cfg.Port)
	slog.Info("starting server", "port", cfg.Port, "env", cfg.Env)
	return httpserver.New(addr, srv.Handler()).Start(ctx)
}

// setupLogger configures slog with a JSON handler for production and a text
// handler for all other environments.
func setupLogger(cfg config.Config) {
	var handler slog.Handler
	if cfg.IsProd() {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	}
	slog.SetDefault(slog.New(handler))
}
