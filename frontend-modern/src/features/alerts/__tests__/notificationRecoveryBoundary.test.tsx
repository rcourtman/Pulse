import { onMount } from 'solid-js';
import { render, screen, fireEvent, waitFor, cleanup } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { apiFetchJSON } from '@/utils/apiClient';
import { notificationStore } from '@/stores/notifications';
import { AlertDeliveryLogCard } from '../AlertDeliveryLogCard';
import { AlertDeliveryHealthCard } from '../AlertDeliveryHealthCard';
import { AlertQueueActionFeedback } from '../AlertQueueActionFeedback';
import { useNotificationDeliveryHealth } from '../useNotificationDeliveryHealth';
import { useNotificationDeliveryLog } from '../useNotificationDeliveryLog';

vi.mock('@/utils/apiClient', () => ({ apiFetchJSON: vi.fn() }));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));
const api = vi.mocked(apiFetchJSON);
const health = {
  queue: {
    status: 'degraded',
    healthy: false,
    failed: 0,
    dlq: 2,
    attention_required: 2,
    pending: 0,
    sending: 0,
    sent: 1,
    completed_retention_days: 7,
    dead_letter_retention_days: 30,
    counts_are_retention_bounded: true,
    retry_attempts_affect_health: false,
    terminal_failures_affect_health: true,
    reason_codes: ['retained_dead_letter_deliveries'],
    failure_classes_7d: {
      authentication: 2,
      rate_limited: 0,
      connectivity: 0,
      tls: 0,
      configuration: 0,
      rejected: 0,
      server_error: 0,
      unknown: 0,
    },
    failure_classes_available: true,
    failure_class_window_days: 7,
  },
};
const failure = {
  notificationId: 'fixture',
  type: 'email',
  alertIds: ['host-offline'],
  alertCount: 1,
  attempts: 1,
  success: false,
  outcome: 'dead_letter',
  timestamp: '2026-10-20T12:00:00Z',
  failureClass: 'authentication',
};
function Harness() {
  const log = useNotificationDeliveryLog();
  const queue = useNotificationDeliveryHealth({ onAfterQueueAction: log.loadDeliveryLog });
  onMount(() => {
    void queue.loadDeliveryHealth();
    void log.loadDeliveryLog();
  });
  return (
    <>
      <AlertQueueActionFeedback
        message={queue.queueActionFeedback()}
        onClear={queue.clearQueueActionFeedback}
      />
      <AlertDeliveryHealthCard
        health={queue.deliveryHealth()?.queue ?? null}
        unavailable={queue.deliveryHealthUnavailable()}
        refreshing={queue.refreshingDeliveryHealth()}
        onRefresh={queue.loadDeliveryHealth}
        onRetryFailures={queue.retryTerminalFailures}
        onDismissFailures={queue.dismissTerminalFailures}
      />
      <AlertDeliveryLogCard
        log={log.deliveryLog()}
        unavailable={log.deliveryLogUnavailable()}
        refreshing={log.refreshingDeliveryLog()}
        onRefresh={log.loadDeliveryLog}
        webhooks={[]}
        heldEvents={log.heldEvents()}
        heldEventsUnavailable={log.heldEventsUnavailable()}
        refreshingHeldEvents={log.refreshingHeldEvents()}
      />
    </>
  );
}
let logPayload: unknown, actionPayload: unknown;
beforeEach(() => {
  vi.clearAllMocks();
  logPayload = { entries: [] };
  actionPayload = { success: false, affected: 2 };
  api.mockImplementation(async (url) => {
    if (url === '/api/notifications/health') return structuredClone(health) as never;
    if (url.startsWith('/api/notifications/delivery-log')) return logPayload as never;
    if (url.startsWith('/api/alerts/events')) return [] as never;
    if (url.startsWith('/api/notifications/terminal-failures/')) return actionPayload as never;
    throw new Error('Unexpected fixture request');
  });
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});
describe('notification recovery response boundary', () => {
  it('renders an unreadable collection as unavailable, never no attempts', async () => {
    logPayload = { entries: {} };
    render(Harness);
    expect(
      await screen.findByText(
        'Pulse could not read the delivery log. Refresh to check delivery attempts.',
      ),
    ).toBeVisible();
    expect(
      screen.queryByText('No alert deliveries were attempted in this window.'),
    ).not.toBeInTheDocument();
  });
  it('keeps readable failure evidence and names the incomplete list', async () => {
    logPayload = { entries: [failure, { ...failure, outcome: 'sent', success: false }] };
    render(Harness);
    expect(await screen.findByText('Failed, no further retries')).toBeVisible();
    expect(await screen.findByText(/This activity list is incomplete/)).toBeVisible();
    expect(screen.queryByText('Delivered')).not.toBeInTheDocument();
    expect(
      screen.queryByText('No alert deliveries were attempted in this window.'),
    ).not.toBeInTheDocument();
  });
  it.each(['retry', 'dismiss'] as const)(
    'never publishes a false %s success from a 2xx body',
    async (action) => {
      logPayload = { entries: [failure] };
      actionPayload =
        action === 'retry' ? { success: false, affected: 2 } : { success: true, affected: '2' };
      vi.spyOn(window, 'confirm').mockReturnValue(true);
      render(Harness);
      await screen.findByText('Failed, no further retries');
      fireEvent.click(
        screen.getByRole('button', {
          name: action === 'retry' ? 'Retry retained deliveries' : 'Dismiss retained failures',
        }),
      );
      expect(
        await screen.findByText(
          action === 'retry'
            ? 'Retry could not be confirmed. Refresh notification activity before trying again.'
            : 'Dismissal could not be confirmed. Refresh notification activity before trying again.',
        ),
      ).toBeVisible();
      await waitFor(() => expect(notificationStore.error).toHaveBeenCalledOnce());
      expect(notificationStore.success).not.toHaveBeenCalled();
      expect(api.mock.calls.filter(([url]) => url.includes('/terminal-failures/'))).toHaveLength(1);
      expect(api.mock.calls.filter(([url]) => url === '/api/notifications/health')).toHaveLength(1);
      expect(screen.getByText('Failed, no further retries')).toBeVisible();
    },
  );
});
