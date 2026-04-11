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

	"github.com/giits/rentmy/backend/internal/agent/agreement"
	"github.com/giits/rentmy/backend/internal/agent/appraisal"
	"github.com/giits/rentmy/backend/internal/agent/backfill"
	"github.com/giits/rentmy/backend/internal/agent/decision"
	"github.com/giits/rentmy/backend/internal/agent/risk"
	"github.com/giits/rentmy/backend/internal/agent/router"
	"github.com/giits/rentmy/backend/internal/agent/verification"
	"github.com/giits/rentmy/backend/internal/booking"
	"github.com/giits/rentmy/backend/internal/discovery"
	"github.com/giits/rentmy/backend/internal/listing"
	"github.com/giits/rentmy/backend/internal/media"
	"github.com/giits/rentmy/backend/internal/messaging"
	"github.com/giits/rentmy/backend/internal/notification"
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

	// Initialize Anthropic model router and decision service.
	// When AnthropicAPIKey is empty (dev/test without AI), the router is nil.
	// Agents must check for nil before calling Route.
	var modelRouter *router.AnthropicRouter
	if cfg.AnthropicAPIKey != "" {
		modelRouter, err = router.New(router.Config{
			APIKey:     cfg.AnthropicAPIKey,
			FullModel:  cfg.AnthropicFullModel,
			CheapModel: cfg.AnthropicCheapModel,
			PromptsDir: "prompts",
		})
		if err != nil {
			return fmt.Errorf("creating model router: %w", err)
		}
		slog.Info("model router initialized", "full_model", cfg.AnthropicFullModel, "cheap_model", cfg.AnthropicCheapModel)
	} else {
		slog.Warn("ANTHROPIC_API_KEY not set — AI agents disabled")
	}
	decisionRepo := decision.NewRepository(pool)
	decisionSvc := decision.NewService(decisionRepo)

	// Build Stripe adapter and payment repository early so the payout worker
	// can be registered before the River client starts.
	stripeAdapter := payment.NewStripeAdapter(cfg.StripeSecretKey)
	paymentRepo := payment.NewRepository(pool)

	// Register River workers.
	// bookingRepo and notificationRepo are created early so workers can be registered before River starts.
	bookingRepo := booking.NewRepository(pool)
	notificationRepo := notification.NewRepository(pool)

	// Build notification push client (nil access token is safe for dev/test).
	notificationPushClient := notification.NewPushClient(cfg.ExpoPushAccessToken)
	// riverClient is not yet available at worker-registration time; notificationSvc is wired after River starts.
	// We use a pointer-to-pointer trick: register a placeholder worker, then swap the service in below.
	// Instead, we build notificationSvc before River starts (without riverClient), then re-create with riverClient after.
	// Simpler: build a pre-river notificationSvc with nil riverClient; scheduled jobs work after River starts.
	notificationSvc := notification.NewService(notificationRepo, notificationPushClient, nil, notification.Config{
		PickupReminderBefore: time.Duration(cfg.PickupReminderMinutes) * time.Minute,
		ReturnReminderBefore: time.Duration(cfg.ReturnReminderMinutes) * time.Minute,
	})

	// Build verification repo and a pre-river service (nil riverClient) so the
	// timeout worker can be registered before River starts.
	verificationRepo := verification.NewRepository(pool)
	verificationStripeAdapter := verification.NewStripeIdentityAdapter(
		cfg.StripeSecretKey, cfg.StripeIdentityWebhookSecret,
	)
	// nil riverClient and userSvc are filled in after River/Redis start.
	verificationSvcPre := verification.NewService(
		verificationRepo, verificationStripeAdapter, modelRouter, decisionSvc, nil, nil,
	)
	verificationTimeoutWorker := verification.NewVerificationTimeoutWorker(verificationRepo, verificationSvcPre)

	// Build a pre-river AppraisalService for worker registration.
	// The real listingSvc and mediaSvc are not yet available; they are injected
	// after all services are constructed below.
	appraisalRepo := appraisal.NewRepository(pool)
	// Placeholder services are replaced after listing/media services are built.
	// The worker only calls Appraise(), which needs listingSvc and mediaSvc —
	// we use a two-phase init: register worker with a service pointer, then
	// swap deps via setters below.
	appraisalSvcPre := appraisal.NewService(
		appraisalRepo, nil, nil, modelRouter, decisionSvc, nil,
	)
	appraisalWorker := appraisal.NewAppraisalJobWorker(appraisalSvcPre)

	// Build RiskAgent service and River workers.
	riskRepo := risk.NewRepository(pool)
	riskSvc := risk.NewService(riskRepo, decisionSvc)
	monthlyReputationWorker := risk.NewMonthlyReputationWorker(riskSvc)
	decayCheckWorker := risk.NewDecayCheckWorker(riskSvc)

	// Build AgreementAgent service (no River workers needed — generation is synchronous).
	agreementRepo := agreement.NewRepository(pool)
	agreementSvc := agreement.NewService(agreementRepo, modelRouter, decisionSvc)

	// Build backfill repository and shared progress tracker.
	// Workers are registered before River starts; appraisalSvcFull is injected after services are built.
	backfillRepo := backfill.NewRepository(pool)
	backfillProgress := backfill.NewJobProgress()
	// Workers are constructed with nil appraisalSvc/riskSvc here to enable pre-River registration.
	// After services are built, SetDeps is called (see below).
	backfillAppraisalWorker := backfill.NewBackfillAppraisalWorker(backfillRepo, nil, backfillProgress)
	backfillReputationWorker := backfill.NewBackfillReputationWorker(backfillRepo, riskSvc, backfillProgress)
	backfillRiskWorker := backfill.NewBackfillRiskScoreWorker(backfillRepo, riskSvc, backfillProgress)

	workers := river.NewWorkers()
	river.AddWorker(workers, &riverpkg.TestJobWorker{})
	river.AddWorker(workers, payment.NewPayoutJobWorker(paymentRepo, stripeAdapter))
	river.AddWorker(workers, booking.NewAutoDeclineJobWorker(bookingRepo, notificationSvc))
	river.AddWorker(workers, notification.NewPickupApproachingWorker(notificationSvc))
	river.AddWorker(workers, notification.NewReturnApproachingWorker(notificationSvc))
	river.AddWorker(workers, notification.NewQuietHoursDeferredWorker(notificationSvc))
	river.AddWorker(workers, verificationTimeoutWorker)
	river.AddWorker(workers, appraisalWorker)
	river.AddWorker(workers, monthlyReputationWorker)
	river.AddWorker(workers, decayCheckWorker)
	river.AddWorker(workers, backfillAppraisalWorker)
	river.AddWorker(workers, backfillReputationWorker)
	river.AddWorker(workers, backfillRiskWorker)

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

	// Build the full AppraisalService now that listingSvc and mediaSvc are available.
	// appraisalSvcPre was registered as a River worker above with nil deps;
	// we build a fresh full service here (worker will use appraisalSvcPre which has
	// nil listingSvc/mediaSvc — jobs enqueued after startup are handled by new workers
	// that use the full service injected into the worker).
	// To keep it simple, build one authoritative service and replace the pre-river one's
	// internal deps isn't possible since NewService is a constructor. Instead, we
	// re-create the service with all deps and re-register the worker below.
	// Note: re-registering the worker is not possible after river.NewWorkers() returns.
	// Solution: the pre-river service receives SetDeps after listing/media are available.
	appraisalSvcFull := appraisal.NewService(
		appraisalRepo, listingSvc, mediaSvc, modelRouter, decisionSvc, riverClient,
	)
	appraisalSvcPre.SetDeps(listingSvc, mediaSvc, riverClient)
	appraisalHandler := appraisal.NewHandler(appraisalSvcFull)

	// Wire the appraisal enqueuer into the listing service so that new listings
	// automatically trigger AI appraisal.
	listingSvc.WithAppraisalEnqueuer(appraisalSvcFull)

	// Inject the full appraisal service into the backfill worker now that it is available.
	backfillAppraisalWorker.SetAppraisalSvc(appraisalSvcFull)

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

	// Re-create notificationSvc now that riverClient is available, so scheduled jobs can be enqueued.
	notificationSvc = notification.NewService(notificationRepo, notificationPushClient, riverClient, notification.Config{
		PickupReminderBefore: time.Duration(cfg.PickupReminderMinutes) * time.Minute,
		ReturnReminderBefore: time.Duration(cfg.ReturnReminderMinutes) * time.Minute,
	})
	notificationHandler := notification.NewHandler(notificationSvc)

	// bookingRepo is created early (before River starts) to register the auto-decline worker.
	// Reuse it here to build the booking service.
	bookingSvc := booking.NewService(bookingRepo, paymentSvc, riverClient, proximitySvc, notificationSvc, booking.Config{
		AutoDeclineTimeout:         time.Duration(cfg.AutoDeclineTimeoutH) * time.Hour,
		FraudNewAccountDays:        cfg.FraudNewAccountDays,
		FraudFirstNTransactions:    cfg.FraudFirstNTransactions,
		FraudPayoutDelay:           time.Duration(cfg.PayoutDelayNewHostH) * time.Hour,
		FraudDamageClaimCapCents:   int64(cfg.FraudDamageClaimCapCents),
		FraudDamageClaimWindowDays: cfg.FraudDamageClaimWindowDays,
		HostCancelLateBPS:          cfg.HostCancelLateBPS,
		HostCancelVeryLateBPS:      cfg.HostCancelVeryLateBPS,
		PickupReminderBefore:       time.Duration(cfg.PickupReminderMinutes) * time.Minute,
		ReturnReminderBefore:       time.Duration(cfg.ReturnReminderMinutes) * time.Minute,
	})
	bookingSvc.WithPusher(pusherClient).WithRiskAgent(riskSvc).WithAgreementAgent(agreementSvc)
	bookingHandler := booking.NewHandler(bookingSvc, paymentSvc)

	// Build MessagingService.
	messagingRepo := messaging.NewRepository(pool)
	messagingSvc := messaging.NewService(messagingRepo, pusherClient, notificationSvc)
	messagingHandler := messaging.NewHandler(messagingSvc)

	// Build the VerificationAgent service (full version with riverClient and userSvc).
	verificationSvc := verification.NewService(
		verificationRepo,
		verificationStripeAdapter,
		modelRouter,
		decisionSvc,
		userSvc,
		riverClient,
	)
	verificationHandler := verification.NewHandler(verificationSvc)

	// Build the RiskAgent HTTP handler.
	riskHandler := risk.NewHandler(riskSvc)

	// Build the AgreementAgent HTTP handler.
	agreementHandler := agreement.NewHandler(agreementSvc)

	// Build the backfill admin handler.
	backfillHandler := backfill.NewHandler(riverClient, backfillProgress)

	// Build a single /api/v1 router and mount all service routes onto it.
	apiV1 := userHandler.Router(authMW)
	mediaHandler.Mount(apiV1, authMW)
	listingHandler.Mount(apiV1, authMW)
	discoveryHandler.Mount(apiV1, authMW)
	paymentHandler.Mount(apiV1, authMW)
	bookingHandler.Mount(apiV1, authMW)
	proximityHandler.Mount(apiV1, authMW)
	notificationHandler.Mount(apiV1, authMW)
	messagingHandler.Mount(apiV1, authMW)
	verificationHandler.Mount(apiV1, authMW)
	appraisalHandler.Mount(apiV1, authMW)
	riskHandler.Mount(apiV1, authMW)
	agreementHandler.Mount(apiV1, authMW)
	backfillHandler.Mount(apiV1, authMW)
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
