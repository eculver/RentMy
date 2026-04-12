import { useState } from 'react'

interface Props {
  onApprove: () => void
  onOverride: (reason: string, notes: string) => void
  onRequestInfo: () => void
  loading?: boolean
}

const overrideReasons = [
  'Incorrect agent assessment',
  'Insufficient evidence',
  'Policy exception',
  'Customer escalation',
  'Other',
]

export default function ActionButtons({ onApprove, onOverride, onRequestInfo, loading }: Props) {
  const [showOverride, setShowOverride] = useState(false)
  const [reason, setReason] = useState(overrideReasons[0])
  const [notes, setNotes] = useState('')

  return (
    <div className="space-y-3">
      <div className="flex gap-2">
        <button
          onClick={onApprove}
          disabled={loading}
          className="px-4 py-2 text-sm font-medium text-white bg-green-600 rounded hover:bg-green-700 disabled:opacity-50 cursor-pointer"
        >
          Approve
        </button>
        <button
          onClick={() => setShowOverride(!showOverride)}
          disabled={loading}
          className="px-4 py-2 text-sm font-medium text-white bg-amber-600 rounded hover:bg-amber-700 disabled:opacity-50 cursor-pointer"
        >
          Override
        </button>
        <button
          onClick={onRequestInfo}
          disabled={loading}
          className="px-4 py-2 text-sm font-medium text-gray-700 bg-gray-100 rounded hover:bg-gray-200 disabled:opacity-50 cursor-pointer"
        >
          Request More Info
        </button>
      </div>

      {showOverride && (
        <div className="rounded border border-gray-200 p-4 bg-gray-50 space-y-3">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Reason</label>
            <select
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              className="w-full rounded border border-gray-300 px-3 py-2 text-sm"
            >
              {overrideReasons.map((r) => (
                <option key={r} value={r}>{r}</option>
              ))}
            </select>
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Notes</label>
            <textarea
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              rows={3}
              className="w-full rounded border border-gray-300 px-3 py-2 text-sm"
              placeholder="Additional context..."
            />
          </div>
          <button
            onClick={() => { onOverride(reason, notes); setShowOverride(false) }}
            disabled={loading}
            className="px-4 py-2 text-sm font-medium text-white bg-amber-600 rounded hover:bg-amber-700 disabled:opacity-50 cursor-pointer"
          >
            Confirm Override
          </button>
        </div>
      )}
    </div>
  )
}
