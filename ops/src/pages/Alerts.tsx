import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { format, parseISO } from 'date-fns'
import api from '../lib/api'
import type { Alert, AlertRule, Severity } from '../lib/types'

function severityBadge(severity: Severity): string {
  if (severity === 'CRITICAL') return 'bg-red-100 text-red-700'
  if (severity === 'WARNING') return 'bg-amber-100 text-amber-700'
  return 'bg-blue-100 text-blue-700'
}

export default function Alerts() {
  const [tab, setTab] = useState<'alerts' | 'rules'>('alerts')
  const [severityFilter, setSeverityFilter] = useState<Severity | ''>('')
  const qc = useQueryClient()

  const { data: alerts } = useQuery({
    queryKey: ['alerts', severityFilter],
    queryFn: () =>
      api.get('ops/alerts', {
        searchParams: {
          ...(severityFilter && { severity: severityFilter }),
          limit: '50',
        },
      }).json<Alert[]>(),
    enabled: tab === 'alerts',
    refetchInterval: 30_000,
  })

  const { data: rules } = useQuery({
    queryKey: ['alert-rules'],
    queryFn: () => api.get('ops/alerts/rules').json<AlertRule[]>(),
    enabled: tab === 'rules',
  })

  const ack = useMutation({
    mutationFn: (alertId: string) => api.put(`ops/alerts/${alertId}/acknowledge`).json(),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['alerts'] }),
  })

  const updateRule = useMutation({
    mutationFn: ({ ruleId, body }: { ruleId: string; body: Partial<AlertRule> }) =>
      api.put(`ops/alerts/rules/${ruleId}`, { json: body }).json(),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['alert-rules'] }),
  })

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-semibold text-gray-900">Alerts</h2>
        <div className="flex gap-1 rounded bg-gray-100 p-0.5">
          <button
            onClick={() => setTab('alerts')}
            className={`px-3 py-1 text-sm rounded cursor-pointer ${tab === 'alerts' ? 'bg-white shadow-sm font-medium' : 'text-gray-600'}`}
          >
            Feed
          </button>
          <button
            onClick={() => setTab('rules')}
            className={`px-3 py-1 text-sm rounded cursor-pointer ${tab === 'rules' ? 'bg-white shadow-sm font-medium' : 'text-gray-600'}`}
          >
            Rules
          </button>
        </div>
      </div>

      {tab === 'alerts' && (
        <>
          <select
            value={severityFilter}
            onChange={(e) => setSeverityFilter(e.target.value as Severity | '')}
            className="rounded border border-gray-300 px-3 py-1.5 text-sm"
          >
            <option value="">All Severities</option>
            <option value="INFO">Info</option>
            <option value="WARNING">Warning</option>
            <option value="CRITICAL">Critical</option>
          </select>

          <div className="space-y-2">
            {alerts?.map((a) => (
              <div key={a.id} className="rounded-lg border border-gray-200 bg-white p-4 flex items-start justify-between">
                <div>
                  <div className="flex items-center gap-2 mb-1">
                    <span className={`rounded px-2 py-0.5 text-xs font-medium ${severityBadge(a.severity)}`}>
                      {a.severity}
                    </span>
                    <span className="text-sm font-medium text-gray-900">{a.metricName}</span>
                  </div>
                  <p className="text-sm text-gray-600">
                    Value: <span className="font-semibold">{a.currentValue.toFixed(2)}</span>
                    {' '}(threshold: {a.threshold.toFixed(2)})
                  </p>
                  <p className="text-xs text-gray-400 mt-1">{format(parseISO(a.firedAt), 'MMM d, yyyy HH:mm')}</p>
                </div>
                <div>
                  {a.acknowledgedAt ? (
                    <span className="text-xs text-green-600">Acknowledged</span>
                  ) : (
                    <button
                      onClick={() => ack.mutate(a.id)}
                      disabled={ack.isPending}
                      className="text-xs text-indigo-600 hover:text-indigo-800 cursor-pointer"
                    >
                      Acknowledge
                    </button>
                  )}
                </div>
              </div>
            ))}
            {alerts?.length === 0 && (
              <p className="text-center text-gray-400 py-8">No alerts.</p>
            )}
          </div>
        </>
      )}

      {tab === 'rules' && (
        <div className="overflow-x-auto">
          <table className="w-full text-sm text-left">
            <thead className="bg-gray-50 text-gray-500 uppercase text-xs">
              <tr>
                <th className="px-4 py-3">Metric</th>
                <th className="px-4 py-3">Operator</th>
                <th className="px-4 py-3">Threshold</th>
                <th className="px-4 py-3">Severity</th>
                <th className="px-4 py-3">Channel</th>
                <th className="px-4 py-3">Enabled</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {rules?.map((r) => (
                <tr key={r.id} className="hover:bg-gray-50">
                  <td className="px-4 py-3">{r.metricName}</td>
                  <td className="px-4 py-3">{r.operator}</td>
                  <td className="px-4 py-3 font-mono">{r.threshold}</td>
                  <td className="px-4 py-3">
                    <span className={`rounded px-2 py-0.5 text-xs font-medium ${severityBadge(r.severity)}`}>
                      {r.severity}
                    </span>
                  </td>
                  <td className="px-4 py-3">{r.channel}</td>
                  <td className="px-4 py-3">
                    <button
                      onClick={() => updateRule.mutate({ ruleId: r.id, body: { enabled: !r.enabled } })}
                      className={`text-xs cursor-pointer ${r.enabled ? 'text-green-600' : 'text-gray-400'}`}
                    >
                      {r.enabled ? 'ON' : 'OFF'}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
