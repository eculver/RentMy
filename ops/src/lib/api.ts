import ky from 'ky'
import type { BeforeRequestHook, AfterResponseHook } from 'ky'

const addAuth: BeforeRequestHook = ({ request }) => {
  const token = localStorage.getItem('ops_token')
  if (token) {
    request.headers.set('Authorization', `Bearer ${token}`)
  }
}

const handle401: AfterResponseHook = ({ response }) => {
  if (response.status === 401) {
    localStorage.removeItem('ops_token')
    window.location.href = '/login'
  }
}

const api = ky.create({
  prefix: '/api/v1',
  hooks: {
    beforeRequest: [addAuth],
    afterResponse: [handle401],
  },
})

export default api
