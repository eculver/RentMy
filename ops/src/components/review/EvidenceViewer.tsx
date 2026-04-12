import { useState } from 'react'

interface Props {
  photos?: { checkin?: string; checkout?: string }
  decisionJson?: unknown
}

export default function EvidenceViewer({ photos, decisionJson }: Props) {
  const [jsonExpanded, setJsonExpanded] = useState(false)

  return (
    <div className="space-y-4">
      {photos && (photos.checkin || photos.checkout) && (
        <div>
          <h4 className="text-sm font-medium text-gray-600 mb-2">Photos</h4>
          <div className="grid grid-cols-2 gap-4">
            {photos.checkin && (
              <div>
                <p className="text-xs text-gray-400 mb-1">Check-in</p>
                <img src={photos.checkin} alt="Check-in" className="rounded border border-gray-200 w-full object-cover" />
              </div>
            )}
            {photos.checkout && (
              <div>
                <p className="text-xs text-gray-400 mb-1">Check-out</p>
                <img src={photos.checkout} alt="Check-out" className="rounded border border-gray-200 w-full object-cover" />
              </div>
            )}
          </div>
        </div>
      )}

      {decisionJson !== undefined && (
        <div>
          <button
            onClick={() => setJsonExpanded(!jsonExpanded)}
            className="text-sm text-indigo-600 hover:text-indigo-800 cursor-pointer"
          >
            {jsonExpanded ? 'Hide' : 'Show'} Agent Decision JSON
          </button>
          {jsonExpanded && (
            <pre className="mt-2 rounded bg-gray-50 p-3 text-xs overflow-auto max-h-80 border border-gray-200">
              {JSON.stringify(decisionJson, null, 2)}
            </pre>
          )}
        </div>
      )}
    </div>
  )
}
