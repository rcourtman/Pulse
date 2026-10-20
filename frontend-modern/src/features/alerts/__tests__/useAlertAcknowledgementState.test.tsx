import { renderHook } from '@solidjs/testing-library';
import { createMemo } from 'solid-js';
import { createStore, reconcile } from 'solid-js/store';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { AlertsAPI } from '@/api/alerts';
import { notificationStore } from '@/stores/notifications';
import type { Alert } from '@/types/api';

import { useAlertAcknowledgementState } from '../useAlertAcknowledgementState';

vi.mock('@/api/alerts', () => ({
  AlertsAPI: {
    acknowledge: vi.fn(),
    bulkAcknowledge: vi.fn(),
    unacknowledge: vi.fn(),
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    error: vi.fn(),
    success: vi.fn(),
  },
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: vi.fn(),
  },
}));

function makeAlert(id: string, acknowledged = false): Alert {
  return {
    id,
    type: 'cpu',
    level: 'warning',
    resourceId: `vm-${id}`,
    resourceName: `VM ${id}`,
    node: 'node-1',
    message: `CPU high on ${id}`,
    startTime: '2026-03-22T11:00:00Z',
    acknowledged,
  } as Alert;
}

function acknowledgedBy(id: string, ackUser: string): Alert {
  return { ...makeAlert(id, true), ackTime: '2026-03-22T11:30:00Z', ackUser };
}

// The keyed alert store the way the websocket store keeps it: updateAlert
// merges a local change into the stored alert, and every server payload
// replaces the stored alert in place through reconcile. The page reads the
// alerts through a memo over the keyed store, as useAlertOverviewState does.
function renderWithAlertStore(initial: Alert[]) {
  return renderHook(() => {
    const [alertsById, setAlertsById] = createStore<Record<string, Alert>>(
      Object.fromEntries(initial.map((alert) => [alert.id, alert])),
    );
    const alerts = createMemo(() => Object.values(alertsById));
    const updateAlert = vi.fn((alertIdentifier: string, updates: Partial<Alert>) => {
      const existing = alertsById[alertIdentifier];
      if (existing) {
        setAlertsById(alertIdentifier, { ...existing, ...updates });
      }
    });
    const serverSends = (alert: Alert) => setAlertsById(alert.id, reconcile(alert));

    return {
      ...useAlertAcknowledgementState({ alerts, updateAlert, allowRestore: true }),
      alerts,
      updateAlert,
      serverSends,
    };
  });
}

