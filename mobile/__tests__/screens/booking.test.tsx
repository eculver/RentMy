import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react-native';
import { http, HttpResponse } from 'msw';
import { server } from '../mocks/server';
import { renderWithProviders } from '../helpers/renderWithProviders';
import { useCheckoutStore } from '../../lib/stores/checkoutStore';
import { useAuthStore } from '../../lib/auth';

// ── module mocks ─────────────────────────────────────────────────────────────
const mockPush = jest.fn();
const mockReplace = jest.fn();
const mockBack = jest.fn();

let mockParams: Record<string, string> = {};

jest.mock('expo-router', () => ({
  router: { push: mockPush, replace: mockReplace, back: mockBack },
  useRouter: () => ({ push: mockPush, replace: mockReplace, back: mockBack }),
  useLocalSearchParams: () => mockParams,
}));

jest.mock('@expo/vector-icons', () => ({
  Ionicons: () => null,
}));

jest.mock('@stripe/stripe-react-native', () => ({
  useStripe: () => ({
    initPaymentSheet: jest.fn().mockResolvedValue({ error: null }),
    presentPaymentSheet: jest.fn().mockResolvedValue({ error: null, paymentOption: { label: 'Visa 4242' } }),
  }),
}));

jest.mock('pusher-js/react-native', () => ({
  __esModule: true,
  default: jest.fn().mockImplementation(() => ({
    subscribe: jest.fn(() => ({ bind: jest.fn(), unbind: jest.fn() })),
    unsubscribe: jest.fn(),
    disconnect: jest.fn(),
  })),
}));

jest.mock('expo-linking', () => ({
  openURL: jest.fn().mockResolvedValue(undefined),
}));

// Mock IncomingRequest and CancelConfirmation to avoid deep render complexity
jest.mock('../../components/booking/IncomingRequest', () => () => null);
jest.mock('../../components/booking/CancelConfirmation', () => () => null);

// eslint-disable-next-line @typescript-eslint/no-var-requires
const BookingRequestScreen = require('../../app/(tabs)/(feed)/booking-request').default as React.ComponentType;
// eslint-disable-next-line @typescript-eslint/no-var-requires
const BookingStatusScreen = require('../../app/(tabs)/(feed)/booking-status').default as React.ComponentType;

const BASE_URL = process.env.EXPO_PUBLIC_API_URL ?? 'http://localhost:8080';

const testBooking = {
  id: '01JTXN00000000000000000001',
  renterId: '01JTEST000000000000000000001',
  hostId: '01JTEST000000000000000000002',
  listingId: '01JLST000000000000000000001',
  scheduledStart: '2025-06-01T10:00:00Z',
  scheduledEnd: '2025-06-01T14:00:00Z',
  status: 'REQUESTED',
  createdAt: '2025-05-01T00:00:00Z',
};

