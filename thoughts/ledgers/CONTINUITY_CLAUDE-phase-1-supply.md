# Phase 1 — Users + Listings (Supply) — Continuity Ledger

## Phase Status: IN PROGRESS (5/6 tasks complete)

---

## Completed Tasks

### 1.1 — UserService (backend) — f19fae3
- Full user registration, login, JWT access/refresh token issuance
- bcrypt cost 12, access token 15min, refresh token 7d in Redis
- Endpoints: POST /api/v1/auth/register, /login, /refresh; GET/PUT /api/v1/users/me
- Auth middleware injects user ID into context for protected routes

### 1.2 — MediaService (backend) — 3feadd5
- S3 upload (original + thumbnail via disintegration/imaging at 800px longest side)
- Orientation metadata (roll/pitch/yaw) stored per media record
- Endpoints: POST /api/v1/media/upload (multipart), GET /api/v1/media/:id

### 1.3 — ListingService (backend) — 4650d50
- 7-day ceiling enforced on maxDuration; default status = PENDING
- PostGIS geography column for location; INTERVAL column for maxDuration
- Endpoints: POST/GET/PUT /api/v1/listings, POST /api/v1/listings/:id/media, GET /api/v1/users/me/listings
- Custom Duration type serializing as Go duration strings (e.g. "168h")

### 1.4 — Auth screens (RN) — 6757a98
- Login and register screens with react-hook-form + Zod validation
- Token refresh on 401 before logout (silent refresh pattern)
- refreshToken stored in SecureStore; loadToken() restores it on app restart
- No circular imports: authApi is a bare ky instance separate from api.ts

### 1.5 — Listing creation flow (RN) — 268851f
- AngleEnforcedCamera: DeviceMotion sensor fusion for roll/pitch/yaw; 30° soft-block
- useGyroscope hook + angularDistance utility
- ListingForm: title, description, pricing (daily + optional hourly), duration picker, lat/lng
- Two-step flow: camera → form → upload photos → create listing → attach media
- (profile) Stack layout added; Profile index updated with Create Listing CTA

---

## Remaining Tasks

### 1.6 — Profile screen (RN) — PENDING
- Depends on: 1.4 (auth screens)
- Fetch user data from GET /api/v1/users/me
- Display name/avatar/reputation
- "My Listings" section with FlatList using GET /api/v1/users/me/listings
- New: ListingCard component, useUser hook, useListings hook

---

## Key Architecture Notes

- All backend services: handler.go returns chi.Router; mounted via Mount(r, authMW) pattern
- All IDs: ULID text(26)
- Mobile API client: ky in lib/api.ts with auto Bearer token injection
- Mobile state: Zustand (auth store) + TanStack Query (server state)
- NativeWind for all styling — no StyleSheet.create except where required by RN primitives
