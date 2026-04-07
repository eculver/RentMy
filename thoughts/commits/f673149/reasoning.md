# Commit f673149 — feat: add ops dashboard

## Why
Task 7.3 requires a web-based ops dashboard for platform monitoring, agent learning visualization, fraud review, and alert management. This is the first web frontend in the repo (the `/ops` directory).

## Decisions
- **ky v2** (not v1) was installed — adapted API to use `prefix` (not `prefixUrl`) and state-object hooks
- **Tailwind CSS v4** with `@tailwindcss/vite` plugin — simpler setup than PostCSS config
- **Auth gate** at router level, not per-page — keeps page components clean
- **CalibrationChart** uses ComposedChart (Area + Lines) for the expected-vs-actual visualization
- **MetricCard** auto-formats values based on metric name patterns (cents → dollars, rate → %, hours → h)
- Some backend endpoints referenced in the dashboard don't exist yet (agents/calibration, agents/metrics, guarantee/health, referrals) — these will come from task 7.4 or a refinement phase
