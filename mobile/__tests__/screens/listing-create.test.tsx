/**
 * Tests for the Create Listing screen (profile tab).
 *
 * The screen has two steps: camera (photo capture) and form (listing details).
 * We mock AngleEnforcedCamera to expose an "Advance" button so tests can
 * skip the native camera and reach the form step directly.
 */
import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react-native';
import { http, HttpResponse } from 'msw';
import { server } from '../mocks/server';
import { renderWithProviders } from '../helpers/renderWithProviders';

// ── module mocks ─────────────────────────────────────────────────────────────
const mockBack = jest.fn();
jest.mock('expo-router', () => ({
  router: { push: jest.fn(), replace: jest.fn(), back: mockBack },
  useRouter: () => ({ push: jest.fn(), replace: jest.fn(), back: mockBack }),
  useLocalSearchParams: () => ({}),
}));

jest.mock('@expo/vector-icons', () => ({
  Ionicons: () => null,
}));

// AngleEnforcedCamera — calls onDone() immediately via a testable button
jest.mock('../../components/camera/AngleEnforcedCamera', () => {
  const React = require('react');
  const { View, Text, Pressable } = require('react-native');
  return function MockCamera({ onDone }: { onDone: () => void }) {
    return (
      <View>
        <Text>Camera step</Text>
        <Pressable onPress={onDone} testID="camera-done-btn">
          <Text>Done with photos</Text>
        </Pressable>
      </View>
    );
  };
});

// Mock expo-sensors used by useGyroscope inside AngleEnforcedCamera
jest.mock('expo-sensors', () => ({
  Gyroscope: {
    addListener: jest.fn(() => ({ remove: jest.fn() })),
    setUpdateInterval: jest.fn(),
  },
}));

// AIAutofillOverlay — simple pass-through
jest.mock('../../components/listing/AIAutofillOverlay', () => {
  const React = require('react');
  const { Text } = require('react-native');
  return function AIAutofillOverlay({ isLoading }: { isLoading: boolean }) {
    return <Text>{isLoading ? 'AI loading…' : 'AI done'}</Text>;
  };
});

// ValueOverridePrompt — not relevant for these tests
jest.mock('../../components/listing/ValueOverridePrompt', () => () => null);

// eslint-disable-next-line @typescript-eslint/no-var-requires
const CreateListingScreen = require('../../app/(tabs)/(profile)/create-listing').default as React.ComponentType;

const BASE_URL = process.env.EXPO_PUBLIC_API_URL ?? 'http://localhost:8080';

/** Advances past the camera step to the form step. */
function advanceToFormStep() {
  fireEvent.press(screen.getByTestId('camera-done-btn'));
}

/** Fill in minimum valid listing form data. */
function fillValidForm() {
  fireEvent.changeText(
    screen.getByPlaceholderText('e.g. Ocean kayak, DSLR camera, power drill'),
    'Mountain Bike',
  );
  fireEvent.changeText(
    screen.getByPlaceholderText('Describe your item, condition, and any special notes'),
    'Lightly used trail bike in great condition.',
  );
  fireEvent.changeText(screen.getByPlaceholderText('25'), '40');
  fireEvent.changeText(screen.getByPlaceholderText('33.77'), '37.7749');
  fireEvent.changeText(screen.getByPlaceholderText('-118.19'), '-122.4194');
}

beforeEach(() => {
  mockBack.mockClear();

  // Default: appraisal endpoint returns pending status
  server.use(
    http.get(`${BASE_URL}/api/v1/listings/:id/appraisal`, () =>
      HttpResponse.json({ status: 'PENDING' }),
    ),
  );
});

describe('CreateListingScreen — camera step', () => {
  it('renders the camera step initially', () => {
    renderWithProviders(<CreateListingScreen />);
    expect(screen.getByText('Camera step')).toBeTruthy();
  });

  it('advances to the form step when camera is done', () => {
    renderWithProviders(<CreateListingScreen />);
    advanceToFormStep();
    expect(screen.getByText('Listing Details')).toBeTruthy();
  });
});

