import React from 'react';
import { screen, fireEvent, waitFor } from '@testing-library/react-native';
import { http, HttpResponse } from 'msw';
import { server } from '../mocks/server';
import { renderWithProviders } from '../helpers/renderWithProviders';
import { useAuthStore } from '../../lib/auth';

const BASE_URL = process.env.EXPO_PUBLIC_API_URL ?? 'http://localhost:8080';

// ── mocks ────────────────────────────────────────────────────────────────────

const mockPush = jest.fn();
jest.mock('expo-router', () => ({
  router: { push: mockPush, back: jest.fn(), replace: jest.fn() },
  useRouter: () => ({ push: mockPush }),
  useLocalSearchParams: () => ({}),
}));

// Silence native Clipboard module warning in test environment.
jest.mock('react-native/Libraries/Components/Clipboard/NativeClipboard', () => null, { virtual: true });

jest.mock('react-native/Libraries/Share/Share', () => ({
  share: jest.fn().mockResolvedValue({ action: 'sharedAction' }),
}));

const mockReferralCode = {
  id: 'rc-01',
  code: 'TESTCODE',
  userId: 'user-01',
  maxUses: 0,
  useCount: 2,
  createdAt: '2026-04-01T00:00:00Z',
};

const mockReferral = {
  id: 'ref-01',
  referralCodeId: 'rc-01',
  referrerId: 'user-01',
  refereeId: 'user-02',
  status: 'SIGNED_UP' as const,
  referrerPayout: 0,
  refereePayout: 0,
  createdAt: '2026-04-02T00:00:00Z',
};

// ── lazy imports (after mocks) ────────────────────────────────────────────────
// eslint-disable-next-line @typescript-eslint/no-var-requires
const ReferralsScreen = require('../../app/(tabs)/(profile)/referrals').default as React.ComponentType;

// ── setup ────────────────────────────────────────────────────────────────────

beforeEach(() => {
  mockPush.mockClear();
  useAuthStore.setState({
    token: 'test-token',
    refreshToken: 'test-refresh',
    user: { id: 'user-01', name: 'Test User', email: 'test@example.com' },
    isAuthenticated: true,
    isLoading: false,
  });

  // Default MSW handlers for referral endpoints.
  server.use(
    http.get(`${BASE_URL}/api/v1/referrals/code`, () =>
      HttpResponse.json(mockReferralCode),
    ),
    http.get(`${BASE_URL}/api/v1/referrals/mine`, () =>
      HttpResponse.json({ referrals: [mockReferral], page: 1, limit: 20 }),
    ),
  );
});

// ── tests ─────────────────────────────────────────────────────────────────────

describe('ReferralsScreen', () => {
  it('renders the invite header', () => {
    renderWithProviders(<ReferralsScreen />);
    expect(screen.getByText('Invite Friends')).toBeTruthy();
  });

  it('shows the referral code after loading', async () => {
    renderWithProviders(<ReferralsScreen />);
    await waitFor(() =>
      expect(screen.getByText('TESTCODE')).toBeTruthy(),
    );
  });

  it('renders Copy Code and Share buttons', async () => {
    renderWithProviders(<ReferralsScreen />);
    await waitFor(() => screen.getByText('Copy Code'));
    expect(screen.getByText('Share')).toBeTruthy();
  });

  it('shows a referral card for each referral', async () => {
    renderWithProviders(<ReferralsScreen />);
    await waitFor(() =>
      expect(screen.getByText('Signed Up')).toBeTruthy(),
    );
  });

  it('shows empty state when there are no referrals', async () => {
    server.use(
      http.get(`${BASE_URL}/api/v1/referrals/mine`, () =>
        HttpResponse.json({ referrals: [], page: 1, limit: 20 }),
      ),
    );
    renderWithProviders(<ReferralsScreen />);
    await waitFor(() =>
      expect(screen.getByText('No referrals yet')).toBeTruthy(),
    );
  });

  it('shows "Copied!" feedback after pressing Copy Code', async () => {
    renderWithProviders(<ReferralsScreen />);
    await waitFor(() => screen.getByText('Copy Code'));
    fireEvent.press(screen.getByText('Copy Code'));
    await waitFor(() =>
      expect(screen.getByText('Copied!')).toBeTruthy(),
    );
  });

  it('auto-generates code when none exists', async () => {
    server.use(
      http.get(`${BASE_URL}/api/v1/referrals/code`, () =>
        new HttpResponse(null, { status: 404 }),
      ),
      http.post(`${BASE_URL}/api/v1/referrals/code`, () =>
        HttpResponse.json({ ...mockReferralCode, code: 'NEWCODE01' }),
      ),
    );
    renderWithProviders(<ReferralsScreen />);
    await waitFor(() =>
      expect(screen.getByText('NEWCODE01')).toBeTruthy(),
    );
  });
});
