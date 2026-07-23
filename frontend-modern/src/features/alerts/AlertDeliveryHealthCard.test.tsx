import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { NotificationQueueHealth } from '@/api/notifications';

import { AlertDeliveryHealthCard } from './AlertDeliveryHealthCard';

const degradedHealth: NotificationQueueHealth = {
  pending: 2,
  sending: 0,
  sent: 12,
  failed: 1,
  deadLetter: 2,
  healthy: false,
  status: 'degraded',
  attentionRequired: 3,
  reasonCodes: ['retained_failed_deliveries', 'retained_dead_letter_deliveries'],
  completedRetentionDays: 7,
  deadLetterRetentionDays: 30,
  countsAreRetentionBounded: true,
  retryAttemptsAffectHealth: false,
  terminalFailuresAffectHealth: true,
};

describe('AlertDeliveryHealthCard', () => {
  afterEach(() => cleanup());

  it('tells operators that retained terminal deliveries were not delivered', () => {
    const onRefresh = vi.fn();
    render(() => (
      <AlertDeliveryHealthCard
        health={degradedHealth}
        unavailable={false}
        refreshing={false}
        onRefresh={onRefresh}
      />
    ));

    expect(screen.getByRole('alert')).toHaveTextContent('Notification delivery needs attention');
    expect(screen.getByRole('alert')).toHaveTextContent('1 failed delivery retained for 7 days');
    expect(screen.getByRole('alert')).toHaveTextContent(
      '2 dead-lettered deliveries retained for 30 days',
    );
    expect(screen.getByRole('alert')).toHaveTextContent(
      'recoverable retry attempts do not trigger this warning',
    );

    fireEvent.click(screen.getByRole('button', { name: 'Refresh delivery status' }));
    expect(onRefresh).toHaveBeenCalledTimes(1);
  });

  it('fails closed when queue health cannot be verified', () => {
    render(() => (
      <AlertDeliveryHealthCard health={null} unavailable refreshing onRefresh={vi.fn()} />
    ));

    expect(screen.getByRole('alert')).toHaveTextContent(
      'Notification delivery status is unavailable',
    );
    expect(screen.getByRole('alert')).toHaveTextContent('send a test before relying on delivery');
    expect(screen.getByRole('button', { name: 'Refresh delivery status' })).toBeDisabled();
  });
});
