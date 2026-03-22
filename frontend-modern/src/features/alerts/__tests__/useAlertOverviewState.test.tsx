import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { AlertsAPI } from '@/api/alerts';
import { notificationStore } from '@/stores/notifications';
import type { Alert } from '@/types/api';

import { useAlertOverviewState } from '../useAlertOverviewState';

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

function makeAlert(id: string, startTime: string, acknowledged = false): Alert {
  return {
    id,
    type: 'cpu',
    level: 'warning',
    resourceId: `vm-${id}`,
    resourceName: `VM ${id}`,
    node: 'node-1',
    message: `CPU high on ${id}`,
    startTime,
    acknowledged,
  } as Alert;
}

describe('useAlertOverviewState', () => {
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

  it('owns overview stats, filtering, and acknowledge flows outside the tab shell', async () => {
    const now = Date.now();
    const [activeAlerts] = createSignal<Record<string, Alert>>({
      warning: makeAlert('warning', new Date(now - 60_000).toISOString(), false),
      acknowledged: makeAlert('acknowledged', new Date(now - 2 * 60_000).toISOString(), true),
      old: makeAlert('old', new Date(now - 3 * 86_400_000).toISOString(), false),
    });
    const [showAcknowledged] = createSignal(false);
    const updateAlert = vi.fn();

    vi.mocked(AlertsAPI.acknowledge).mockResolvedValue(undefined as any);
    vi.mocked(AlertsAPI.unacknowledge).mockResolvedValue(undefined as any);
    vi.mocked(AlertsAPI.bulkAcknowledge).mockResolvedValue({
      results: [
        { alertIdentifier: 'warning', success: true },
        { alertIdentifier: 'old', success: false },
      ],
    } as any);

    const { result } = renderHook(() =>
      useAlertOverviewState({
        activeAlerts,
        overrides: () => [],
        showAcknowledged,
        updateAlert,
      }),
    );

    expect(result.alertStats()).toMatchObject({
      active: 2,
      acknowledged: 1,
      total24h: 2,
      overrides: 0,
    });
    expect(result.filteredAlerts().map((alert) => alert.id)).toEqual(['warning', 'old']);

    await result.handleAlertAcknowledgement(activeAlerts().warning);

    expect(AlertsAPI.acknowledge).toHaveBeenCalledWith('warning');
    expect(updateAlert).toHaveBeenCalledWith(
      'warning',
      expect.objectContaining({ acknowledged: true }),
    );
    expect(notificationStore.success).toHaveBeenCalledWith('Alert acknowledged');
    expect(result.processingAlerts().has('warning')).toBe(true);

    vi.advanceTimersByTime(1500);
    expect(result.processingAlerts().has('warning')).toBe(false);

    await result.handleBulkAcknowledge();

    expect(AlertsAPI.bulkAcknowledge).toHaveBeenCalledWith(['warning', 'old']);
    expect(updateAlert).toHaveBeenCalledWith(
      'warning',
      expect.objectContaining({ acknowledged: true }),
    );
    expect(notificationStore.error).toHaveBeenCalledWith('Failed to acknowledge 1 alert.');
  });
});
