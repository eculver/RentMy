import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router'
import { format, parseISO } from 'date-fns'
import api from '../lib/api'
import type { FraudFlag } from '../lib/types'

type FilterAction = '' | 'MONITOR' | 'FLAG' | 'SUSPEND'
type FilterStatus = '' | 'OPEN' | 'RESOLVED'

function severityBadge(action: string): string {
  if (action === 'SUSPEND') return 'bg-red-100 text-red-700'
  if (action === 'FLAG') return 'bg-amber-100 text-amber-700'
  return 'bg-gray-100 text-gray-600'
}

export default function ReviewQueue() {
  const [status, setStatus] = useState<FilterStatus>('OPEN')
  const [action, setAction] = useState<FilterAction>('')

  const { data: flags, isLoading } = useQuery({
    queryKey: ['fraud', 'flags', status, action],
    queryFn: () =>
      api.get('ops/fraud/flags', {
        searchParams: {
          ...(status && { status }),
          ...(action && { action }),
          limit: '50',
        },
      }).json<FraudFlag[]>(),
  })

  return (
    <div className="space-y-4">
      <h2 className="text-xl font-semibold text-gray-900">Review Queue</h2>

      <div className="flex gap-4 items-center">
        <select
          value={status}
          onChange={(e) => setStatus(e.target.value as FilterStatus)}
          className="rounded border border-gray-300 px-3 py-1.5 text-sm"
        >
          <option value="">All Status</option>
          <option value="OPEN">Open</option>
          <option value="RESOLVED">Resolved</option>
        </select>
        <select
          value={action}
          onChange={(e) => setAction(e.target.value as FilterAction)}
          className="rounded border border-gray-300 px-3 py-1.5 text-sm"
        >
          <option value="">All Actions</option>
          <option value="MONITOR">Monitor</option>
          <option value="FLAG">Flag</option>
          <option value="SUSPEND">Suspend</option>
        </select>
      </div>

      {isLoading && <p className="text-gray-500 text-sm">Loading...</p>}

      <div className="overflow-x-auto">
        <table className="w-full text-sm text-left">
          <thead className="bg-gray-50 text-gray-500 uppercase text-xs">
            <tr>
              <th className="px-4 py-3">Flag ID</th>
              <th className="px-4 py-3">User</th>
              <th className="px-4 py-3">Score</th>
              <th className="px-4 py-3">Action</th>
              <th className="px-4 py-3">Signals</th>
              <th className="px-4 py-3">Created</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {flags?.map((f) => (
              <tr key={f.id} className="hover:bg-gray-50">
                <td className="px-4 py-3">
                  <Link to={`/reviews/${f.id}`} className="text-indigo-600 hover:underline font-mono text-xs">
                    {f.id.slice(0, 10)}...
                  </Link>
                </td>
                <td className="px-4 py-3 font-mono text-xs">{f.userId.slice(0, 10)}...</td>
                <td className="px-4 py-3 font-semibold">{f.totalScore}</td>
                <td className="px-4 py-3">
                  <span className={`inline-block rounded px-2 py-0.5 text-xs font-medium ${severityBadge(f.action)}`}>
                    {f.action}
                  </span>
                </td>
                <td className="px-4 py-3 text-gray-500">{f.signals.length} signals</td>
                <td className="px-4 py-3 text-gray-500">
                  {format(parseISO(f.createdAt), 'MMM d, HH:mm')}
                </td>
              </tr>
            ))}
            {flags?.length === 0 && (
              <tr><td colSpan={6} className="px-4 py-8 text-center text-gray-400">No items found.</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
