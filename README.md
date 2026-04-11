# RentMy

A mobile-only, hyperlocal P2P rental marketplace. List anything, rent anything, from anyone nearby.

## Architecture

```
                        +-----------+
                        |  Mobile   |
                        | (Expo/RN) |
                        +-----+-----+
                              |
                              | REST + WebSocket
                              |
                        +-----v-----+
                        |  Backend  |
                        | (Go/chi)  |
                        +-----+-----+
                              |
            +---------+-------+-------+---------+
            |         |       |       |         |
        +---v---+ +---v---+ +v----+ +v------+ +v-------+
        |Postgres| | Redis | | S3  | |Pusher | | River  |
        |PostGIS | |       | |MinIO| |Soketi | |Job Queue|
        +--------+ +-------+ +-----+ +-------+ +--------+
```

**Backend:** Go modular monolith — each domain (users, listings, bookings, payments, agents) is a self-contained package with its own handlers, services, and repositories. No ORM — raw SQL via pgx.

**Mobile:** React Native with Expo Router (file-based routing), NativeWind for styling, Zustand + TanStack Query for state management.

**AI Agents:** Five AI agents (Verification, Appraisal, Risk, Agreement, Dispute) powered by Anthropic's Claude API, with a model router that selects the right tier (Haiku/Sonnet/Opus) per task.

**Infrastructure:** PostgreSQL + PostGIS for geo queries, Redis for caching + rate limiting, S3/MinIO for media, Pusher/Soketi for real-time WebSocket events, River for background job processing.

## Prerequisites

- **Go** 1.24+ (`brew install go`)
- **Node.js** 22+ (`brew install node`)
- **Docker** + Docker Compose (`brew install --cask docker`)
- **Expo CLI** (`npm install -g expo-cli` — or use `npx expo`)
- **iOS Simulator** (Xcode) or **Android Emulator** (Android Studio) for mobile testing

Optional:
- **Stripe CLI** for webhook testing (`brew install stripe/stripe-cli/stripe`)

## Getting Started

### 1. Clone and configure

```bash
git clone git@github.com:Brett2thered/RentMy.git
cd RentMy
cp .env.example .env
```

Edit `.env` if you need to change defaults. The defaults work with the Docker Compose services out of the box.

### 2. Start infrastructure

```bash
docker compose up -d
```

This starts:
| Service | Port | Purpose |
|---------|------|---------|
| PostgreSQL + PostGIS | 5432 | Primary database |
| Redis | 6380 | Cache, rate limiting, job queue metadata |
| MinIO | 9002 (API), 9003 (console) | S3-compatible media storage |
| Soketi | 6001 | WebSocket server (Pusher-compatible) |

Verify everything is healthy:
```bash
docker compose ps
```

### 3. Start the backend

```bash
cd backend
make dev
```

This runs database migrations automatically, then starts the Go server on port 8080.

Verify:
```bash
curl http://localhost:8080/health
# {"status":"ok","postgres":"ok","redis":"ok","s3":"ok"}
```

### 4. Start the mobile app

```bash
cd mobile
npm ci
npx expo start
```

Press `i` for iOS Simulator or `a` for Android Emulator. Or scan the QR code with Expo Go on a physical device.

### 5. Run tests

```bash
# Backend unit tests
cd backend && make test

# Backend integration tests (requires Docker for testcontainers)
cd backend && go test ./tests/integration/... -v -count=1 -timeout 180s

# Mobile tests
cd mobile && npx jest
```

## Project Structure

```
RentMy/
├── backend/                    # Go modular monolith API server
│   ├── cmd/server/             #   Server entrypoint and wiring
│   ├── internal/               #   Application code
│   │   ├── user/               #     User registration, auth, profiles
│   │   ├── listing/            #     Listing CRUD, 7-day ceiling
│   │   ├── media/              #     S3 upload, thumbnails
│   │   ├── discovery/          #     Feed, search, map (PostGIS)
│   │   ├── payment/            #     Stripe, holds, payouts, guarantee fund
│   │   ├── booking/            #     State machine, fraud velocity
│   │   ├── proximity/          #     GPS verification, PIN handoff
│   │   ├── notification/       #     Push notifications, preferences
│   │   ├── messaging/          #     Real-time chat (Pusher)
│   │   ├── agent/              #     AI agents (verification, appraisal, risk, agreement)
│   │   └── platform/           #     Shared infrastructure clients
│   │       ├── postgres/       #       pgx pool + health check
│   │       ├── redis/          #       go-redis client
│   │       ├── s3/             #       AWS SDK v2 client
│   │       ├── pusher/         #       Pusher trigger client
│   │       ├── river/          #       River job queue
│   │       ├── config/         #       Env-based config
│   │       ├── httpserver/     #       HTTP server + middleware
│   │       ├── auth/           #       JWT middleware
│   │       └── ulid/           #       ULID generator
│   ├── tests/integration/      #   Integration tests (testcontainers)
│   └── Makefile
│
├── mobile/                     # React Native (Expo) mobile app
│   ├── app/                    #   Screens (Expo Router file-based)
│   │   ├── (auth)/             #     Login, register
│   │   ├── (tabs)/             #     Tab navigation
│   │   │   ├── (feed)/         #       Feed, search, map, listings, bookings
│   │   │   ├── (create)/       #       Listing creation with camera
│   │   │   ├── (messages)/     #       Chat conversations
│   │   │   └── (profile)/      #       User profile, settings
│   │   └── _layout.tsx         #     Root layout
│   ├── components/             #   Reusable components
│   ├── lib/                    #   Utilities (API client, auth, hooks)
│   └── __tests__/              #   Jest + RNTL tests
│
├── migrations/                 # Database migrations (goose SQL)
├── terraform/                  # Infrastructure as Code
├── scripts/                    # Helper scripts
├── ops/                        # Ops dashboard (Vite + React, Phase 7)
├── docker-compose.yml          # Local development services
├── .env.example                # Environment variable template
└── .github/workflows/ci.yml   # CI pipeline
```

