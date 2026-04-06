// Package app wires together all RentMy application services and builds the
// chi router. It is shared by cmd/server/main.go (production) and the
// integration test helpers so that both entry points use the same service
// graph and routing.
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	agentrouter "github.com/giits/rentmy/backend/internal/agent/router"
	"github.com/giits/rentmy/backend/internal/dispute"
	"github.com/giits/rentmy/backend/internal/latereturn"
	"github.com/giits/rentmy/backend/internal/photodiff"
	"github.com/giits/rentmy/backend/internal/platform/cv"
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
	localredis "github.com/giits/rentmy/backend/internal/platform/redis"
	riverpkg "github.com/giits/rentmy/backend/internal/platform/river"
	locals3 "github.com/giits/rentmy/backend/internal/platform/s3"
	"github.com/giits/rentmy/backend/internal/proximity"
	"github.com/giits/rentmy/backend/internal/rating"
	"github.com/giits/rentmy/backend/internal/reputation"
	"github.com/giits/rentmy/backend/internal/user"
)

// PusherClient is the interface used by booking and messaging services for
// real-time event delivery. A nil value disables real-time events.
type PusherClient interface {
	Trigger(channel, event string, data interface{}) error
}

// Deps holds all infrastructure clients needed to build the application.
type Deps struct {
	Pool   *pgxpool.Pool
	Redis  *goredis.Client
	S3     *locals3.Client // may be nil: S3 health check is skipped
	Pusher PusherClient   // may be nil: real-time events are disabled
	Config config.Config
}

