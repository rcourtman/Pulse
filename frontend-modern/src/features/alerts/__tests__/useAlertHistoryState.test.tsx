import { renderHook, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { AlertsAPI } from '@/api/alerts';
import { eventBus } from '@/stores/events';

import { useAlertHistoryState } from '../useAlertHistoryState';

vi.mock('@/api/alerts', () => ({
  AlertsAPI: {
    addIncidentNote: vi.fn(),
    clearHistory: vi.fn(),
    getHistory: vi.fn(),
    getIncidentTimeline: vi.fn(),
    getIncidentsForResource: vi.fn(),
  },
}));

vi.mock('@/stores/events', () => ({
  eventBus: {
    on: vi.fn(() => vi.fn()),
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

describe('useAlertHistoryState', () => {
  beforeEach(() => {
    vi.mocked(AlertsAPI.getHistory).mockReset();
    vi.mocked(AlertsAPI.getIncidentsForResource).mockReset();
    vi.mocked(AlertsAPI.clearHistory).mockReset();
    vi.mocked(eventBus.on).mockClear();
    vi.stubGlobal('confirm', vi.fn(() => true));
    localStorage.clear();
  });

  it('owns alert history fetch, filters, resource incidents, and clear behavior outside the render tab', async () => {
    const [activeAlerts] = createSignal({});
    const now = Date.now();
    const startTime = new Date(now - 30 * 60 * 1000).toISOString();
    const lastSeen = new Date(now - 10 * 60 * 1000).toISOString();

    vi.mocked(AlertsAPI.getHistory).mockResolvedValue([
      {
        id: 'alert-1',
        type: 'cpu',
        level: 'warning',
        startTime,
        lastSeen,
        resourceId: 'resource-1',
        resourceName: 'db-01',
        message: 'CPU high',
        acknowledged: false,
      },
    ] as any);
    vi.mocked(AlertsAPI.getIncidentsForResource).mockResolvedValue([
      {
        id: 'incident-1',
        alertType: 'CPU',
        level: 'warning',
        status: 'resolved',
        openedAt: startTime,
        closedAt: lastSeen,
        events: [],
      },
    ] as any);
    vi.mocked(AlertsAPI.clearHistory).mockResolvedValue(undefined as any);

    const { result } = renderHook(() =>
      useAlertHistoryState({
        activeAlerts,
        getResource: () => undefined,
        allResources: () => [],
      }),
    );

    await waitFor(() => expect(AlertsAPI.getHistory).toHaveBeenCalledTimes(1));
    expect(result.alertData()).toHaveLength(1);
    expect(eventBus.on).toHaveBeenCalledWith('org_switched', expect.any(Function));

    await result.openResourceIncidentPanel('resource-1', 'db-01');

    expect(AlertsAPI.getIncidentsForResource).toHaveBeenCalledWith('resource-1', 10);
    expect(result.resourceIncidentPanel()).toEqual({
      resourceId: 'resource-1',
      resourceName: 'db-01',
    });
    expect(result.resourceIncidents()['resource-1']).toHaveLength(1);

    result.setTimeFilter('24h');
    await waitFor(() => expect(AlertsAPI.getHistory).toHaveBeenCalledTimes(2));

    await result.clearAlertHistory();
    expect(AlertsAPI.clearHistory).toHaveBeenCalledTimes(1);
    expect(result.alertHistory()).toEqual([]);
  });
});
