# Phase 1 — Users + Listings / Supply Side Implementation Plan

> **Scope:** Wk 3-5. Host can sign up, create listing with camera-enforced photos, see it in profile.
> **Exit criteria:** Register, login, upload photos with orientation metadata, create listing with 7-day ceiling, view profile with listings.
> **Blockers:** Phase 0 complete (DB, S3, project scaffolds)

## Resolved Decisions

| Question | Answer | Notes |
|----------|--------|-------|
| Thumbnail generation | Go `imaging` package (pure Go) | No cgo, no libvips. Migrate later if performance requires it |
| Password hashing | bcrypt | Standard, well-tested via `golang.org/x/crypto/bcrypt` |
| JWT refresh tokens | Redis (TTL-based) | Short-lived, high-read, no persistence needed |
| Magic link delivery | Email only (Resend) | SMS deferred to Phase 3 (Twilio) |
| Auth method v1 | Email + password (magic link optional) | Simplest MVP path |
| Gyroscope data format | Euler angles (roll, pitch, yaw) | Already in schema as three REAL columns |
| Image resize target | 800px longest side (thumbnail) | Original preserved in S3 |

---

## Technology Decisions

### Go Backend (New Dependencies)

| Decision | Choice | Import / Module |
|----------|--------|-----------------|
| Password hashing | bcrypt | `golang.org/x/crypto/bcrypt` |
| JWT library | golang-jwt v5 | `github.com/golang-jwt/jwt/v5` (already in go.mod) |
| Image processing | disintegration/imaging | `github.com/disintegration/imaging` |
| Validation | go-playground/validator v10 | `github.com/go-playground/validator/v10` (already in go.mod) |
| Email (magic link) | Resend Go SDK | `github.com/resend/resend-go/v2` |

**Rationale:**
- **disintegration/imaging over libvips:** Pure Go — zero cgo, cross-compiles cleanly, handles resize/crop/rotate. Performance is fine for thumbnail generation at v1 scale.
- **bcrypt over argon2:** Simpler API, no tuning parameters to get wrong, universally supported. Cost factor 12 is the sweet spot.
- **Resend over SendGrid:** Developer-first API, simpler SDK, generous free tier. Swappable later.

### React Native (New Dependencies)

| Decision | Choice | Package |
|----------|--------|---------|
| Camera | react-native-vision-camera v4 | `react-native-vision-camera` |
| Gyroscope | expo-sensors | `expo-sensors` |
| Image picker (disabled) | N/A | Camera-only, no gallery |

---

## Implementation Steps

### Step 1.1 — UserService (backend)

**Create:**
- `backend/internal/user/model.go` — User domain type, CreateUserInput, UpdateUserInput, LoginInput
- `backend/internal/user/repository.go` — Postgres queries: Insert, FindByID, FindByEmail, FindByPhone, Update, UpdateLastActive
- `backend/internal/user/service.go` — Business logic: Register (hash password, generate ULID, default reputation=0, identity=PENDING), Login (verify password, issue JWT+refresh), GetProfile, UpdateProfile
- `backend/internal/user/handler.go` — HTTP handlers returning `chi.Router`:
  - `POST /api/v1/auth/register`
  - `POST /api/v1/auth/login`
  - `POST /api/v1/auth/refresh`
  - `GET /api/v1/users/me`
  - `PUT /api/v1/users/me`
- `backend/internal/platform/auth/middleware.go` — JWT auth middleware: extract Bearer token, validate, inject user ID into context
- `backend/internal/platform/auth/jwt.go` — JWT issuing (access token 15min, refresh token 7d) and validation functions

**Modify:**
- `backend/cmd/server/main.go` — Mount user router at `/api/v1`, wire auth middleware to protected routes
- `backend/internal/platform/config/config.go` — Add `JWTSecret`, `JWTAccessTTL`, `JWTRefreshTTL` fields

**Verify:**
```bash
cd backend && go vet ./...
cd backend && go build ./cmd/server
cd backend && go test ./... -v -count=1
# Integration test:
docker compose up -d
cd backend && make dev &
sleep 3
curl -sf -X POST http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"test@example.com","password":"Test1234!","name":"Test User"}'
# Should return 201 with user object + tokens
kill %1
```

