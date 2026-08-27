import { renderHook, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { AlertsAPI } from '@/api/alerts';
import { eventBus } from '@/stores/events';

import { useAlertHistoryState } from '../useAlertHistoryState';

const mockRouterPathname = '/alerts/history';
const [mockRouterSearch, setMockRouterSearch] = createSignal('');

const setMockLocation = (search: string) => {
  setMockRouterSearch(search);
  if (typeof window !== 'undefined') {
    Object.defineProperty(window, 'location', {
      configurable: true,
      writable: true,
      value: {
        ...window.location,
        pathname: mockRouterPathname,
        search,
      },
    });
  }
};

const navigateSpy = vi.fn((path: string) => {
  const queryIndex = path.indexOf('?');
  setMockLocation(queryIndex >= 0 ? path.slice(queryIndex) : '');
});

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({
    get pathname() {
      return mockRouterPathname;
    },
    get search() {
      return mockRouterSearch();
    },
  }),
  useNavigate: () => navigateSpy,
}));

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
    navigateSpy.mockClear();
    setMockLocation('');
    vi.stubGlobal(
      'confirm',
      vi.fn(() => true),
    );
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

    await result.openResourceIncidentPanel('resource-1', 'db-01', 'row-1');

    expect(AlertsAPI.getIncidentsForResource).toHaveBeenCalledWith('resource-1', 10);
    expect(result.resourceIncidentPanel()).toEqual({
      resourceId: 'resource-1',
      resourceName: 'db-01',
      rowKey: 'row-1',
    });
    expect(result.resourceIncidents()['resource-1']).toHaveLength(1);

    // Re-opening from the same row closes the panel, the way the neighbouring
    // Timeline button toggles. A different row re-targets it instead.
    await result.openResourceIncidentPanel('resource-1', 'db-01', 'row-1');
    expect(result.resourceIncidentPanel()).toBeNull();

    await result.openResourceIncidentPanel('resource-1', 'db-01', 'row-2');
    expect(result.resourceIncidentPanel()?.rowKey).toBe('row-2');

    result.setTimeFilter('24h');
    await waitFor(() => expect(AlertsAPI.getHistory).toHaveBeenCalledTimes(2));

    await result.clearAlertHistory();
    expect(AlertsAPI.clearHistory).toHaveBeenCalledTimes(1);
    expect(result.alertHistory()).toEqual([]);
  });

  it('counts each severity chip from the same predicate the list filters with', async () => {
    const [activeAlerts] = createSignal({});
    const now = Date.now();
    const startTime = new Date(now - 30 * 60 * 1000).toISOString();
    const makeEntry = (id: string, level: string, resourceName: string) => ({
      id,
      type: 'cpu',
      level,
      startTime,
      lastSeen: startTime,
      resourceId: `resource-${id}`,
      resourceName,
      message: 'CPU high',
      acknowledged: false,
    });
    vi.mocked(AlertsAPI.getHistory).mockResolvedValue([
      makeEntry('alert-1', 'critical', 'db-01'),
      makeEntry('alert-2', 'warning', 'db-01'),
      makeEntry('alert-3', 'warning', 'web-01'),
      makeEntry('alert-4', 'info', 'control-01'),
    ] as any);

    const { result } = renderHook(() =>
      useAlertHistoryState({
        activeAlerts,
        getResource: () => undefined,
        allResources: () => [],
      }),
    );

    await waitFor(() => expect(result.countForSeverity('all')).toBe(4));
    expect(result.countForSeverity('critical')).toBe(1);
    expect(result.countForSeverity('warning')).toBe(2);
    expect(result.countForSeverity('info')).toBe(1);

    // Counts ignore the selected severity (each chip shows what its own
    // selection would render) but follow the search term.
    result.setSeverityFilter('critical');
    expect(result.countForSeverity('warning')).toBe(2);
    result.setSearchTerm('db-01');
    await waitFor(() => expect(result.countForSeverity('warning')).toBe(1));
    expect(result.countForSeverity('all')).toBe(2);
  });

  it('restores the informational severity filter from the canonical URL', async () => {
    const [activeAlerts] = createSignal({});
    vi.mocked(AlertsAPI.getHistory).mockResolvedValue([] as any);
    setMockLocation('?severity=info');

    const { result } = renderHook(() =>
      useAlertHistoryState({
        activeAlerts,
        getResource: () => undefined,
        allResources: () => [],
      }),
    );

    await waitFor(() => expect(AlertsAPI.getHistory).toHaveBeenCalledTimes(1));
    expect(result.severityFilter()).toBe('info');
  });

  it('clears search, period, and severity in one route write', async () => {
    const [activeAlerts] = createSignal({});
    vi.mocked(AlertsAPI.getHistory).mockResolvedValue([] as any);
    setMockLocation('?q=backup+failed&period=30d&severity=critical');

    const { result } = renderHook(() =>
      useAlertHistoryState({
        activeAlerts,
        getResource: () => undefined,
        allResources: () => [],
      }),
    );

    await waitFor(() => expect(AlertsAPI.getHistory).toHaveBeenCalledTimes(1));
    expect(result.activeFilterCount()).toBe(3);
    expect(result.searchTerm()).toBe('backup failed');
    expect(result.timeFilter()).toBe('30d');
    expect(result.severityFilter()).toBe('critical');

    navigateSpy.mockClear();
    result.clearFilters();

    expect(navigateSpy).toHaveBeenCalledTimes(1);
    expect(navigateSpy).toHaveBeenCalledWith('/alerts/history', { replace: true });
    expect(result.activeFilterCount()).toBe(0);
    expect(result.searchTerm()).toBe('');
    expect(result.timeFilter()).toBe('7d');
    expect(result.severityFilter()).toBe('all');
  });
});