// Server holds the built HTTP handler and manages the River job-queue lifecycle.
type Server struct {
	handler     http.Handler
	riverClient *river.Client[pgx.Tx]
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler { return s.handler }

// Stop gracefully shuts down the River worker pool.
func (s *Server) Stop(ctx context.Context) error {
	return s.riverClient.Stop(ctx)
}

// New builds all application services and mounts their routes onto a chi.Router.
// River is started here; call Stop on the returned Server before process exit.
func New(ctx context.Context, deps Deps) (*Server, error) {
	cfg := deps.Config
	pool := deps.Pool
	redisClient := deps.Redis
	s3Client := deps.S3

	// Optional AI model router — nil when AnthropicAPIKey is empty.
	var modelRouter *agentrouter.AnthropicRouter
	if cfg.AnthropicAPIKey != "" {
		var err error
		modelRouter, err = agentrouter.New(agentrouter.Config{
			APIKey:     cfg.AnthropicAPIKey,
			FullModel:  cfg.AnthropicFullModel,
			CheapModel: cfg.AnthropicCheapModel,
			PromptsDir: "prompts",
		})
		if err != nil {
			return nil, fmt.Errorf("creating model router: %w", err)
		}
	}

	decisionRepo := decision.NewRepository(pool)
	decisionSvc := decision.NewService(decisionRepo)

	stripeAdapter := payment.NewStripeAdapter(cfg.StripeSecretKey)
	paymentRepo := payment.NewRepository(pool)
	bookingRepo := booking.NewRepository(pool)
	notificationRepo := notification.NewRepository(pool)
	notificationPushClient := notification.NewPushClient(cfg.ExpoPushAccessToken)

	// Pre-river notification service — riverClient injected after River starts.
	notificationSvcPre := notification.NewService(notificationRepo, notificationPushClient, nil, notification.Config{
		PickupReminderBefore: time.Duration(cfg.PickupReminderMinutes) * time.Minute,
		ReturnReminderBefore: time.Duration(cfg.ReturnReminderMinutes) * time.Minute,
	})

	verificationRepo := verification.NewRepository(pool)
	verificationStripeAdapter := verification.NewStripeIdentityAdapter(
		cfg.StripeSecretKey, cfg.StripeIdentityWebhookSecret,
	)
	verificationSvcPre := verification.NewService(
		verificationRepo, verificationStripeAdapter, modelRouter, decisionSvc, nil, nil,
	)
	verificationTimeoutWorker := verification.NewVerificationTimeoutWorker(verificationRepo, verificationSvcPre)

	appraisalRepo := appraisal.NewRepository(pool)
	appraisalSvcPre := appraisal.NewService(appraisalRepo, nil, nil, modelRouter, decisionSvc, nil)
	appraisalWorker := appraisal.NewAppraisalJobWorker(appraisalSvcPre)

	riskRepo := risk.NewRepository(pool)
	riskSvc := risk.NewService(riskRepo, decisionSvc)
	monthlyReputationWorker := risk.NewMonthlyReputationWorker(riskSvc)
	decayCheckWorker := risk.NewDecayCheckWorker(riskSvc)

	agreementRepo := agreement.NewRepository(pool)
	agreementSvc := agreement.NewService(agreementRepo, modelRouter, decisionSvc)

	backfillRepo := backfill.NewRepository(pool)
	backfillProgress := backfill.NewJobProgress()
	backfillAppraisalWorker := backfill.NewBackfillAppraisalWorker(backfillRepo, nil, backfillProgress)
	backfillReputationWorker := backfill.NewBackfillReputationWorker(backfillRepo, riskSvc, backfillProgress)
	backfillRiskWorker := backfill.NewBackfillRiskScoreWorker(backfillRepo, riskSvc, backfillProgress)

	// Reputation service — pre-river (riverClient injected after River starts).
	reputationRepo := reputation.NewRepository(pool)
	reputationSvcPre := reputation.NewService(reputationRepo, nil)

	workers := river.NewWorkers()
	river.AddWorker(workers, &riverpkg.TestJobWorker{})
	river.AddWorker(workers, payment.NewPayoutJobWorker(paymentRepo, stripeAdapter))
	river.AddWorker(workers, booking.NewAutoDeclineJobWorker(bookingRepo, notificationSvcPre))
	river.AddWorker(workers, notification.NewPickupApproachingWorker(notificationSvcPre))
	river.AddWorker(workers, notification.NewReturnApproachingWorker(notificationSvcPre))
	river.AddWorker(workers, notification.NewQuietHoursDeferredWorker(notificationSvcPre))
	river.AddWorker(workers, verificationTimeoutWorker)
	river.AddWorker(workers, appraisalWorker)
	river.AddWorker(workers, monthlyReputationWorker)
	river.AddWorker(workers, decayCheckWorker)
	river.AddWorker(workers, backfillAppraisalWorker)
	river.AddWorker(workers, backfillReputationWorker)
	river.AddWorker(workers, backfillRiskWorker)
	river.AddWorker(workers, reputation.NewReputationRecalcWorker(reputationSvcPre))
	river.AddWorker(workers, reputation.NewMonthlyHostReputationWorker(reputationSvcPre))
	river.AddWorker(workers, reputation.NewNegativeDecayWorker(reputationSvcPre))

	// LateReturnAgent pre-river workers (riverClient injected after River starts).
	lateReturnRepo := latereturn.NewRepository(pool)
	lateReturnSvcPre := latereturn.NewService(lateReturnRepo, decisionSvc, nil, modelRouter, nil, latereturn.Config{
		EscalationThresholdHours: cfg.LateReturnEscalationThresholdH,
		DamageReserveRateBPS:     cfg.DamageReserveRate,
		ReCheckIntervalMinutes:   cfg.LateReturnReCheckMinutes,
	})
	river.AddWorker(workers, latereturn.NewLateReturnCheckWorker(lateReturnSvcPre))
	river.AddWorker(workers, latereturn.NewLateReturnEscalationWorker(lateReturnSvcPre))

	// DisputeAgent pre-river workers (riverClient injected after River starts).
	disputeRepo := dispute.NewRepository(pool)
	disputeSvcPre := dispute.NewService(disputeRepo, decisionSvc, nil, nil, modelRouter, nil, dispute.Config{
		SLAActiveHours:     cfg.DisputeSLAActiveHours,
		SLAPostReturnHours: cfg.DisputeSLAPostReturnHours,
	})
	river.AddWorker(workers, dispute.NewDisputeResolutionWorker(disputeSvcPre))
	river.AddWorker(workers, dispute.NewRePromptExpiryWorker(disputeSvcPre))
	river.AddWorker(workers, dispute.NewSLAMonitorWorker(disputeSvcPre))

	riverClient, err := riverpkg.New(ctx, pool, workers)
	if err != nil {
		return nil, fmt.Errorf("starting river: %w", err)
	}

	// Rebuild notification service with the real riverClient now available.
	notificationSvc := notification.NewService(notificationRepo, notificationPushClient, riverClient, notification.Config{
		PickupReminderBefore: time.Duration(cfg.PickupReminderMinutes) * time.Minute,
		ReturnReminderBefore: time.Duration(cfg.ReturnReminderMinutes) * time.Minute,
	})

	// Build application services.
	userRepo := user.NewRepository(pool)
	redisStore := localredis.NewStore(redisClient)
	issuer := auth.NewIssuer(
		cfg.JWTSecret,
		time.Duration(cfg.JWTAccessTTL)*time.Second,
		time.Duration(cfg.JWTRefreshTTL)*time.Second,
	)
	userSvc := user.NewService(userRepo, issuer, redisStore)
	userHandler := user.NewHandler(userSvc)

	mediaRepo := media.NewRepository(pool)
	mediaSvc := media.NewService(mediaRepo, s3Client, cfg.S3Endpoint)
	mediaHandler := media.NewHandler(mediaSvc)

	listingRepo := listing.NewRepository(pool)
	listingSvc := listing.NewService(listingRepo)
	listingHandler := listing.NewHandler(listingSvc)

	appraisalSvcFull := appraisal.NewService(appraisalRepo, listingSvc, mediaSvc, modelRouter, decisionSvc, riverClient)
	appraisalSvcPre.SetDeps(listingSvc, mediaSvc, riverClient)
	appraisalHandler := appraisal.NewHandler(appraisalSvcFull)

	listingSvc.WithAppraisalEnqueuer(appraisalSvcFull)
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

	paymentSvc := payment.NewService(paymentRepo, stripeAdapter, riverClient, payment.Config{
		TakeRateBPS:         cfg.TakeRateBPS,
		GuaranteeRateBPS:    cfg.GuaranteeRateBPS,
		DamageReserveRate:   cfg.DamageReserveRate,
		PayoutDelayNewHostH: cfg.PayoutDelayNewHostH,
	})
	paymentHandler := payment.NewHandler(paymentSvc)

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

	notificationHandler := notification.NewHandler(notificationSvc)

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
	bookingSvc.WithRiskAgent(riskSvc).WithAgreementAgent(agreementSvc)
	if deps.Pusher != nil {
		bookingSvc.WithPusher(deps.Pusher)
	}
	bookingHandler := booking.NewHandler(bookingSvc, paymentSvc)

	messagingRepo := messaging.NewRepository(pool)
	messagingSvc := messaging.NewService(messagingRepo, deps.Pusher, notificationSvc)
	messagingHandler := messaging.NewHandler(messagingSvc)

	verificationSvc := verification.NewService(
		verificationRepo, verificationStripeAdapter, modelRouter, decisionSvc, userSvc, riverClient,
	)
	verificationHandler := verification.NewHandler(verificationSvc)

	riskHandler := risk.NewHandler(riskSvc)
	agreementHandler := agreement.NewHandler(agreementSvc)
	backfillHandler := backfill.NewHandler(riverClient, backfillProgress)

	// PhotoDiff pipeline — optional, requires cv-service sidecar.
	cvClient := cv.New(cfg.CVServiceURL)
	photodiffRepo := photodiff.NewRepository(pool)
	photodiffSvc := photodiff.NewService(photodiffRepo, mediaRepo, cvClient, modelRouter, s3Client)
	photodiffHandler := photodiff.NewHandler(photodiffSvc)

	// Reputation service — full, with riverClient for async enqueue.
	reputationSvc := reputation.NewService(reputationRepo, riverClient)
	*reputationSvcPre = *reputationSvc

	// DisputeAgent — full service with all dependencies.
	disputeHoldSvc := dispute.NewHoldService(paymentSvc)
	disputeSvc := dispute.NewService(disputeRepo, decisionSvc, disputeHoldSvc, paymentSvc, modelRouter, riverClient, dispute.Config{
		SLAActiveHours:     cfg.DisputeSLAActiveHours,
		SLAPostReturnHours: cfg.DisputeSLAPostReturnHours,
	})
	disputeSvc.WithReputation(reputationSvc)
	// Inject real dependencies into the pre-river workers.
	*disputeSvcPre = *disputeSvc
	disputeHandler := dispute.NewHandler(disputeSvc)

	// LateReturnAgent — full service with all dependencies.
	lateReturnSvc := latereturn.NewService(lateReturnRepo, decisionSvc, paymentSvc, modelRouter, riverClient, latereturn.Config{
		EscalationThresholdHours: cfg.LateReturnEscalationThresholdH,
		DamageReserveRateBPS:     cfg.DamageReserveRate,
		ReCheckIntervalMinutes:   cfg.LateReturnReCheckMinutes,
	})
	*lateReturnSvcPre = *lateReturnSvc
	lateReturnHandler := latereturn.NewHandler(lateReturnSvc)

	// Rating system.
	ratingRepo := rating.NewRepository(pool)
	ratingSvc := rating.NewService(ratingRepo, riskSvc).WithReputation(reputationSvc)
	ratingHandler := rating.NewHandler(ratingSvc)

	// Build chi router.
	r := chi.NewRouter()
	r.Use(httpserver.RequestID)
	r.Use(httpserver.Logger)
	r.Use(httpserver.Recoverer)

	r.Get("/health", handleHealth(pool, redisClient, s3Client))

	authMW := auth.Middleware(issuer)
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
	photodiffHandler.Mount(apiV1, authMW)
	disputeHandler.Mount(apiV1, authMW)
	lateReturnHandler.Mount(apiV1, authMW)
	ratingHandler.Mount(apiV1, authMW)
	r.Mount("/api/v1", apiV1)

	// Debug routes (non-production utilities).
	r.Route("/debug", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		if deps.Pusher != nil {
			r.Post("/trigger-event", handleTriggerEvent(deps.Pusher))
		}
		r.Post("/enqueue-test", handleEnqueueTest(riverClient))
	})

	return &Server{handler: r, riverClient: riverClient}, nil
}

// healthResponse is the JSON structure returned by the health endpoint.
type healthResponse struct {
	Status   string `json:"status"`
	Postgres string `json:"postgres"`
	Redis    string `json:"redis"`
	S3       string `json:"s3"`
}

// handleHealth returns a handler that checks Postgres, Redis, and S3 connectivity.
// S3 is skipped when s3Client is nil.
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

		if s3Client != nil {
			if err := s3Client.HealthCheck(r.Context()); err != nil {
				degraded = true
				resp.S3 = fmt.Sprintf("error: %v", err)
			}
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

// handleTriggerEvent returns a handler that fires a Pusher event for debugging.
func handleTriggerEvent(p PusherClient) http.HandlerFunc {
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
		if err := p.Trigger(req.Channel, req.Event, req.Data); err != nil {
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
