import { http, HttpResponse } from 'msw';
import { server } from '../mocks/server';
import { api } from '../../lib/api';
import { useAuthStore } from '../../lib/auth';

beforeEach(() => {
  // Reset store to unauthenticated state before each test.
  useAuthStore.setState({
    token: null,
    refreshToken: null,
    user: null,
    isAuthenticated: false,
    isLoading: false,
  });
});

describe('api client — Authorization header injection', () => {
  it('does not add Authorization header when no token is set', async () => {
    let capturedHeader: string | null = undefined as unknown as string | null;
    server.use(
      http.get('http://localhost:8080/api/v1/probe', ({ request }) => {
        capturedHeader = request.headers.get('Authorization');
        return HttpResponse.json({ ok: true });
      }),
    );

    await api.get('api/v1/probe').json();
    expect(capturedHeader).toBeNull();
  });

  it('adds Bearer token when store has a token', async () => {
    useAuthStore.setState({ token: 'my-test-jwt' });

    let capturedHeader: string | null = null;
    server.use(
      http.get('http://localhost:8080/api/v1/probe', ({ request }) => {
        capturedHeader = request.headers.get('Authorization');
        return HttpResponse.json({ ok: true });
      }),
    );

    await api.get('api/v1/probe').json();
    expect(capturedHeader).toBe('Bearer my-test-jwt');
  });
});

describe('api client — 401 handling', () => {
  it('calls refreshTokens when the server returns 401', async () => {
    const refreshMock = jest.fn().mockResolvedValue(true);
    useAuthStore.setState(s => ({ ...s, token: 'expired', refreshTokens: refreshMock }));

    server.use(
      http.get('http://localhost:8080/api/v1/probe', () => new HttpResponse(null, { status: 401 })),
    );

    // ky throws on non-2xx; we only care that refreshTokens was called.
    await api.get('api/v1/probe').json().catch(() => undefined);
    expect(refreshMock).toHaveBeenCalled();
  });

  it('calls logout when refresh fails on 401', async () => {
    const refreshMock = jest.fn().mockResolvedValue(false);
    const logoutMock = jest.fn().mockResolvedValue(undefined);
    useAuthStore.setState(s => ({ ...s, token: 'expired', refreshTokens: refreshMock, logout: logoutMock }));

    server.use(
      http.get('http://localhost:8080/api/v1/probe', () => new HttpResponse(null, { status: 401 })),
    );

    await api.get('api/v1/probe').json().catch(() => undefined);
    expect(refreshMock).toHaveBeenCalled();
    expect(logoutMock).toHaveBeenCalled();
  });
});
