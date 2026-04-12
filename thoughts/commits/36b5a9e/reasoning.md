# Commit 36b5a9e — chore: move docs to docs/ and update all path references

## Why this commit

Task 8.17 of Phase 8 establishes a single `docs/` directory as the home for all project documentation. Having PRD, roadmap, and cross-cutting concerns files at the repo root created noise in top-level listings and made it unclear where to look for documentation vs. code.

## What changed

- Three root-level markdown files moved via `git mv` (preserves history)
- `00-index.md` renamed to `roadmap.md` for clarity
- CLAUDE.md documentation map updated (agents and humans reading it will find the right paths)
- README.md project structure updated (new contributors see `docs/` in the tree)
- All 36 `prdRefs` in `progress.json` updated from `rentmy-prd-v8.md §N` to `docs/rentmy-prd-v8.md §N`

## Alternatives considered

- Leaving files at root: rejected — root is already cluttered with config files
- Symlinking: unnecessary complexity when a clean move suffices
- Updating all phase plan `.md` files: those are read-only historical records, not live navigation aids
