# Phase 4 — AI Agents (Intelligence) Continuity Ledger

## Phase Goal

AI takes over verification, appraisal, risk scoring, and agreement generation. Listings get smart. Bookings get scored. Existing data backfilled. Phase 5 unblocked.

---

## Task Status

| Task | Name | Status | Commit | Branch |
|------|------|--------|--------|--------|
| 4.1 | Model Router | completed | 9e82f13 | task-4.1-model-router |
| 4.2 | VerificationAgent | completed | f67e68b | task-4.2-verification-agent |
| 4.3 | AppraisalAgent | completed | 4f86e34 | task-4.3-appraisal-agent |
| 4.4 | RiskAgent | completed | e613b60 | task-4.4-risk-agent |
| 4.5 | AgreementAgent | pending | — | — |
| 4.6 | Wire KYC into booking flow (RN) | pending | — | — |
| 4.7 | Wire AI autofill into listing creation (RN) | pending | — | — |
| 4.8 | Backfill existing data | pending | — | — |

---

## Key Architectural Decisions

### Model Router (4.1)
- Anthropic SDK (`anthropic-sdk-go v1.30.0`) — single vendor for Haiku + Sonnet
- Tier matrix hardcoded in `tier_matrix.go` — reviewable, not config-driven
- Prompt templates in `backend/prompts/{agent}/v{N}.txt` — versioned, git-diff reviewable
- `AnthropicRouter` is nil-safe — dev without API key degrades gracefully

### VerificationAgent (4.2)
- Stripe Identity for KYC (same SDK as payments — zero extra vendor)
- Edge cases (blurry selfie, expired doc) routed to Haiku for interpretation
- Stripe webhook: `POST /api/v1/webhooks/stripe-identity`
- One-time +50 reputation bonus on first verification (via `user.AddReputationScore`)

### AppraisalAgent (4.3)
- Sonnet (vision) for item identification + pricing from photos
- Haiku for additional semantic tags (cheaper, text-only)
- Async via River job (`AppraisalJob`) — non-blocking on listing creation
- Override review: host declared value > 100% of AI estimate → Sonnet review
- `appraisals` table, `appraisal_status` column on listings

### RiskAgent (4.4)
- Fully deterministic rules engine (no LLM for v1)
- Dual-score system: reputation (0–1000 per user), risk score (0–100 per transaction)
- Risk score blocks CreateBooking if ≥ 71 (CRITICAL) via `booking.Service.WithRiskAgent()`
- `reputation_signals` table with 180-day decay on negative signals
- `risk_scores` table stores per-transaction breakdown
- River workers: `MonthlyReputationWorker`, `DecayCheckWorker` (registered, scheduling in Phase 6)

---

## Known Limitations / Next Session Notes

- **4.4**: Admin recalculation endpoint (`POST /api/v1/admin/reputation/:userId/recalculate`) deferred to Phase 6 when admin auth is available.
- **4.4**: River periodic jobs not yet scheduled — workers are registered but must be triggered manually or wired to River's `PeriodicJobs` config in Phase 6.
- **4.5 (next)**: AgreementAgent requires base template JSON (`base_template_v1.json` exists from 4.1), guardrail validation, and `agreement_snapshot` JSONB on transactions.
- **4.6**: KYC gate in mobile requires `@stripe/stripe-identity-react-native` install.
- **4.8**: Backfill jobs need 4.3 and 4.4 both complete — ready to implement.

---

## Infrastructure Notes

- All migrations are additive: 009_reputation_signals.sql adds `reputation_signals` and `risk_scores` tables
- `booking.Service` is now nil-safe for riskSvc — safe to deploy before risk service is configured
- Existing `user.AddReputationScore` remains for VerificationAgent's one-time KYC bonus; `risk.Repository.SetReputationScore` is used for full recalculation
