import { useQuery } from '@tanstack/react-query'
import { format, parseISO } from 'date-fns'
import api from '../lib/api'
import type { Referral } from '../lib/types'

interface ReferralStats {
  totalCodes: number
  totalConversions: number
  totalPayoutsCents: number
  conversionRate: number
}

function statusColor(status: string): string {
  if (status === 'PAID') return 'text-green-600'
  if (status === 'FRAUDULENT') return 'text-red-600'
  if (status === 'FIRST_RENTAL_COMPLETED') return 'text-blue-600'
  return 'text-gray-500'
}

export default function Referrals() {
  const { data: stats } = useQuery({
    queryKey: ['referrals', 'stats'],
    queryFn: () => api.get('ops/referrals/stats').json<ReferralStats>(),
  })

  const { data: referrals } = useQuery({
    queryKey: ['referrals', 'list'],
    queryFn: () => api.get('ops/referrals', { searchParams: { limit: '50' } }).json<Referral[]>(),
  })

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-semibold text-gray-900">Referrals</h2>

      {stats && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <div className="rounded-lg border border-gray-200 bg-white p-4">
            <p className="text-xs text-gray-500">Total Codes</p>
            <p className="text-2xl font-semibold">{stats.totalCodes.toLocaleString()}</p>
          </div>
          <div className="rounded-lg border border-gray-200 bg-white p-4">
            <p className="text-xs text-gray-500">Conversions</p>
            <p className="text-2xl font-semibold">{stats.totalConversions.toLocaleString()}</p>
          </div>
          <div className="rounded-lg border border-gray-200 bg-white p-4">
            <p className="text-xs text-gray-500">Total Payouts</p>
            <p className="text-2xl font-semibold">${(stats.totalPayoutsCents / 100).toLocaleString()}</p>
          </div>
          <div className="rounded-lg border border-gray-200 bg-white p-4">
            <p className="text-xs text-gray-500">Conversion Rate</p>
            <p className="text-2xl font-semibold">{(stats.conversionRate * 100).toFixed(1)}%</p>
          </div>
        </div>
      )}

      <div className="overflow-x-auto">
        <table className="w-full text-sm text-left">
          <thead className="bg-gray-50 text-gray-500 uppercase text-xs">
            <tr>
              <th className="px-4 py-3">Referrer</th>
              <th className="px-4 py-3">Referee</th>
              <th className="px-4 py-3">Code</th>
              <th className="px-4 py-3">Status</th>
              <th className="px-4 py-3">Created</th>
              <th className="px-4 py-3">Completed</th>
              <th className="px-4 py-3">Payout</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {referrals?.map((r) => (
              <tr key={r.id} className="hover:bg-gray-50">
                <td className="px-4 py-3">{r.referrerName}</td>
                <td className="px-4 py-3">{r.refereeName}</td>
                <td className="px-4 py-3 font-mono text-xs">{r.code}</td>
                <td className={`px-4 py-3 font-medium ${statusColor(r.status)}`}>{r.status}</td>
                <td className="px-4 py-3 text-gray-500">{format(parseISO(r.createdAt), 'MMM d, yyyy')}</td>
                <td className="px-4 py-3 text-gray-500">
                  {r.completedAt ? format(parseISO(r.completedAt), 'MMM d, yyyy') : '-'}
                </td>
                <td className="px-4 py-3">
                  {r.payoutAmount != null ? `$${(r.payoutAmount / 100).toFixed(2)}` : '-'}
                </td>
              </tr>
            ))}
            {referrals?.length === 0 && (
              <tr><td colSpan={7} className="px-4 py-8 text-center text-gray-400">No referrals yet.</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