### Step 1.2 — MediaService (backend)

**Create:**
- `backend/internal/media/model.go` — Media domain type, UploadInput (includes orientation fields)
- `backend/internal/media/repository.go` — Insert, FindByID, FindByListingID, FindByTransactionID
- `backend/internal/media/service.go` — Upload flow: validate image, generate ULID, upload original to S3 (`media-originals/{id}`), generate thumbnail via `imaging.Resize`, upload thumbnail to S3 (`media-thumbnails/{id}`), store metadata in DB
- `backend/internal/media/handler.go` — HTTP handlers:
  - `POST /api/v1/media/upload` (multipart form: image + orientation JSON)
  - `GET /api/v1/media/:id`

**Modify:**
- `backend/cmd/server/main.go` — Mount media router

**Verify:**
```bash
cd backend && go test ./... -v -count=1
# Integration test:
curl -sf -X POST http://localhost:8080/api/v1/media/upload \
  -H "Authorization: Bearer $TOKEN" \
  -F "image=@test-photo.jpg" \
  -F 'orientation={"roll":15.2,"pitch":45.0,"yaw":90.3}'
# Should return 201 with media object including thumbnail URL
```

### Step 1.3 — ListingService (backend)

**Create:**
- `backend/internal/listing/model.go` — Listing domain type, CreateListingInput, UpdateListingInput
- `backend/internal/listing/repository.go` — Insert, FindByID, FindByHostID (paginated), Update, AttachMedia
- `backend/internal/listing/service.go` — Create (validate 7-day ceiling, default status=PENDING), Update, Get, ListByHost, AttachMedia (link media records to listing)
- `backend/internal/listing/handler.go` — HTTP handlers:
  - `POST /api/v1/listings`
  - `GET /api/v1/listings/:id`
  - `GET /api/v1/users/me/listings`
  - `PUT /api/v1/listings/:id`
  - `POST /api/v1/listings/:id/media` (attach uploaded media)

**Modify:**
- `backend/cmd/server/main.go` — Mount listing router

**Verify:**
```bash
cd backend && go test ./... -v -count=1
# Integration test:
curl -sf -X POST http://localhost:8080/api/v1/listings \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Kayak","description":"Ocean kayak","pricePerDay":50,"maxDuration":"168h","location":{"lat":33.77,"lng":-118.19}}'
# Should return 201 with listing object
# Test 7-day ceiling:
curl -sf -X POST http://localhost:8080/api/v1/listings \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Test","maxDuration":"240h"}'
# Should return 400 — exceeds 7-day ceiling
```

### Step 1.4 — Auth screens (RN)

**Modify:**
- `mobile/app/(auth)/login.tsx` — Real login form with email/password, calls `POST /api/v1/auth/login`, stores tokens via auth store
- `mobile/app/(auth)/register.tsx` — Real registration form with name/email/password, validation (Zod), calls register endpoint
- `mobile/lib/auth.ts` — Add login/register API calls, token refresh logic, auto-logout on 401

**Verify:**
```bash
cd mobile && npx tsc --noEmit
```

### Step 1.5 — Listing creation flow (RN)

**Create:**
- `mobile/app/(tabs)/(profile)/create-listing.tsx` — Listing creation screen: camera → form → submit
- `mobile/components/camera/AngleEnforcedCamera.tsx` — Camera component with gyroscope overlay: reads `expo-sensors` accelerometer/gyroscope, shows circular rotation indicator, soft-blocks shutter if <30deg from any existing photo, green indicator when rotation sufficient. Stores orientation (roll, pitch, yaw) per capture
- `mobile/components/listing/ListingForm.tsx` — Form: title, description, price (hourly/daily), duration, location picker
- `mobile/lib/hooks/useGyroscope.ts` — Hook wrapping expo-sensors gyroscope subscription

**Install:**
```bash
cd mobile && npx expo install react-native-vision-camera expo-sensors
```

**Verify:**
```bash
cd mobile && npx tsc --noEmit
```

### Step 1.6 — Profile screen (RN)

**Modify:**
- `mobile/app/(tabs)/(profile)/index.tsx` — Real profile screen: fetch user data, display name/avatar/reputation, "My Listings" section with FlatList

