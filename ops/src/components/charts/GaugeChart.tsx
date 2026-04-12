import { PieChart, Pie, Cell, ResponsiveContainer } from 'recharts'

interface Props {
  value: number
  max: number
  title: string
  unit?: string
}

function gaugeColor(pct: number): string {
  if (pct >= 0.7) return '#22c55e'
  if (pct >= 0.4) return '#f59e0b'
  return '#ef4444'
}

export default function GaugeChart({ value, max, title, unit = '' }: Props) {
  const pct = Math.min(value / max, 1)
  const data = [
    { name: 'filled', value: pct },
    { name: 'empty', value: 1 - pct },
  ]
  const color = gaugeColor(pct)

  return (
    <div className="rounded-lg border border-gray-200 bg-white p-4 text-center">
      <h4 className="text-sm font-medium text-gray-600 mb-2">{title}</h4>
      <ResponsiveContainer width="100%" height={140}>
        <PieChart>
          <Pie
            data={data}
            startAngle={180}
            endAngle={0}
            innerRadius="60%"
            outerRadius="80%"
            dataKey="value"
            stroke="none"
          >
            <Cell fill={color} />
            <Cell fill="#f3f4f6" />
          </Pie>
        </PieChart>
      </ResponsiveContainer>
      <p className="text-xl font-semibold -mt-6">
        {value.toLocaleString()}{unit} <span className="text-sm text-gray-400">/ {max.toLocaleString()}{unit}</span>
      </p>
    </div>
  )
}
