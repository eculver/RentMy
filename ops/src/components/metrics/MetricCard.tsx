import type { MetricValue } from '../../lib/types'

function trendArrow(trend: string): string {
  if (trend === 'up') return '\u2191'
  if (trend === 'down') return '\u2193'
  return '\u2192'
}

function trendColor(trend: string): string {
  if (trend === 'up') return 'text-green-600'
  if (trend === 'down') return 'text-red-600'
  return 'text-gray-400'
}

function formatValue(name: string, value: number): string {
  if (name.toLowerCase().includes('cents')) {
    return `$${(value / 100).toLocaleString(undefined, { minimumFractionDigits: 2 })}`
  }
  if (name.toLowerCase().includes('rate') || name.toLowerCase().includes('conversion')) {
    return `${(value * 100).toFixed(1)}%`
  }
  if (name.toLowerCase().includes('hours')) {
    return `${value.toFixed(1)}h`
  }
  return value.toLocaleString()
}

function pctChange(curr: number, prev: number): string {
  if (prev === 0) return 'N/A'
  const pct = ((curr - prev) / prev) * 100
  return `${pct >= 0 ? '+' : ''}${pct.toFixed(1)}%`
}

function humanLabel(name: string): string {
  return name
    .replace(/([A-Z])/g, ' $1')
    .replace(/^./, (s) => s.toUpperCase())
    .replace(/Cents$/, '')
    .replace(/7d$/, '(7d)')
    .trim()
}

interface Props {
  metric: MetricValue
}

export default function MetricCard({ metric }: Props) {
  return (
    <div className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm">
      <p className="text-xs text-gray-500 mb-1">{humanLabel(metric.name)}</p>
      <p className="text-2xl font-semibold text-gray-900">
        {formatValue(metric.name, metric.value)}
      </p>
      <div className="mt-1 flex items-center gap-2 text-sm">
        <span className={trendColor(metric.trend)}>
          {trendArrow(metric.trend)} {pctChange(metric.value, metric.previousValue)}
        </span>
        <span className="text-gray-400">vs prev {metric.period}</span>
      </div>
    </div>
  )
}
