# Phase 8 — Visual QA, Bug Fixing & v0 Stabilization

> **Scope:** Systematically screenshot every mobile screen in iOS Simulator, document bugs, fix them (mobile + backend), consolidate docs, verify compilation/tests.
> **Exit criteria:** App runs in iOS Simulator with no red screens, all screens render correctly with seeded data, all tests pass, documentation is consolidated.
> **Blockers:** All Phase 0-7 tasks completed.

---

## Step 8.0 — Bootstrap: Full Stack + iOS Simulator + Seed Data

**Goal:** Get the entire development stack running and the app visible in iOS Simulator with test data.

**What to do:**
1. Start docker services: `docker compose up -d` (postgres, redis, minio, soketi)
2. Verify/create `.env` from `.env.example`
3. Start Go backend: `cd backend && make dev` (runs migrations automatically)
4. Verify backend health: `curl -sf http://localhost:8080/health`
5. Install mobile deps if needed: `cd mobile && npm ci`
6. Build and launch in iOS Simulator: `cd mobile && npx expo run:ios`
7. Set simulated location: `xcrun simctl location booted set 34.05 -118.24`
8. Seed test data via curl:
   - 2 test users via `POST /api/v1/auth/register`
   - 5+ listings via `POST /api/v1/listings` with varied prices/locations
   - 2 bookings in different states (REQUESTED, CONFIRMED)
   - 1 message thread between users
9. Save credentials to `.claude/plan/phase-8-seed-credentials.md`
10. Take first screenshot: `xcrun simctl io booted screenshot /tmp/rentmy-bootstrap.png`

**Note:** Build/infra failures get fixed here. This is the "make everything run" task.

**Verification:**
- `curl -sf http://localhost:8080/health` returns 200
- App visible in simulator, screenshot captured

---

## Step 8.1 — Audit: Auth Flow (Login + Register)

**Goal:** Visually inspect auth screens and document all bugs.

**Screens:**
- `mobile/app/(auth)/login.tsx`
- `mobile/app/(auth)/register.tsx`

**Components under test:**
- `mobile/components/ui/Input.tsx`
- `mobile/components/ui/Button.tsx`

**Audit checklist:**
1. Screenshot login screen default state
2. Test form validation (empty fields, invalid email, short password)
3. Test successful login with seeded credentials
4. Screenshot register screen
5. Test register form validation
6. Test successful registration
7. Check Metro console for JS errors
8. Check backend logs for API errors

**Deliverable:** `thoughts/audits/phase-8-visual-qa/audit-auth.md`

---

## Step 8.2 — Fix: Auth Flow Bugs

**Goal:** Fix all bugs documented in audit-auth.md.

**Verification:**
- Re-screenshot auth screens, confirm fixes
- `cd mobile && npx tsc --noEmit`
- `cd mobile && npx jest`
- `cd backend && go vet ./...`
- `cd backend && go build -o /dev/null ./cmd/server`

---

## Step 8.3 — Audit: Feed + Listing Detail + Checkout

**Goal:** Visually inspect feed, listing detail, and checkout screens.

**Screens:**
- `mobile/app/(tabs)/(feed)/index.tsx`
- `mobile/app/(tabs)/(feed)/listing/[id].tsx`
- `mobile/app/(tabs)/(feed)/listing/[id]/checkout.tsx`
- `mobile/app/(tabs)/(feed)/listing/[id]/confirmation.tsx`

**Components under test:**
- `ListingFeedCard`, `ListingCard`, `PhotoGallery`, `HostInfoCard`
- `AvailabilityCalendar`, `HoldExplainer`, `CostBreakdown`
- `DurationPicker`, `PaymentMethodSelector`

**Audit checklist:**
1. Screenshot feed with seeded listings visible
2. Verify listing cards render (title, price, image placeholder)
3. Tap listing → screenshot detail view
4. Check photo gallery rendering
5. Navigate to checkout → screenshot
6. Check cost breakdown display
7. Check console/network for errors

**Deliverable:** `thoughts/audits/phase-8-visual-qa/audit-feed-listing.md`

---

## Step 8.4 — Fix: Feed + Listing Detail + Checkout Bugs

**Verification:** Same as Step 8.2 pattern.

---

## Step 8.5 — Audit: Search + Map

**Goal:** Visually inspect search and map screens.

**Screens:**
- `mobile/app/(tabs)/(search)/index.tsx`
- `mobile/app/(tabs)/(map)/index.tsx`

**Components under test:**
- `FilterSheet`, `MapScreen.native.tsx`, `ListingMarker.native.tsx`
- `ListingPreviewCard`, `MapCallout.native.tsx`

**Audit checklist:**
1. Screenshot search screen default state
2. Type a query, verify results
3. Toggle filters, verify updates
4. Navigate to map tab → screenshot
5. Verify map renders with markers for seeded listings
6. Tap a marker, verify callout/preview

**Deliverable:** `thoughts/audits/phase-8-visual-qa/audit-search-map.md`

---

## Step 8.6 — Fix: Search + Map Bugs

**Verification:** Same pattern.

---

## Step 8.7 — Audit: Booking + Handoff (Check-in/Check-out)

**Goal:** Visually inspect booking and handoff screens.

**Screens:**
- `mobile/app/(tabs)/(feed)/booking-request.tsx`
- `mobile/app/(tabs)/(feed)/booking-status.tsx`
- `mobile/app/(tabs)/(feed)/active-rental.tsx`
- `mobile/app/(tabs)/(feed)/check-in.tsx`
- `mobile/app/(tabs)/(feed)/check-out.tsx`

