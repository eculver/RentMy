import api from './api'

const TOKEN_KEY = 'ops_token'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function isAuthenticated(): boolean {
  return getToken() !== null
}

export async function login(email: string, password: string): Promise<void> {
  const res = await api.post('auth/login', { json: { email, password } }).json<{ token: string }>()
  localStorage.setItem(TOKEN_KEY, res.token)
}

export function logout(): void {
  localStorage.removeItem(TOKEN_KEY)
  window.location.href = '/login'
}
