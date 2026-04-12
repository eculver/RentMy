# Phase 8 — Visual QA, Bug Fixing & v0 Stabilization

> **Scope:** Systematically screenshot every mobile screen in iOS Simulator, audit the ops dashboard via Chrome MCP, document bugs, fix them (mobile + backend + ops), consolidate docs, verify compilation/tests.
> **Exit criteria:** App runs in iOS Simulator with no red screens, all screens render correctly with seeded data, ops dashboard renders and functions correctly, all tests pass, documentation is consolidated.
> **Blockers:** All Phase 0-7 tasks completed.
>
> **Tools:**
> - **iOS Simulator** — `xcrun simctl` for mobile screenshots and interaction
> - **Chrome MCP** (`mcp__Control_Chrome__*`) — for ops dashboard page inspection, console error checking, and content verification. Requires the Claude Chrome extension with `localhost` added to allowed sites.

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
9. Record credentials in the [Appendix: Test Seed Data](#appendix-test-seed-data) section below
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

## Step 8.15 — Audit: Ops Dashboard + Backend Ops Tools

**Goal:** Verify all ops tools work end-to-end: dashboard renders correctly, backend endpoints return valid data, periodic jobs are scheduled, and alerts route properly. Ops tools must be coherent with the core business — metrics, fraud signals, and agent calibration should reflect real platform state.

**Prerequisites:**
- Backend running with seeded data (`cd backend && make dev`)
- Ops dashboard running (`cd ops && npm run dev`)
- At least one completed booking in the database (for meaningful metrics)
- Chrome MCP connected (Claude Chrome extension installed, `localhost` in allowed sites)

**Tools — Chrome MCP:**

Use Chrome MCP to navigate to each page, read rendered content, and check for errors:
```
mcp__Control_Chrome__open_url        → Navigate to each dashboard route
mcp__Control_Chrome__get_page_content → Read rendered HTML to verify data displays
mcp__Control_Chrome__execute_javascript → Check for console errors, read DOM state, click buttons
mcp__Control_Chrome__list_tabs       → Verify the ops dashboard tab is active
```

For each page: (1) navigate via `open_url`, (2) read content via `get_page_content`, (3) check console for errors via `execute_javascript` with `JSON.stringify(window.__consoleErrors || [])`, (4) compare displayed values against API responses from `curl`.

### Ops Dashboard (ops/)

**Pages to audit (use Chrome MCP for each):**

1. **Dashboard** (`http://localhost:5173/`)
   - Navigate with `open_url`, verify 4 metric category cards render (Business, Trust & Safety, Supply, Demand)
   - Verify trend charts load (7d gross revenue, fraud flag rate)
   - Check anomaly alert banner appears/hides correctly
   - Check browser console for JS errors

2. **Alerts** (`http://localhost:5173/alerts`)
   - Verify "Feed" tab shows alert instances (or empty state)
   - Verify "Rules" tab shows all alert rules with correct fields
   - Test acknowledge button on an alert via `execute_javascript`
   - Test toggling a rule's enabled state
   - Test editing a rule's threshold

3. **Review Queue** (`http://localhost:5173/reviews`)
   - Verify fraud flag table renders with filters (Status, Action)
   - Test clicking into a flag detail

4. **Review Detail** (`http://localhost:5173/reviews/{flagId}`)
   - Verify metadata renders (score, action, signals)
   - Verify evidence viewer component
   - Test resolution buttons (Approve / Override / Request Info)

5. **Agent Decisions** (`http://localhost:5173/agents/decisions`)
   - Verify table renders with agent type filter
   - Test expanding a row to see reasoning/input/decision JSON

6. **Agent Learning** (`http://localhost:5173/agents/learning`)
   - Verify calibration charts render for each agent type
   - Verify guarantee fund gauges show balance/reserve ratio

7. **Referrals** (`http://localhost:5173/referrals`)
   - Verify stats cards render (Total Codes, Conversions, Payouts, Rate)
   - Verify referral table with status column

8. **Login** (`http://localhost:5173/login`)
   - Verify login works with seeded user credentials
   - Verify 401 redirect when token is missing/expired

### Backend Ops Endpoints

**OpsAgent endpoints:**
```bash
# Health metrics
curl -sf http://localhost:8080/api/v1/ops/metrics/current -H "Authorization: Bearer $TOKEN"
curl -sf http://localhost:8080/api/v1/ops/metrics/history?duration=7d -H "Authorization: Bearer $TOKEN"

# Alerts
curl -sf http://localhost:8080/api/v1/ops/alerts -H "Authorization: Bearer $TOKEN"
curl -sf http://localhost:8080/api/v1/ops/alerts/rules -H "Authorization: Bearer $TOKEN"
```

**FraudAgent endpoints:**
```bash
curl -sf http://localhost:8080/api/v1/ops/fraud/flags -H "Authorization: Bearer $TOKEN"
curl -sf "http://localhost:8080/api/v1/ops/fraud/users/{userId}/signals" -H "Authorization: Bearer $TOKEN"
```

**Agent learning + guarantee fund:**
```bash
curl -sf http://localhost:8080/api/v1/ops/agents/decisions -H "Authorization: Bearer $TOKEN"
curl -sf "http://localhost:8080/api/v1/ops/agents/calibration?agent_type=RISK" -H "Authorization: Bearer $TOKEN"
curl -sf http://localhost:8080/api/v1/ops/agents/metrics -H "Authorization: Bearer $TOKEN"
curl -sf http://localhost:8080/api/v1/guarantee/health -H "Authorization: Bearer $TOKEN"
curl -sf http://localhost:8080/api/v1/ops/referrals/stats -H "Authorization: Bearer $TOKEN"
```

### Periodic Jobs

Verify River workers are registered and scheduled:
- `ops_health_check` — runs every 15 minutes
- `fraud_pattern_scan` — runs every 6 hours

### Coherence Checks

- Metrics from `/ops/metrics/current` should reflect actual DB state (listings count, user count, etc.)
- Fraud flags should reference real users and real signals
- Agent decisions should link to real transactions
- Guarantee fund health should match ledger entries
- Alert rules should cover the 16 documented metrics
- Dashboard displays should match API response shapes (no empty cards when data exists)

**Deliverable:** `thoughts/audits/phase-8-visual-qa/audit-ops-tools.md`

---

## Step 8.16 — Fix: Ops Dashboard + Backend Ops Tools Bugs

**Goal:** Fix all bugs documented in audit-ops-tools.md.

**Verification (use Chrome MCP to verify dashboard fixes):**
- Re-navigate each dashboard page via `mcp__Control_Chrome__open_url` and confirm fixes render correctly
- Check browser console via `mcp__Control_Chrome__execute_javascript` — zero JS errors on all pages
- All ops API endpoints return 200 with valid JSON (`curl`)
- `cd ops && npx tsc --noEmit`
- `cd backend && go vet ./...`
- `cd backend && go build -o /dev/null ./cmd/server`

---

## Step 8.17 — Documentation Cleanup & Consolidation (formerly 8.15)

**Goal:** Single source of truth for all docs. Remove redundant root-level phase files.

**What to do:**
1. ~~Delete redundant root-level files~~ — **Done** (cleaned up in `chore/cleanup-phase-plan-docs` branch)
2. Move to `docs/`:
   - `rentmy-prd-v8.md` → `docs/rentmy-prd-v8.md`
   - `00-index.md` → `docs/roadmap.md`
   - `cross-cutting.md` → `docs/cross-cutting.md`
3. Update `CLAUDE.md` Documentation Map with new paths
4. Update `README.md` if it references moved files
5. Clean up stale `prdRefs` in `.claude/progress.json`

**Verification:** No broken doc references anywhere.

---

## Step 8.18 — Compilation & Test Suite Health Check (formerly 8.16)

**Goal:** Ensure everything compiles and all tests pass.

**Commands:**
```bash
cd backend && go build -o /dev/null ./cmd/server
cd backend && go vet ./...
cd backend && go test ./... -count=1
cd mobile && npx tsc --noEmit
cd mobile && npx expo export --platform ios
cd mobile && npx jest --ci
cd ops && npx tsc --noEmit
cd ops && npx vite build
```

Fix any failures found.

**Verification:** All 8 commands pass with zero failures.

---

## Step 8.19 — Final Verification Pass (formerly 8.17)

**Goal:** Full end-to-end confirmation that v0 is stable.

**What to do:**
1. Full walkthrough of every tab in iOS Simulator
2. Screenshot each main screen in final state
3. Full walkthrough of every ops dashboard page via Chrome MCP (`mcp__Control_Chrome__open_url` + `get_page_content`)
4. Verify zero JS console errors across all ops pages
5. Run full compilation and test suites
6. Verify all doc references are valid
7. Write summary: `thoughts/audits/phase-8-visual-qa/final-report.md`

**Verification:**
```bash
cd backend && go build -o /dev/null ./cmd/server
cd backend && go vet ./...
cd backend && go test ./... -count=1
cd mobile && npx tsc --noEmit
cd mobile && npx jest --ci
```

---

## Appendix: Test Seed Data

### Test Users

| Name | Email | Password | User ID |
|------|-------|----------|---------|
| Alice Test | alice@test.com | password123 | 01KNZ4649E3NV0PJB82YHG38CQ |
| Bob Renter | bob@test.com | password123 | 01KNZ468WYRDRAGT1VSA3B4GHA |

### Test Listings (all owned by Alice)

| Title | Listing ID | Daily Rate |
|-------|-----------|------------|
| DeWalt Power Drill | 01KNZ47G1A5JWQNZ8YR8YFKS66 | $15 |
| Canon EOS R6 Camera | 01KNZ47ZKWMNR3MRWH1AKYEDT3 | $75 |
| Stand Up Paddleboard | 01KNZ47ZRQ4CA2NWFQ31FD7HF0 | $35 |
| Camping Tent 4-Person | 01KNZ47ZT5QHZJFDEEK43AVN8R | $20 |
| Pressure Washer 3000 PSI | 01KNZ47ZV9EJN9WVECTRSZDX1K | $45 |

### Limitations

- **No bookings**: POST /api/v1/bookings requires Stripe payment method — fails with placeholder API keys
- **No messages**: Messages are nested under /bookings/{id}/messages — require a booking first
- **Location**: Simulator location set to 34.0522, -118.2437 (Los Angeles)

### Known Issues Found During Seeding

1. `.env` had wrong postgres port (5432 vs 5433 from docker-compose) — fixed
2. App bypasses auth gate on fresh install — goes straight to feed instead of login
3. Location shows "unavailable" despite simulated location being set
