# RentMy — Implementation Roadmap

> Sequenced by dependency. Each phase unlocks the next. Nothing starts until its blockers are done.
> **Critical path: 16 weeks** to full platform. Phases overlap 1–2 weeks where backend and frontend work can parallelize.

## Phases

| Phase | Name | Weeks | File |
|---|---|---|---|
| 0 | Foundation | Wk 1–2 | `phase-0-foundation.md` |
| 1 | Users + Listings (Supply) | Wk 3–5 | `phase-1-supply.md` |
| 2 | Discovery + Payments (Demand) | Wk 5–8 | `phase-2-demand.md` |
| 3 | Core Transaction Loop | Wk 7–10 | `phase-3-transaction.md` |
| 4 | AI Agents (Intelligence) | Wk 9–12 | `phase-4-intelligence.md` |
| 5 | Test Infrastructure + Retroactive Coverage | Wk 11–12 | `phase-5-test-infrastructure.md` |
| 6 | Returns, Disputes, Trust | Wk 13–16 | `phase-6-closing-loop.md` |
| 7 | Operations + Growth | Wk 15–18 | `phase-7-operations.md` |
| — | Cross-Cutting Concerns | Throughout | `cross-cutting.md` |

## Milestone Checkpoints

| Milestone | Phase | What's true |
|---|---|---|
| **Skeleton running** | End of Phase 0 | Server boots, DB migrated, S3/Redis/River/Pusher connected, RN app navigates |
| **Supply exists** | End of Phase 1 | Host can sign up, capture angle-enforced photos, create listing, see it in profile |
| **Demand can find + pay** | End of Phase 2 | Renter can search/browse/map (normalized ranking), see tiered hold amount, check out with Stripe. Hold allocation ledger and guarantee fund tracking active |
| **Transaction completes** | End of Phase 3 | Full loop: book → handoff → return. Complete state machine enforced. Fraud velocity rules block bad bookings. 7-day ceiling enforced. Angle-enforced photos at check-in/check-out |
| **Platform is smart** | End of Phase 4 | AI fills listings, dual-score system active (reputation 0–1000 + per-transaction risk 0–100), generates agreements, verifies identity. Existing data backfilled |
| **Tests pass** | End of Phase 5 | Integration tests run against real Postgres+Redis, mobile component tests run, retroactive coverage for Phases 1-4, CI pipeline validates all test types |
| **Loop closes** | End of Phase 6 | CV+LLM photo diff detects damage, disputes resolve via hold allocation with damage reserve, reputation scores update, guarantee fund tracks reserve ratio, outcome linking feeds learning framework |
| **Launch-ready** | End of Phase 7 | Ops team has dashboard with agent calibration metrics, fraud detection active (WiFi compound-only), referrals work, alerts fire |
