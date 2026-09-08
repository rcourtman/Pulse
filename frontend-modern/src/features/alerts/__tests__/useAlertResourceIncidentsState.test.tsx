import { renderHook } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { AlertsAPI } from '@/api/alerts';
import { notificationStore } from '@/stores/notifications';
import { useAlertResourceIncidentsState } from '../useAlertResourceIncidentsState';

vi.mock('@/api/alerts', () => ({ AlertsAPI: { getIncidentsForResource: vi.fn() } }));
vi.mock('@/stores/notifications', () => ({ notificationStore: { error: vi.fn() } }));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn() } }));

type Incidents = Awaited<ReturnType<typeof AlertsAPI.getIncidentsForResource>>;
function deferred() {
  let resolve!: (value: Incidents) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<Incidents>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}

// Request ownership covers success, failure and loading writes independently.
describe('resource incident request ownership', () => {
  beforeEach(() => vi.resetAllMocks());

  it('does not repopulate cleared history after reset', async () => {
    const pending = deferred();
    vi.mocked(AlertsAPI.getIncidentsForResource).mockReturnValueOnce(pending.promise);
    const { result } = renderHook(useAlertResourceIncidentsState);
    const load = result.openResourceIncidentPanel('host', 'Host', 'row');
    result.resetResourceIncidentsState();
    pending.resolve([]);
    await load;
    expect(result.resourceIncidents()).toEqual({});
    expect(result.resourceIncidentLoading()).toEqual({});
    expect(result.resourceIncidentError()).toEqual({});
    expect(result.resourceIncidentPanel()).toBeNull();
  });

  it('keeps the newer same-resource result when requests finish backwards', async () => {
    const old = deferred();
    const latest = [{ id: 'latest' }] as Incidents;
    vi.mocked(AlertsAPI.getIncidentsForResource)
      .mockReturnValueOnce(old.promise)
      .mockResolvedValueOnce(latest);
    const { result } = renderHook(useAlertResourceIncidentsState);
    const load = result.openResourceIncidentPanel('host', 'Host', 'row');
    await result.refreshResourceIncidentPanel();
    old.resolve([]);
    await load;
    expect(result.resourceIncidents().host).toEqual(latest);
  });

  it('does not clear newer loading state or report a superseded failure', async () => {
    const old = deferred();
    const current = deferred();
    vi.mocked(AlertsAPI.getIncidentsForResource)
      .mockReturnValueOnce(old.promise)
      .mockReturnValueOnce(current.promise);
    const { result } = renderHook(useAlertResourceIncidentsState);
    const load = result.openResourceIncidentPanel('host', 'Host', 'row');
    const refresh = result.refreshResourceIncidentPanel();
    old.reject(new Error('obsolete read'));
    await load;
    const loadingAfterOldFailure = result.resourceIncidentLoading().host;
    const errorAfterOldFailure = result.resourceIncidentError().host;
    const obsoleteNotifications = vi.mocked(notificationStore.error).mock.calls.length;
    current.resolve([]);
    await refresh;
    expect(loadingAfterOldFailure).toBe(true);
    expect(errorAfterOldFailure).toBe(false);
    expect(obsoleteNotifications).toBe(0);
    expect(result.resourceIncidentLoading().host).toBe(false);
  });

  it('ignores failed reads after disposal', async () => {
    const pending = deferred();
    vi.mocked(AlertsAPI.getIncidentsForResource).mockReturnValueOnce(pending.promise);
    const { result, cleanup } = renderHook(useAlertResourceIncidentsState);
    const load = result.openResourceIncidentPanel('host', 'Host', 'row');
    cleanup();
    pending.reject(new Error('disposed read'));
    await load;
    expect(result.resourceIncidentError().host).toBe(false);
    expect(notificationStore.error).not.toHaveBeenCalled();
  });

  it('keeps a reopened request owned after an older reset-era failure', async () => {
    const old = deferred();
    const current = deferred();
    vi.mocked(AlertsAPI.getIncidentsForResource)
      .mockReturnValueOnce(old.promise)
      .mockReturnValueOnce(current.promise);
    const { result } = renderHook(useAlertResourceIncidentsState);
    const load = result.openResourceIncidentPanel('host', 'Host', 'row');
    result.resetResourceIncidentsState();
    const reopened = result.openResourceIncidentPanel('host', 'Host', 'row');
    old.reject(new Error('reset-era failure'));
    await load;
    expect(result.resourceIncidentLoading().host).toBe(true);
    expect(result.resourceIncidentError().host).toBe(false);
    expect(notificationStore.error).not.toHaveBeenCalled();
    current.resolve([]);
    await reopened;
    expect(result.resourceIncidents().host).toEqual([]);
    expect(result.resourceIncidentLoading().host).toBe(false);
  });

  it('ignores an obsolete failure after the newer request succeeded', async () => {
    const old = deferred();
    vi.mocked(AlertsAPI.getIncidentsForResource)
      .mockReturnValueOnce(old.promise)
      .mockResolvedValueOnce([]);
    const { result } = renderHook(useAlertResourceIncidentsState);
    const load = result.openResourceIncidentPanel('host', 'Host', 'row');
    await result.refreshResourceIncidentPanel();
    old.reject(new Error('obsolete failure'));
    await load;
    expect(result.resourceIncidentError().host).toBe(false);
    expect(notificationStore.error).not.toHaveBeenCalled();
    expect(result.resourceIncidents().host).toEqual([]);
    expect(result.resourceIncidentLoading().host).toBe(false);
  });

  it('ignores successful reads and new loads after disposal', async () => {
    const pending = deferred();
    vi.mocked(AlertsAPI.getIncidentsForResource).mockReturnValueOnce(pending.promise);
    const { result, cleanup } = renderHook(useAlertResourceIncidentsState);
    const load = result.openResourceIncidentPanel('host', 'Host', 'row');
    cleanup();
    pending.resolve([]);
    await load;
    await result.refreshResourceIncidentPanel();
    await result.openResourceIncidentPanel('other', 'Other', 'other-row');
    expect(result.resourceIncidents()).toEqual({});
    expect(result.resourceIncidentLoading()).toEqual({ host: true });
    expect(result.resourceIncidentError()).toEqual({ host: false });
    expect(AlertsAPI.getIncidentsForResource).toHaveBeenCalledTimes(1);
  });

  it('retains independent resource results and reports current failures', async () => {
    const first = deferred();
    vi.mocked(AlertsAPI.getIncidentsForResource)
      .mockReturnValueOnce(first.promise)
      .mockRejectedValueOnce(new Error('current read'));
    const { result } = renderHook(useAlertResourceIncidentsState);
    const load = result.openResourceIncidentPanel('first', 'First', 'row-1');
    await result.openResourceIncidentPanel('second', 'Second', 'row-2');
    expect(notificationStore.error).toHaveBeenCalledTimes(1);
    expect(result.resourceIncidentError().second).toBe(true);
    first.resolve([]);
    await load;
    expect(result.resourceIncidents().first).toEqual([]);
    expect(result.resourceIncidentLoading()).toEqual({ first: false, second: false });
    expect(result.resourceIncidentError()).toEqual({ first: false, second: true });
  });
  it('preserves the current error when a superseded success arrives', async () => {
    const old = deferred();
    vi.mocked(AlertsAPI.getIncidentsForResource)
      .mockReturnValueOnce(old.promise)
      .mockRejectedValueOnce(new Error('current failure'));
    const { result } = renderHook(useAlertResourceIncidentsState);
    const load = result.openResourceIncidentPanel('host', 'Host', 'row');
    await result.refreshResourceIncidentPanel();
    old.resolve([]);
    await load;
    expect(result.resourceIncidentError().host).toBe(true);
    expect(result.resourceIncidents()).toEqual({});
    expect(notificationStore.error).toHaveBeenCalledTimes(1);
  });

  it('clears a current error on retry without discarding cached history', async () => {
    const cached = [{ id: 'retained' }] as Incidents;
    const retry = deferred();
    vi.mocked(AlertsAPI.getIncidentsForResource)
      .mockResolvedValueOnce(cached)
      .mockRejectedValueOnce(new Error('refresh failed'))
      .mockReturnValueOnce(retry.promise);
    const { result } = renderHook(useAlertResourceIncidentsState);
    await result.openResourceIncidentPanel('host', 'Host', 'row');
    await result.refreshResourceIncidentPanel();
    expect(result.resourceIncidentError().host).toBe(true);
    expect(result.resourceIncidents().host).toEqual(cached);
    const refresh = result.refreshResourceIncidentPanel();
    expect(result.resourceIncidentError().host).toBe(false);
    expect(result.resourceIncidentLoading().host).toBe(true);
    retry.resolve([]);
    await refresh;
    expect(result.resourceIncidentError().host).toBe(false);
    expect(result.resourceIncidents().host).toEqual([]);
    result.resetResourceIncidentsState();
    expect(result.resourceIncidentError()).toEqual({});
  });

  it('toggles the opening row and reuses loaded history for another row', async () => {
    vi.mocked(AlertsAPI.getIncidentsForResource).mockResolvedValue([]);
    const { result } = renderHook(useAlertResourceIncidentsState);
    await result.openResourceIncidentPanel('host', 'Host', 'row-1');
    await result.openResourceIncidentPanel('host', 'Host', 'row-1');
    expect(result.resourceIncidentPanel()).toBeNull();
    await result.openResourceIncidentPanel('host', 'Host', 'row-2');
    expect(result.resourceIncidentPanel()?.rowKey).toBe('row-2');
    expect(AlertsAPI.getIncidentsForResource).toHaveBeenCalledTimes(1);
    await result.refreshResourceIncidentPanel();
    expect(AlertsAPI.getIncidentsForResource).toHaveBeenCalledTimes(2);
  });

  it('does not request history without a resource or selection', async () => {
    const { result } = renderHook(useAlertResourceIncidentsState);
    await result.openResourceIncidentPanel('', 'Missing', 'row');
    await result.refreshResourceIncidentPanel();
    expect(AlertsAPI.getIncidentsForResource).not.toHaveBeenCalled();
  });
});
