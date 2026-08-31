import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { NotificationDeliveryLog, Webhook } from '@/api/notifications';
import type { AlertEvent } from '@/types/api';

import { AlertDeliveryLogCard, mergeDeliveryLogRows } from './AlertDeliveryLogCard';

const webhooks: Webhook[] = [
  {
    id: 'wh-ops',
    name: 'Ops Discord',
    url: 'https://discord.example.test/hook',
    method: 'POST',
    headers: {},
    enabled: true,
  },
];

const log: NotificationDeliveryLog = {
  entries: [
    {
      notificationId: 'webhook-1',
      type: 'webhook',
      destinationId: 'webhook:wh-ops',
      outcome: 'dead_letter',
      alertIds: ['vm-offline-101', 'vm-offline-102'],
      alertCount: 2,
      attempts: 3,
      success: false,
      errorMessage: 'HTTP 401 Unauthorized',
      failureClass: 'authentication',
      timestamp: new Date(Date.now() - 60_000).toISOString(),
    },
    {
      notificationId: 'email-1',
      type: 'email',
      destinationId: 'destination:abcd',
      outcome: 'sent',
      alertIds: ['disk-critical-1'],
      alertCount: 1,
      attempts: 1,
      success: true,
      timestamp: new Date(Date.now() - 120_000).toISOString(),
    },
  ],
  windowDays: 7,
  completedRetentionDays: 7,
  deadLetterRetentionDays: 30,
};

describe('AlertDeliveryLogCard', () => {
  afterEach(() => cleanup());

  it('shows each attempt with a plain-language outcome and the resolved destination name', () => {
    render(() => (
      <AlertDeliveryLogCard
        log={log}
        unavailable={false}
        refreshing={false}
        onRefresh={vi.fn()}
        webhooks={webhooks}
      />
    ));

    expect(screen.getByText('Ops Discord')).toBeInTheDocument();
    expect(screen.getByText('Email')).toBeInTheDocument();
    expect(screen.getByText('Failed, retries exhausted')).toBeInTheDocument();
    expect(screen.getByText('Delivered')).toBeInTheDocument();
    expect(screen.getByText('vm-offline-101 +1 more')).toBeInTheDocument();
    expect(screen.getByText(/Authentication failure/)).toBeInTheDocument();
    expect(screen.getByText(/HTTP 401 Unauthorized/)).toBeInTheDocument();
  });

  it('says test sends are not listed so their absence is not read as failure', () => {
    render(() => (
      <AlertDeliveryLogCard
        log={log}
        unavailable={false}
        refreshing={false}
        onRefresh={vi.fn()}
        webhooks={webhooks}
      />
    ));

    expect(screen.getByText(/Test sends skip the queue/)).toBeInTheDocument();
  });

  it('renders an honest empty state when no deliveries were attempted', () => {
    render(() => (
      <AlertDeliveryLogCard
        log={{
          entries: [],
          windowDays: 30,
          completedRetentionDays: 7,
          deadLetterRetentionDays: 30,
        }}
        unavailable={false}
        refreshing={false}
        onRefresh={vi.fn()}
        webhooks={[]}
      />
    ));

    expect(screen.getByText(/No alert deliveries were attempted/)).toBeInTheDocument();
  });

  it('shows the mixed retention windows and visible correlation timestamps', () => {
    const { container } = render(() => (
      <AlertDeliveryLogCard
        log={log}
        unavailable={false}
        refreshing={false}
        onRefresh={vi.fn()}
        webhooks={webhooks}
      />
    ));

    expect(screen.getByText(/Completed attempts are retained for 7 days/)).toBeInTheDocument();
    expect(
      screen.getByText(/Failures that exhausted retries remain available for 30 days/),
    ).toBeInTheDocument();
    const timestamps = Array.from(container.querySelectorAll('time'));
    expect(timestamps[0]).toHaveAttribute('datetime', log.entries[0].timestamp);
    expect(timestamps[0]).not.toHaveTextContent(/ago$/);
  });

  it('interleaves held notifications with delivery attempts, newest first', () => {
    const heldEvents: AlertEvent[] = [
      {
        id: 1,
        occurredAt: new Date(Date.now() - 30_000).toISOString(),
        type: 'notification_suppressed',
        alertId: 'cpu-alert-1',
        resourceName: 'pve1',
        alertType: 'cpu',
        reason: 'flapping',
        message: 'Notification suppressed: alert is flapping.',
      },
      {
        id: 2,
        occurredAt: new Date(Date.now() - 90_000).toISOString(),
        type: 'notification_deferred',
        alertId: 'mem-alert-1',
        resourceName: 'pve2',
        alertType: 'memory',
        reason: 'quiet_hours:performance',
        message: 'Notification deferred by quiet hours; the queue replays it when they end.',
      },
    ];

    render(() => (
      <AlertDeliveryLogCard
        log={log}
        unavailable={false}
        refreshing={false}
        onRefresh={vi.fn()}
        webhooks={webhooks}
        heldEvents={heldEvents}
      />
    ));

    expect(screen.getByText('Held')).toBeInTheDocument();
    expect(screen.getByText('pve1 (cpu)')).toBeInTheDocument();
    expect(screen.getByText('Flapping')).toBeInTheDocument();
    expect(screen.getByText('Deferred')).toBeInTheDocument();
    expect(screen.getByText('pve2 (memory)')).toBeInTheDocument();
    expect(screen.getByText('Quiet hours')).toBeInTheDocument();

    const rows = mergeDeliveryLogRows(log.entries, heldEvents);
    expect(rows.map((row) => row.kind)).toEqual(['held', 'attempt', 'held', 'attempt']);
  });

  it('shows held rows even when no delivery was attempted', () => {
    render(() => (
      <AlertDeliveryLogCard
        log={{
          entries: [],
          windowDays: 30,
          completedRetentionDays: 7,
          deadLetterRetentionDays: 30,
        }}
        unavailable={false}
        refreshing={false}
        onRefresh={vi.fn()}
        webhooks={[]}
        heldEvents={[
          {
            id: 3,
            occurredAt: new Date().toISOString(),
            type: 'notification_suppressed',
            alertId: 'held-only',
            resourceName: 'nas-1',
            alertType: 'usage',
            reason: 'notifications_inactive',
            message: 'Notification suppressed: alert delivery is not turned on.',
          },
        ]}
      />
    ));

    expect(screen.getByText('Held')).toBeInTheDocument();
    expect(screen.getByText('Delivery not turned on')).toBeInTheDocument();
    expect(screen.queryByText(/No alert deliveries were attempted/)).not.toBeInTheDocument();
  });

  it('reports an unreadable log as unavailable instead of empty', () => {
    render(() => (
      <AlertDeliveryLogCard
        log={null}
        unavailable={true}
        refreshing={false}
        onRefresh={vi.fn()}
        webhooks={[]}
      />
    ));

    expect(screen.getByRole('alert')).toHaveTextContent(/could not read the delivery log/);
    expect(screen.queryByText(/No alert deliveries were attempted/)).not.toBeInTheDocument();
  });
});
