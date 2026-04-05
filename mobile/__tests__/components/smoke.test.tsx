import React from 'react';
import { Text, View } from 'react-native';
import { render, screen } from '@testing-library/react-native';

function Greeting({ name }: { name: string }) {
  return (
    <View>
      <Text testID="greeting">{`Hello, ${name}!`}</Text>
    </View>
  );
}

describe('component rendering smoke test', () => {
  it('renders a React Native component without crashing', () => {
    render(<Greeting name="RentMy" />);
    expect(screen.getByTestId('greeting')).toBeTruthy();
    expect(screen.getByText('Hello, RentMy!')).toBeTruthy();
  });
});
