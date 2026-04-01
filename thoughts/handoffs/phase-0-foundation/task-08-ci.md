# Task 08: GitHub Actions CI

## Status: COMPLETE

## What Was Done
- `.github/workflows/ci.yml` — Single workflow with two jobs:
  - **backend**: Postgres (PostGIS) + Redis services, go vet, go build, go test
  - **mobile**: Node 22, npm ci, tsc --noEmit, expo lint (graceful fallback)

## Key Decisions
- Uses `go-version-file: backend/go.mod` so CI picks up the Go version from the project
- Postgres uses the same `postgis/postgis:16-3.4` image as Docker Compose for parity
- Mobile lint step has a graceful fallback since Expo lint config may not be configured yet
- Both jobs run on every push to main and every PR — no path filtering (simpler for now, can add later if CI gets slow)

## Notes for Next Task
- CI won't fully pass until tests exist (go test will pass vacuously with no test files)
- Expo lint config can be added when ESLint is configured in Phase 1
