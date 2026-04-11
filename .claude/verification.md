# Verification Framework

## Verification Levels

### Level 1: Compilation (REQUIRED for every task)
```bash
cd backend && go vet ./...
cd backend && go build ./cmd/server
cd mobile && npx tsc --noEmit          # if mobile changes
terraform fmt -check                    # if IaC changes
terraform validate                      # if IaC changes
```

### Level 2: Tests (REQUIRED for every backend task)
```bash
cd backend && go test ./... -v -count=1
cd mobile && npm test                   # when tests exist
go test ./tests/...                     # Terratest for IaC modules
```

### Level 3: Integration (REQUIRED for service tasks)
```bash
docker compose up -d                    # ensure services running
cd backend && make dev &                # start server
sleep 3 && curl -sf http://localhost:8080/health
# Run task-specific curl commands from the phase plan
kill %1                                 # stop server
```

For IaC: `terraform plan` (never `apply` without human review).

### Level 4: End-to-End (REQUIRED at phase exit)
- All Level 1-3 verifications pass
- CI passes: push and check GitHub Actions
- Phase exit criteria from `00-index.md` verified
- Continuity ledger updated with all tasks marked complete

---

## Pre-Commit Checklist

Before committing, verify ALL of the following:

1. [ ] `go vet ./...` passes (no warnings)
2. [ ] `go build ./cmd/server` succeeds
3. [ ] `go test ./... -v -count=1` passes (all tests green)
4. [ ] `npx tsc --noEmit` passes (if mobile changes)
5. [ ] `terraform fmt -check` passes (if IaC changes)
6. [ ] New migrations are idempotent (run twice, no errors)
7. [ ] No hardcoded secrets or credentials
8. [ ] Error handling follows wrapping pattern: `fmt.Errorf("context: %w", err)`
9. [ ] Handoff document written with verification results
10. [ ] `progress.json` updated with task status and commit SHA
11. [ ] `progress.json` validated: `cat .claude/progress.json | python3 -m json.tool > /dev/null`
