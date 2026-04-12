import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react-native';
import HoldStatusCard from '../../components/rental/HoldStatusCard';
import PhotoDiffResult from '../../components/rental/PhotoDiffResult';
import DisputeTimeline from '../../components/rental/DisputeTimeline';
import type { DisputeStatus } from '../../lib/hooks/useDispute';

// ── HoldStatusCard ─────────────────────────────────────────────────────────────

describe('HoldStatusCard', () => {
  const baseAllocation = {
    authorizedCents: 10000,
    capturedLateCents: 0,
    capturedDamageCents: 0,
    damageReserveCents: 0,
    releasedCents: 0,
  };

  it('renders the authorized amount', () => {
    render(<HoldStatusCard allocation={baseAllocation} />);
    expect(screen.getByText('$100.00')).toBeTruthy();
  });

  it('shows pending release notice when no amounts allocated', () => {
    render(<HoldStatusCard allocation={baseAllocation} />);
    expect(
      screen.getByText(/Hold release is pending/i),
    ).toBeTruthy();
  });

  it('shows released amount when greater than zero', () => {
    render(
      <HoldStatusCard
        allocation={{ ...baseAllocation, releasedCents: 10000 }}
      />,
    );
    expect(screen.getByText(/released back to your payment method/i)).toBeTruthy();
  });

  it('renders bar segments for late fee', () => {
    render(
      <HoldStatusCard
        allocation={{ ...baseAllocation, capturedLateCents: 2500 }}
      />,
    );
    expect(screen.getByText('Late return fee')).toBeTruthy();
    expect(screen.getByText('$25.00')).toBeTruthy();
  });

  it('renders bar segments for damage charge', () => {
    render(
      <HoldStatusCard
        allocation={{ ...baseAllocation, capturedDamageCents: 5000 }}
      />,
    );
    expect(screen.getByText('Damage charge')).toBeTruthy();
    expect(screen.getByText('$50.00')).toBeTruthy();
  });
});

// ── PhotoDiffResult ───────────────────────────────────────────────────────────

describe('PhotoDiffResult', () => {
  it('renders empty state when no pairs provided', () => {
    render(
      <PhotoDiffResult
        pairs={[]}
        overallClassification="NO_DAMAGE"
        overallConfidence={0.95}
      />,
    );
    expect(screen.getByText(/Photo comparison pending/i)).toBeTruthy();
  });

  it('renders overall classification badge', () => {
    render(
      <PhotoDiffResult
        pairs={[]}
        overallClassification="MINOR_DAMAGE"
        overallConfidence={0.8}
      />,
    );
    expect(screen.getByText('Minor damage')).toBeTruthy();
    expect(screen.getByText('80%')).toBeTruthy();
  });

  it('renders check-in / check-out labels for each pair', () => {
    render(
      <PhotoDiffResult
        pairs={[
          {
            checkInUrl: 'https://example.com/ci.jpg',
            checkOutUrl: 'https://example.com/co.jpg',
            classification: 'NO_DAMAGE',
            confidence: 0.92,
          },
        ]}
        overallClassification="NO_DAMAGE"
        overallConfidence={0.92}
      />,
    );
    expect(screen.getAllByText('Check-in').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Check-out').length).toBeGreaterThan(0);
  });
});

// ── DisputeTimeline ───────────────────────────────────────────────────────────

describe('DisputeTimeline', () => {
  const statuses: DisputeStatus[] = [
    'PENDING',
    'EVIDENCE_GATHERING',
    'UNDER_REVIEW',
    'RESOLVED',
  ];

  it.each(statuses)('renders without crashing for status %s', (status) => {
    const { toJSON } = render(<DisputeTimeline currentStatus={status} />);
    expect(toJSON()).toBeTruthy();
  });

  it('shows all four timeline steps', () => {
    render(<DisputeTimeline currentStatus="PENDING" />);
    // "Filed" text node may include the "← current" sibling text, so use substring match
    expect(screen.getByText(/Filed/)).toBeTruthy();
    expect(screen.getByText(/Evidence gathered/)).toBeTruthy();
    expect(screen.getByText(/Under review/)).toBeTruthy();
    expect(screen.getByText(/Resolved/)).toBeTruthy();
  });

  it('marks the current status as active', () => {
    render(<DisputeTimeline currentStatus="UNDER_REVIEW" />);
    expect(screen.getByText(/← current/)).toBeTruthy();
  });
});

// ── notifications ─────────────────────────────────────────────────────────────

describe('notifications module', () => {
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  const { NOTIFICATION_ROUTES, getNotificationParams } = require('../../lib/notifications');

  it('maps RATING_PROMPT to the rate screen', () => {
    expect(NOTIFICATION_ROUTES['RATING_PROMPT']).toBe('/(tabs)/(rentals)/rate');
  });

  it('maps DISPUTE_RESOLVED to dispute-status screen', () => {
    expect(NOTIFICATION_ROUTES['DISPUTE_RESOLVED']).toBe(
      '/(tabs)/(rentals)/dispute-status',
    );
  });

  it('getNotificationParams returns transactionId for RATING_PROMPT', () => {
    const params = getNotificationParams({
      type: 'RATING_PROMPT',
      transactionId: 'tx-123',
      counterpartyName: 'Alice',
    });
    expect(params).toEqual({ transactionId: 'tx-123' });
  });

  it('getNotificationParams returns transactionId + disputeId for DISPUTE_FILED', () => {
    const params = getNotificationParams({
      type: 'DISPUTE_FILED',
      transactionId: 'tx-123',
      disputeId: 'dis-456',
      reason: 'DAMAGE',
    });
    expect(params).toEqual({ transactionId: 'tx-123', disputeId: 'dis-456' });
  });
});
