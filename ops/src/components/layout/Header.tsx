import { logout } from '../../lib/auth'

export default function Header() {
  return (
    <header className="flex items-center justify-between border-b border-gray-200 bg-white px-6 py-3">
      <span className="text-sm text-gray-500">Internal Operations Dashboard</span>
      <button
        onClick={logout}
        className="text-sm text-gray-600 hover:text-gray-900 cursor-pointer"
      >
        Log out
      </button>
    </header>
  )
}
