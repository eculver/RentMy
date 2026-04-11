# Task 1.1 — UserService (backend)

## Status: COMPLETE

**Commit:** f19fae3  
**Branch:** task-1.1-user-service  
**Branching mode:** git fallback (Graphite unavailable)

---

## What Was Done

### New files
- `backend/internal/platform/auth/jwt.go` — `Issuer` type: `Issue(userID)` returns `TokenPair{AccessToken, RefreshToken}`. Uses HMAC-SHA256. Adds ULID `jti` claim so tokens issued within the same second are distinct.
- `backend/internal/platform/auth/middleware.go` — `Middleware(issuer)` HTTP middleware: extracts Bearer token, validates, injects `userID` into context via `UserIDKey`.
- `backend/internal/platform/redis/store.go` — `Store` wraps `*redis.Client` with `Set/Get/Del` methods to satisfy the `RedisStore` interface.
- `backend/internal/user/model.go` — `User`, `RegisterInput`, `LoginInput`, `RefreshInput`, `UpdateInput`, `AuthResponse`.
- `backend/internal/user/repository.go` — `Repository` with `Insert`, `FindByID`, `FindByEmail`, `UpdateLastActive`, `Update`. Raw SQL via pgx. Handles unique-constraint violations → `ErrEmailTaken`.
- `backend/internal/user/service.go` — `Service` with `Register`, `Login`, `Refresh`, `GetProfile`, `UpdateProfile`. `RepositoryInterface` allows fake in tests. `NewServiceWithInterfaces` for test injection.
- `backend/internal/user/handler.go` — `Handler.Router(authMW)` mounts:
  - `POST /auth/register`
  - `POST /auth/login`
  - `POST /auth/refresh`
  - `GET /users/me` (protected)
  - `PUT /users/me` (protected)
- `backend/internal/user/service_test.go` — 6 unit tests covering validation, bad credentials, register/login round trip, refresh token rotation, duplicate email.

### Modified files
- `backend/internal/platform/config/config.go` — Added `JWTSecret`, `JWTAccessTTL`, `JWTRefreshTTL`.
- `backend/cmd/server/main.go` — Wires `auth.Issuer`, `redis.Store`, `user.Repository`, `user.Service`, `user.Handler`. Mounts user router at `/api/v1`.
- `backend/go.mod` / `backend/go.sum` — Added `github.com/golang-jwt/jwt/v5`, `github.com/go-playground/validator/v10` as direct dependencies.

---

## API Endpoints

| Method | Path | Auth | Status |
|--------|------|------|--------|
| POST | `/api/v1/auth/register` | No | 201 with `{user, accessToken, refreshToken}` |
| POST | `/api/v1/auth/login` | No | 200 with `{user, accessToken, refreshToken}` |
| POST | `/api/v1/auth/refresh` | No | 200 with `{user, accessToken, refreshToken}` |
| GET | `/api/v1/users/me` | Bearer | 200 with `{user}` |
| PUT | `/api/v1/users/me` | Bearer | 200 with `{user}` |

---

## Verification Results

- `go vet ./...` — PASS
- `go build ./cmd/server` — PASS
- `go test ./... -v -count=1` — PASS (6/6 tests)
- `curl -sf http://localhost:8080/health` — SKIPPED (Docker daemon not running)

---

## Key Decisions

- **bcrypt cost 12**: ~250ms on commodity hardware; acceptable for auth endpoints.
- **ULID jti in JWT**: `NumericDate` has second precision; without `jti`, two tokens issued in the same second are identical, which breaks refresh-token rotation in tests.
- **Refresh token rotation**: old token deleted from Redis before new one is stored.
- **`RepositoryInterface`**: Service depends on interface, not concrete `*Repository`, enabling unit tests without Postgres.

---

## Notes for Next Task (1.2 — MediaService)

- The JWT auth middleware is ready. All upload endpoints should apply `authMW`.
- `auth.UserIDFromContext(ctx)` retrieves the authenticated user ID in any handler.
- `ulid.New()` from `internal/platform/ulid` is the shared ID generator — use it for media IDs.
- The `users` table is populated and constraints are live; media FKs to `listings` will need listing records (Phase 1.3).
