# CLAUDE.md — RentMy Development Guide

## Project Overview

RentMy is a mobile-only, hyperlocal P2P rental marketplace. Small AI-native team — agents run the platform, humans watch dashboards and make strategic decisions.

**Monorepo layout:**

| Directory | Purpose |
|-----------|---------|
| `/backend` | Go modular monolith API server |
| `/mobile` | React Native (Expo) mobile app |
| `/terraform` | Infrastructure as Code (Terragrunt + Terraform modules) |
| `/packer` | VM images (Packer) |
| `/ansible` | Configuration management |
| `/scripts` | Helper/utility scripts (bash or Go) |
| `/ops` | Internal ops dashboard (Vite + React, Phase 6) |
| `/migrations` | Database migrations (goose SQL files) |

**Current state:** Phases 0-4 complete. Phase 7 (Test Infrastructure) runs next, then Phases 5-6. See `.claude/progress.json` for exact task status.

---

## Quick Start Commands

### Backend
```bash
cd backend && make dev          # Start Go server (runs migrations automatically)
cd backend && make test         # Run all Go tests
cd backend && make lint         # Run go vet
cd backend && make build        # Build binary to bin/server
```

### Mobile
```bash
cd mobile && npm ci             # Install dependencies
cd mobile && npx expo start     # Start Expo dev server
cd mobile && npx tsc --noEmit   # TypeScript check
```

### Infrastructure
```bash
docker compose up -d            # Start Postgres/Redis/MinIO/Soketi
docker compose down             # Stop all services
curl http://localhost:8080/health  # Health check (all deps)
```

### Terraform
```bash
terraform fmt                   # Format (run before every commit)
terraform validate              # Validate syntax
terraform -chdir=terraform/environments/dev plan -out=plans/dev.tfplan
terraform -chdir=terraform/environments/dev apply plans/dev.tfplan
```

---

## Documentation Map

| Document | Purpose |
|----------|---------|
| `rentmy-prd-v8.md` | Complete PRD — data models, business logic, agent specs |
| `00-index.md` | Roadmap — 7 phases, milestone checkpoints |
| `phase-{0-6}-*.md` | Phase-level task breakdowns with exit criteria |
| `cross-cutting.md` | Testing, observability, error handling, rate limiting |
| `.claude/plan/phase-{N}-*.md` | Detailed implementation plans (Phase 0 is the template) |
| `.claude/plan/cross-cutting-integration.md` | Cross-cutting concerns mapped per phase |
| `.claude/progress.json` | **Source of truth** for task status |
| `.claude/verification.md` | Verification levels and checklist |
| `thoughts/handoffs/phase-{N}-*/task-*.md` | Per-task completion handoff documents |
| `thoughts/ledgers/CONTINUITY_CLAUDE-phase-{N}-*.md` | Phase-level progress summaries |
| `thoughts/commits/*/reasoning.md` | Commit reasoning documents |

---

## Session Workflow (READ THIS BEFORE DOING ANYTHING)

Every session follows this protocol. No exceptions.

1. **Read** `.claude/progress.json` to determine current state
2. **Find** the next task: first `"pending"` task whose `dependencies` are all `"completed"`
3. **Read** the task's phase plan (`.claude/plan/phase-{N}-*.md`) and PRD sections listed in `prdRefs`
4. **Read** handoff docs for completed dependency tasks (for context)
5. **Set** task status to `"in_progress"` in progress.json
6. **Create branch** for this task (see Branching below):
   - Try: `/opt/homebrew/bin/gt create task-{N}.{M}-{short-name}`
   - If gt fails or is unavailable: `git checkout -b task-{N}.{M}-{short-name}`
7. **Implement** ONE task only — do not start a second task
8. **Verify** using the task's `verificationCommands` plus the checklist in `.claude/verification.md`
9. **If verification passes:**
   - Stage files and commit with Conventional Commit message (`feat:`, `fix:`, `chore:`)
   - Write handoff doc: `thoughts/handoffs/phase-{N}-{name}/task-{NN}-{name}.md`
   - Update continuity ledger: `thoughts/ledgers/CONTINUITY_CLAUDE-phase-{N}-{name}.md`
   - Update progress.json: set task `"status": "completed"`, record `commitSha`
   - Validate JSON: `cat .claude/progress.json | python3 -m json.tool > /dev/null`
   - Push the branch:
     - Try: `/opt/homebrew/bin/gt submit --no-edit`
     - If gt fails: `git push -u origin task-{N}.{M}-{short-name}`