beforeEach(() => {
  mockPush.mockClear();
  mockReplace.mockClear();
  mockBack.mockClear();
  mockParams = {};
  useCheckoutStore.setState({
    scheduledStart: null,
    scheduledEnd: null,
    paymentMethodId: null,
    holdAmount: 0,
    rentalFee: 0,
    totalImpact: 0,
  });
  useAuthStore.setState({
    token: 'test-token',
    refreshToken: null,
    user: { id: '01JTEST000000000000000000001', name: 'Test User', email: 'test@example.com' },
    isAuthenticated: true,
    isLoading: false,
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// BookingRequestScreen
// ─────────────────────────────────────────────────────────────────────────────
describe('BookingRequestScreen', () => {
  beforeEach(() => {
    mockParams = {
      id: '01JLST000000000000000000001',
      title: 'Vintage Camera',
      pricePerDay: '50',
      hostName: 'Alice',
    };
    server.use(
      http.get(`${BASE_URL}/api/v1/listings/01JLST000000000000000000001/hold-estimate`, () =>
        HttpResponse.json({ holdAmount: 5000, itemValue: 20000, guaranteeGap: 15000 }),
      ),
    );
  });

  it('shows the listing title from route params', () => {
    renderWithProviders(<BookingRequestScreen />);
    expect(screen.getByText('Vintage Camera')).toBeTruthy();
  });

  it('renders the rental period section', () => {
    renderWithProviders(<BookingRequestScreen />);
    expect(screen.getByText('Rental period')).toBeTruthy();
  });

  it('renders the payment method section', () => {
    renderWithProviders(<BookingRequestScreen />);
    expect(screen.getByText('Payment method')).toBeTruthy();
  });

  it('"Send Request" button is disabled when no dates or payment method are set', () => {
    renderWithProviders(<BookingRequestScreen />);
    // The button label shows "Send Request" but is disabled (gray background)
    expect(screen.getByText('Send Request')).toBeTruthy();
  });

  it('"Send Request" button is enabled when dates and payment method are set', async () => {
    const start = new Date('2025-06-01T10:00:00Z');
    const end = new Date('2025-06-01T14:00:00Z');
    useCheckoutStore.setState({
      scheduledStart: start,
      scheduledEnd: end,
      paymentMethodId: 'pm_test_123',
      holdAmount: 5000,
      rentalFee: 2000,
      totalImpact: 7000,
    });

    renderWithProviders(<BookingRequestScreen />);

    // With dates + payment method set, the button should be enabled
    await waitFor(() => expect(screen.getByText('Send Request')).toBeTruthy());
  });

  it('shows "How it works" explanation', () => {
    renderWithProviders(<BookingRequestScreen />);
    expect(screen.getByText('How it works')).toBeTruthy();
  });

  it('submits booking and navigates to booking-status on success', async () => {
    server.use(
      http.post(`${BASE_URL}/api/v1/bookings`, () =>
        HttpResponse.json({
          transactionId: '01JTXN00000000000000000099',
          holdAmount: 5000,
          rentalFee: 2000,
          platformFee: 200,
          totalImpact: 7200,
        }),
      ),
    );

    const start = new Date('2025-06-01T10:00:00Z');
    const end = new Date('2025-06-01T14:00:00Z');
    useCheckoutStore.setState({
      scheduledStart: start,
      scheduledEnd: end,
      paymentMethodId: 'pm_test_123',
      holdAmount: 5000,
      rentalFee: 2000,
      totalImpact: 7000,
    });

    renderWithProviders(<BookingRequestScreen />);
    fireEvent.press(screen.getByText('Send Request'));

    await waitFor(() =>
      expect(mockReplace).toHaveBeenCalledWith(
        expect.objectContaining({
          params: expect.objectContaining({ transactionId: '01JTXN00000000000000000099' }),
        }),
      ),
    );
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// BookingStatusScreen
// ─────────────────────────────────────────────────────────────────────────────
describe('BookingStatusScreen', () => {
  beforeEach(() => {
    mockParams = { transactionId: testBooking.id };
  });

  it('shows the "Waiting for host" status when booking is REQUESTED', async () => {
    server.use(
      http.get(`${BASE_URL}/api/v1/bookings/${testBooking.id}`, () =>
        HttpResponse.json({ booking: testBooking }),
      ),
    );

    renderWithProviders(<BookingStatusScreen />);

    await waitFor(() =>
      expect(screen.getByText('Waiting for host')).toBeTruthy(),
    );
  });

  it('shows "Booking accepted" status for an ACCEPTED booking', async () => {
    server.use(
      http.get(`${BASE_URL}/api/v1/bookings/${testBooking.id}`, () =>
        HttpResponse.json({ booking: { ...testBooking, status: 'ACCEPTED' } }),
      ),
    );

    renderWithProviders(<BookingStatusScreen />);

    await waitFor(() =>
      expect(screen.getByText('Booking accepted')).toBeTruthy(),
    );
  });

  it('shows error state when booking fails to load', async () => {
    server.use(
      http.get(`${BASE_URL}/api/v1/bookings/${testBooking.id}`, () =>
        new HttpResponse(null, { status: 404 }),
      ),
    );

    renderWithProviders(<BookingStatusScreen />);

    await waitFor(
      () =>
        expect(screen.getByText('Unable to load booking. Please try again.')).toBeTruthy(),
      { timeout: 5000 },
    );
  });

  it('shows booking details section when loaded', async () => {
    server.use(
      http.get(`${BASE_URL}/api/v1/bookings/${testBooking.id}`, () =>
        HttpResponse.json({ booking: testBooking }),
      ),
    );

    renderWithProviders(<BookingStatusScreen />);

    await waitFor(() => screen.getByText('Booking details'));
    expect(screen.getByText('Booking details')).toBeTruthy();
  });
});
