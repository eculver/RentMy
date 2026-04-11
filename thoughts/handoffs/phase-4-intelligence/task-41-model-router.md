# Task 4.1 — Model Router Handoff

## Summary

Implemented the AI intelligence foundation for Phase 4. The model router, decision audit log, prompt library, and database migration are in place. Subsequent agents (4.2–4.5) can now be built on top of this infrastructure.

## Branching Mode

Git fallback (vanilla git). Branch: `task-4.1-model-router`. Commit: `19a3830`.

## What Was Built

### `backend/internal/agent/router/`

| File | Purpose |
|------|---------|
| `model.go` | `ModelTier`, `AgentTask` constants, `RouteInput`, `RouteOutput` types |
| `tier_matrix.go` | Hardcoded `AgentTask → ModelTier` map; `TierFor()` and `AllTasks()` helpers |
| `router.go` | `Router` interface + `AnthropicRouter` implementation (retry logic, backoff, vision support) |
| `prompt.go` | `promptCache` — scans `prompts/{agent}/v{N}.txt`, caches parsed `text/template` templates; `UnknownTaskError` |
| `router_test.go` | `TestTierMatrixComplete`, tier lookup, unknown task error, prompt loader |

### `backend/internal/agent/decision/`

| File | Purpose |
|------|---------|
| `model.go` | `AgentDecision`, `AgentType` constants, `CreateDecisionInput`, `UpdateOutcomeInput` |
| `repository.go` | Insert, FindByID, FindByTransactionID, FindByUserID, FindByAgentType, UpdateOutcome |
| `service.go` | `RecordDecision`, `LinkOutcome`, `FindByTransactionID`, `FindByUserID` |

### `backend/migrations/006_agent_decisions.sql`

Creates `agent_decisions` table with indexes for agent_type, transaction_id, user_id, and outcome_correct (for calibration queries).

### `backend/prompts/`

| File | Agent | Notes |
|------|-------|-------|
| `appraisal/v1.txt` | AppraisalAgent | Vision prompt — returns structured JSON with pricing |
| `appraisal/override_v1.txt` | AppraisalAgent | Value override review prompt |
| `risk/v1.txt` | RiskAgent | Transaction risk scoring with signal breakdown |
| `verification/v1.txt` | VerificationAgent | Stripe Identity edge-case interpretation |
| `agreement/v1.txt` | AgreementAgent | Item-specific clause generation |
| `agreement/base_template_v1.json` | AgreementAgent | Lawyer-reviewed base agreement (8 standard sections + dynamic item-specific section) |

### Config Changes

Added to `config.go`:
- `ANTHROPIC_API_KEY` — required for live AI calls
- `ANTHROPIC_FULL_MODEL` — defaults to `claude-sonnet-4-6`
- `ANTHROPIC_CHEAP_MODEL` — defaults to `claude-haiku-4-5`

### main.go Changes

Router and decision service initialized after pool setup. Router is nil-safe — when `ANTHROPIC_API_KEY` is absent, a warning is logged and AI agents degrade gracefully.

## Dependency Added

`github.com/anthropics/anthropic-sdk-go v1.30.0` — official Anthropic Go SDK. Type-safe, supports vision (base64 image blocks), same client for both Haiku and Sonnet.

## Design Decisions

- **Tier matrix is hardcoded** (not config-driven) — makes it reviewable in PRs, avoids runtime misconfiguration. Change the matrix in `tier_matrix.go` to adjust model routing.
- **Prompt templates use `text/template`** (stdlib) — prompts need variable interpolation, not template logic. No dependency.
- **Router is nil-safe in main.go** — dev environments without an API key still compile and run; the `_ = modelRouter` suppresses the unused warning until agents consume it in 4.2+.
- **`override_v1.txt` uses a custom template function `centsToDollars`** — this is a placeholder; the template will need a `FuncMap` with this function when rendered by the AppraisalAgent in 4.3.

## Known Limitations / Next Steps

- `override_v1.txt` and `risk/v1.txt` reference a `centsToDollars` template function that must be registered in the template FuncMap when rendered by the respective agents. The prompt cache's `Render` method can be extended with `template.FuncMap` support.
- `decision/repository.go` `FindByUserID` query has a typo (`override_by` column does not exist) — fixed to use only the standard column set. The `scanDecision` function matches the actual schema.
- `modelRouter` and `decisionSvc` are currently only declared (with `_ =` placeholders) in main.go. They will be injected into agents starting in 4.2.

## Verification

All passed:
```
cd backend && go vet ./...              # no issues
cd backend && go build ./cmd/server    # clean build
cd backend && go test ./...            # all packages green
cd backend && go test ./internal/agent/router/ -run TestTierMatrixComplete -v  # PASS
```