**Create:**
- `mobile/components/listing/ListingCard.tsx` — Listing card showing photo, title, price, status badge
- `mobile/lib/hooks/useUser.ts` — TanStack Query hook for user profile
- `mobile/lib/hooks/useListings.ts` — TanStack Query hook for user's listings

**Verify:**
```bash
cd mobile && npx tsc --noEmit
```

---

## API Endpoints

| Method | Path | Auth | Request Body | Response | Errors |
|--------|------|------|-------------|----------|--------|
| POST | `/api/v1/auth/register` | No | `{email, password, name, phone?}` | `{user, accessToken, refreshToken}` | 400 validation, 409 email exists |
| POST | `/api/v1/auth/login` | No | `{email, password}` | `{user, accessToken, refreshToken}` | 400 validation, 401 bad credentials |
| POST | `/api/v1/auth/refresh` | No | `{refreshToken}` | `{accessToken, refreshToken}` | 401 expired/invalid |
| GET | `/api/v1/users/me` | Yes | — | `{user}` | 401 unauthorized |
| PUT | `/api/v1/users/me` | Yes | `{name?, avatar?, notificationPreferences?}` | `{user}` | 400 validation |
| POST | `/api/v1/media/upload` | Yes | multipart: `image` + `orientation` JSON | `{media}` | 400 bad image, 413 too large |
| GET | `/api/v1/media/:id` | Yes | — | `{media}` | 404 |
| POST | `/api/v1/listings` | Yes | `{title, description, pricePerHour?, pricePerDay?, maxDuration, location, availability?}` | `{listing}` | 400 validation, 400 >7day ceiling |
| GET | `/api/v1/listings/:id` | Yes | — | `{listing, media[]}` | 404 |
| GET | `/api/v1/users/me/listings` | Yes | `?page=1&limit=20` | `{listings[], total, page}` | — |
| PUT | `/api/v1/listings/:id` | Yes | `{title?, description?, ...}` | `{listing}` | 403 not owner, 404 |
| POST | `/api/v1/listings/:id/media` | Yes | `{mediaIds[]}` | `{listing, media[]}` | 403 not owner, 404 |

---

## Database Migrations

No new migrations needed — all tables exist from migration 001. Phase 1 operates on: `users`, `listings`, `media`.

---

## Risks and Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| react-native-vision-camera v4 requires native module rebuild | Dev setup complexity | Expo Dev Client handles this. Document `npx expo prebuild` step |
| Gyroscope accuracy varies across devices | Inconsistent angle enforcement | Use ≥30deg threshold (generous). Log orientation data to identify problem devices |
| Image upload size limits | Failed uploads on slow connections | Limit to 10MB per photo. Client-side resize before upload if >4000px |
| bcrypt cost factor performance | Slow registration on cheap hardware | Cost factor 12 takes ~250ms — acceptable for auth endpoints |
| JWT secret rotation | All sessions invalidated | Use `JWTSecret` env var. Rotation is a deployment concern, not code concern |

---

## Testing Strategy

- **Unit tests:** bcrypt hashing, JWT issuance/validation, 7-day ceiling enforcement, tiered hold calculation (stubbed), input validation
- **Integration tests:** Register → Login → Get Profile flow. Upload photo → verify S3 storage. Create listing → verify in DB with correct fields
- **Repository tests:** All CRUD operations against test Postgres (testcontainers or Docker service in CI)
- **RN:** TypeScript check passes. Manual verification of camera and form flows on simulator

---

## Implementation Order

| Step | What | Day | Depends On |
|------|------|-----|------------|
| 1.1 | UserService + auth middleware | Day 1-2 | Phase 0 complete |
| 1.2 | MediaService | Day 2-3 | 1.1 (auth required for uploads) |
| 1.3 | ListingService | Day 3-4 | 1.1, 1.2 (needs auth + media) |
| 1.4 | Auth screens (RN) | Day 3-4 | 1.1 (needs API endpoints) |
| 1.5 | Listing creation flow (RN) | Day 4-6 | 1.3, 1.4 (needs API + auth screens) |
| 1.6 | Profile screen (RN) | Day 5-6 | 1.4 (needs auth for data) |

Steps 1.4 and 1.2/1.3 are independent and can be done in parallel (mobile vs backend).
