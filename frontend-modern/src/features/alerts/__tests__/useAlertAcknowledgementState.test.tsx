import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
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

describe('useAlertAcknowledgementState', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-03-22T12:00:00Z'));
    vi.mocked(AlertsAPI.acknowledge).mockReset();
    vi.mocked(AlertsAPI.unacknowledge).mockReset();
    vi.mocked(AlertsAPI.bulkAcknowledge).mockReset();
    vi.mocked(notificationStore.success).mockReset();
    vi.mocked(notificationStore.error).mockReset();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('owns shared alert acknowledge, restore, and bulk-ack runtime with optimistic state', async () => {
    const [alerts] = createSignal<Alert[]>([
      makeAlert('alert-1'),
      makeAlert('alert-2', true),
      makeAlert('alert-3'),
    ]);
    const updateAlert = vi.fn();

    vi.mocked(AlertsAPI.acknowledge).mockResolvedValue(undefined as never);
    vi.mocked(AlertsAPI.unacknowledge).mockResolvedValue(undefined as never);
    vi.mocked(AlertsAPI.bulkAcknowledge).mockResolvedValue({
      results: [
        { alertIdentifier: 'alert-1', success: true },
        { alertIdentifier: 'alert-3', success: false },
      ],
    } as never);

    const { result } = renderHook(() =>
      useAlertAcknowledgementState({
        alerts,
        updateAlert,
        allowRestore: true,
      }),
    );

    expect(result.unacknowledgedAlerts().map((alert) => alert.id)).toEqual(['alert-1', 'alert-3']);

    await result.handleAlertAcknowledgement(alerts()[0]);

    expect(AlertsAPI.acknowledge).toHaveBeenCalledWith('alert-1');
    expect(updateAlert).toHaveBeenCalledWith(
      'alert-1',
      expect.objectContaining({ acknowledged: true }),
    );
    expect(notificationStore.success).toHaveBeenCalledWith('Alert acknowledged');
    expect(result.unacknowledgedAlerts().map((alert) => alert.id)).toEqual(['alert-3']);

    vi.advanceTimersByTime(1500);
    expect(result.processingAlerts().has('alert-1')).toBe(false);

    await result.handleAlertAcknowledgement(alerts()[1]);

    expect(AlertsAPI.unacknowledge).toHaveBeenCalledWith('alert-2');
    expect(updateAlert).toHaveBeenCalledWith(
      'alert-2',
      expect.objectContaining({ acknowledged: false }),
    );
    expect(notificationStore.success).toHaveBeenCalledWith('Alert restored');

    await result.handleBulkAcknowledge();

    expect(AlertsAPI.bulkAcknowledge).toHaveBeenCalledWith(['alert-2', 'alert-3']);
    expect(notificationStore.error).toHaveBeenCalledWith('Failed to acknowledge 1 alert.');
  });

  it('supports optimistic acknowledgement even without an upstream update callback', async () => {
    const [alerts] = createSignal<Alert[]>([makeAlert('alert-1'), makeAlert('alert-2')]);
    vi.mocked(AlertsAPI.bulkAcknowledge).mockResolvedValue({
      results: [
        { alertIdentifier: 'alert-1', success: true },
        { alertIdentifier: 'alert-2', success: true },
      ],
    } as never);

    const { result } = renderHook(() =>
      useAlertAcknowledgementState({
        alerts,
      }),
    );

    await result.handleBulkAcknowledge();

    expect(result.unacknowledgedAlerts()).toHaveLength(0);
    expect(notificationStore.success).toHaveBeenCalledWith('Acknowledged 2 alerts.');
  });
});
