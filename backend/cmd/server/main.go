// Package main is the entry point for the RentMy API server.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"github.com/riverqueue/river"

	"github.com/giits/rentmy/backend/internal/platform/auth"
	"github.com/giits/rentmy/backend/internal/platform/config"
	"github.com/giits/rentmy/backend/internal/platform/httpserver"
	"github.com/giits/rentmy/backend/internal/platform/postgres"
	"github.com/giits/rentmy/backend/internal/platform/pusher"
	localredis "github.com/giits/rentmy/backend/internal/platform/redis"
	riverpkg "github.com/giits/rentmy/backend/internal/platform/river"
	locals3 "github.com/giits/rentmy/backend/internal/platform/s3"
	"github.com/giits/rentmy/backend/internal/user"
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

	// Run database migrations.
	slog.Info("running database migrations")
	if err := postgres.RunMigrations(cfg.DatabaseURL, migrations.FS, "."); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	// Connect to Postgres.
	ctx := context.Background()
	pool, err := postgres.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connecting to postgres: %w", err)
	}
	defer pool.Close()

	// Run River schema migrations (creates river_job, river_leader, etc.).
	slog.Info("running river migrations")
	if err := riverpkg.RunMigrations(ctx, pool); err != nil {
		return fmt.Errorf("running river migrations: %w", err)
	}

	// Register River workers.
	workers := river.NewWorkers()
	river.AddWorker(workers, &riverpkg.TestJobWorker{})

	// Start River job queue client.
	riverClient, err := riverpkg.New(ctx, pool, workers)
	if err != nil {
		return fmt.Errorf("starting river: %w", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		slog.Info("stopping river job queue")
		if err := riverClient.Stop(stopCtx); err != nil {
			slog.Error("river shutdown error", "error", err)
		}
		slog.Info("river job queue stopped")
	}()

	// Connect to Redis.
	redisClient, err := localredis.New(ctx, cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("connecting to redis: %w", err)
	}
	defer redisClient.Close()

	// Create S3 client.
	s3Client, err := locals3.New(ctx, locals3.Config{
		Endpoint:  cfg.S3Endpoint,
		AccessKey: cfg.S3AccessKey,
		SecretKey: cfg.S3SecretKey,
		Region:    cfg.S3Region,
	})
	if err != nil {
		return fmt.Errorf("creating s3 client: %w", err)
	}

	// Ensure required S3 buckets exist.
	for _, bucket := range []string{"media-originals", "media-thumbnails"} {
		if err := s3Client.EnsureBucket(ctx, bucket); err != nil {
			return fmt.Errorf("ensuring bucket %s: %w", bucket, err)
		}
	}

	// Create Pusher client.
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

	r := chi.NewRouter()

	// Global middleware.
	r.Use(httpserver.RequestID)
	r.Use(httpserver.Logger)
	r.Use(httpserver.Recoverer)

	// Health check.
	r.Get("/health", handleHealth(pool, redisClient, s3Client))

	// Build shared infrastructure for application services.
	issuer := auth.NewIssuer(
		cfg.JWTSecret,
		time.Duration(cfg.JWTAccessTTL)*time.Second,
		time.Duration(cfg.JWTRefreshTTL)*time.Second,
	)
	redisStore := localredis.NewStore(redisClient)
	authMW := auth.Middleware(issuer)

	// User service and routes.
	userRepo := user.NewRepository(pool)
	userSvc := user.NewService(userRepo, issuer, redisStore)
	userHandler := user.NewHandler(userSvc)
	r.Mount("/api/v1", userHandler.Router(authMW))

	// Debug route group.
	r.Route("/debug", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		r.Post("/trigger-event", handleTriggerEvent(pusherClient))
		r.Post("/enqueue-test", handleEnqueueTest(riverClient))
	})

	addr := fmt.Sprintf(":%d", cfg.Port)
	slog.Info("starting server", "port", cfg.Port, "env", cfg.Env)

	srv := httpserver.New(addr, r)
	return srv.Start(context.Background())
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

// healthResponse is the JSON structure returned by the health endpoint.
type healthResponse struct {
	Status   string `json:"status"`
	Postgres string `json:"postgres"`
	Redis    string `json:"redis"`
	S3       string `json:"s3"`
}

// handleHealth returns a handler that checks all service connectivity.
func handleHealth(pool *pgxpool.Pool, redisClient *goredis.Client, s3Client *locals3.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		resp := healthResponse{
			Status:   "ok",
			Postgres: "connected",
			Redis:    "connected",
			S3:       "connected",
		}

		degraded := false

		if err := postgres.HealthCheck(r.Context(), pool); err != nil {
			degraded = true
			resp.Postgres = fmt.Sprintf("error: %v", err)
		}

		if err := localredis.HealthCheck(r.Context(), redisClient); err != nil {
			degraded = true
			resp.Redis = fmt.Sprintf("error: %v", err)
		}

		if err := s3Client.HealthCheck(r.Context()); err != nil {
			degraded = true
			resp.S3 = fmt.Sprintf("error: %v", err)
		}

		if degraded {
			resp.Status = "degraded"
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		_ = json.NewEncoder(w).Encode(resp)
	}
}

// triggerEventRequest is the JSON body for the debug trigger-event endpoint.
type triggerEventRequest struct {
	Channel string      `json:"channel"`
	Event   string      `json:"event"`
	Data    interface{} `json:"data"`
}

// handleTriggerEvent returns a handler that triggers a Pusher event for debugging.
func handleTriggerEvent(pusherClient *pusher.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req triggerEventRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}

		if req.Channel == "" || req.Event == "" {
			http.Error(w, "channel and event are required", http.StatusBadRequest)
			return
		}

		if err := pusherClient.Trigger(req.Channel, req.Event, req.Data); err != nil {
			http.Error(w, fmt.Sprintf("trigger failed: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "sent"})
	}
}

// handleEnqueueTest returns a handler that enqueues a test job into River.
func handleEnqueueTest(riverClient *river.Client[pgx.Tx]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, err := riverClient.Insert(r.Context(), riverpkg.TestJobArgs{
			Message: "Hello from RentMy at " + time.Now().Format(time.RFC3339),
		}, nil)
		if err != nil {
			http.Error(w, fmt.Sprintf("enqueue failed: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"enqueued"}`))
	}
}
