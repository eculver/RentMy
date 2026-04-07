import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts'
import { format, parseISO } from 'date-fns'

interface DataPoint {
  date: string
  value: number
}

interface Props {
  data: DataPoint[]
  title: string
  color?: string
  valueFormatter?: (v: number) => string
}

export default function TrendChart({ data, title, color = '#6366f1', valueFormatter }: Props) {
  const fmt = valueFormatter ?? ((v: number) => v.toLocaleString())

  return (
    <div className="rounded-lg border border-gray-200 bg-white p-4">
      <h4 className="text-sm font-medium text-gray-600 mb-3">{title}</h4>
      <ResponsiveContainer width="100%" height={200}>
        <LineChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
          <XAxis
            dataKey="date"
            tick={{ fontSize: 11 }}
            tickFormatter={(v: string) => {
              try { return format(parseISO(v), 'MMM d') } catch { return v }
            }}
          />
          <YAxis tick={{ fontSize: 11 }} tickFormatter={fmt} />
          <Tooltip formatter={(v) => fmt(Number(v))} />
          <Line type="monotone" dataKey="value" stroke={color} strokeWidth={2} dot={false} />
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}
