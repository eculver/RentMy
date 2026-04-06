import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react-native';
import RatingBubbles from '../../components/rating/RatingBubbles';
import { RENTER_BUBBLES, HOST_BUBBLES, BUBBLE_LABELS, RatingBubble } from '../../lib/hooks/useRatings';

describe('RatingBubbles', () => {
  it('renders all available bubbles', () => {
    render(
      <RatingBubbles
        availableBubbles={RENTER_BUBBLES}
        selected={[]}
        onToggle={jest.fn()}
      />
    );

    for (const bubble of RENTER_BUBBLES) {
      expect(screen.getByText(BUBBLE_LABELS[bubble])).toBeTruthy();
    }
  });

  it('calls onToggle when a bubble is tapped', () => {
    const onToggle = jest.fn();
    render(
      <RatingBubbles
        availableBubbles={RENTER_BUBBLES}
        selected={[]}
        onToggle={onToggle}
      />
    );

    fireEvent.press(screen.getByText(BUBBLE_LABELS['GOOD_COMMUNICATION']));
    expect(onToggle).toHaveBeenCalledWith('GOOD_COMMUNICATION');
  });

  it('does not call onToggle in readOnly mode', () => {
    const onToggle = jest.fn();
    render(
      <RatingBubbles
        availableBubbles={RENTER_BUBBLES}
        selected={['GOOD_COMMUNICATION']}
        onToggle={onToggle}
        readOnly
      />
    );

    fireEvent.press(screen.getByText(BUBBLE_LABELS['GOOD_COMMUNICATION']));
    expect(onToggle).not.toHaveBeenCalled();
  });

  it('renders host bubbles correctly', () => {
    render(
      <RatingBubbles
        availableBubbles={HOST_BUBBLES}
        selected={[]}
        onToggle={jest.fn()}
      />
    );

    for (const bubble of HOST_BUBBLES) {
      expect(screen.getByText(BUBBLE_LABELS[bubble])).toBeTruthy();
    }
  });

  it('renders an empty list without crashing', () => {
    const { toJSON } = render(
      <RatingBubbles
        availableBubbles={[]}
        selected={[]}
        onToggle={jest.fn()}
      />
    );
    expect(toJSON()).toBeTruthy();
  });
});

describe('BUBBLE_LABELS', () => {
  it('has a label for every renter bubble', () => {
    for (const b of RENTER_BUBBLES) {
      expect(BUBBLE_LABELS[b]).toBeTruthy();
    }
  });

  it('has a label for every host bubble', () => {
    for (const b of HOST_BUBBLES) {
      expect(BUBBLE_LABELS[b]).toBeTruthy();
    }
  });
});