**Components under test:**
- `BookingCard`, `IncomingRequest`, `CancelConfirmation`
- `GPSStatus`, `PINDisplay`, `PINEntry`, `PhotoGrid`
- `AngleEnforcedCamera.native.tsx`

**Audit checklist:**
1. Navigate to booking request screen with seeded booking params
2. Screenshot booking status for different states
3. Screenshot active rental screen
4. Navigate to check-in screen → screenshot (camera view)
5. Navigate to check-out screen → screenshot
6. Check for GPS permission prompts

**Deliverable:** `thoughts/audits/phase-8-visual-qa/audit-booking-handoff.md`

---

## Step 8.8 — Fix: Booking + Handoff Bugs

**Verification:** Same pattern.

---

## Step 8.9 — Audit: Messaging

**Goal:** Visually inspect messaging screens.

**Screens:**
- `mobile/app/(tabs)/(messages)/index.tsx`
- `mobile/app/(tabs)/(messages)/conversation.tsx`

**Components under test:**
- `ConversationList`, `MessageBubble`, `MessageInput`

**Audit checklist:**
1. Screenshot conversation list with seeded thread
2. Open conversation → screenshot
3. Test sending a message
4. Check for Pusher/WebSocket errors in console
5. Verify unread count display

**Deliverable:** `thoughts/audits/phase-8-visual-qa/audit-messaging.md`

---

## Step 8.10 — Fix: Messaging Bugs

**Verification:** Same pattern.

---

## Step 8.11 — Audit: Rentals + Disputes + Ratings

**Goal:** Visually inspect post-rental screens.

**Screens:**
- `mobile/app/(tabs)/(rentals)/index.tsx`
- `mobile/app/(tabs)/(rentals)/dispute.tsx`
- `mobile/app/(tabs)/(rentals)/dispute-status.tsx`
- `mobile/app/(tabs)/(rentals)/rate.tsx`
- `mobile/app/(tabs)/(rentals)/return-confirmation.tsx`

**Components under test:**
- `DisputeTimeline`, `HoldStatusCard`, `PhotoDiffResult`
- `RatingBubbles`, `RatingPrompt`

**Audit checklist:**
1. Screenshot rentals list with different booking states
2. Navigate to dispute screen → screenshot
3. Navigate to dispute status → screenshot timeline
4. Navigate to rate screen → screenshot
5. Navigate to return confirmation → screenshot

**Deliverable:** `thoughts/audits/phase-8-visual-qa/audit-rentals-disputes.md`

---

## Step 8.12 — Fix: Rentals + Disputes + Ratings Bugs

**Verification:** Same pattern.

---

## Step 8.13 — Audit: Profile + Referrals + KYC

**Goal:** Visually inspect profile area screens.

**Screens:**
- `mobile/app/(tabs)/(profile)/index.tsx`
- `mobile/app/(tabs)/(profile)/create-listing.tsx`
- `mobile/app/(tabs)/(profile)/verify.tsx`
- `mobile/app/(tabs)/(profile)/referrals.tsx`

**Components under test:**
- `Avatar`, `Badge`, `KYCGate`, `ReferralCard`
- `ListingForm`, `AIAutofillOverlay`, `ValueOverridePrompt`
- `VerifyScreen.native.tsx`, `CreateListingScreen.native.tsx`

**Audit checklist:**
1. Screenshot profile tab (name, avatar, reputation)
2. Navigate to referrals → screenshot
3. Navigate to verify (KYC) → screenshot
4. Navigate to create listing → screenshot form
5. Test form validation on listing creation

**Deliverable:** `thoughts/audits/phase-8-visual-qa/audit-profile.md`

---

## Step 8.14 — Fix: Profile + Referrals + KYC Bugs

**Verification:** Same pattern.

---

## Step 8.15 — Documentation Cleanup & Consolidation

**Goal:** Single source of truth for all docs. Remove redundant root-level phase files.

**What to do:**
1. Delete redundant root-level files:
   - `phase-0-foundation.md` through `phase-6-operations.md` (7 files)
   - `.claude/plan/` has the authoritative, more detailed versions
2. Move to `docs/`:
   - `rentmy-prd-v8.md` → `docs/rentmy-prd-v8.md`
   - `00-index.md` → `docs/roadmap.md`
   - `cross-cutting.md` → `docs/cross-cutting.md`
3. Update `CLAUDE.md` Documentation Map with new paths
4. Update `README.md` if it references moved files
5. Clean up stale `prdRefs` in `.claude/progress.json`

**Verification:** No broken doc references anywhere.

---

## Step 8.16 — Compilation & Test Suite Health Check

**Goal:** Ensure everything compiles and all tests pass.

**Commands:**
```bash
cd backend && go build -o /dev/null ./cmd/server
cd backend && go vet ./...
cd backend && go test ./... -count=1
cd mobile && npx tsc --noEmit
cd mobile && npx expo export --platform ios
cd mobile && npx jest --ci
```

Fix any failures found.

**Verification:** All 6 commands pass with zero failures.

---

## Step 8.17 — Final Verification Pass

**Goal:** Full end-to-end confirmation that v0 is stable.

**What to do:**
1. Full walkthrough of every tab in iOS Simulator
2. Screenshot each main screen in final state
3. Run full compilation and test suites
4. Verify all doc references are valid
5. Write summary: `thoughts/audits/phase-8-visual-qa/final-report.md`

**Verification:**
```bash
cd backend && go build -o /dev/null ./cmd/server
cd backend && go vet ./...
cd backend && go test ./... -count=1
cd mobile && npx tsc --noEmit
cd mobile && npx jest --ci
```
