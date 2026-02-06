import { batch, createRoot, createSignal } from 'solid-js';
import { createStore, reconcile } from 'solid-js/store';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

type UseUnifiedResourcesModule = typeof import('@/hooks/useUnifiedResources');

const v2Resource = {
  id: 'node-1',
  type: 'host',
  name: 'node-1',
  status: 'online',
  lastSeen: '2026-02-06T12:00:00Z',
  sources: ['agent'],
  metrics: {
    cpu: { percent: 15 },
    memory: { used: 4 * 1024 * 1024, total: 8 * 1024 * 1024, percent: 50 },
    disk: { used: 30 * 1024 * 1024, total: 100 * 1024 * 1024, percent: 30 },
  },
  discoveryTarget: {
    resourceType: 'host',
    hostId: 'host-1',
    resourceId: 'host-1',
    hostname: 'pve1',
  },
};

const flushAsync = async () => {
  await Promise.resolve();
  await Promise.resolve();
};

describe('useUnifiedResources', () => {
  let apiFetchMock: ReturnType<typeof vi.fn>;
  let setWsState: ReturnType<typeof createStore>[1];
  let useUnifiedResources: UseUnifiedResourcesModule['useUnifiedResources'];

  beforeEach(async () => {
    vi.useFakeTimers();
    vi.resetModules();

    apiFetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ data: [v2Resource] }),
    });

    const [connected] = createSignal(true);
    const [initialDataReceived] = createSignal(true);
    const [state, _setWsState] = createStore({
      resources: [v2Resource],
      nodes: [],
      hosts: [],
      dockerHosts: [],
      kubernetesClusters: [],
      pbs: [],
      pmg: [],
      lastUpdate: '',
    });
    setWsState = _setWsState;

    const wsStore = { connected, initialDataReceived, state };

    vi.doMock('@/utils/apiClient', () => ({
      apiFetch: apiFetchMock,
    }));
    vi.doMock('@/stores/websocket-global', () => ({
      getGlobalWebSocketStore: () => wsStore,
    }));

    ({ useUnifiedResources } = await import('@/hooks/useUnifiedResources'));
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
    vi.resetModules();
  });

  it('refetches when lastUpdate changes even if resources are reconciled in place', async () => {
    let dispose = () => {};
    let result: ReturnType<UseUnifiedResourcesModule['useUnifiedResources']> | undefined;
    createRoot((d) => {
      dispose = d;
      result = useUnifiedResources();
    });

    await flushAsync();
    expect(apiFetchMock).toHaveBeenCalledTimes(1);
    const originalResourceRef = result!.resources()[0];

    // Initial effect schedules a debounced refresh once connected.
    vi.advanceTimersByTime(800);
    await flushAsync();
    expect(apiFetchMock).toHaveBeenCalledTimes(2);

    // Simulate a metric update where reconcile keeps array identity stable.
    batch(() => {
      setWsState(
        'resources',
        reconcile(
          [
            {
              ...v2Resource,
              metrics: {
                ...v2Resource.metrics,
                cpu: { percent: 42 },
              },
            },
          ],
          { key: 'id' },
        ),
      );
      setWsState('lastUpdate', '2026-02-06T12:00:01Z');
    });

    vi.advanceTimersByTime(799);
    await flushAsync();
    expect(apiFetchMock).toHaveBeenCalledTimes(2);

    vi.advanceTimersByTime(1);
    await flushAsync();
    expect(apiFetchMock).toHaveBeenCalledTimes(3);
    expect(result!.resources()[0]).toBe(originalResourceRef);
    expect(result!.resources()[0].discoveryTarget).toEqual({
      resourceType: 'host',
      hostId: 'host-1',
      resourceId: 'host-1',
      hostname: 'pve1',
    });

    dispose();
  });
});
