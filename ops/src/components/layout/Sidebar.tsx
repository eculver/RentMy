import { NavLink } from 'react-router'

const links = [
  { to: '/', label: 'Dashboard' },
  { to: '/reviews', label: 'Review Queue' },
  { to: '/decisions', label: 'Agent Decisions' },
  { to: '/learning', label: 'Agent Learning' },
  { to: '/alerts', label: 'Alerts' },
  { to: '/fraud', label: 'Fraud Flags' },
  { to: '/referrals', label: 'Referrals' },
]

export default function Sidebar() {
  return (
    <aside className="w-56 shrink-0 border-r border-gray-200 bg-gray-50 p-4 flex flex-col gap-1">
      <h1 className="text-lg font-semibold mb-4 px-2">RentMy Ops</h1>
      <nav className="flex flex-col gap-0.5">
        {links.map((l) => (
          <NavLink
            key={l.to}
            to={l.to}
            end={l.to === '/'}
            className={({ isActive }) =>
              `block rounded px-3 py-2 text-sm ${isActive ? 'bg-indigo-100 text-indigo-700 font-medium' : 'text-gray-700 hover:bg-gray-100'}`
            }
          >
            {l.label}
          </NavLink>
        ))}
      </nav>
    </aside>
  )
}
