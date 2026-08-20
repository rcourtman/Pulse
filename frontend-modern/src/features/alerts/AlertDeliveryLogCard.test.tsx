import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { NotificationDeliveryLog, Webhook } from '@/api/notifications';

import { AlertDeliveryLogCard } from './AlertDeliveryLogCard';

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
        log={{ entries: [], windowDays: 7 }}
        unavailable={false}
        refreshing={false}
        onRefresh={vi.fn()}
        webhooks={[]}
      />
    ));

    expect(screen.getByText(/No alert deliveries were attempted/)).toBeInTheDocument();
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