describe('CreateListingScreen — form step', () => {
  it('renders all required form fields', () => {
    renderWithProviders(<CreateListingScreen />);
    advanceToFormStep();

    expect(screen.getByText('Listing Details')).toBeTruthy();
    expect(
      screen.getByPlaceholderText('e.g. Ocean kayak, DSLR camera, power drill'),
    ).toBeTruthy();
    expect(
      screen.getByPlaceholderText('Describe your item, condition, and any special notes'),
    ).toBeTruthy();
    expect(screen.getByPlaceholderText('25')).toBeTruthy(); // price per day
    expect(screen.getByText('Max rental duration')).toBeTruthy();
    expect(screen.getByText('1 day')).toBeTruthy();
    expect(screen.getByText('7 days (max)')).toBeTruthy();
  });

  it('shows validation error when title is missing on submit', async () => {
    renderWithProviders(<CreateListingScreen />);
    advanceToFormStep();

    // Fill everything except title
    fireEvent.changeText(
      screen.getByPlaceholderText('Describe your item, condition, and any special notes'),
      'Lightly used trail bike in great condition.',
    );
    fireEvent.changeText(screen.getByPlaceholderText('25'), '40');
    fireEvent.changeText(screen.getByPlaceholderText('33.77'), '37.7749');
    fireEvent.changeText(screen.getByPlaceholderText('-118.19'), '-122.4194');

    fireEvent.press(screen.getByText('Create Listing'));

    await waitFor(() =>
      expect(screen.getByText('Title must be at least 3 characters')).toBeTruthy(),
    );
  });

  it('shows validation error when description is too short', async () => {
    renderWithProviders(<CreateListingScreen />);
    advanceToFormStep();

    fireEvent.changeText(
      screen.getByPlaceholderText('e.g. Ocean kayak, DSLR camera, power drill'),
      'Mountain Bike',
    );
    fireEvent.changeText(
      screen.getByPlaceholderText('Describe your item, condition, and any special notes'),
      'Short',
    );
    fireEvent.changeText(screen.getByPlaceholderText('25'), '40');
    fireEvent.changeText(screen.getByPlaceholderText('33.77'), '37.7749');
    fireEvent.changeText(screen.getByPlaceholderText('-118.19'), '-122.4194');

    fireEvent.press(screen.getByText('Create Listing'));

    await waitFor(() =>
      expect(screen.getByText('Description must be at least 10 characters')).toBeTruthy(),
    );
  });

  it('calls POST /listings on successful submission', async () => {
    let calledBody: unknown = null;
    server.use(
      http.post(`${BASE_URL}/api/v1/listings`, async ({ request }) => {
        calledBody = await request.json();
        return HttpResponse.json({ id: '01JLST111111111111111111111', title: 'Mountain Bike' });
      }),
    );

    renderWithProviders(<CreateListingScreen />);
    advanceToFormStep();
    fillValidForm();

    fireEvent.press(screen.getByText('Create Listing'));

    await waitFor(() => expect(calledBody).not.toBeNull(), { timeout: 5000 });
    expect(calledBody).toMatchObject({ title: 'Mountain Bike' });
  });

  it('shows AI suggestions when provided via aiSuggestions prop', () => {
    // Test ListingForm directly with AI suggestions
    const ListingForm =
      // eslint-disable-next-line @typescript-eslint/no-var-requires
      require('../../components/listing/ListingForm').default as React.ComponentType<{
        onSubmit: () => Promise<void>;
        aiSuggestions: { title: string; description: string; tags: string[] };
      }>;

    render(
      <ListingForm
        onSubmit={jest.fn().mockResolvedValue(undefined)}
        aiSuggestions={{
          title: 'AI Suggested Title',
          description: 'AI suggested description text.',
          tags: ['outdoor', 'sports'],
        }}
      />,
    );

    expect(screen.getByText('outdoor')).toBeTruthy();
    expect(screen.getByText('sports')).toBeTruthy();
  });
});
