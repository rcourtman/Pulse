import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
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
  failureClasses7d: {
    authentication: 2,
    rate_limited: 0,
    connectivity: 1,
    tls: 0,
    configuration: 0,
    rejected: 0,
    server_error: 0,
    unknown: 0,
  },
  failureClassesAvailable: true,
  failureClassWindowDays: 7,
};

describe('AlertDeliveryHealthCard', () => {
  afterEach(() => cleanup());

  it('tells operators that retained terminal deliveries were not delivered', () => {
    const onRefresh = vi.fn();
    const onRetryFailures = vi.fn();
    const onDismissFailures = vi.fn();
    render(() => (
      <Router>
        <Route
          path="/"
          component={() => (
            <AlertDeliveryHealthCard
              health={degradedHealth}
              unavailable={false}
              refreshing={false}
              onRefresh={onRefresh}
              onRetryFailures={onRetryFailures}
              onDismissFailures={onDismissFailures}
              detailsHref="/alerts/notifications"
            />
          )}
        />
      </Router>
    ));

    expect(screen.getByRole('alert')).toHaveTextContent('Notification delivery needs attention');
    expect(screen.getByRole('alert')).toHaveTextContent('1 failed delivery retained for 7 days');
    expect(screen.getByRole('alert')).toHaveTextContent(
      '2 dead-lettered deliveries retained for 30 days',
    );
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Recoverable retry attempts do not trigger this warning',
    );
    expect(screen.getByRole('alert')).toHaveTextContent('classified as authentication (2)');
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Check destination credentials, tokens, and account permissions',
    );
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Dismiss retained failures to clear this warning without deleting delivery history',
    );
    expect(screen.getByRole('link', { name: 'Review delivery activity' })).toHaveAttribute(
      'href',
      '/alerts/notifications',
    );

    fireEvent.click(screen.getByRole('button', { name: 'Refresh delivery status' }));
    expect(onRefresh).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole('button', { name: 'Retry retained deliveries' }));
    expect(onRetryFailures).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole('button', { name: 'Dismiss retained failures' }));
    expect(onDismissFailures).toHaveBeenCalledTimes(1);
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

  it('keeps the overview treatment concise and points directly to delivery evidence', () => {
    render(() => (
      <Router>
        <Route
          path="/"
          component={() => (
            <AlertDeliveryHealthCard
              health={degradedHealth}
              unavailable={false}
              refreshing={false}
              onRefresh={vi.fn()}
              onRetryFailures={vi.fn()}
              onDismissFailures={vi.fn()}
              detailsHref="/alerts/notifications#notification-delivery-activity"
              detailLevel="summary"
              showRefresh={false}
            />
          )}
        />
      </Router>
    ));

    const alert = screen.getByRole('alert');
    expect(alert).toHaveTextContent('Most recent failures: authentication (2)');
    expect(alert).toHaveTextContent(
      'Review delivery activity for timestamps, destinations, alerts, and errors',
    );
    expect(alert).not.toHaveTextContent('Otherwise Pulse removes expired records hourly');
    expect(screen.queryByRole('button', { name: 'Refresh delivery status' })).toBeNull();
    expect(screen.getByRole('link', { name: 'Review delivery activity' })).toHaveAttribute(
      'href',
      '/alerts/notifications#notification-delivery-activity',
    );
  });
});
