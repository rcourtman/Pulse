import { createSignal } from 'solid-js';
import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type {
  NotificationDeliveryLog,
  NotificationDeliveryLogEntry,
  Webhook,
} from '@/api/notifications';
import type { AlertEvent } from '@/types/api';
import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';

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
    expect(screen.getByText('Failed, no further retries')).toBeInTheDocument();
    expect(screen.getByText('Delivered')).toBeInTheDocument();
    expect(screen.getByText('vm-offline-101 +1 more')).toBeInTheDocument();
    expect(screen.getByText(/Authentication failure/)).toBeInTheDocument();
    expect(screen.getByText(/HTTP 401 Unauthorized/)).toBeInTheDocument();
  });

  // These outcomes are produced by the queue with a three-attempt budget.
  // Authentication/configuration/rejection stop on attempt one; connectivity
  // and server errors can retry, and connectivity can also exhaust the budget.
  // The audit API exposes attempts but not the budget or the stop decision.
  it.each([
    ['authentication', 1, 'dead_letter', 'Failed, no further retries', 'Authentication failure'],
    ['configuration', 1, 'dead_letter', 'Failed, no further retries', 'Configuration problem'],
    ['rejected', 1, 'dead_letter', 'Failed, no further retries', 'Rejected by destination'],
    ['connectivity', 3, 'dead_letter', 'Failed, no further retries', 'Connectivity failure'],
    ['connectivity', 1, 'retry', 'Retrying', 'Connectivity failure'],
    ['server_error', 1, 'retry', 'Retrying', 'Destination server error'],
  ] as const)(
    'describes %s after %i attempt(s) as %s without inventing a stop reason',
    (failureClass, attempts, outcome, label, classification) => {
      const entry: NotificationDeliveryLogEntry = {
        ...log.entries[0],
        notificationId: `${failureClass}-${attempts}`,
        attempts,
        outcome,
        failureClass,
        success: false,
        errorMessage: 'Synthetic destination failure',
      };
      render(() => (
        <AlertDeliveryLogCard
          log={{ ...log, entries: [entry] }}
          unavailable={false}
          refreshing={false}
          onRefresh={vi.fn()}
          webhooks={webhooks}
        />
      ));

      const row = screen.getByRole('listitem');
      expect(within(row).getByText(label, { exact: true })).toBeVisible();
      expect(within(row).getByText(new RegExp(classification))).toBeVisible();
      expect(within(row).getByText(/Synthetic destination failure/)).toBeVisible();
      expect(row).not.toHaveTextContent(/exhaust/i);
      expect(row).not.toHaveTextContent('Delivered');
      expect(screen.getByText(/Entries marked "Failed, no further retries"/)).toHaveTextContent(
        'remain available for 30 days',
      );
    },
  );

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

  it('names the activity list and makes older evidence keyboard-scrollable', () => {
    render(() => (
      <AlertDeliveryLogCard
        log={log}
        unavailable={false}
        refreshing={false}
        onRefresh={vi.fn()}
        webhooks={webhooks}
      />
    ));

    const list = screen.getByRole('list', { name: 'Notification activity' });
    expect(list).toHaveAttribute('tabindex', '0');
    list.focus();
    expect(list).toHaveFocus();
  });

  it('makes every grouped alert ID available without hover', () => {
    render(() => (
      <AlertDeliveryLogCard
        log={log}
        unavailable={false}
        refreshing={false}
        onRefresh={vi.fn()}
        webhooks={webhooks}
      />
    ));

    const summary = screen.getByText('vm-offline-101 +1 more');
    expect(summary.tagName).toBe('SUMMARY');
    const details = summary.closest('details')!;
    expect(details).not.toHaveAttribute('open');
    fireEvent.click(summary);
    expect(details).toHaveAttribute('open');
    expect(within(details).getByText(/vm-offline-101\s+vm-offline-102/)).toBeVisible();
  });

  it('shows the held explanation as readable content rather than a hover-only title', () => {
    const message = 'Delivery held by quiet hours until 08:00 UTC; monitoring continues.';
    render(() => (
      <AlertDeliveryLogCard
        log={{ ...log, entries: [] }}
        unavailable={false}
        refreshing={false}
        onRefresh={vi.fn()}
        webhooks={[]}
        heldEvents={[
          {
            id: 99,
            occurredAt: new Date().toISOString(),
            type: 'notification_deferred',
            alertId: 'held-only',
            resourceName: 'nas-1',
            alertType: 'usage',
            reason: 'quiet_hours:performance',
            message,
          },
        ]}
      />
    ));

    expect(screen.getByText(message)).toBeVisible();
    expect(screen.getByRole('listitem')).not.toHaveAttribute('title', message);
  });

  it.each([
    ['retry', 'Retrying'],
    ['failed', 'Failed'],
    ['dead_letter', 'Failed, no further retries'],
    ['cancelled', 'Cancelled'],
  ] as const)(
    'replaces a previous successful row with %s evidence on refresh',
    (outcome, label) => {
      const [currentLog, setCurrentLog] = createSignal<NotificationDeliveryLog>({
        ...log,
        entries: [log.entries[1]],
      });
      render(() => (
        <AlertDeliveryLogCard
          log={currentLog()}
          unavailable={false}
          refreshing={false}
          onRefresh={vi.fn()}
          webhooks={webhooks}
        />
      ));
      expect(screen.getByText('Delivered')).toBeInTheDocument();

      setCurrentLog({
        ...log,
        entries: [{ ...log.entries[0], outcome }],
      });

      expect(screen.getByText(label)).toBeInTheDocument();
      expect(screen.queryByText('Delivered')).not.toBeInTheDocument();
      expect(screen.queryByText('disk-critical-1')).not.toBeInTheDocument();
      expect(screen.getByText('Ops Discord')).toBeInTheDocument();
      expect(screen.getAllByRole('listitem')).toHaveLength(1);
    },
  );

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

    expect(screen.getByText(/Other completed attempts are retained for 7 days/)).toBeInTheDocument();
    expect(
      screen.getByText(/Entries marked "Failed, no further retries" remain available for 30 days/),
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

  it('hides previously loaded rows during an outage and restores fresh results after recovery', () => {
    const [currentLog, setCurrentLog] = createSignal<NotificationDeliveryLog>(log);
    const [unavailable, setUnavailable] = createSignal(false);
    render(() => (
      <AlertDeliveryLogCard
        log={currentLog()}
        unavailable={unavailable()}
        refreshing={false}
        onRefresh={vi.fn()}
        webhooks={webhooks}
      />
    ));

    expect(screen.getByText('Ops Discord')).toBeInTheDocument();
    setUnavailable(true);
    expect(screen.getByRole('alert')).toHaveTextContent(/could not read the delivery log/);
    expect(screen.queryByRole('list')).not.toBeInTheDocument();
    expect(screen.queryByText('Delivered')).not.toBeInTheDocument();
    expect(screen.queryByText(/No alert deliveries were attempted/)).not.toBeInTheDocument();

    setCurrentLog({ ...log, entries: [log.entries[1]] });
    setUnavailable(false);
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(screen.getByText('Delivered')).toBeInTheDocument();
    expect(screen.queryByText('Ops Discord')).not.toBeInTheDocument();
    expect(screen.getAllByRole('listitem')).toHaveLength(1);

    setCurrentLog({ ...log, entries: [] });
    expect(screen.getByText(/No alert deliveries were attempted/)).toBeInTheDocument();
    expect(screen.queryByRole('list')).not.toBeInTheDocument();
  });

  it('hides held rows when their own read fails and restores them on recovery', () => {
    const [unavailable, setUnavailable] = createSignal(false);
    const [heldEvents, setHeldEvents] = createSignal<AlertEvent[]>([
      {
        id: 4,
        occurredAt: '2026-09-07T08:00:00Z',
        type: 'notification_deferred',
        alertId: 'quiet-hours-alert',
        resourceName: 'nas-quiet',
        alertType: 'usage',
        reason: 'quiet_hours:performance',
        message: 'Notification deferred by quiet hours.',
      },
    ]);
    render(() => (
      <AlertDeliveryLogCard
        log={{ ...log, entries: [] }}
        unavailable={false}
        heldEventsUnavailable={unavailable()}
        refreshing={false}
        onRefresh={vi.fn()}
        webhooks={[]}
        heldEvents={heldEvents()}
      />
    ));

    expect(screen.getByText('Deferred')).toBeInTheDocument();
    setUnavailable(true);
    expect(screen.getByRole('alert')).toHaveTextContent(
      /could not read held or deferred notifications/,
    );
    expect(screen.queryByRole('list')).not.toBeInTheDocument();
    expect(screen.queryByText('nas-quiet (usage)')).not.toBeInTheDocument();
    expect(screen.queryByText(/No alert deliveries were attempted/)).not.toBeInTheDocument();

    setUnavailable(false);
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(screen.getByText('Deferred')).toBeInTheDocument();
    expect(screen.getAllByRole('listitem')).toHaveLength(1);

    setUnavailable(true);
    setHeldEvents([]);
    setUnavailable(false);
    expect(screen.queryByText('Deferred')).not.toBeInTheDocument();
    expect(screen.queryByRole('list')).not.toBeInTheDocument();
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
  it('keeps independently readable held evidence when delivery attempts are unavailable', () => {
    render(() => (
      <AlertDeliveryLogCard
        log={log}
        unavailable={true}
        refreshing={false}
        heldEvents={[
          {
            id: 7,
            type: 'notification_deferred',
            alertId: 'held-known',
            occurredAt: '2026-10-03T10:00:00Z',
            reason: 'quiet_hours:performance',
          },
        ]}
        onRefresh={vi.fn()}
        webhooks={webhooks}
      />
    ));
    expect(screen.getByRole('alert')).toHaveTextContent(/could not read the delivery log/);
    expect(screen.getByText('held-known')).toBeInTheDocument();
    expect(screen.getByText('Quiet hours')).toBeInTheDocument();
    expect(screen.queryByText('Delivered')).not.toBeInTheDocument();
    expect(screen.queryByText(/No alert deliveries were attempted/)).not.toBeInTheDocument();
  });

  it('reports missing held evidence without hiding readable attempts', () => {
    render(() => (
      <AlertDeliveryLogCard
        log={log}
        unavailable={false}
        refreshing={false}
        heldEventsUnavailable={true}
        heldEvents={[
          {
            id: 8,
            type: 'notification_suppressed',
            alertId: 'stale-held',
            occurredAt: '2026-10-03T10:00:00Z',
          },
        ]}
        onRefresh={vi.fn()}
        webhooks={webhooks}
      />
    ));
    expect(screen.getByRole('alert')).toHaveTextContent(
      /could not read held or deferred notifications/,
    );
    expect(screen.getByText('Delivered')).toBeInTheDocument();
    expect(screen.getByText('Ops Discord')).toBeInTheDocument();
    expect(screen.queryByText('stale-held')).not.toBeInTheDocument();
  });

  it('does not claim an empty activity window while held evidence is pending', () => {
    render(() => (
      <AlertDeliveryLogCard
        log={{ ...log, entries: [] }}
        unavailable={false}
        refreshing={false}
        refreshingHeldEvents={true}
        onRefresh={vi.fn()}
        webhooks={[]}
      />
    ));
    expect(screen.getByRole('status')).toHaveTextContent(
      'Loading held and deferred notifications...',
    );
    expect(screen.queryByText(/No alert deliveries were attempted/)).not.toBeInTheDocument();
    // A slow held read must not prevent a new delivery-attempt refresh.
    expect(screen.getByRole('button', { name: 'Refresh delivery status' })).toBeEnabled();
  });

  it('does not claim an empty activity window when held evidence is unavailable', () => {
    render(() => (
      <AlertDeliveryLogCard
        log={{ ...log, entries: [] }}
        unavailable={false}
        refreshing={false}
        heldEventsUnavailable={true}
        onRefresh={vi.fn()}
        webhooks={[]}
      />
    ));
    expect(screen.getByRole('alert')).toHaveTextContent(
      /could not read held or deferred notifications/,
    );
    expect(screen.queryByText(/No alert deliveries were attempted/)).not.toBeInTheDocument();
  });

  it('labels the initial attempt read as loading rather than empty', () => {
    render(() => (
      <AlertDeliveryLogCard
        log={null}
        unavailable={false}
        refreshing={true}
        onRefresh={vi.fn()}
        webhooks={[]}
      />
    ));
    expect(screen.getByRole('status')).toHaveTextContent('Loading delivery attempts...');
    expect(screen.queryByText(/No alert deliveries were attempted/)).not.toBeInTheDocument();
  });

  it("keeps each row's age tooltip moving while the log is not refreshed", () => {
    vi.useFakeTimers({
      toFake: ['Date', 'setInterval', 'clearInterval'],
      now: Date.parse('2026-08-27T12:05:00Z'),
    });

    try {
      render(() => (
        <AlertDeliveryLogCard
          log={{
            ...log,
            entries: [{ ...log.entries[1], timestamp: '2026-08-27T12:00:00Z' }],
          }}
          unavailable={false}
          refreshing={false}
          onRefresh={vi.fn()}
          webhooks={webhooks}
        />
      ));
      const time = () => document.querySelector('time[datetime="2026-08-27T12:00:00Z"]');

      expect(time()).toHaveAttribute('title', '5 mins ago');

      // The log is not re-read: the entry never changes and only the clock moves.
      vi.advanceTimersByTime(2 * 60 * 2 * RELATIVE_TIME_TICK_MS);

      expect(time()).toHaveAttribute('title', '2 hours ago');
    } finally {
      vi.useRealTimers();
    }
  });
});
