# Task 8.17 — Documentation Cleanup & Consolidation

**Status:** Completed  
**Branch:** `task-8.17-doc-consolidation`  
**Date:** 2026-04-12

## What Was Done

Moved the three root-level documentation files into a dedicated `docs/` directory and updated all references to reflect the new paths.

### File Moves (via `git mv`)

| Old Path | New Path |
|----------|----------|
| `rentmy-prd-v8.md` | `docs/rentmy-prd-v8.md` |
| `00-index.md` | `docs/roadmap.md` |
| `cross-cutting.md` | `docs/cross-cutting.md` |

### Reference Updates

- **CLAUDE.md** — Updated Documentation Map table with new paths (`docs/rentmy-prd-v8.md`, `docs/roadmap.md`, `docs/cross-cutting.md`). Added `docs/` row to monorepo layout table.
- **README.md** — Added `docs/` directory entry to the Project Structure section.
- **`.claude/progress.json`** — Updated all 36 `prdRefs` entries from `rentmy-prd-v8.md §...` to `docs/rentmy-prd-v8.md §...`.

## Verification

- All three files present in `docs/` with correct names
- `CLAUDE.md` Documentation Map reflects new paths
- `README.md` project structure includes `docs/`
- `progress.json` JSON is valid (`python3 -m json.tool`)
- All 36 prdRefs updated with `docs/` prefix

## Notes

- Git rename tracking preserved via `git mv` (not copy+delete)
- No other files in the repo reference these docs by path (checked via grep)
- The `.claude/plan/phase-8-visual-qa.md` and other plan files reference these docs by name but are read-only reference docs — not updated as they are historical records

## Branching Mode

Graphite mode (`gt create`).