describe('useAlertAcknowledgementState', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-03-22T12:00:00Z'));
    vi.mocked(AlertsAPI.acknowledge).mockReset();
    vi.mocked(AlertsAPI.unacknowledge).mockReset();
    vi.mocked(AlertsAPI.bulkAcknowledge).mockReset();
    vi.mocked(notificationStore.success).mockReset();
    vi.mocked(notificationStore.error).mockReset();
    vi.mocked(AlertsAPI.acknowledge).mockResolvedValue(undefined as never);
    vi.mocked(AlertsAPI.unacknowledge).mockResolvedValue(undefined as never);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  const unacknowledgedIds = (result: ReturnType<typeof renderWithAlertStore>['result']) =>
    result.unacknowledgedAlerts().map((alert) => alert.id);

  it('shows an unacknowledge from another session after a local acknowledge', async () => {
    const { result } = renderWithAlertStore([makeAlert('alert-1'), makeAlert('alert-2')]);
    expect(unacknowledgedIds(result)).toEqual(['alert-1', 'alert-2']);

    await result.handleAlertAcknowledgement(result.alerts()[0]);
    expect(AlertsAPI.acknowledge).toHaveBeenCalledWith('alert-1');
    expect(notificationStore.success).toHaveBeenCalledWith('Alert acknowledged');
    expect(unacknowledgedIds(result)).toEqual(['alert-2']);

    result.serverSends(acknowledgedBy('alert-1', 'admin'));
    expect(unacknowledgedIds(result)).toEqual(['alert-2']);
    expect(result.alerts()[0]).toMatchObject({ acknowledged: true, ackUser: 'admin' });

    // Unacknowledged from another session: the payload omits the ack fields.
    result.serverSends(makeAlert('alert-1'));
    expect(unacknowledgedIds(result)).toEqual(['alert-1', 'alert-2']);
    expect(result.alerts()[0].acknowledged).toBe(false);
    expect(result.alerts()[0]).not.toHaveProperty('ackUser');

    // The card acknowledges again rather than restoring a stale local state.
    vi.advanceTimersByTime(1500);
    await result.handleAlertAcknowledgement(result.alerts()[0]);
    expect(AlertsAPI.acknowledge).toHaveBeenCalledTimes(2);
    expect(AlertsAPI.unacknowledge).not.toHaveBeenCalled();
    expect(unacknowledgedIds(result)).toEqual(['alert-2']);
  });

  it('shows an acknowledge from another session after a local restore', async () => {
    const { result } = renderWithAlertStore([acknowledgedBy('alert-1', 'admin')]);
    expect(unacknowledgedIds(result)).toEqual([]);

    await result.handleAlertAcknowledgement(result.alerts()[0]);
    expect(AlertsAPI.unacknowledge).toHaveBeenCalledWith('alert-1');
    expect(notificationStore.success).toHaveBeenCalledWith('Alert restored');
    expect(unacknowledgedIds(result)).toEqual(['alert-1']);

    result.serverSends(makeAlert('alert-1'));
    expect(unacknowledgedIds(result)).toEqual(['alert-1']);

    result.serverSends(acknowledgedBy('alert-1', 'operator'));
    expect(unacknowledgedIds(result)).toEqual([]);
    expect(result.alerts()[0]).toMatchObject({ acknowledged: true, ackUser: 'operator' });
  });

  it('acknowledges, restores and bulk-acknowledges through the shared alert store', async () => {
    const { result } = renderWithAlertStore([
      makeAlert('alert-1'),
      makeAlert('alert-2', true),
      makeAlert('alert-3'),
    ]);
    vi.mocked(AlertsAPI.bulkAcknowledge).mockResolvedValue({
      results: [
        { alertIdentifier: 'alert-2', success: true },
        { alertIdentifier: 'alert-3', success: false },
      ],
    } as never);

    expect(unacknowledgedIds(result)).toEqual(['alert-1', 'alert-3']);

    await result.handleAlertAcknowledgement(result.alerts()[0]);
    expect(result.updateAlert).toHaveBeenCalledWith(
      'alert-1',
      expect.objectContaining({ acknowledged: true }),
    );
    expect(unacknowledgedIds(result)).toEqual(['alert-3']);

    vi.advanceTimersByTime(1500);
    expect(result.processingAlerts().has('alert-1')).toBe(false);

    await result.handleAlertAcknowledgement(result.alerts()[1]);
    expect(AlertsAPI.unacknowledge).toHaveBeenCalledWith('alert-2');
    expect(result.updateAlert).toHaveBeenCalledWith(
      'alert-2',
      expect.objectContaining({ acknowledged: false }),
    );
    expect(notificationStore.success).toHaveBeenCalledWith('Alert restored');
    expect(unacknowledgedIds(result)).toEqual(['alert-2', 'alert-3']);

    vi.advanceTimersByTime(1500);
    await result.handleBulkAcknowledge();
    expect(AlertsAPI.bulkAcknowledge).toHaveBeenCalledWith(['alert-2', 'alert-3']);
    expect(notificationStore.success).toHaveBeenCalledWith('Acknowledged 1 alert.');
    expect(notificationStore.error).toHaveBeenCalledWith('Failed to acknowledge 1 alert.');
    expect(unacknowledgedIds(result)).toEqual(['alert-3']);
  });

  it.each(['group', 'all'] as const)(
    'reports partial %s failure without acknowledging the rejected card',
    async (scope) => {
      const { result } = renderWithAlertStore([makeAlert('alert-1'), makeAlert('alert-2')]);
      vi.mocked(AlertsAPI.bulkAcknowledge).mockResolvedValue({
        results: [
          { alertIdentifier: 'alert-1', success: true },
          { alertIdentifier: 'alert-2', success: false, error: 'Synthetic refusal' },
        ],
      });
      await (scope === 'group'
        ? result.handleGroupAcknowledge(result.alerts())
        : result.handleBulkAcknowledge());
      expect(unacknowledgedIds(result)).toEqual(['alert-2']);
      expect(result.updateAlert).toHaveBeenCalledTimes(1);
      expect(notificationStore.success).toHaveBeenCalledWith('Acknowledged 1 alert.');
      expect(notificationStore.error).toHaveBeenCalledWith('Failed to acknowledge 1 alert.');
    },
  );

  it.each(['group', 'all'] as const)(
    'reports all %s failures and permits a deliberate retry after release',
    async (scope) => {
      const { result } = renderWithAlertStore([makeAlert('alert-1'), makeAlert('alert-2')]);
      const run = () =>
        scope === 'group'
          ? result.handleGroupAcknowledge(result.alerts())
          : result.handleBulkAcknowledge();
      vi.mocked(AlertsAPI.bulkAcknowledge).mockResolvedValueOnce({
        results: result.alerts().map((alert) => ({ alertIdentifier: alert.id, success: false })),
      });
      await run();
      expect(unacknowledgedIds(result)).toEqual(['alert-1', 'alert-2']);
      expect(result.updateAlert).not.toHaveBeenCalled();
      expect(notificationStore.success).not.toHaveBeenCalled();
      expect(notificationStore.error).toHaveBeenCalledWith('Failed to acknowledge 2 alerts.');
      vi.advanceTimersByTime(1500);
      vi.mocked(AlertsAPI.bulkAcknowledge).mockResolvedValueOnce({
        results: result.alerts().map((alert) => ({ alertIdentifier: alert.id, success: true })),
      });
      await run();
      expect(unacknowledgedIds(result)).toEqual([]);
      expect(notificationStore.success).toHaveBeenCalledWith('Acknowledged 2 alerts.');
    },
  );

  it('counts only unique requested confirmations and leaves missing or conflicting results active', async () => {
    const { result } = renderWithAlertStore([
      makeAlert('alert-1'),
      makeAlert('alert-2'),
      makeAlert('alert-3'),
      makeAlert('same-name-other'),
    ]);
    vi.mocked(AlertsAPI.bulkAcknowledge).mockResolvedValue({
      results: [
        { alertIdentifier: 'alert-1', success: true },
        { alertIdentifier: 'alert-1', success: true },
        { alertIdentifier: 'alert-2', success: true },
        { alertIdentifier: 'alert-2', success: false },
        { alertIdentifier: 'same-name-other', success: true },
      ],
    });
    await result.handleGroupAcknowledge(result.alerts().slice(0, 3));
    expect(result.updateAlert).toHaveBeenCalledTimes(1);
    expect(unacknowledgedIds(result)).toEqual(['alert-2', 'alert-3', 'same-name-other']);
    expect(notificationStore.success).toHaveBeenCalledWith('Acknowledged 1 alert.');
    expect(notificationStore.error).toHaveBeenCalledWith('Failed to acknowledge 2 alerts.');
  });

  it('uses current membership, excludes already acknowledged or removed alerts and deduplicates input', async () => {
    const { result } = renderWithAlertStore([makeAlert('alert-1'), makeAlert('alert-2', true)]);
    vi.mocked(AlertsAPI.bulkAcknowledge).mockResolvedValue({ results: [] });
    await result.handleGroupAcknowledge([
      makeAlert('alert-1'),
      makeAlert('alert-1'),
      makeAlert('alert-2'),
      makeAlert('removed'),
    ]);
    expect(AlertsAPI.bulkAcknowledge).toHaveBeenCalledWith(['alert-1']);
    expect(result.updateAlert).not.toHaveBeenCalled();
    expect(notificationStore.success).not.toHaveBeenCalled();
    expect(notificationStore.error).toHaveBeenCalledWith('Failed to acknowledge 1 alert.');
    vi.advanceTimersByTime(1500);
    vi.mocked(AlertsAPI.bulkAcknowledge).mockClear();
    await result.handleGroupAcknowledge([makeAlert('alert-2'), makeAlert('removed')]);
    expect(AlertsAPI.bulkAcknowledge).not.toHaveBeenCalled();
  });

  it('prevents duplicate and overlapping group, individual and all requests while preserving independent groups', async () => {
    const { result } = renderWithAlertStore([
      makeAlert('alert-1'),
      makeAlert('alert-2'),
      makeAlert('alert-3'),
    ]);
    type Response = Awaited<ReturnType<typeof AlertsAPI.bulkAcknowledge>>;
    let finish!: (response: Response) => void;
    vi.mocked(AlertsAPI.bulkAcknowledge).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    );
    const original = result.handleGroupAcknowledge(result.alerts().slice(0, 2));
    expect(result.processingAlerts()).toEqual(new Set(['alert-1', 'alert-2']));
    await result.handleGroupAcknowledge(result.alerts());
    await result.handleAlertAcknowledgement(result.alerts()[0]);
    await result.handleBulkAcknowledge();
    expect(AlertsAPI.bulkAcknowledge).toHaveBeenCalledTimes(1);
    expect(AlertsAPI.acknowledge).not.toHaveBeenCalled();
    vi.mocked(AlertsAPI.bulkAcknowledge).mockResolvedValueOnce({
      results: [{ alertIdentifier: 'alert-3', success: true }],
    });
    await result.handleGroupAcknowledge([result.alerts()[2]]);
    expect(AlertsAPI.bulkAcknowledge).toHaveBeenNthCalledWith(2, ['alert-3']);
    finish({
      results: [
        { alertIdentifier: 'alert-1', success: false },
        { alertIdentifier: 'alert-2', success: false },
      ],
    });
    await original;
    vi.advanceTimersByTime(1500);
    expect(result.processingAlerts().size).toBe(0);
    expect(unacknowledgedIds(result)).toEqual(['alert-1', 'alert-2']);
  });

  it('reserves Acknowledge all members against card and group requests until completion', async () => {
    const { result } = renderWithAlertStore([makeAlert('alert-1'), makeAlert('alert-2')]);
    type Response = Awaited<ReturnType<typeof AlertsAPI.bulkAcknowledge>>;
    let finish!: (response: Response) => void;
    vi.mocked(AlertsAPI.bulkAcknowledge).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    );
    const original = result.handleBulkAcknowledge();
    expect(result.bulkAckProcessing()).toBe(true);
    expect(result.processingAlerts()).toEqual(new Set(['alert-1', 'alert-2']));
    await result.handleBulkAcknowledge();
    await result.handleGroupAcknowledge(result.alerts());
    await result.handleAlertAcknowledgement(result.alerts()[0]);
    expect(AlertsAPI.bulkAcknowledge).toHaveBeenCalledTimes(1);
    expect(AlertsAPI.acknowledge).not.toHaveBeenCalled();
    finish({ results: [] });
    await original;
    vi.advanceTimersByTime(1500);
    expect(result.bulkAckProcessing()).toBe(false);
    expect(result.processingAlerts().size).toBe(0);
    expect(notificationStore.error).toHaveBeenCalledWith('Failed to acknowledge 2 alerts.');
  });

  it('does not start a group or all request over an individual acknowledgement', async () => {
    const { result } = renderWithAlertStore([makeAlert('alert-1'), makeAlert('alert-2')]);
    let finish!: () => void;
    vi.mocked(AlertsAPI.acknowledge).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finish = () => resolve({ success: true });
        }),
    );
    const original = result.handleAlertAcknowledgement(result.alerts()[0]);
    await result.handleGroupAcknowledge(result.alerts());
    await result.handleBulkAcknowledge();
    expect(AlertsAPI.bulkAcknowledge).not.toHaveBeenCalled();
    finish();
    await original;
  });

  it('retains every alert and releases processing after a rejected group request', async () => {
    const { result } = renderWithAlertStore([makeAlert('alert-1'), makeAlert('alert-2')]);
    vi.mocked(AlertsAPI.bulkAcknowledge).mockRejectedValue(new Error('Synthetic request failure'));
    await result.handleGroupAcknowledge(result.alerts());
    expect(result.updateAlert).not.toHaveBeenCalled();
    expect(unacknowledgedIds(result)).toEqual(['alert-1', 'alert-2']);
    expect(notificationStore.success).not.toHaveBeenCalled();
    expect(notificationStore.error).toHaveBeenCalledWith('Failed to acknowledge alerts');
    vi.advanceTimersByTime(1500);
    expect(result.processingAlerts().size).toBe(0);
  });
});
