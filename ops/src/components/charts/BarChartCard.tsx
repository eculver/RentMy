import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts'

interface DataPoint {
  label: string
  value: number
}

interface Props {
  data: DataPoint[]
  title: string
  color?: string
  valueFormatter?: (v: number) => string
}

export default function BarChartCard({ data, title, color = '#6366f1', valueFormatter }: Props) {
  const fmt = valueFormatter ?? ((v: number) => `${v}`)

  return (
    <div className="rounded-lg border border-gray-200 bg-white p-4">
      <h4 className="text-sm font-medium text-gray-600 mb-3">{title}</h4>
      <ResponsiveContainer width="100%" height={200}>
        <BarChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
          <XAxis dataKey="label" tick={{ fontSize: 11 }} />
          <YAxis tick={{ fontSize: 11 }} tickFormatter={fmt} />
          <Tooltip formatter={(v) => fmt(Number(v))} />
          <Bar dataKey="value" fill={color} radius={[4, 4, 0, 0]} />
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}
