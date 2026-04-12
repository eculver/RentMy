import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import api from '../lib/api'
import type { CalibrationBucket } from '../lib/types'
import CalibrationChart from '../components/charts/CalibrationChart'
import BarChartCard from '../components/charts/BarChartCard'
import GaugeChart from '../components/charts/GaugeChart'

const agentTypes = [
  'RISK', 'VERIFICATION', 'APPRAISAL', 'DISPUTE', 'AGREEMENT', 'FRAUD', 'LATE_RETURN',
]

interface AgentMetrics {
  agentType: string
  correctnessRate: number
  overrideRate: number
  totalDecisions: number
  primaryMetric: string
  primaryValue: number
  primaryThreshold: number
  primaryStatus: string
  secondaryMetric: string
  secondaryValue: number
  secondaryThreshold: number
  secondaryStatus: string
}

interface FundHealth {
  currentBalance: number
  reserveRequired: number
  lossRatio: number
  contributions: number
  claims: number
}

export default function AgentLearning() {
  const [selected, setSelected] = useState(agentTypes[0])

  const { data: calibration } = useQuery({
    queryKey: ['agents', 'calibration', selected],
    queryFn: () =>
      api.get('ops/agents/calibration', { searchParams: { agent_type: selected } }).json<CalibrationBucket[]>(),
  })

  const { data: metrics } = useQuery({
    queryKey: ['agents', 'metrics'],
    queryFn: () => api.get('ops/agents/metrics').json<AgentMetrics[]>(),
  })

  const { data: fund } = useQuery({
    queryKey: ['guarantee', 'health'],
    queryFn: () => api.get('ops/guarantee/health').json<FundHealth>(),
  })

  const correctnessData = metrics?.map((m) => ({
    label: m.agentType,
    value: m.correctnessRate * 100,
  })) ?? []

  const overrideData = metrics?.map((m) => ({
    label: m.agentType,
    value: m.overrideRate * 100,
  })) ?? []

  const selectedMetrics = metrics?.find((m) => m.agentType === selected)

  function statusBadge(status: string): string {
    if (status === 'OK') return 'bg-green-100 text-green-700'
    if (status === 'WARNING') return 'bg-amber-100 text-amber-700'
    return 'bg-red-100 text-red-700'
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-semibold text-gray-900">Agent Learning</h2>
        <select
          value={selected}
          onChange={(e) => setSelected(e.target.value)}
          className="rounded border border-gray-300 px-3 py-1.5 text-sm"
        >
          {agentTypes.map((t) => (
            <option key={t} value={t}>{t}</option>
          ))}
        </select>
      </div>

      {calibration && calibration.length > 0 && (
        <CalibrationChart buckets={calibration} agentType={selected} />
      )}

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <BarChartCard
          data={correctnessData}
          title="Outcome Correctness Rate (90d)"
          color="#22c55e"
          valueFormatter={(v) => `${v.toFixed(0)}%`}
        />
        <BarChartCard
          data={overrideData}
          title="Human Override Rate"
          color="#f59e0b"
          valueFormatter={(v) => `${v.toFixed(1)}%`}
        />
      </div>

      {selectedMetrics && (
        <div>
          <h3 className="text-sm font-medium text-gray-500 uppercase tracking-wider mb-3">
            Evaluation Metrics — {selected}
          </h3>
          <table className="w-full text-sm text-left">
            <thead className="bg-gray-50 text-gray-500 uppercase text-xs">
              <tr>
                <th className="px-4 py-3">Metric</th>
                <th className="px-4 py-3">Value</th>
                <th className="px-4 py-3">Threshold</th>
                <th className="px-4 py-3">Status</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              <tr>
                <td className="px-4 py-3">{selectedMetrics.primaryMetric}</td>
                <td className="px-4 py-3 font-semibold">{selectedMetrics.primaryValue.toFixed(2)}</td>
                <td className="px-4 py-3 text-gray-500">{selectedMetrics.primaryThreshold.toFixed(2)}</td>
                <td className="px-4 py-3">
                  <span className={`rounded px-2 py-0.5 text-xs font-medium ${statusBadge(selectedMetrics.primaryStatus)}`}>
                    {selectedMetrics.primaryStatus}
                  </span>
                </td>
              </tr>
              <tr>
                <td className="px-4 py-3">{selectedMetrics.secondaryMetric}</td>
                <td className="px-4 py-3 font-semibold">{selectedMetrics.secondaryValue.toFixed(2)}</td>
                <td className="px-4 py-3 text-gray-500">{selectedMetrics.secondaryThreshold.toFixed(2)}</td>
                <td className="px-4 py-3">
                  <span className={`rounded px-2 py-0.5 text-xs font-medium ${statusBadge(selectedMetrics.secondaryStatus)}`}>
                    {selectedMetrics.secondaryStatus}
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      )}

      {fund && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <GaugeChart
            value={fund.currentBalance / 100}
            max={fund.reserveRequired / 100}
            title="Guarantee Fund Health"
            unit="$"
          />
          <GaugeChart
            value={1 - fund.lossRatio}
            max={1}
            title="Loss Ratio (lower = better)"
          />
        </div>
      )}
    </div>
  )
}
