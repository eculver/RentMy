import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react-native';
import { http, HttpResponse } from 'msw';
import { server } from '../mocks/server';
import { renderWithProviders } from '../helpers/renderWithProviders';
import { useLocationStore } from '../../lib/stores/locationStore';
import { useSearchStore } from '../../lib/stores/searchStore';

// ── module mocks ─────────────────────────────────────────────────────────────
const mockPush = jest.fn();
jest.mock('expo-router', () => ({
  router: { push: mockPush, replace: jest.fn(), back: jest.fn() },
  useRouter: () => ({ push: mockPush, replace: jest.fn(), back: jest.fn() }),
  useLocalSearchParams: () => ({}),
}));

jest.mock('@expo/vector-icons', () => ({
  Ionicons: () => null,
}));

jest.mock('@gorhom/bottom-sheet', () => {
  const React = require('react');
  const BottomSheet = React.forwardRef(
    ({ children }: { children?: React.ReactNode }, ref: React.Ref<unknown>) => {
      React.useImperativeHandle(ref, () => ({
        expand: jest.fn(),
        close: jest.fn(),
        snapToIndex: jest.fn(),
      }));
      return children ?? null;
    },
  );
  BottomSheet.displayName = 'BottomSheet';
  return { __esModule: true, default: BottomSheet };
});

jest.mock('../../components/search/FilterSheet', () => {
  const React = require('react');
  return React.forwardRef(() => null);
});

// eslint-disable-next-line @typescript-eslint/no-var-requires
const SearchScreen = require('../../app/(tabs)/(search)/index').default as React.ComponentType;

const BASE_URL = process.env.EXPO_PUBLIC_API_URL ?? 'http://localhost:8080';

const testListing = {
  id: '01JTEST200000000000000000001',
  hostId: '01JTEST000000000000000000001',
  title: 'Power Drill',
  description: 'Cordless drill',
  pricePerDay: 15,
  status: 'ACTIVE',
  hasVideo: false,
  createdAt: '2024-01-01T00:00:00Z',
  hostName: 'Bob',
  hostReputation: 600,
  distanceMeters: 800,
  driveTimeMin: 8,
  rankScore: 0.75,
  lat: 37.775,
  lng: -122.42,
  thumbnailUrl: '',
};

beforeEach(() => {
  mockPush.mockClear();
  useLocationStore.setState({ lat: 37.7749, lng: -122.4194 });
  // Reset search state to empty query between tests
  useSearchStore.setState({ query: '', filters: {} });
});

describe('SearchScreen', () => {
  it('renders the search input', () => {
    renderWithProviders(<SearchScreen />);
    expect(screen.getByPlaceholderText('Search listings…')).toBeTruthy();
  });

  it('shows idle empty state when query is empty', () => {
    renderWithProviders(<SearchScreen />);
    expect(screen.getByText('Search for anything nearby')).toBeTruthy();
  });

  it('shows results when query is typed', async () => {
    server.use(
      http.get(`${BASE_URL}/api/v1/discovery/search`, () =>
        HttpResponse.json({ listings: [testListing], nextCursor: null }),
      ),
    );

    renderWithProviders(<SearchScreen />);

    // Directly set the store query (bypasses debounce for test reliability)
    useSearchStore.setState({ query: 'drill' });

    await waitFor(() =>
      expect(screen.getByText('Power Drill')).toBeTruthy(),
    );
    expect(screen.getByText('Bob')).toBeTruthy();
  });

  it('shows "no results" empty state when API returns empty listings for a query', async () => {
    server.use(
      http.get(`${BASE_URL}/api/v1/discovery/search`, () =>
        HttpResponse.json({ listings: [], nextCursor: null }),
      ),
    );

    renderWithProviders(<SearchScreen />);
    useSearchStore.setState({ query: 'nonexistent' });

    await waitFor(() =>
      expect(screen.getByText('No results for "nonexistent"')).toBeTruthy(),
    );
  });

  it('navigates to listing detail when a result is pressed', async () => {
    server.use(
      http.get(`${BASE_URL}/api/v1/discovery/search`, () =>
        HttpResponse.json({ listings: [testListing], nextCursor: null }),
      ),
    );

    renderWithProviders(<SearchScreen />);
    useSearchStore.setState({ query: 'drill' });

    await waitFor(() => screen.getByText('Power Drill'));
    fireEvent.press(screen.getByText('Power Drill'));

    expect(mockPush).toHaveBeenCalledWith(
      expect.objectContaining({ params: expect.objectContaining({ id: testListing.id }) }),
    );
  });
});
