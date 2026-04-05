# Task 4.2 — VerificationAgent Handoff

## Summary

Implemented the VerificationAgent that orchestrates KYC using Stripe Identity with AI-assisted edge case interpretation. All decisions are logged to the `agent_decisions` audit table.

## Branching Mode

Git fallback (vanilla git). Branch: `task-4.2-verification-agent`. Commit: `f67e68b`.

## What Was Built

### `backend/internal/agent/verification/`

| File | Purpose |
|------|---------|
| `model.go` | Domain types: `VerificationAttempt`, `VerificationStatus`, `StartVerificationResult`, `VerificationStatusResult`, template input/output types |
| `repository.go` | Postgres CRUD for `verification_attempts`: Insert, FindByUserID, FindBySessionID, UpdateStatus, IncrementRetryCount. Uses `attemptRepository` interface for testability |
| `service.go` | VerificationAgent business logic: `StartVerification`, `GetStatus`, `HandleWebhook`. Decision tree: auto-approve (Stripe verified) → auto-reject (fraud codes) → Haiku interpretation (edge cases) → escalate |
| `stripe.go` | `stripeIdentityAdapter`: concrete Stripe SDK implementation of `StripeIdentityAdapter`. Uses `stripeclient.API.IdentityVerificationSessions.New` and `webhook.ConstructEvent` |
| `handler.go` | HTTP handlers: `POST /api/v1/verification/start`, `GET /api/v1/verification/status`, `POST /api/v1/webhooks/stripe-identity` |
| `timeout_job.go` | `VerificationTimeoutWorker` River job: polls stalled PENDING sessions every 10 minutes, retries up to 3 times, then escalates |
| `service_test.go` | Unit tests: auto-approve (Stripe verified), auto-reject (selfie_manipulated), edge case escalation (no router), canceled session, status with no attempt, isFraudCode |

### `backend/migrations/007_verification_attempts.sql`

Creates `verification_attempts` table with indexes on user_id, stripe_session_id, and status (partial index for PENDING).

### `backend/internal/agent/router/router.go` (modified)

Added `RenderPrompt(agentName string, data any) (text, version string, err error)` — public wrapper around the internal `promptCache.Render` for use by agents.

### `backend/internal/user/` (modified)

- `repository.go`: Added `UpdateIdentityStatus` and `AddReputationScore` (clamps to [0, 1000]) methods
- `service.go`: Added `UpdateIdentityStatus` and `AddReputationScore` service methods; extended `RepositoryInterface` to include both new methods
- `service_test.go`: Added `UpdateIdentityStatus` and `AddReputationScore` to `fakeRepo`

### `backend/internal/platform/config/config.go` (modified)

Added `StripeIdentityWebhookSecret` (`STRIPE_IDENTITY_WEBHOOK_SECRET`) — separate signing secret for Stripe Identity webhooks.

### `backend/cmd/server/main.go` (modified)

- Removed `_ = modelRouter` and `_ = decisionSvc` placeholders (now consumed)
- Builds `verificationRepo` and a pre-River `verificationSvcPre` (nil userSvc/riverClient) to register `VerificationTimeoutWorker` before River starts
- After all services are up, builds the real `verificationSvc` with full deps and mounts `verificationHandler`

## Decision Logic

```
Stripe webhook received
├── identity.verification_session.verified  → auto-approve → VERIFIED + +50 rep
├── identity.verification_session.requires_input
│   ├── fraud code (selfie_manipulated, document_fraudulent)  → auto-reject → REJECTED
│   └── edge case (expired doc, low selfie match, etc.)
│       ├── modelRouter nil  → ESCALATED (model_router_unavailable)
│       └── modelRouter set → Haiku (TaskKYCInterpretation)
│           ├── APPROVE  → VERIFIED + +50 rep
│           ├── REJECT   → REJECTED
│           └── ESCALATE → ESCALATED (escalation_reason from AI)
└── identity.verification_session.canceled  → CANCELED
```

## Interfaces Defined

- `StripeIdentityAdapter` — allows test fakes and future mock in unit tests
- `UserService` — subset of `user.Service` needed by the verification agent
- `DecisionService` — subset of `decision.Service` (for testability)
- `attemptRepository` — unexported interface used by `Service` and `VerificationTimeoutWorker`

## Key Design Decisions

- **Pre-River service pattern**: mirrors the notification service pattern — a placeholder service (nil userSvc/riverClient) registered before River starts, real service built after. Workers hold the pre-river service (nil riverClient is nil-guarded).
- **No user identity_status change on escalation**: when escalated, the user's `identity_status` stays PENDING. Human reviewers update it manually via a future admin API.
- **Reputation bonus only for first-time verification**: the service checks `prevUser.IdentityStatus != VERIFIED` before awarding +50 points.
- **Separate Stripe webhook secret**: `STRIPE_IDENTITY_WEBHOOK_SECRET` is separate from the payment `STRIPE_WEBHOOK_SECRET`, matching Stripe's recommended setup (separate webhook endpoint, separate secret).

## Verification

All passed:
```
cd backend && go vet ./...              # no issues
cd backend && go build ./cmd/server    # clean build
cd backend && go test ./...            # all packages green
cd backend && go test ./internal/agent/verification/... -v  # 6 tests PASS
```
