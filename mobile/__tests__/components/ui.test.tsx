import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react-native';
import Button from '../../components/ui/Button';
import Input from '../../components/ui/Input';

describe('Button', () => {
  it('renders the title text', () => {
    render(<Button title="Tap me" onPress={jest.fn()} />);
    expect(screen.getByText('Tap me')).toBeTruthy();
  });

  it('calls onPress when tapped', () => {
    const onPress = jest.fn();
    render(<Button title="Tap me" onPress={onPress} />);
    fireEvent.press(screen.getByText('Tap me'));
    expect(onPress).toHaveBeenCalledTimes(1);
  });

  it('does not call onPress when disabled', () => {
    const onPress = jest.fn();
    render(<Button title="Tap me" onPress={onPress} disabled />);
    fireEvent.press(screen.getByText('Tap me'));
    expect(onPress).not.toHaveBeenCalled();
  });

  it('shows ActivityIndicator when loading', () => {
    render(<Button title="Tap me" onPress={jest.fn()} loading />);
    // Title text is hidden while loading — only the spinner renders
    expect(screen.queryByText('Tap me')).toBeNull();
  });

  it('renders secondary variant without crashing', () => {
    render(<Button title="Secondary" onPress={jest.fn()} variant="secondary" />);
    expect(screen.getByText('Secondary')).toBeTruthy();
  });

  it('renders ghost variant without crashing', () => {
    render(<Button title="Ghost" onPress={jest.fn()} variant="ghost" />);
    expect(screen.getByText('Ghost')).toBeTruthy();
  });
});

describe('Input', () => {
  it('renders with a label', () => {
    render(<Input label="Email" />);
    expect(screen.getByText('Email')).toBeTruthy();
  });

  it('renders without a label', () => {
    render(<Input placeholder="Enter text" />);
    expect(screen.getByPlaceholderText('Enter text')).toBeTruthy();
  });

  it('shows error message when error prop is set', () => {
    render(<Input label="Email" error="Enter a valid email address" />);
    expect(screen.getByText('Enter a valid email address')).toBeTruthy();
  });

  it('does not show error message when error is not set', () => {
    render(<Input label="Email" />);
    expect(screen.queryByText('Enter a valid email address')).toBeNull();
  });

  it('fires onChangeText when text changes', () => {
    const onChange = jest.fn();
    render(<Input label="Name" onChangeText={onChange} />);
    fireEvent.changeText(screen.getByDisplayValue(''), 'hello');
    expect(onChange).toHaveBeenCalledWith('hello');
  });
});
