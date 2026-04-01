# Task 0.3 + 0.5 + 0.6: Redis, S3, and Pusher Integration

**Status:** Complete
**Date:** 2026-03-31

## What Was Done

### Step 0.3 — Redis Integration
- Created `internal/platform/redis/redis.go` with `New()` and `HealthCheck()` functions
- Uses `github.com/redis/go-redis/v9` with URL-based configuration
- Wired into `cmd/server/main.go` with defer close
- Added to `/health` endpoint

### Step 0.5 — S3 Storage
- Created `internal/platform/s3/s3.go` with `Client` wrapper struct
- Uses `github.com/aws/aws-sdk-go-v2` with `BaseEndpoint` (modern approach, no deprecated resolver)
- Methods: `New`, `EnsureBucket`, `Upload`, `Download`, `Delete`, `HealthCheck`
- Configured for MinIO compatibility (`UsePathStyle: true`)
- Wired into `cmd/server/main.go` — ensures `media-originals` and `media-thumbnails` buckets on startup
- Added to `/health` endpoint
- Added config fields: `S3Endpoint`, `S3AccessKey`, `S3SecretKey`, `S3Region`

### Step 0.6 — Pusher Integration
- Created `internal/platform/pusher/pusher.go` with `Client` wrapper struct
- Uses `github.com/pusher/pusher-http-go/v5`
- Methods: `New`, `Trigger`, `TriggerBatch`
- Supports custom host for Soketi local dev (disables TLS when host is set)
- Wired into `cmd/server/main.go`
- Added `POST /debug/trigger-event` endpoint (accepts `channel`, `event`, `data` JSON body)
- Added config fields: `PusherAppID`, `PusherKey`, `PusherSecret`, `PusherHost`, `PusherCluster`

### Health Endpoint
Updated `/health` to check all three services:
```json
{"status":"ok","postgres":"connected","redis":"connected","s3":"connected"}
```
Returns 503 with `"status":"degraded"` if any service is unreachable.

## Files Created/Modified
- **Created:** `internal/platform/redis/redis.go`
- **Created:** `internal/platform/s3/s3.go`
- **Created:** `internal/platform/pusher/pusher.go`
- **Modified:** `internal/platform/config/config.go` — added S3 and Pusher config fields
- **Modified:** `cmd/server/main.go` — wired all three services, updated health, added debug endpoint
- **Modified:** `go.mod` / `go.sum` — new dependencies

## Dependencies Added
- `github.com/redis/go-redis/v9` v9.18.0
- `github.com/aws/aws-sdk-go-v2/config` v1.32.13
- `github.com/aws/aws-sdk-go-v2/service/s3` v1.98.0
- `github.com/aws/aws-sdk-go-v2/credentials` v1.19.13
- `github.com/pusher/pusher-http-go/v5` v5.1.1

## Verification
- `go vet ./...` — passes clean
- `go build ./cmd/server` — compiles successfully

## Environment Variables
| Variable | Default | Service |
|----------|---------|---------|
| `REDIS_URL` | `redis://localhost:6379` | Redis |
| `S3_ENDPOINT` | `http://localhost:9000` | S3/MinIO |
| `S3_ACCESS_KEY` | `minioadmin` | S3/MinIO |
| `S3_SECRET_KEY` | `minioadmin` | S3/MinIO |
| `S3_REGION` | `us-east-1` | S3/MinIO |
| `PUSHER_APP_ID` | `app-id` | Pusher/Soketi |
| `PUSHER_KEY` | `app-key` | Pusher/Soketi |
| `PUSHER_SECRET` | `app-secret` | Pusher/Soketi |
| `PUSHER_HOST` | `localhost:6001` | Pusher/Soketi |
| `PUSHER_CLUSTER` | `mt1` | Pusher/Soketi |

## Notes
- S3 client uses the modern `BaseEndpoint` option instead of the deprecated `EndpointResolverWithOptions`
- RedisURL was already in config from a previous step; this task added the actual client package
- The health endpoint checks Postgres, Redis, and S3 — Pusher has no health check (fire-and-forget)
- The debug trigger-event endpoint validates that both `channel` and `event` are provided
