// Package config provides application configuration loaded from environment variables.
package config

import (
	"github.com/caarlos0/env/v11"
)

// Config holds all application configuration values parsed from the environment.
type Config struct {
	// Port is the HTTP server listen port.
	Port int `env:"PORT" envDefault:"8080"`

	// Env is the runtime environment name (development, staging, production).
	Env string `env:"ENV" envDefault:"development"`

	// DatabaseURL is the PostgreSQL connection string.
	DatabaseURL string `env:"DATABASE_URL" envDefault:"postgres://rentmy:rentmy@localhost:5432/rentmy?sslmode=disable"`

	// RedisURL is the Redis connection string.
	RedisURL string `env:"REDIS_URL" envDefault:"redis://localhost:6380"`

	// S3Endpoint is the S3-compatible storage endpoint (MinIO in dev).
	S3Endpoint string `env:"S3_ENDPOINT" envDefault:"http://localhost:9002"`

	// S3AccessKey is the access key for S3-compatible storage.
	S3AccessKey string `env:"S3_ACCESS_KEY" envDefault:"minioadmin"`

	// S3SecretKey is the secret key for S3-compatible storage.
	S3SecretKey string `env:"S3_SECRET_KEY" envDefault:"minioadmin"`

	// S3Region is the AWS region for S3-compatible storage.
	S3Region string `env:"S3_REGION" envDefault:"us-east-1"`

	// PusherAppID is the Pusher application ID.
	PusherAppID string `env:"PUSHER_APP_ID" envDefault:"app-id"`

	// PusherKey is the Pusher application key.
	PusherKey string `env:"PUSHER_KEY" envDefault:"app-key"`

	// PusherSecret is the Pusher application secret.
	PusherSecret string `env:"PUSHER_SECRET" envDefault:"app-secret"`

	// PusherHost is the Pusher-compatible WebSocket host (Soketi in dev).
	PusherHost string `env:"PUSHER_HOST" envDefault:"localhost:6001"`

	// PusherCluster is the Pusher cluster identifier.
	PusherCluster string `env:"PUSHER_CLUSTER" envDefault:"mt1"`

	// JWTSecret is the HMAC-SHA256 signing key for access and refresh tokens.
	JWTSecret string `env:"JWT_SECRET" envDefault:"dev-secret-change-in-production"`

	// JWTAccessTTL is the access token lifetime in seconds (default 15 minutes).
	JWTAccessTTL int `env:"JWT_ACCESS_TTL" envDefault:"900"`

	// JWTRefreshTTL is the refresh token lifetime in seconds (default 7 days).
	JWTRefreshTTL int `env:"JWT_REFRESH_TTL" envDefault:"604800"`

	// OSRMBaseURL is the base URL for the OSRM routing service.
	OSRMBaseURL string `env:"OSRM_BASE_URL" envDefault:"http://localhost:5000"`

	// Ranking weights for the discovery feed (PRD section 13 defaults).
	WeightAvailability float64 `env:"RANK_WEIGHT_AVAILABILITY" envDefault:"0.35"`
	WeightProximity    float64 `env:"RANK_WEIGHT_PROXIMITY" envDefault:"0.30"`
	WeightReputation   float64 `env:"RANK_WEIGHT_REPUTATION" envDefault:"0.20"`
	WeightReliability  float64 `env:"RANK_WEIGHT_RELIABILITY" envDefault:"0.15"`

	// Discovery defaults.
	DefaultFeedRadiusMeters int `env:"DEFAULT_FEED_RADIUS_METERS" envDefault:"30000"`
	MaxFeedLimit            int `env:"MAX_FEED_LIMIT" envDefault:"50"`
	MaxMapLimit             int `env:"MAX_MAP_LIMIT" envDefault:"200"`
}

// IsProd reports whether the application is running in a production environment.
func (c Config) IsProd() bool {
	return c.Env == "production"
}

// Load parses environment variables into a Config struct and returns it.
func Load() (Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return Config{}, err
	}
	return cfg, nil
}
