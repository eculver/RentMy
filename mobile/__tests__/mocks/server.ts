import { setupServer } from 'msw/node';
import { http, HttpResponse } from 'msw';

const BASE_URL = process.env.EXPO_PUBLIC_API_URL ?? 'http://localhost:8080';

export const handlers = [
  http.post(`${BASE_URL}/api/v1/auth/login`, () =>
    HttpResponse.json({
      user: { id: '01JTEST000000000000000000001', name: 'Test User', email: 'test@example.com' },
      accessToken: 'test-access-token',
      refreshToken: 'test-refresh-token',
    }),
  ),
  http.post(`${BASE_URL}/api/v1/auth/register`, () =>
    HttpResponse.json({
      user: { id: '01JTEST000000000000000000001', name: 'Test User', email: 'test@example.com' },
      accessToken: 'test-access-token',
      refreshToken: 'test-refresh-token',
    }),
  ),
  http.post(`${BASE_URL}/api/v1/auth/refresh`, () =>
    HttpResponse.json({
      user: { id: '01JTEST000000000000000000001', name: 'Test User', email: 'test@example.com' },
      accessToken: 'new-access-token',
      refreshToken: 'new-refresh-token',
    }),
  ),
  http.get(`${BASE_URL}/api/v1/discovery/feed`, () =>
    HttpResponse.json({ listings: [], nextCursor: null }),
  ),
];

export const server = setupServer(...handlers);
