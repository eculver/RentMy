// Package integration provides shared test infrastructure for RentMy integration tests.
// Tests in this package run against real Postgres and Redis via testcontainers-go,
// and exercise the full HTTP API surface through a real httptest.Server backed by
// the same chi router used in production.
//
// Usage:
//
//	TestMain in this package starts containers once for the entire test binary,
//	so container startup cost is paid once regardless of how many test files exist.
//	Individual tests call NewTestServer to get a fresh HTTP test server that shares
//	the same underlying database as all other tests in the run.
//
//	Use CleanupDB between tests to truncate all tables and start with a clean slate.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/Brett2thered/RentMy/backend/app"
	"github.com/Brett2thered/RentMy/backend/internal/platform/config"
	"github.com/Brett2thered/RentMy/backend/internal/platform/postgres"
	localredis "github.com/Brett2thered/RentMy/backend/internal/platform/redis"
	riverpkg "github.com/Brett2thered/RentMy/backend/internal/platform/river"
	"github.com/Brett2thered/RentMy/backend/migrations"
)

// package-level vars set by TestMain and shared across all tests in the binary.
var (
	testPool     *pgxpool.Pool
	testRedis    *goredis.Client
	testDBURL    string
	testRedisURL string
	testCfg      config.Config
)

// TestMain starts shared Postgres and Redis testcontainers once, runs all tests,
// then terminates the containers. This function must be present in exactly one
// _test.go file in the package.
func TestMain(m *testing.M) {
	ctx := context.Background()
	code := 0

	defer func() {
		os.Exit(code)
	}()

	// Start Postgres container.
	pgContainer, err := tcpostgres.Run(ctx,
		"imresamu/postgis:16-3.5",
		tcpostgres.WithDatabase("rentmy_test"),
		tcpostgres.WithUsername("rentmy"),
		tcpostgres.WithPassword("rentmy"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start postgres container: %v\n", err)
		code = 1
		return
	}
	defer pgContainer.Terminate(ctx) //nolint:errcheck

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "get postgres connection string: %v\n", err)
		code = 1
		return
	}
	testDBURL = dsn

	// Run goose migrations against the test database.
	if err := postgres.RunMigrations(testDBURL, migrations.FS, "."); err != nil {
		fmt.Fprintf(os.Stderr, "run migrations: %v\n", err)
		code = 1
		return
	}

	// Connect pgxpool.
	pool, err := postgres.New(ctx, testDBURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect test postgres: %v\n", err)
		code = 1
		return
	}
	defer pool.Close()
	testPool = pool

	// Run River schema migrations so River can enqueue/dequeue jobs.
	if err := riverpkg.RunMigrations(ctx, pool); err != nil {
		fmt.Fprintf(os.Stderr, "run river migrations: %v\n", err)
		code = 1
		return
	}

	// Start Redis container.
	redisContainer, err := tcredis.Run(ctx, "redis:7-alpine")
	if err != nil {
		fmt.Fprintf(os.Stderr, "start redis container: %v\n", err)
		code = 1
		return
	}
	defer redisContainer.Terminate(ctx) //nolint:errcheck

	redisURL, err := redisContainer.ConnectionString(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "get redis connection string: %v\n", err)
		code = 1
		return
	}
	testRedisURL = redisURL

	redisClient, err := localredis.New(ctx, testRedisURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect test redis: %v\n", err)
		code = 1
		return
	}
	defer redisClient.Close()
	testRedis = redisClient

	// Build a base config using defaults. Sensitive external-service keys are
	// placeholders — callers that need real Stripe/Anthropic integration must
	// override via environment variables before running tests.
	testCfg = config.Config{
		Port:          8080,
		Env:           "test",
		DatabaseURL:   testDBURL,
		RedisURL:      testRedisURL,
		JWTSecret:     "integration-test-secret",
		JWTAccessTTL:  900,
		JWTRefreshTTL: 604800,
		// Payment — placeholder keys; Stripe calls will fail but are not exercised
		// by the smoke test. Tests that need Stripe should set STRIPE_SECRET_KEY.
		StripeSecretKey:             "sk_test_placeholder",
		StripePublishableKey:        "pk_test_placeholder",
		StripeWebhookSecret:         "whsec_placeholder",
		StripeIdentityWebhookSecret: "whsec_identity_placeholder",
		TakeRateBPS:                 2000,
		GuaranteeRateBPS:            1000,
		DamageReserveRate:           4000,
		PayoutDelayNewHostH:         48,
		// Discovery defaults.
		WeightAvailability:  0.35,
		WeightProximity:     0.30,
		WeightReputation:    0.20,
		WeightReliability:   0.15,
		DefaultFeedRadiusMeters: 30000,
		MaxFeedLimit:            50,
		MaxMapLimit:             200,
		OSRMBaseURL:             "http://localhost:5000",
		// Proximity.
		GPSThresholdMeters: 100,
		PINValidityMinutes: 30,
		// Booking.
		AutoDeclineTimeoutH:        2,
		FraudNewAccountDays:        30,
		FraudFirstNTransactions:    3,
		FraudDamageClaimCapCents:   50000,
		FraudDamageClaimWindowDays: 60,
		HostCancelLateBPS:          2500,
		HostCancelVeryLateBPS:      5000,
		// Notifications.
		PickupReminderMinutes: 30,
		ReturnReminderMinutes: 30,
		// Dispute.
		DisputeSLAActiveHours:     4,
		DisputeSLAPostReturnHours: 24,
	}

	code = m.Run()
}