10. **If verification fails:** fix the issue and retry. Do NOT move to the next task

### Recovery Protocol

If progress.json shows a task as `"in_progress"` at session start, **you are resuming an interrupted session**. Follow these steps in order:

**Step 1 — Identify the interrupted task and its expected branch:**
```bash
# Find the in-progress task ID and name from progress.json
# Expected branch: task-{N}.{M}-{short-name}
git branch --show-current
git log --oneline -10
git diff --stat
git stash list
```

**Step 2 — Assess the state (pick the first matching scenario):**

| Scenario | Git state | Action |
|----------|-----------|--------|
| **A. Committed & verified** | On task branch, commits exist, tests pass | Complete the task: write handoff doc, update progress.json to `"completed"`, push |
| **B. Committed but broken** | On task branch, commits exist, tests fail | Fix the failing code, re-verify, then complete |
| **C. Uncommitted changes** | On task branch, `git diff` shows changes | Read the diff to understand what was done. Continue implementing from where it left off. Do NOT discard the changes. |
| **D. Branch exists, no changes** | On task branch, clean working tree, no new commits | The previous session created the branch but didn't start coding. Begin implementation. |
| **E. Wrong branch** | Not on the expected task branch | Check if the task branch exists (`git branch --list "task-*"`). If yes, switch to it and re-assess. If no, create it. |
| **F. Stashed changes** | `git stash list` shows entries | Pop the stash (`git stash pop`), assess the changes, continue implementation. |

**Step 3 — Resume:**
- Read the task's phase plan and PRD refs (you don't have context from the previous session)
- Read any handoff docs for dependency tasks
- Continue from wherever the previous session left off
- Follow the normal workflow from step 7 onward (implement → verify → commit → handoff → push)

**Rules for autonomous recovery (no human present):**
- NEVER discard uncommitted changes — always attempt to continue from them
- NEVER reset the branch — work with what exists
- If progress.json is corrupted (invalid JSON), fix it by re-reading git log to reconstruct the state
- If you genuinely cannot determine what the previous session was doing, start the task fresh on a clean branch (rename the broken branch to `task-{N}.{M}-{short-name}-abandoned`)

---

## Architecture Patterns

### Go Backend (Modular Monolith)

Each service owns its own package in `backend/internal/{service}/`:

```
backend/internal/{service}/
  handler.go      # HTTP handlers, returns chi.Router for mounting
  service.go      # Business logic
  repository.go   # Postgres queries (pgx, raw SQL)
  model.go        # Domain types
```

Shared infrastructure lives in `backend/internal/platform/{package}/`:
- `postgres/` — pgx connection pool, health check
- `redis/` — go-redis client, health check
- `s3/` — AWS SDK v2 client, bucket ops
- `pusher/` — Pusher trigger client
- `river/` — River job queue, worker lifecycle
- `config/` — Env-based config (caarlos0/env v11)
- `httpserver/` — HTTP server, middleware
- `auth/` — JWT middleware
- `ulid/` — ULID generator

`cmd/server/main.go` wires all services into chi router and starts server + River workers.

### Mobile (React Native / Expo)

- Screens: `mobile/app/{route-group}/screen-name.tsx` (Expo Router file-based)
- Components: `mobile/components/ui/`
- Libraries: `mobile/lib/` (api.ts, auth.ts, query.ts)
- State: Zustand (client state) + TanStack Query (server state)

### Terraform (Infrastructure as Code)

