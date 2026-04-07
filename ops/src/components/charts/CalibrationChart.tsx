import {
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Area,
  ResponsiveContainer,
  ComposedChart,
} from 'recharts'
import type { CalibrationBucket } from '../../lib/types'

interface Props {
  buckets: CalibrationBucket[]
  agentType: string
}

export default function CalibrationChart({ buckets, agentType }: Props) {
  const data = buckets.map((b) => ({
    label: `${(b.bucketMin * 100).toFixed(0)}-${(b.bucketMax * 100).toFixed(0)}%`,
    expected: b.expectedAccuracy * 100,
    actual: b.actualAccuracy * 100,
    count: b.decisionCount,
    correct: b.correctCount,
  }))

  return (
    <div className="rounded-lg border border-gray-200 bg-white p-4">
      <h4 className="text-sm font-medium text-gray-600 mb-3">
        Confidence Calibration — {agentType}
      </h4>
      <ResponsiveContainer width="100%" height={280}>
        <ComposedChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
          <XAxis dataKey="label" tick={{ fontSize: 11 }} />
          <YAxis
            tick={{ fontSize: 11 }}
            domain={[0, 100]}
            tickFormatter={(v: number) => `${v}%`}
          />
          <Tooltip
            formatter={(v, name) => [`${Number(v).toFixed(1)}%`, name]}
            labelFormatter={(label) => `Confidence: ${label}`}
          />
          <Area
            type="monotone"
            dataKey="expected"
            fill="#e0e7ff"
            stroke="none"
            fillOpacity={0.3}
          />
          <Line
            type="monotone"
            dataKey="expected"
            stroke="#6366f1"
            strokeWidth={2}
            strokeDasharray="5 5"
            name="Expected"
            dot={false}
          />
          <Line
            type="monotone"
            dataKey="actual"
            stroke="#f97316"
            strokeWidth={2}
            name="Actual"
          />
        </ComposedChart>
      </ResponsiveContainer>
      <p className="text-xs text-gray-400 mt-2">
        Dashed = expected (perfect calibration). Orange = actual accuracy.
      </p>
    </div>
  )
}
