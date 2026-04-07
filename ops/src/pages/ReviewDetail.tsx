import { useParams } from 'react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { format, parseISO } from 'date-fns'
import api from '../lib/api'
import type { FraudFlag } from '../lib/types'
import EvidenceViewer from '../components/review/EvidenceViewer'
import ActionButtons from '../components/review/ActionButtons'

export default function ReviewDetail() {
  const { flagId } = useParams<{ flagId: string }>()
  const qc = useQueryClient()

  const { data: flag, isLoading } = useQuery({
    queryKey: ['fraud', 'flag', flagId],
    queryFn: () => api.get(`ops/fraud/flags/${flagId}`).json<FraudFlag>(),
    enabled: !!flagId,
  })

  const resolve = useMutation({
    mutationFn: (body: { outcome: string; notes: string }) =>
      api.put(`ops/fraud/flags/${flagId}/resolve`, { json: body }).json(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['fraud'] })
    },
  })

  if (isLoading) return <p className="p-8 text-gray-500">Loading...</p>
  if (!flag) return <p className="p-8 text-red-500">Flag not found.</p>

  const isResolved = !!flag.resolvedAt

  return (
    <div className="space-y-6 max-w-4xl">
      <div>
        <h2 className="text-xl font-semibold text-gray-900">Fraud Flag Detail</h2>
        <p className="text-sm text-gray-500 font-mono mt-1">{flag.id}</p>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div>
          <p className="text-xs text-gray-500">User</p>
          <p className="text-sm font-mono">{flag.userId.slice(0, 16)}...</p>
        </div>
        <div>
          <p className="text-xs text-gray-500">Score</p>
          <p className="text-sm font-semibold">{flag.totalScore}</p>
        </div>
        <div>
          <p className="text-xs text-gray-500">Action</p>
          <p className="text-sm font-medium">{flag.action}</p>
        </div>
        <div>
          <p className="text-xs text-gray-500">Created</p>
          <p className="text-sm">{format(parseISO(flag.createdAt), 'MMM d, yyyy HH:mm')}</p>
        </div>
      </div>

      <div>
        <h3 className="text-sm font-medium text-gray-700 mb-2">Signals ({flag.signals.length})</h3>
        <div className="space-y-2">
          {flag.signals.map((s, i) => (
            <div key={i} className="rounded border border-gray-200 bg-gray-50 p-3 text-sm">
              <div className="flex items-center justify-between">
                <span className="font-medium">{s.type}</span>
                <span className="text-gray-500">+{s.score} pts</span>
              </div>
              {s.relatedUserId && (
                <p className="text-xs text-gray-400 mt-1">Related: {s.relatedUserId}</p>
              )}
              {s.isCompoundOnly && (
                <span className="text-xs text-amber-600">Compound-only</span>
              )}
            </div>
          ))}
        </div>
      </div>

      <EvidenceViewer decisionJson={flag} />

      {isResolved ? (
        <div className="rounded border border-green-200 bg-green-50 p-4">
          <p className="text-sm font-medium text-green-800">
            Resolved{flag.resolvedBy ? ` by ${flag.resolvedBy}` : ''} on{' '}
            {format(parseISO(flag.resolvedAt!), 'MMM d, yyyy HH:mm')}
          </p>
          {flag.resolutionNotes && <p className="text-sm text-green-700 mt-1">{flag.resolutionNotes}</p>}
        </div>
      ) : (
        <ActionButtons
          loading={resolve.isPending}
          onApprove={() => resolve.mutate({ outcome: 'CONFIRMED', notes: 'Approved by ops' })}
          onOverride={(reason, notes) => resolve.mutate({ outcome: 'FALSE_POSITIVE', notes: `${reason}: ${notes}` })}
          onRequestInfo={() => resolve.mutate({ outcome: 'PENDING_INFO', notes: 'More information requested' })}
        />
      )}
    </div>
  )
}
