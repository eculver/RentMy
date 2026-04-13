# Commit 2fff99c — Reasoning

## Why this commit

Task 9.0 is the foundation for the entire Phase 9 E2E suite. Without Maestro installed, directory structure in place, testID hooks on key elements, and a verified login flow, every subsequent task (9.1–9.11) has no infrastructure to build on.

## Key choices

**testID on Input/Button components rather than screens directly** — Adding testID as an optional prop to the shared UI components means all future screens automatically get the ability to tag elements without touching the component internals again. This is the minimal-change approach that avoids touching every existing screen.

**Tab navigation by text, not testID** — Expo Router's Tabs.Screen `options` doesn't support a `tabBarTestID` prop (TypeScript error confirmed). Maestro's `tapOn: "Feed"` text match is reliable for labeled tabs and requires zero app changes. This is standard practice in Maestro testing.

**`~/.maestro/bin/maestro` in Makefile rather than bare `maestro`** — CI agents running in a fresh shell won't have `~/.zshrc` sourced. Absolute path is defensive and works in both dev and CI contexts.

**Committed `dev.env`** — The credentials in `dev.env` are test-only (alice/bob seed accounts with `password123`). These are not secrets — they exist solely for local E2E testing and appear in handoff docs. Committing dev.env lets every developer (human or agent) run the suite without separate secret setup.
