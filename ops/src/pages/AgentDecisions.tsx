import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { format, parseISO } from 'date-fns'
import api from '../lib/api'
import type { AgentDecision } from '../lib/types'

const agentTypes = [
  '', 'RISK', 'VERIFICATION', 'APPRAISAL', 'DISPUTE', 'AGREEMENT', 'FRAUD', 'LATE_RETURN',
]

export default function AgentDecisions() {
  const [agentType, setAgentType] = useState('')
  const [escalatedOnly, setEscalatedOnly] = useState(false)
  const [expanded, setExpanded] = useState<string | null>(null)

  const { data: decisions, isLoading } = useQuery({
    queryKey: ['decisions', agentType, escalatedOnly],
    queryFn: () =>
      api.get('ops/agents/decisions', {
        searchParams: {
          ...(agentType && { agent_type: agentType }),
          ...(escalatedOnly && { escalated: 'true' }),
          limit: '50',
        },
      }).json<AgentDecision[]>(),
  })

  return (
    <div className="space-y-4">
      <h2 className="text-xl font-semibold text-gray-900">Agent Decisions</h2>

      <div className="flex gap-4 items-center">
        <select
          value={agentType}
          onChange={(e) => setAgentType(e.target.value)}
          className="rounded border border-gray-300 px-3 py-1.5 text-sm"
        >
          <option value="">All Agents</option>
          {agentTypes.filter(Boolean).map((t) => (
            <option key={t} value={t}>{t}</option>
          ))}
        </select>
        <label className="flex items-center gap-1.5 text-sm text-gray-600">
          <input
            type="checkbox"
            checked={escalatedOnly}
            onChange={(e) => setEscalatedOnly(e.target.checked)}
          />
          Escalated only
        </label>
      </div>

      {isLoading && <p className="text-gray-500 text-sm">Loading...</p>}

      <div className="overflow-x-auto">
        <table className="w-full text-sm text-left">
          <thead className="bg-gray-50 text-gray-500 uppercase text-xs">
            <tr>
              <th className="px-4 py-3">ID</th>
              <th className="px-4 py-3">Agent</th>
              <th className="px-4 py-3">Transaction</th>
              <th className="px-4 py-3">Confidence</th>
              <th className="px-4 py-3">Escalated</th>
              <th className="px-4 py-3">Outcome</th>
              <th className="px-4 py-3">Created</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {decisions?.map((d) => (
              <tr
                key={d.id}
                className="hover:bg-gray-50 cursor-pointer"
                onClick={() => setExpanded(expanded === d.id ? null : d.id)}
              >
                <td className="px-4 py-3 font-mono text-xs">{d.id.slice(0, 10)}...</td>
                <td className="px-4 py-3">{d.agentType}</td>
                <td className="px-4 py-3 font-mono text-xs">{d.transactionId.slice(0, 10)}...</td>
                <td className="px-4 py-3">{(d.confidence * 100).toFixed(0)}%</td>
                <td className="px-4 py-3">{d.escalated ? 'Yes' : 'No'}</td>
                <td className="px-4 py-3">
                  {d.outcomeCorrect === true && <span className="text-green-600">Correct</span>}
                  {d.outcomeCorrect === false && <span className="text-red-600">Incorrect</span>}
                  {d.outcomeCorrect == null && <span className="text-gray-400">Pending</span>}
                </td>
                <td className="px-4 py-3 text-gray-500">
                  {format(parseISO(d.createdAt), 'MMM d, HH:mm')}
                </td>
              </tr>
            ))}
          </tbody>
        </table>

        {expanded && decisions && (() => {
          const d = decisions.find((x) => x.id === expanded)
          if (!d) return null
          return (
            <div className="border border-gray-200 rounded bg-gray-50 p-4 mt-2 text-sm">
              {d.model && <p><span className="font-medium">Model:</span> {d.model}</p>}
              {d.promptVersion && <p><span className="font-medium">Prompt Version:</span> {d.promptVersion}</p>}
              {d.reasoning && <p className="mt-2"><span className="font-medium">Reasoning:</span> {d.reasoning}</p>}
              {d.input != null && (
                <details className="mt-2">
                  <summary className="cursor-pointer text-indigo-600">Input JSON</summary>
                  <pre className="mt-1 text-xs overflow-auto max-h-48">{JSON.stringify(d.input, null, 2)}</pre>
                </details>
              )}
              {d.decision != null && (
                <details className="mt-2">
                  <summary className="cursor-pointer text-indigo-600">Decision JSON</summary>
                  <pre className="mt-1 text-xs overflow-auto max-h-48">{JSON.stringify(d.decision, null, 2)}</pre>
                </details>
              )}
            </div>
          )
        })()}
      </div>
    </div>
  )
}