- Modules: `terraform/modules/<module_name>/`
- Environments: `terraform/environments/{dev,stage,prod}/`
- Follow [Terragrunt Recommended Folder Structure](https://docs.gruntwork.io/2.0/docs/overview/concepts/infrastructure-live/#suggested-folder-hierarchy) (modules stay in-repo)

---

## Coding Conventions

### Go

- Follow the [Google Go Style Guide](https://google.github.io/styleguide/go/)
- Use [functional options](https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis) for constructor functions — no large config structs or many arguments
- HTTP routing: chi v5 (do NOT switch to stdlib ServeMux — chi chosen for subrouter mounting)
- Error wrapping: `fmt.Errorf("context: %w", err)`
- Context: pass `context.Context` as first parameter
- Handler signatures: return `http.HandlerFunc` (closures over dependencies)
- Health checks: every infrastructure client exposes `HealthCheck(ctx context.Context) error`
- Tests: table-driven, use testify where needed (see Testing section below)
- No ORM — raw SQL via pgx
- Input validation on all API endpoints
- Logging: `slog` (stdlib)
- Middleware for cross-cutting concerns (logging, auth, rate limiting)
- Leave NO todos, placeholders, or missing pieces in implementations
- Run `go vet ./...` before every commit
- IDs: ULID as text(26), generated via `ulid.New()`

### Terraform

- Two-space indentation (enforced by `terraform fmt`)
- Concise hyphenated lowercase names (`vpc-core`, `eks-node-group`)
- Environment dirs lowercase, match workspace names (`dev`, `stage`, `prod`)
- Variables in `<env>.tfvars`; sensitive values via secret stores
- Pin remote state to locked backend (S3 + DynamoDB or equivalent)
- Never commit `.tfstate`
- Run `terraform fmt` before every commit
- Run `terraform validate` in each environment directory
- Keep plan files out of version control
- Terratest in `tests/` for important modules (mirror module name: `tests/vpc_core_test.go`)

### React Native / TypeScript

- Functional components only
- NativeWind `className` for styling (not `StyleSheet.create`)
- Server state via TanStack Query hooks (`useQuery`, `useMutation`)
- Client state via Zustand stores in `lib/`
- Form validation via Zod schemas
- API client: `ky` (in `lib/api.ts`) with auth header injection

---

## Testing Requirements

Every task MUST include tests. Code without tests is not complete.

### Backend: Integration Tests (testcontainers-go)

Integration tests live in `backend/tests/integration/` and run against real Postgres + Redis via testcontainers-go. They test the full HTTP API surface — request in, response out, database state verified.

```
backend/tests/integration/
  helpers.go              # TestMain, container setup, migration runner, HTTP client factory
  user_api_test.go        # Register, login, profile CRUD
  listing_api_test.go     # Create, update, search, feed
  booking_api_test.go     # Book → confirm → check-in → check-out
  ...
```

**Rules:**
- Every new backend endpoint MUST have at least one integration test
- Tests use real SQL (no mocks) — the whole point is to catch query bugs
- Test helpers provide factory functions: `createTestUser(t)`, `createTestListing(t, userID)`, etc.
- Use `t.Parallel()` where safe; each test gets its own transaction or schema
- Existing integration tests MUST keep passing when new code is added
- Run with: `cd backend && go test ./tests/integration/... -v -count=1`

### Backend: Unit Tests

Unit tests live alongside the code (`internal/{service}/*_test.go`) and test business logic with mocks/fakes. These already exist for most services. Keep writing them for:
- State machine transitions
- Calculation logic (fees, scores, decay)
- Validation rules
- Pure functions

### Mobile: Component & Screen Tests (Jest + RNTL)

Tests live in `mobile/__tests__/` and use Jest + React Native Testing Library.

```
mobile/__tests__/
  components/       # UI component renders, interactions
  screens/          # Screen-level tests with mocked API (MSW)
  lib/              # Hook and utility tests
  setup.ts          # Global test setup (MSW, mocks)
```

**Rules:**
- Every new screen MUST have at least one screen test
- Tests render the component, interact with it, and assert on the result
- API calls are mocked via MSW (Mock Service Worker) — no real backend needed
- Test forms: fill inputs, submit, verify validation errors and success states
- Test navigation: verify the right screen renders after user actions
- Run with: `cd mobile && npx jest`

### What "Done" Means

A task is complete when:
1. Code compiles (`go build`, `tsc --noEmit`)
2. Linter passes (`go vet`)
3. **New unit tests pass** for business logic
4. **New integration tests pass** for API endpoints (backend tasks)
5. **New component/screen tests pass** (mobile tasks)
6. **All existing tests still pass** (`go test ./...`, `npx jest`)

---

## Git & Branching Conventions

### Stacked Branches (Graphite preferred, git fallback)

We use [Graphite](https://graphite.dev) for stacked PRs when available. **Each task gets its own branch** that stacks on top of the previous task's branch. The Graphite CLI is at `/opt/homebrew/bin/gt`.

**If Graphite is not available** (gt not installed, repo not initialized for Graphite, or gt commands fail), fall back to vanilla git. The branch naming and one-branch-per-task rule still apply — we maintain a manual stack via sequential branching.

**Branch naming:** `task-{N}.{M}-{short-name}` (e.g., `task-1.1-user-service`, `task-1.2-media-service`)

**Workflow per task (Graphite mode):**
```bash
# 1. Create a new stacked branch for this task
/opt/homebrew/bin/gt create task-1.1-user-service

# 2. Implement the task, then stage and commit
git add <files>
git commit -m "feat: add UserService with registration and JWT auth"

# 3. If you need to amend (do NOT use git commit --amend):
/opt/homebrew/bin/gt modify --no-edit

# 4. Push branch and create/update PR:
/opt/homebrew/bin/gt submit --no-edit
```

**Workflow per task (git fallback mode):**
```bash
# 1. Create a new branch from current HEAD
git checkout -b task-1.1-user-service

# 2. Implement the task, then stage and commit
git add <files>
git commit -m "feat: add UserService with registration and JWT auth"

# 3. If you need to amend:
git commit --amend --no-edit

# 4. Push branch to remote:
git push -u origin task-1.1-user-service
```

**How to decide which mode to use:**
1. Try `/opt/homebrew/bin/gt create <branch>` first
2. If it succeeds, you are in Graphite mode for this session — use `gt modify`, `gt submit`
3. If it fails (command not found, repo not initialized, permission error, etc.), use git fallback for the rest of the session
4. Log which mode was used in the handoff doc

**Graphite rules (when in Graphite mode):**
- **Never use `git rebase` directly** — it breaks Graphite stack metadata
- **Never use `git merge` or `git pull`** — use `/opt/homebrew/bin/gt sync --no-interactive` instead
- **Use `gt modify` instead of `git commit --amend`** — it automatically restacks descendants
- **One commit per branch** (use `gt modify` to amend as you iterate)
- **Always pass `--no-edit`** to `gt submit` to skip interactive prompts

### Commit Messages

- **Conventional Commits:** `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`
- Imperative, present-tense messages — explain *why*, not *what*
- One logical change per commit
- Write `thoughts/commits/{short-sha}/reasoning.md` for each commit
- Include Terraform plan snippets in PRs when IaC changes are involved
- Never commit credentials, `.env`, `.tfstate`, or plan files

---

## Security & Secrets

- Never commit credentials or `.tfstate` — they are in `.gitignore`
- Load cloud access keys via environment variables or `direnv`
- Reference sensitive values through secret stores, not checked-in files
- Rotate service principals when modifying high-privilege modules

---

## Documentation

README.md files at every significant directory level. Keep them up to date. When you create a new directory, add a README.md explaining its purpose.

---

## Guardrails

- Do NOT modify Phase 0 files (infrastructure is complete and verified)
- Do NOT work on more than one task per session
- Do NOT skip verification steps
- Do NOT commit code that doesn't compile (`go build`, `npx tsc --noEmit` must pass)
- Do NOT add dependencies without documenting rationale in the handoff doc
- Do NOT remove or edit existing tests to make them pass
- Do NOT commit secrets or credentials

---

## Phase Revision Policy (Always Forward)

Phases are **append-only** once any task within them has been completed. If a completed phase needs rework:

1. **Do NOT** re-run the agent against a modified phase spec — the agent cannot diff old vs. new specs and will either re-implement from scratch (breaking downstream code) or skip changes entirely.
2. **Instead**, create a new refinement phase with surgical tasks that describe the delta:
   - Task names describe the change: "Migrate SearchService from tsvector to Meilisearch"
   - Dependencies point to the original tasks being refined
   - The agent gets clear "change X to Y" instructions
3. **Exception:** if a phase hasn't started yet (all tasks `"pending"`), it's safe to update the plan docs and run normally — no existing code to conflict with.
4. Add the refinement phase to `.claude/progress.json` with the next available phase ID.
5. Create a corresponding plan file: `.claude/plan/phase-{N}-{name}.md`
