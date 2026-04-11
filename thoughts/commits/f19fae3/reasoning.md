# Commit f19fae3 — UserService with JWT auth

## Why this commit

Task 1.1 requires a user registration/login system before any other Phase 1 work can proceed. All subsequent backend tasks (MediaService, ListingService) and mobile tasks (Auth screens) depend on working auth endpoints and a JWT middleware.

## Key decisions

- **bcrypt cost 12**: ~250ms on commodity hardware. Chosen over argon2 for simplicity (no tuning parameters to misconfigure at v1 scale).
- **ULID jti in JWT**: `NumericDate` has second precision, so two `Issue` calls within the same second produce identical tokens. Added a random ULID as the `jti` claim to guarantee uniqueness. This is important for refresh token rotation — without it, a rotated-away token would compare equal to the new one and remain "valid" in the fakeRedis store.
- **Redis-backed refresh tokens**: Short-lived access tokens (15min) paired with Redis-stored refresh tokens (7 days). On rotation, the old token is deleted before the new one is stored — prevents replay.
- **Interface-typed repository in Service**: `RepositoryInterface` allows unit tests to use a fake without a running Postgres. `NewService` and `NewServiceWithInterfaces` co-exist for ergonomics.
- **RedisStore in platform/redis**: Added `Store` wrapper (`NewStore`) that adapts `*redis.Client` to the `RedisStore` interface the service depends on. Keeps the platform package composable.
