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

	"github.com/giits/rentmy/backend/internal/booking"
	"github.com/giits/rentmy/backend/internal/discovery"
	"github.com/giits/rentmy/backend/internal/listing"
	"github.com/giits/rentmy/backend/internal/media"
	"github.com/giits/rentmy/backend/internal/payment"
	"github.com/giits/rentmy/backend/internal/platform/auth"
	"github.com/giits/rentmy/backend/internal/platform/config"
	"github.com/giits/rentmy/backend/internal/platform/httpserver"
	"github.com/giits/rentmy/backend/internal/platform/postgres"
	"github.com/giits/rentmy/backend/internal/platform/pusher"
	localredis "github.com/giits/rentmy/backend/internal/platform/redis"
	riverpkg "github.com/giits/rentmy/backend/internal/platform/river"
	locals3 "github.com/giits/rentmy/backend/internal/platform/s3"
	"github.com/giits/rentmy/backend/internal/proximity"
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

	// Build Stripe adapter and payment repository early so the payout worker
	// can be registered before the River client starts.
	stripeAdapter := payment.NewStripeAdapter(cfg.StripeSecretKey)
	paymentRepo := payment.NewRepository(pool)

	// Register River workers.
	// bookingRepo is created early so the auto-decline worker can be registered before River starts.
	bookingRepo := booking.NewRepository(pool)

	workers := river.NewWorkers()
	river.AddWorker(workers, &riverpkg.TestJobWorker{})
	river.AddWorker(workers, payment.NewPayoutJobWorker(paymentRepo, stripeAdapter))
	river.AddWorker(workers, booking.NewAutoDeclineJobWorker(bookingRepo))

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

	// Build application services.
	userRepo := user.NewRepository(pool)
	userSvc := user.NewService(userRepo, issuer, redisStore)
	userHandler := user.NewHandler(userSvc)

	mediaRepo := media.NewRepository(pool)
	mediaSvc := media.NewService(mediaRepo, s3Client, cfg.S3Endpoint)
	mediaHandler := media.NewHandler(mediaSvc)

	listingRepo := listing.NewRepository(pool)
	listingSvc := listing.NewService(listingRepo)
	listingHandler := listing.NewHandler(listingSvc)

	driveTimeClient := discovery.NewDriveTimeClient(cfg.OSRMBaseURL, redisClient)
	discoveryRepo := discovery.NewRepository(pool)
	discoverySvc := discovery.NewService(discoveryRepo, driveTimeClient, discovery.Config{
		WeightAvailability:  cfg.WeightAvailability,
		WeightProximity:     cfg.WeightProximity,
		WeightReputation:    cfg.WeightReputation,
		WeightReliability:   cfg.WeightReliability,
		DefaultRadiusMeters: cfg.DefaultFeedRadiusMeters,
		MaxFeedLimit:        cfg.MaxFeedLimit,
		MaxMapLimit:         cfg.MaxMapLimit,
	})
	discoveryHandler := discovery.NewHandler(discoverySvc)

	// paymentRepo is created early (before River starts) to register the payout worker.
	// Reuse it here to build the payment service.
	paymentSvc := payment.NewService(paymentRepo, stripeAdapter, riverClient, payment.Config{
		TakeRateBPS:         cfg.TakeRateBPS,
		GuaranteeRateBPS:    cfg.GuaranteeRateBPS,
		DamageReserveRate:   cfg.DamageReserveRate,
		PayoutDelayNewHostH: cfg.PayoutDelayNewHostH,
	})
	paymentHandler := payment.NewHandler(paymentSvc)

	// Build Proximity service.
	// Twilio SMS client is nil when credentials are absent (safe for dev/test).
	var smsSender proximity.SMSSender
	if cfg.TwilioAccountSID != "" && cfg.TwilioAuthToken != "" && cfg.TwilioFromNumber != "" {
		smsSender = proximity.NewTwilioClient(cfg.TwilioAccountSID, cfg.TwilioAuthToken, cfg.TwilioFromNumber)
	}
	proximityRepo := proximity.NewRepository(pool)
	proximitySvc := proximity.NewService(proximityRepo, smsSender, proximity.Config{
		GPSThresholdMeters:  cfg.GPSThresholdMeters,
		PINValidityDuration: time.Duration(cfg.PINValidityMinutes) * time.Minute,
	})
	proximityHandler := proximity.NewHandler(proximitySvc)

	// bookingRepo is created early (before River starts) to register the auto-decline worker.
	// Reuse it here to build the booking service.
	bookingSvc := booking.NewService(bookingRepo, paymentSvc, riverClient, proximitySvc, booking.Config{
		AutoDeclineTimeout:         time.Duration(cfg.AutoDeclineTimeoutH) * time.Hour,
		FraudNewAccountDays:        cfg.FraudNewAccountDays,
		FraudFirstNTransactions:    cfg.FraudFirstNTransactions,
		FraudPayoutDelay:           time.Duration(cfg.PayoutDelayNewHostH) * time.Hour,
		FraudDamageClaimCapCents:   int64(cfg.FraudDamageClaimCapCents),
		FraudDamageClaimWindowDays: cfg.FraudDamageClaimWindowDays,
		HostCancelLateBPS:          cfg.HostCancelLateBPS,
		HostCancelVeryLateBPS:      cfg.HostCancelVeryLateBPS,
	})
	bookingHandler := booking.NewHandler(bookingSvc, paymentSvc)

	// Build a single /api/v1 router and mount all service routes onto it.
	apiV1 := userHandler.Router(authMW)
	mediaHandler.Mount(apiV1, authMW)
	listingHandler.Mount(apiV1, authMW)
	discoveryHandler.Mount(apiV1, authMW)
	paymentHandler.Mount(apiV1, authMW)
	bookingHandler.Mount(apiV1, authMW)
	proximityHandler.Mount(apiV1, authMW)
	r.Mount("/api/v1", apiV1)

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
