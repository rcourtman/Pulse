import { cleanup, renderHook, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

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

afterEach(cleanup);

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

  type History = Awaited<ReturnType<typeof AlertsAPI.getHistory>>;
  const row = (id: string): History[number] =>
    ({
      id,
      type: 'cpu',
      level: 'warning',
      startTime: new Date(Date.now() - 60000).toISOString(),
      lastSeen: new Date().toISOString(),
      resourceId: `host-${id}`,
      resourceName: id,
      message: `Reading for ${id}`,
      acknowledged: false,
    }) as History[number];
  const deferred = <T,>() => {
    let resolve!: (value: T) => void;
    let reject!: (reason: Error) => void;
    const promise = new Promise<T>((yes, no) => {
      resolve = yes;
      reject = no;
    });
    return { promise, resolve, reject };
  };
  const mount = () =>
    renderHook(() =>
      useAlertHistoryState({
        activeAlerts: () => ({}),
        getResource: () => undefined,
        allResources: () => [],
      }),
    );
  const switchOrg = () => {
    const onSwitch = vi
      .mocked(eventBus.on)
      .mock.calls.find(([event]) => event === 'org_switched')![1];
    onSwitch('replacement');
  };

  it('exposes initial failure and admits only one explicit current-range retry', async () => {
    vi.mocked(AlertsAPI.getHistory).mockRejectedValueOnce(new Error('Private provider body'));
    const retry = deferred<History>();
    vi.mocked(AlertsAPI.getHistory).mockReturnValueOnce(retry.promise);
    const { result } = mount();
    await waitFor(() => expect(result.historyLoadError()).toBe(true));
    expect(result.loading()).toBe(false);
    const pending = result.retryHistory();
    result.retryHistory();
    expect(AlertsAPI.getHistory).toHaveBeenCalledTimes(2);
    expect(result.loading()).toBe(true);
    expect(result.historyLoadError()).toBe(true);
    retry.resolve([]);
    await pending;
    expect(result.historyLoadError()).toBe(false);
    expect(result.loading()).toBe(false);
    expect(result.alertData()).toEqual([]);
    expect(AlertsAPI.clearHistory).not.toHaveBeenCalled();
  });

  it('keeps same-range entries after a failed read without losing URL filters', async () => {
    setMockLocation('?period=30d&severity=warning&q=retained');
    vi.mocked(AlertsAPI.getHistory).mockResolvedValueOnce([row('retained')]);
    const { result } = mount();
    await waitFor(() => expect(result.loading()).toBe(false));
    vi.mocked(AlertsAPI.getHistory).mockRejectedValueOnce(new Error('Unavailable'));
    await result.retryHistory();
    expect(result.historyLoadError()).toBe(true);
    expect(result.alertData().map(({ id }) => id)).toEqual(['retained']);
    expect(result.searchTerm()).toBe('retained');
    expect(result.severityFilter()).toBe('warning');
    expect(result.timeFilter()).toBe('30d');
    expect(navigateSpy).not.toHaveBeenCalled();
  });

  it('does not reuse saved rows after a different-range read fails; live alerts remain', async () => {
    vi.mocked(AlertsAPI.getHistory).mockResolvedValueOnce([row('old-range')]);
    const live = row('live');
    const { result } = renderHook(() =>
      useAlertHistoryState({
        activeAlerts: () => ({ live }),
        getResource: () => undefined,
        allResources: () => [],
      }),
    );
    await waitFor(() => expect(result.loading()).toBe(false));
    vi.mocked(AlertsAPI.getHistory).mockRejectedValueOnce(new Error('Unavailable'));
    result.setTimeFilter('30d');
    await waitFor(() => expect(result.historyLoadError()).toBe(true));
    expect(result.alertHistory().map(({ id }) => id)).toEqual(['old-range']);
    expect(result.alertData().map(({ id }) => id)).toEqual(['live']);
    vi.mocked(AlertsAPI.getHistory).mockResolvedValueOnce([row('new-range')]);
    await result.retryHistory();
    expect(result.alertData().map(({ id }) => id)).toEqual(
      expect.arrayContaining(['live', 'new-range']),
    );
    expect(result.historyLoadError()).toBe(false);
  });

  it.each(['obsolete-success', 'obsolete-failure'] as const)(
    'ignores %s after a newer range result',
    async (kind) => {
      const old = deferred<History>();
      vi.mocked(AlertsAPI.getHistory)
        .mockReturnValueOnce(old.promise)
        .mockResolvedValueOnce([row('latest')]);
      const { result } = mount();
      result.setTimeFilter('30d');
      await waitFor(() => expect(result.alertHistory()[0]?.id).toBe('latest'));
      if (kind === 'obsolete-success') old.resolve([row('obsolete')]);
      else old.reject(new Error('Obsolete failure'));
      await old.promise.catch(() => {});
      expect(result.alertHistory()[0]?.id).toBe('latest');
      expect(result.historyLoadError()).toBe(false);
      expect(result.loading()).toBe(false);
    },
  );

  it('invalidates old-context entries and errors when the organisation changes', async () => {
    vi.mocked(AlertsAPI.getHistory).mockResolvedValueOnce([row('previous-org')]);
    const { result } = mount();
    await waitFor(() => expect(result.loading()).toBe(false));
    const oldRetry = deferred<History>();
    vi.mocked(AlertsAPI.getHistory)
      .mockReturnValueOnce(oldRetry.promise)
      .mockResolvedValueOnce([row('replacement-org')]);
    const pending = result.retryHistory();
    switchOrg();
    expect(result.alertHistory()).toEqual([]);
    await waitFor(() => expect(result.alertHistory()[0]?.id).toBe('replacement-org'));
    oldRetry.reject(new Error('Old org unavailable'));
    await pending;
    expect(result.historyLoadError()).toBe(false);
    expect(result.alertHistory()[0]?.id).toBe('replacement-org');
  });

  it.each([true, false])(
    'does not settle a retired-context clear (success=%s) into new history',
    async (success) => {
      vi.mocked(AlertsAPI.getHistory)
        .mockResolvedValueOnce([row('previous-org')])
        .mockResolvedValueOnce([row('replacement-org')]);
      const clear = deferred<Awaited<ReturnType<typeof AlertsAPI.clearHistory>>>();
      vi.mocked(AlertsAPI.clearHistory).mockReturnValueOnce(clear.promise);
      const { result } = mount();
      await waitFor(() => expect(result.loading()).toBe(false));
      const pending = result.clearAlertHistory();
      switchOrg();
      await waitFor(() => expect(result.alertHistory()[0]?.id).toBe('replacement-org'));
      if (success) clear.resolve(undefined as never);
      else clear.reject(new Error('Previous org unavailable'));
      await pending;
      expect(result.alertHistory()[0]?.id).toBe('replacement-org');
      expect(result.historyLoadError()).toBe(false);
    },
  );

  it('does not apply a late failure or dispatch another retry after disposal', async () => {
    const old = deferred<History>();
    vi.mocked(AlertsAPI.getHistory).mockReturnValueOnce(old.promise);
    const { result, cleanup: dispose } = mount();
    dispose();
    old.reject(new Error('Disposed read'));
    await old.promise.catch(() => {});
    result.retryHistory();
    expect(AlertsAPI.getHistory).toHaveBeenCalledTimes(1);
    expect(result.historyLoadError()).toBe(false);
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

  it.each([true, false])(
    'handles a pending fetch when clearing history succeeds=%s',
    async (succeeds) => {
      type History = Awaited<ReturnType<typeof AlertsAPI.getHistory>>;
      let resolveHistory!: (value: History) => void;
      const pendingHistory = new Promise<History>((resolve) => {
        resolveHistory = resolve;
      });
      vi.mocked(AlertsAPI.getHistory).mockReturnValueOnce(pendingHistory);
      if (succeeds) {
        vi.mocked(AlertsAPI.clearHistory).mockResolvedValue(undefined as any);
      } else {
        vi.mocked(AlertsAPI.clearHistory).mockRejectedValue(new Error('Unavailable'));
      }
      const staleHistory = [
        {
          id: 'old-alert',
          type: 'cpu',
          level: 'warning',
          startTime: new Date().toISOString(),
          lastSeen: new Date().toISOString(),
          resourceId: 'resource-1',
          resourceName: 'db-01',
          message: 'CPU high',
          acknowledged: false,
        },
      ] as History;
      const { result } = renderHook(() =>
        useAlertHistoryState({
          activeAlerts: () => ({}),
          getResource: () => undefined,
          allResources: () => [],
        }),
      );
      await waitFor(() => expect(AlertsAPI.getHistory).toHaveBeenCalledTimes(1));
      expect(result.loading()).toBe(true);

      await result.clearAlertHistory();
      resolveHistory(staleHistory);
      await pendingHistory;
      await waitFor(() => expect(result.loading()).toBe(false));
      expect(result.alertHistory()).toEqual(succeeds ? [] : staleHistory);

      // Successful clearing must not suppress subsequent range refreshes.
      vi.mocked(AlertsAPI.getHistory).mockResolvedValue(staleHistory);
      result.setTimeFilter('24h');
      await waitFor(() => expect(AlertsAPI.getHistory).toHaveBeenCalledTimes(2));
      await waitFor(() => expect(result.alertHistory()).toEqual(staleHistory));
    },
  );

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
