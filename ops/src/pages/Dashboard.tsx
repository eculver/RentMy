import { useQuery } from '@tanstack/react-query'
import api from '../lib/api'
import type { HealthSnapshot, MetricValue } from '../lib/types'
import MetricGrid from '../components/metrics/MetricGrid'
import TrendChart from '../components/charts/TrendChart'

function metricsToArray(obj: Record<string, MetricValue>): MetricValue[] {
  return Object.values(obj)
}

export default function Dashboard() {
  const { data: snapshot, isLoading, error } = useQuery({
    queryKey: ['metrics', 'current'],
    queryFn: () => api.get('ops/metrics/current').json<HealthSnapshot>(),
    refetchInterval: 60_000,
  })

  const { data: history } = useQuery({
    queryKey: ['metrics', 'history'],
    queryFn: () => api.get('ops/metrics/history', { searchParams: { duration: '7d' } }).json<HealthSnapshot[]>(),
    refetchInterval: 120_000,
  })

  if (isLoading) return <p className="p-8 text-gray-500">Loading metrics...</p>
  if (error) return <p className="p-8 text-red-500">Failed to load metrics.</p>
  if (!snapshot) return null

  const revenueTrend = history?.map((s) => ({
    date: s.capturedAt,
    value: s.business.grossRevenueCents.value / 100,
  })) ?? []

  const fraudTrend = history?.map((s) => ({
    date: s.capturedAt,
    value: s.trust.fraudFlagRate.value * 100,
  })) ?? []

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-semibold text-gray-900">Platform Dashboard</h2>

      {snapshot.anomalies.length > 0 && (
        <div className="rounded-lg border border-amber-300 bg-amber-50 p-4">
          <h3 className="text-sm font-medium text-amber-800 mb-1">Anomalies Detected</h3>
          <ul className="list-disc list-inside text-sm text-amber-700">
            {snapshot.anomalies.map((a) => <li key={a}>{a}</li>)}
          </ul>
        </div>
      )}

      <MetricGrid title="Business" metrics={metricsToArray(snapshot.business as unknown as Record<string, MetricValue>)} />
      <MetricGrid title="Trust & Safety" metrics={metricsToArray(snapshot.trust as unknown as Record<string, MetricValue>)} />
      <MetricGrid title="Supply" metrics={metricsToArray(snapshot.supply as unknown as Record<string, MetricValue>)} />
      <MetricGrid title="Demand" metrics={metricsToArray(snapshot.demand as unknown as Record<string, MetricValue>)} />

      {history && history.length > 1 && (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          <TrendChart
            data={revenueTrend}
            title="Gross Revenue (7d)"
            color="#22c55e"
            valueFormatter={(v) => `$${v.toLocaleString()}`}
          />
          <TrendChart
            data={fraudTrend}
            title="Fraud Flag Rate (7d)"
            color="#ef4444"
            valueFormatter={(v) => `${v.toFixed(2)}%`}
          />
        </div>
      )}
    </div>
  )
}
