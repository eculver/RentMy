# Commit 35e1a90 — 3 consecutive green E2E runs

## Why

Task 9.9 requires proving E2E suite reliability: 28 flows must pass identically across 3 consecutive runs with zero flakiness.

## What was broken

Two infrastructure issues caused intermittent failures after ~15+ test cycles:

1. **iOS EXC_GUARD XPC crash:** Rapid `clearState` → `launchApp` cycles accumulated stale XPC handles. After ~15 flows, the kernel raised `EXC_GUARD` via `_XPC_MISUSE_FAULT`, crashing the app on launch. Fix: `stopApp` before `clearState` in both login helpers.

2. **Stale Maestro driver processes:** Previous runs left behind `xcodebuild` and `maestro-driver-iosUITests-Runner` processes holding port 7001. Fix: `_e2e-clean-drivers` Makefile prerequisite target that kills stale processes before each run.

## Results

| Run | Result | Duration |
|-----|--------|----------|
| 1   | 28/28  | 18m 25s  |
| 2   | 28/28  | 18m 24s  |
| 3   | 28/28  | 18m 57s  |

Zero flaky tests. Identical pass/fail results across all 3 runs.
