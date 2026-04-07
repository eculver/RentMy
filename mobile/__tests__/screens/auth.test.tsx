import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react-native';
import { HTTPError } from 'ky';
import { useAuthStore } from '../../lib/auth';

// ── expo-router mock ──────────────────────────────────────────────────────────
const mockPush = jest.fn();
const mockBack = jest.fn();
jest.mock('expo-router', () => ({
  router: { push: mockPush, back: mockBack, replace: jest.fn() },
  useRouter: () => ({ push: mockPush, back: mockBack, replace: jest.fn() }),
  useLocalSearchParams: () => ({}),
}));

// ── lazy screen imports (after mocks) ────────────────────────────────────────
// eslint-disable-next-line @typescript-eslint/no-var-requires
const LoginScreen = require('../../app/(auth)/login').default as React.ComponentType;
// eslint-disable-next-line @typescript-eslint/no-var-requires
const RegisterScreen = require('../../app/(auth)/register').default as React.ComponentType;

beforeEach(() => {
  mockPush.mockClear();
  mockBack.mockClear();
  useAuthStore.setState({
    token: null,
    refreshToken: null,
    user: null,
    isAuthenticated: false,
    isLoading: false,
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// LoginScreen
// ─────────────────────────────────────────────────────────────────────────────
describe('LoginScreen', () => {
  it('renders email and password fields', () => {
    render(<LoginScreen />);
    expect(screen.getByPlaceholderText('you@example.com')).toBeTruthy();
    expect(screen.getByPlaceholderText('••••••••')).toBeTruthy();
  });

  it('renders the Sign In button', () => {
    render(<LoginScreen />);
    expect(screen.getByText('Sign In')).toBeTruthy();
  });

  it('shows validation error for invalid email on submit', async () => {
    render(<LoginScreen />);
    fireEvent.changeText(screen.getByPlaceholderText('you@example.com'), 'not-an-email');
    fireEvent.changeText(screen.getByPlaceholderText('••••••••'), 'Password1');
    fireEvent.press(screen.getByText('Sign In'));
    await waitFor(() =>
      expect(screen.getByText('Enter a valid email address')).toBeTruthy(),
    );
  });

  it('shows validation error for short password on submit', async () => {
    render(<LoginScreen />);
    fireEvent.changeText(screen.getByPlaceholderText('you@example.com'), 'user@example.com');
    fireEvent.changeText(screen.getByPlaceholderText('••••••••'), 'short');
    fireEvent.press(screen.getByText('Sign In'));
    await waitFor(() =>
      expect(screen.getByText('Password must be at least 8 characters')).toBeTruthy(),
    );
  });

  it('calls loginWithCredentials with valid credentials', async () => {
    const loginSpy = jest.fn().mockResolvedValue(undefined);
    useAuthStore.setState((s) => ({ ...s, loginWithCredentials: loginSpy }));

    render(<LoginScreen />);
    fireEvent.changeText(screen.getByPlaceholderText('you@example.com'), 'user@example.com');
    fireEvent.changeText(screen.getByPlaceholderText('••••••••'), 'Password1');
    fireEvent.press(screen.getByText('Sign In'));

    await waitFor(() =>
      expect(loginSpy).toHaveBeenCalledWith('user@example.com', 'Password1'),
    );
  });

  it('shows API error on 401 response', async () => {
    const error = new HTTPError(
      new Response(null, { status: 401 }),
      new Request('http://localhost:8080/api/v1/auth/login'),
      {} as never,
    );
    const loginSpy = jest.fn().mockRejectedValue(error);
    useAuthStore.setState((s) => ({ ...s, loginWithCredentials: loginSpy }));

    render(<LoginScreen />);
    fireEvent.changeText(screen.getByPlaceholderText('you@example.com'), 'user@example.com');
    fireEvent.changeText(screen.getByPlaceholderText('••••••••'), 'wrongpassword1A');
    fireEvent.press(screen.getByText('Sign In'));

    await waitFor(() =>
      expect(screen.getByText('Invalid email or password.')).toBeTruthy(),
    );
  });

  it('navigates to register when "Create an account" is pressed', () => {
    render(<LoginScreen />);
    fireEvent.press(screen.getByText('Create an account'));
    expect(mockPush).toHaveBeenCalledWith('/register');
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// RegisterScreen
// ─────────────────────────────────────────────────────────────────────────────
describe('RegisterScreen', () => {
  it('renders name, email, and password fields', () => {
    render(<RegisterScreen />);
    expect(screen.getByPlaceholderText('Jane Smith')).toBeTruthy();
    expect(screen.getByPlaceholderText('you@example.com')).toBeTruthy();
    expect(screen.getByPlaceholderText('••••••••')).toBeTruthy();
  });

  it('renders the Create Account button', () => {
    render(<RegisterScreen />);
    // Heading and button both use the text "Create Account"
    expect(screen.getAllByText('Create Account').length).toBeGreaterThanOrEqual(1);
  });

  // The button is the last "Create Account" text element
  function pressCreateAccount() {
    const matches = screen.getAllByText('Create Account');
    fireEvent.press(matches[matches.length - 1]);
  }

  it('shows validation error for empty name', async () => {
    render(<RegisterScreen />);
    fireEvent.changeText(screen.getByPlaceholderText('you@example.com'), 'user@example.com');
    fireEvent.changeText(screen.getByPlaceholderText('••••••••'), 'Password1');
    pressCreateAccount();
    await waitFor(() =>
      expect(screen.getByText('Name is required')).toBeTruthy(),
    );
  });

  it('shows validation error for invalid email', async () => {
    render(<RegisterScreen />);
    fireEvent.changeText(screen.getByPlaceholderText('Jane Smith'), 'Jane');
    fireEvent.changeText(screen.getByPlaceholderText('you@example.com'), 'bad-email');
    fireEvent.changeText(screen.getByPlaceholderText('••••••••'), 'Password1');
    pressCreateAccount();
    await waitFor(() =>
      expect(screen.getByText('Enter a valid email address')).toBeTruthy(),
    );
  });

  it('shows validation error when password lacks uppercase letter', async () => {
    render(<RegisterScreen />);
    fireEvent.changeText(screen.getByPlaceholderText('Jane Smith'), 'Jane');
    fireEvent.changeText(screen.getByPlaceholderText('you@example.com'), 'jane@example.com');
    fireEvent.changeText(screen.getByPlaceholderText('••••••••'), 'nouppercase1');
    pressCreateAccount();
    await waitFor(() =>
      expect(
        screen.getByText('Password must contain at least one uppercase letter'),
      ).toBeTruthy(),
    );
  });

  it('shows validation error when password lacks a number', async () => {
    render(<RegisterScreen />);
    fireEvent.changeText(screen.getByPlaceholderText('Jane Smith'), 'Jane');
    fireEvent.changeText(screen.getByPlaceholderText('you@example.com'), 'jane@example.com');
    fireEvent.changeText(screen.getByPlaceholderText('••••••••'), 'NoNumbersHere');
    pressCreateAccount();
    await waitFor(() =>
      expect(
        screen.getByText('Password must contain at least one number'),
      ).toBeTruthy(),
    );
  });

  it('calls register with valid credentials', async () => {
    const registerSpy = jest.fn().mockResolvedValue(undefined);
    useAuthStore.setState((s) => ({ ...s, register: registerSpy }));

    render(<RegisterScreen />);
    fireEvent.changeText(screen.getByPlaceholderText('Jane Smith'), 'Jane Smith');
    fireEvent.changeText(screen.getByPlaceholderText('you@example.com'), 'jane@example.com');
    fireEvent.changeText(screen.getByPlaceholderText('••••••••'), 'Password1');
    pressCreateAccount();

    await waitFor(() =>
      expect(registerSpy).toHaveBeenCalledWith('Jane Smith', 'jane@example.com', 'Password1', undefined),
    );
  });

  it('shows API error on 409 (duplicate email) response', async () => {
    const error = new HTTPError(
      new Response(null, { status: 409 }),
      new Request('http://localhost:8080/api/v1/auth/register'),
      {} as never,
    );
    const registerSpy = jest.fn().mockRejectedValue(error);
    useAuthStore.setState((s) => ({ ...s, register: registerSpy }));

    render(<RegisterScreen />);
    fireEvent.changeText(screen.getByPlaceholderText('Jane Smith'), 'Jane Smith');
    fireEvent.changeText(screen.getByPlaceholderText('you@example.com'), 'existing@example.com');
    fireEvent.changeText(screen.getByPlaceholderText('••••••••'), 'Password1');
    pressCreateAccount();

    await waitFor(() =>
      expect(
        screen.getByText('An account with this email already exists.'),
      ).toBeTruthy(),
    );
  });
});
