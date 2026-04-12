---
name: graphite
description: Use when creating branches, amending commits, or managing stacked PRs with the Graphite CLI (gt).
metadata:
  author: @eculver
  version: 1.0.0
---

# Graphite CLI

We use [Graphite](https://graphite.dev) for stacked PRs. Each logical change gets its own branch.

## Creating a branch

```bash
gt create <branch-name>
```

Stage and commit changes separately, or pass `-m "message"` to commit in one step.

## Amending a commit

Always use `gt modify`, not `git commit --amend`:

```bash
# Amend with staged changes (opens editor for message)
gt modify

# Amend without editing the message
gt modify --no-edit

# Amend and stage all tracked file changes
gt modify -a --no-edit

# Create a new commit on the branch instead of amending
gt modify --commit -m "message"
```

`gt modify` automatically restacks all descendant branches. `git commit --amend` does not — it
leaves child branches pointing at the old parent commit.

## Restacking

```bash
gt restack
```

Rebases each branch onto its parent so the stack is linear. Run this after `gt sync` or if Graphite
warns that branches need restacking.

## Submitting PRs

```bash
# Push current branch and all ancestors to GitHub, creating/updating PRs
gt submit --no-edit

# Push the entire stack (ancestors + descendants)
gt submit --stack --no-edit
```

Always pass `--no-edit` to skip interactive PR metadata prompts.

To update an existing PR's title or description, use `gh pr edit --title "..." --body "..."`.

## Syncing with remote

```bash
gt sync --no-interactive
```

Fetches remote changes, cleans up merged branches, and restacks where possible. Always pass
`--no-interactive` to avoid prompts about deleting branches.

## Gotchas

- **Never use `git rebase` directly** — it can break Graphite's stack metadata.
- **Never use `git merge` or `git pull`** — use `gt sync` instead.
- **Prefer one commit per branch.** `gt modify` amends the single commit as you iterate. If you need
  a second commit, use `gt modify --commit`.
- **Always use `gt modify` over `git commit --amend`** — the latter works at the git level but
  doesn't restack descendants, so you'd have to `gt restack` manually.
- **Never use `gt submit --edit`** — it is an interactive command. Use `gh pr edit` instead.
