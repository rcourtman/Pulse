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

    await result.handleBulkAcknowledge();
    expect(AlertsAPI.bulkAcknowledge).toHaveBeenCalledWith(['alert-2', 'alert-3']);
    expect(notificationStore.success).toHaveBeenCalledWith('Acknowledged 1 alert.');
    expect(notificationStore.error).toHaveBeenCalledWith('Failed to acknowledge 1 alert.');
    expect(unacknowledgedIds(result)).toEqual(['alert-3']);
  });
});
