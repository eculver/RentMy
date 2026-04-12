# Ops Dashboard

Internal operations dashboard for RentMy platform monitoring.

## Stack

- **Vite** + **React** + **TypeScript**
- **Tailwind CSS** for styling
- **Recharts** for charts (calibration, trend, bar, gauge)
- **TanStack Query** for server state
- **ky** v2 for HTTP client
- **react-router** for routing
- **date-fns** for date formatting

## Quick Start

```bash
npm ci
npm run dev     # http://localhost:5173
npm run build   # Production build
```

## Pages

| Route | Description |
|-------|-------------|
| `/` | Platform dashboard — metric grids, trend charts, anomaly alerts |
| `/reviews` | Fraud flag review queue — filterable table |
| `/reviews/:flagId` | Single flag detail — signals, evidence, approve/override |
| `/decisions` | Agent decision log — filterable, expandable rows |
| `/learning` | Agent learning — calibration charts, correctness/override rates, guarantee fund |
| `/alerts` | Alert feed + rule editor |
| `/referrals` | Referral stats + table |

## API Proxy

In dev, Vite proxies `/api/*` to `http://localhost:8080` (the Go backend).
