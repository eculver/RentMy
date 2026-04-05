# Task 4.3 — AppraisalAgent Handoff

## Summary

Implemented the AppraisalAgent — AI-powered item identification, pricing suggestions, and host-declared value override review using the model router. New listings automatically trigger async appraisal via a River job.

## Branching Mode

Git fallback (vanilla git). Branch: `task-4.3-appraisal-agent`. Commit: `4f86e34`.

## What Was Built

### `backend/internal/agent/appraisal/`

| File | Purpose |
|------|---------|
| `model.go` | Domain types: `Appraisal`, `OverrideRequest`, `AppraisalResponse`, prompt template inputs, AI result structs |
| `repository.go` | Postgres CRUD for `appraisals` table: Insert, FindByListingID, Update, UpdateOverride |
| `service.go` | AppraisalAgent logic: `Appraise`, `ReviewOverride`, `GetAppraisal`, `EnqueueAppraisal`, `SetDeps` |
| `job.go` | `AppraisalJobArgs` + `AppraisalJobWorker` River job for async appraisal |
| `handler.go` | HTTP handlers: `POST /listings/:id/appraise`, `POST /listings/:id/override`, `GET /listings/:id/appraisal` |
| `service_test.go` | Unit tests covering: no router, no media, no listing photos, idempotency, not found, override flows, tag merging |

### `backend/migrations/008_appraisals.sql`

- Adds `appraisal_status TEXT CHECK('PENDING','COMPLETE','FAILED')` column to `listings` table (default PENDING)
- Creates `appraisals` table with one-per-listing UNIQUE constraint, storing full AI result including pricing, tags, confidence, model/prompt version, and override decisions

### `backend/prompts/appraisal_override/v1.txt`

Override review prompt in standard `{agent_name}/v{N}.txt` convention. The existing `appraisal/override_v1.txt` from task 4.1 was placed in the wrong directory pattern for the prompt loader; `appraisal_override/v1.txt` is the correct path.

### Changes to `backend/internal/listing/`

| File | Change |
|------|--------|
| `model.go` | Added `AppraisalStatus` type + constants; `AppraisalStatus` field on `Listing`; `AppraisalFieldsUpdate` struct |
| `repository.go` | Added `appraisal_status` to all SELECT/RETURNING clauses; added `UpdateAppraisalFields` method with COALESCE logic to preserve existing non-empty fields |
| `service.go` | Added `AppraisalEnqueuer` interface; `WithAppraisalEnqueuer` setter; `UpdateAppraisalResult` method; `slog` import; enqueues after successful listing creation |
| `service_test.go` | Added `UpdateAppraisalFields` stub to `fakeRepo` |

### Changes to `backend/internal/media/service.go`

Added `GetByListingID` method exposing the repository's `FindByListingID` to the service layer.

### Changes to `backend/cmd/server/main.go`

- Imports `appraisal` package
- Builds `appraisalSvcPre` (nil listingSvc/mediaSvc) before River starts, registers `AppraisalJobWorker`
- After listing/media services are built: creates `appraisalSvcFull`, calls `appraisalSvcPre.SetDeps(...)`, mounts `appraisalHandler`
- Calls `listingSvc.WithAppraisalEnqueuer(appraisalSvcFull)` to trigger AI on every new listing

## Appraisal Pipeline

```
POST /api/v1/listings/:id
→ listingSvc.Create() succeeds
→ appraisalEnqueuer.EnqueueAppraisal() → River queue

AppraisalJobWorker.Work()
→ appraisalSvc.Appraise(listingID)
  ├── fetch listing (listing.Service.Get)
  ├── upsert PENDING appraisal row (idempotent)
  ├── fetch listing media (media.Service.GetByListingID)
  ├── filter to LISTING_PHOTO type (up to 5)
  ├── download images via net/http GET on OriginalURL
  ├── render appraisal/v1.txt prompt
  ├── Route(TaskItemIdentification → Sonnet vision)
  │   └── parse JSON: item_name, category, condition, estimated_value_usd, pricing, description, tags, confidence
  ├── fetchExtraTags: Route(TaskTagGeneration → Haiku, text-only)
  │   └── merge + deduplicate tags (lowercase)
  ├── appraisals.Update(COMPLETE, full result)
  ├── listing.UpdateAppraisalFields(ai_generated_tags, estimated_value, title/desc/price only if empty)
  └── decision.RecordDecision(AgentTypeAppraisal)

Failure path: any error → appraisals.Update(FAILED, reason) + listing.UpdateAppraisalFields(FAILED)
River retries: up to 3 times by default before marking permanently failed
```

## Override Review

```
POST /api/v1/listings/:id/override {declaredValueCents, justification}
→ appraisalSvc.ReviewOverride()
  ├── require existing appraisal with estimated_value_cents
  ├── render appraisal_override/v1.txt prompt
  ├── Route(TaskValueOverrideReview → Sonnet)
  ├── parse: {approved, reasoning, confidence}
  ├── appraisals.UpdateOverride(override_approved, override_reasoning)
  └── decision.RecordDecision(AgentTypeAppraisal)
```

## Key Design Decisions

- **Two-phase service init**: `appraisalSvcPre` (nil deps for River registration) + `SetDeps()` to inject after services are built. Mirrors the verification service pattern. The worker uses the pre-river service which has its deps filled before any jobs execute.
- **Separate `appraisalSvcFull`** is used for HTTP handlers and as the `AppraisalEnqueuer` for the listing service. Both service instances share the same repository.
- **Image download via `net/http`**: `OriginalURL` is a full HTTP URL (e.g., `http://minio:9000/media-originals/key`). Direct HTTP GET avoids needing to parse bucket/key from the URL.
- **`appraisal_override` prompt directory**: The prompt loader expects `{dir}/{agent_name}/v{N}.txt`. The override prompt is logically a separate agent task, so it lives in `prompts/appraisal_override/v1.txt` (not `appraisal/override_v1.txt` from the initial scaffold).
- **Tag merging**: Primary tags from Sonnet vision + extra semantic tags from Haiku. Both lowercased and deduplicated. Graceful degradation: Haiku failure is non-fatal.
- **COALESCE in UpdateAppraisalFields SQL**: AI-suggested title/description only fills empty strings; pricing only fills NULL values. Hosts who manually set these fields before appraisal runs are not overwritten.

## Verification

All passed:
```
cd backend && go vet ./...              # no issues
cd backend && go build ./cmd/server    # clean build
cd backend && go test ./... -count=1   # all packages green
cd backend && go test ./internal/agent/appraisal/... -v  # 10 tests PASS
```
