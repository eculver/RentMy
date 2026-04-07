# Task 7.3 — Ops Dashboard (web)

**Branch:** task-7.3-ops-dashboard  
**Branch mode:** Graphite (gt)

## What was implemented

Full internal ops dashboard built with Vite + React + TypeScript + Tailwind CSS.

### Project scaffold (`/ops`)

- Vite React-TS template with Tailwind CSS v4 (`@tailwindcss/vite` plugin)
- API proxy to `http://localhost:8080` in dev mode
- Dependencies: recharts, @tanstack/react-query, ky v2, react-router, date-fns

### Library layer (`src/lib/`)

| File | Purpose |
|------|---------|
| `types.ts` | TypeScript types mirroring Go models: HealthSnapshot, MetricValue, Alert, AlertRule, FraudFlag, FraudSignal, AgentDecision, CalibrationBucket, Referral |
| `api.ts` | ky v2 client with prefix `/api/v1`, JWT auth hook, 401 redirect |
| `query.ts` | TanStack QueryClient (30s staleTime) |
| `auth.ts` | Login/logout, token management in localStorage |

### Layout components (`src/components/layout/`)

| File | Purpose |
|------|---------|
| `Sidebar.tsx` | NavLink sidebar: Dashboard, Review Queue, Agent Decisions, Agent Learning, Alerts, Fraud Flags, Referrals |
| `Header.tsx` | Top bar with logout button |

### Chart components (`src/components/charts/`)

| File | Purpose |
|------|---------|
| `TrendChart.tsx` | Generic time-series line chart (Recharts LineChart) |
| `BarChartCard.tsx` | Generic bar chart wrapper |
| `CalibrationChart.tsx` | Expected vs actual accuracy with calibration error shading (ComposedChart) |
| `GaugeChart.tsx` | Semi-circular gauge (PieChart 180°) for fund health / ratios |

### Metric components (`src/components/metrics/`)

| File | Purpose |
|------|---------|
| `MetricCard.tsx` | Single metric: large value, trend arrow, % change |
| `MetricGrid.tsx` | Responsive CSS grid of MetricCards |

### Review components (`src/components/review/`)

| File | Purpose |
|------|---------|
| `EvidenceViewer.tsx` | Side-by-side photo viewer + collapsible JSON viewer |
| `ActionButtons.tsx` | Approve / Override (with reason form) / Request More Info |

### Pages (`src/pages/`)

| Page | Route | Backend API |
|------|-------|-------------|
| Dashboard | `/` | `GET ops/metrics/current`, `GET ops/metrics/history` |
| ReviewQueue | `/reviews` | `GET ops/fraud/flags` |
| ReviewDetail | `/reviews/:flagId` | `GET ops/fraud/flags/:id`, `PUT ops/fraud/flags/:id/resolve` |
| AgentDecisions | `/decisions` | `GET ops/agents/decisions` |
| AgentLearning | `/learning` | `GET ops/agents/calibration`, `GET ops/agents/metrics`, `GET ops/guarantee/health` |
| Alerts | `/alerts` | `GET ops/alerts`, `PUT ops/alerts/:id/acknowledge`, `GET/PUT ops/alerts/rules` |
| Referrals | `/referrals` | `GET ops/referrals/stats`, `GET ops/referrals` |
| Login | `/login` | `POST auth/login` |

### Routing & Auth

- BrowserRouter with auth gate — unauthenticated users redirected to `/login`
- JWT stored in `localStorage` as `ops_token`
- 401 responses automatically clear token and redirect

## Verification

| Check | Result |
|-------|--------|
| `npx tsc --noEmit` | PASS |
| `npm run build` | PASS (733 kB bundle) |

## Dependencies Added

| Package | Version | Reason |
|---------|---------|--------|
| tailwindcss | 4.x | Utility CSS |
| @tailwindcss/vite | 4.x | Vite plugin for Tailwind |
| recharts | latest | Charts (line, bar, pie, composed) |
| @tanstack/react-query | latest | Server state management |
| ky | 2.0.0 | HTTP client |
| react-router | latest | Client-side routing |
| date-fns | latest | Date formatting |

## Notes for next session

- Task 7.4 (Referral system) is the next pending task
- Some dashboard API endpoints referenced (e.g. `ops/agents/decisions`, `ops/agents/calibration`, `ops/agents/metrics`, `ops/guarantee/health`, `ops/referrals`) do not exist yet in the backend — they'll be added as part of later tasks or a refinement phase
- The chunk size warning (733 kB) can be addressed with code splitting in a future optimization pass
