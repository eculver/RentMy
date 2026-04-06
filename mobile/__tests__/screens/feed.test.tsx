import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react-native';
import { http, HttpResponse } from 'msw';
import { server } from '../mocks/server';
import { renderWithProviders } from '../helpers/renderWithProviders';
import { useLocationStore } from '../../lib/stores/locationStore';

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

// eslint-disable-next-line @typescript-eslint/no-var-requires
const FeedScreen = require('../../app/(tabs)/(feed)/index').default as React.ComponentType;

const BASE_URL = process.env.EXPO_PUBLIC_API_URL ?? 'http://localhost:8080';

const testListing = {
  id: '01JTEST100000000000000000001',
  hostId: '01JTEST000000000000000000001',
  title: 'Mountain Bike',
  description: 'Great for trails',
  pricePerDay: 40,
  pricePerHour: undefined,
  status: 'ACTIVE',
  hasVideo: false,
  createdAt: '2024-01-01T00:00:00Z',
  hostName: 'Alice',
  hostReputation: 800,
  distanceMeters: 500,
  driveTimeMin: 5,
  rankScore: 0.9,
  lat: 37.775,
  lng: -122.42,
  thumbnailUrl: '',
};

beforeEach(() => {
  mockPush.mockClear();
  // Seed location so useLocation skips the async permission flow
  useLocationStore.setState({ lat: 37.7749, lng: -122.4194 });
});

describe('FeedScreen', () => {
  it('renders the empty state when there are no listings', async () => {
    // Default MSW handler returns empty listings
    renderWithProviders(<FeedScreen />);

    await waitFor(() =>
      expect(screen.getByText('No listings nearby')).toBeTruthy(),
    );
  });

  it('renders listing cards returned by the API', async () => {
    server.use(
      http.get(`${BASE_URL}/api/v1/discovery/feed`, () =>
        HttpResponse.json({ listings: [testListing], nextCursor: null }),
      ),
    );

    renderWithProviders(<FeedScreen />);

    await waitFor(() =>
      expect(screen.getByText('Mountain Bike')).toBeTruthy(),
    );
    expect(screen.getByText('$40/day')).toBeTruthy();
    expect(screen.getByText('Alice')).toBeTruthy();
  });

  it('navigates to listing detail on card press', async () => {
    server.use(
      http.get(`${BASE_URL}/api/v1/discovery/feed`, () =>
        HttpResponse.json({ listings: [testListing], nextCursor: null }),
      ),
    );

    renderWithProviders(<FeedScreen />);

    await waitFor(() => screen.getByText('Mountain Bike'));
    fireEvent.press(screen.getByText('Mountain Bike'));

    expect(mockPush).toHaveBeenCalledWith(
      expect.objectContaining({ params: expect.objectContaining({ id: testListing.id }) }),
    );
  });

  it('renders the "Rent Now" shortcut for each listing', async () => {
    server.use(
      http.get(`${BASE_URL}/api/v1/discovery/feed`, () =>
        HttpResponse.json({ listings: [testListing], nextCursor: null }),
      ),
    );

    renderWithProviders(<FeedScreen />);

    await waitFor(() => screen.getByText('Rent Now'));
    expect(screen.getByText('Rent Now')).toBeTruthy();
  });

  it('shows location loading indicator while location is being fetched', async () => {
    // Clear location so the hook goes through its loading state
    useLocationStore.setState({ lat: null, lng: null });

    renderWithProviders(<FeedScreen />);

    // The screen shows "Getting your location…" while waiting
    expect(screen.getByText('Getting your location…')).toBeTruthy();
  });
});
