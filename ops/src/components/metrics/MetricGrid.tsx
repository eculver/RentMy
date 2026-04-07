import type { MetricValue } from '../../lib/types'
import MetricCard from './MetricCard'

interface Props {
  title: string
  metrics: MetricValue[]
}

export default function MetricGrid({ title, metrics }: Props) {
  return (
    <section className="mb-6">
      <h3 className="text-sm font-medium text-gray-500 uppercase tracking-wider mb-3">{title}</h3>
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
        {metrics.map((m) => (
          <MetricCard key={m.name} metric={m} />
        ))}
      </div>
    </section>
  )
}