// NewTestDB returns the shared pgxpool.Pool connected to the testcontainer database.
// Each call returns the same pool — the pool itself is thread-safe and shared.
func NewTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testPool == nil {
		t.Fatal("testPool is nil: TestMain did not run successfully")
	}
	return testPool
}

// NewTestRedis returns the shared Redis client connected to the testcontainer.
func NewTestRedis(t *testing.T) *goredis.Client {
	t.Helper()
	if testRedis == nil {
		t.Fatal("testRedis is nil: TestMain did not run successfully")
	}
	return testRedis
}

// NewTestServer builds a full app.Server backed by the shared testcontainer
// infrastructure and wraps it in an *httptest.Server. The server is shut down
// (including River) via t.Cleanup.
//
// S3 is intentionally nil — the health endpoint reports S3 as "connected" but
// skips the actual health check, keeping smoke tests free of MinIO dependencies.
// Tests that exercise media upload must provide their own S3 client.
func NewTestServer(t *testing.T) (*httptest.Server, *http.Client) {
	t.Helper()

	ctx := context.Background()

	srv, err := app.New(ctx, app.Deps{
		Pool:   testPool,
		Redis:  testRedis,
		S3:     nil, // S3 health check skipped when nil
		Pusher: nil, // real-time events disabled in tests
		Config: testCfg,
	})
	if err != nil {
		t.Fatalf("build test server: %v", err)
	}

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		ts.Close()
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Stop(stopCtx); err != nil {
			t.Logf("warning: river stop error: %v", err)
		}
	})

	client := ts.Client()
	return ts, client
}

// CleanupDB truncates all application tables in dependency order (CASCADE handles
// FK constraints). Call this at the start of each test that needs a clean slate.
func CleanupDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	_, err := pool.Exec(ctx, `
		TRUNCATE TABLE
			ops_alerts,
			ops_alert_rules,
			ops_health_snapshots,
			guarantee_fund_entries,
			disputes,
			agent_decisions,
			agreements,
			agreement_acceptances,
			appraisals,
			transactions,
			messages,
			notifications,
			proximity_proofs,
			push_tokens,
			ratings,
			reputation_signals,
			risk_scores,
			verification_attempts,
			media,
			listings,
			users
		RESTART IDENTITY CASCADE
	`)
	if err != nil {
		t.Fatalf("cleanup db: %v", err)
	}
}

// DoJSON sends a JSON-encoded request to the test server and returns the response.
// It sets Content-Type and, if token is non-empty, Authorization headers.
func DoJSON(t *testing.T, client *http.Client, method, url string, body interface{}, token string) *http.Response {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

// MustDecodeJSON decodes resp.Body into dst and closes the body.
func MustDecodeJSON(t *testing.T, resp *http.Response, dst interface{}) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
}

// DrainBody reads and discards the response body, closing it.
func DrainBody(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		io.Copy(io.Discard, resp.Body) //nolint:errcheck
		resp.Body.Close()
	}
}
