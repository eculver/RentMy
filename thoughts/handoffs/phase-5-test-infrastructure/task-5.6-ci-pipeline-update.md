# Task 5.6 — CI Pipeline Update

**Status:** Completed
**Commit:** d0604d3d1da3a482a6f9b5f40dbac30f439c731b
**Branch:** task-5.6-ci-pipeline-update
**Graphite mode:** Yes

## What was done

Updated `.github/workflows/ci.yml` to unconditionally run both integration and mobile tests now that Phase 5 coverage is in place.

### Changes

**`backend-integration` job:**
- Removed conditional skip guard (`if [ -d tests/integration ] && ls *_test.go ...`)
- Added two env vars needed for testcontainers-go on GitHub Actions:
  - `TESTCONTAINERS_RYUK_DISABLED=true` — Ryuk (the resource reaper) requires a privileged container, which ubuntu-latest runners don't provide; disabling it is safe for CI runs since containers are cleaned up when the runner exits
  - `TESTCONTAINERS_RYUK_VERBOSE=false` — suppresses noisy startup logs

**`mobile-test` job:**
- Removed conditional skip guard (`if npx jest --listTests | grep -q '.'`)
- Now runs `npx jest --ci --coverage --verbose` directly

No structural changes to the job ordering or dependencies — `backend-integration` still needs `backend-lint`, `mobile-test` still needs `mobile-lint`.

## Verification

Both verification commands passed locally:

```
cd backend && go test ./tests/integration/... -v -count=1 -timeout 300s
# ok  github.com/giits/rentmy/backend/tests/integration  10.283s

cd mobile && npx jest --ci
# Test Suites: 8 passed, 8 total
# Tests:       59 passed, 59 total
```

## Dependencies added

None.

## Notes for next session

Phase 5 is now complete. All tasks 5.1–5.6 are done. The next phase is Phase 6 (check progress.json for the first pending Phase 6 task).