## Development

### Backend commands

```bash
cd backend
make dev          # Start server (auto-runs migrations)
make test         # Run all unit tests
make build        # Build binary to bin/server
make lint         # Run go vet
make docker-up    # Start Docker services
make docker-down  # Stop Docker services
```

### Mobile commands

```bash
cd mobile
npm ci              # Install dependencies
npx expo start      # Start dev server
npx tsc --noEmit    # TypeScript check
npx jest            # Run tests
npx jest --watch    # Run tests in watch mode
```

### Adding a new backend service

Each service follows the same structure:

```
backend/internal/{service}/
  handler.go      # HTTP handlers (return chi.Router)
  service.go      # Business logic
  repository.go   # SQL queries (pgx)
  model.go        # Domain types
  *_test.go       # Unit tests
```

1. Create the package in `backend/internal/`
2. Implement the handler → service → repository layers
3. Mount the router in `cmd/server/main.go`
4. Write unit tests + integration tests
5. Add a database migration in `migrations/` if needed

### Adding a new mobile screen

Screens use Expo Router's file-based routing:

```
mobile/app/(tabs)/(feed)/my-screen.tsx
```

1. Create the screen file in the appropriate route group
2. Use NativeWind `className` for styling
3. Fetch data with TanStack Query (`useQuery`)
4. Write a screen test in `mobile/__tests__/screens/`

## Testing

### Test hierarchy

| Layer | Tool | Location | What it tests |
|-------|------|----------|---------------|
| Backend unit | `go test` | `internal/*/..._test.go` | Business logic with mocks |
| Backend integration | testcontainers-go | `tests/integration/` | Full API surface, real DB |
| Mobile component | Jest + RNTL | `__tests__/` | UI rendering, interactions |
| Mobile E2E | (future) | — | Full app flows on simulator |

### Running tests

```bash
# Everything
cd backend && make test && go test ./tests/integration/... -v -timeout 180s
cd mobile && npx jest

# Backend unit tests only (fast, no Docker needed)
cd backend && go test ./internal/... -v -count=1

# Backend integration tests only (needs Docker)
cd backend && go test ./tests/integration/... -v -count=1 -timeout 180s

# Mobile tests with coverage
cd mobile && npx jest --coverage

# Single test file
cd backend && go test ./internal/booking/ -run TestStateMachine -v
cd mobile && npx jest __tests__/screens/auth.test.tsx
```

### Writing tests

**Backend integration tests** use testcontainers-go to spin up real Postgres + Redis. See `backend/tests/integration/helpers.go` for factory functions (`CreateTestUser`, `CreateTestListing`, etc.).

**Mobile component tests** use React Native Testing Library with MSW for API mocking. See `mobile/__tests__/setup.ts` for the global test configuration.

## Configuration

All configuration is via environment variables. Copy `.env.example` to `.env` for local development.

| Variable | Default | Purpose |
|----------|---------|---------|
| `PORT` | 8080 | Backend HTTP port |
| `DATABASE_URL` | `postgres://rentmy:rentmy@localhost:5433/rentmy` | PostgreSQL connection string |
| `REDIS_URL` | `redis://localhost:6380` | Redis connection string |
| `S3_ENDPOINT` | `http://localhost:9002` | S3/MinIO endpoint |
| `S3_ACCESS_KEY` | `minioadmin` | S3 access key |
| `S3_SECRET_KEY` | `minioadmin` | S3 secret key |
| `JWT_SECRET` | `dev-secret-change-in-production` | JWT signing secret |
| `STRIPE_SECRET_KEY` | — | Stripe API key (test mode) |
| `ANTHROPIC_API_KEY` | — | Claude API key (for AI agents) |

## CI/CD

GitHub Actions runs on every push to `main` and every pull request:

| Job | What it does |
|-----|-------------|
| **Backend Lint & Build** | `go vet`, `go build` |
| **Backend Unit Tests** | `go test ./internal/...` with Postgres + Redis services |
| **Backend Integration Tests** | `go test ./tests/integration/...` with testcontainers |
| **Mobile TypeScript** | `npx tsc --noEmit` |
| **Mobile Tests** | `npx jest --ci` |

See `.github/workflows/ci.yml` for the full pipeline.

## Roadmap

| Phase | Status | Description |
|-------|--------|-------------|
| 0 | Complete | Foundation (Go scaffold, Docker, Postgres, Redis, S3, River, RN scaffold, CI) |
| 1 | Complete | Users + Listings (UserService, MediaService, ListingService, auth/listing screens) |
| 2 | Complete | Discovery + Payments (search, maps, Stripe, feed/search/checkout screens) |
| 3 | Complete | Transactions (booking state machine, proximity, notifications, messaging) |
| 4 | Complete | AI Agents (model router, verification, appraisal, risk, agreement agents) |
| 5 | Next | Test Infrastructure (testcontainers, Jest+RNTL, retroactive coverage, CI) |
| 6 | Planned | Returns, Disputes, Trust (photo diff, dispute agent, reputation, guarantee fund) |
| 7 | Planned | Operations + Growth (ops dashboard, fraud agent, referrals) |

See `.claude/progress.json` for detailed task-level status.

## License

Proprietary. All rights reserved.
