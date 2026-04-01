# Continuity Ledger: Phase 0 Foundation

## Goal
Build the foundation for RentMy: Go backend scaffold, all infrastructure integrations, React Native mobile scaffold, GitHub Actions CI. Exit criteria: server boots, DB migrated, S3/Redis/River/Pusher connected, RN app navigates, CI green.

## Constraints
- Monorepo: `backend/` (Go) + `mobile/` (Expo RN)
- All tech decisions in `.claude/plan/phase-0-foundation.md`
- Go module path: `github.com/giits/rentmy/backend`
- ULID as text(26) for all PKs
- Pusher hosted (Soketi for local dev)

## Key Decisions
- chi v5 for HTTP routing (subrouter mounting per module)
- pgx v5 for Postgres (required by River)
- goose v3 for migrations (embeddable)
- caarlos0/env v11 for config (12-factor)
- Expo Router v4 for navigation (file-based)
- Zustand + TanStack Query for state
- NativeWind v4 for styling

## State
- Done:
  - [x] Step 0.1: Go project scaffold
  - [x] Step 0.2: Docker Compose + Postgres/PostGIS + migrations
  - [x] Step 0.3: Redis integration
  - [x] Step 0.4: River job queue
  - [x] Step 0.5: S3 storage
  - [x] Step 0.6: Pusher integration
  - [x] Step 0.7: React Native scaffold
  - [x] Step 0.8: GitHub Actions CI
- Now: Manual verification
- Next: Phase 1 planning

## Open Questions
- None

## Working Set
- Plan: `.claude/plan/phase-0-foundation.md`
- Handoffs: `thoughts/handoffs/phase-0-foundation/`
- Branch: `main`
- Backend test: `cd backend && make test`
- Mobile test: `cd mobile && npx expo start`
