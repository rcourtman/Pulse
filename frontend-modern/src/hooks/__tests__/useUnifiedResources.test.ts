import { batch, createEffect, createRoot, createSignal } from 'solid-js';
import { createStore, reconcile, type SetStoreFunction } from 'solid-js/store';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

type UseUnifiedResourcesModule = typeof import('@/hooks/useUnifiedResources');

type TestWsState = {
  resources: typeof v2Resource[];
  nodes: unknown[];
  hosts: unknown[];
  dockerHosts: unknown[];
  kubernetesClusters: unknown[];
  pbs: unknown[];
  pmg: unknown[];
  lastUpdate: string;
};

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

const waitForResourceCount = async (getCount: () => number, expectedMin = 1) => {
  for (let i = 0; i < 20; i++) {
    if (getCount() >= expectedMin) {
      return;
    }
    await flushAsync();
  }
  throw new Error(`Timed out waiting for at least ${expectedMin} resources`);
};

describe('useUnifiedResources', () => {
  let apiFetchMock: ReturnType<typeof vi.fn>;
  let setWsState: SetStoreFunction<TestWsState>;
  let useUnifiedResources: UseUnifiedResourcesModule['useUnifiedResources'];
  let useStorageBackupsResources: UseUnifiedResourcesModule['useStorageBackupsResources'];
  let resetUnifiedResourcesCacheForTests: UseUnifiedResourcesModule['__resetUnifiedResourcesCacheForTests'];
  let eventBus: (typeof import('@/stores/events'))['eventBus'];

  beforeEach(async () => {
    vi.useFakeTimers();
    vi.resetModules();

    apiFetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ data: [v2Resource] }),
    });

    const [connected] = createSignal(true);
    const [initialDataReceived] = createSignal(true);
    const [state, _setWsState] = createStore<TestWsState>({
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
      getOrgID: () => 'default',
    }));
    vi.doMock('@/stores/websocket-global', () => ({
      getGlobalWebSocketStore: () => wsStore,
    }));

    ({
      useUnifiedResources,
      useStorageBackupsResources,
      __resetUnifiedResourcesCacheForTests: resetUnifiedResourcesCacheForTests,
    } = await import('@/hooks/useUnifiedResources'));
    ({ eventBus } = await import('@/stores/events'));
    resetUnifiedResourcesCacheForTests();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
    vi.resetModules();
  });

  it('refetches when lastUpdate changes even if resources are reconciled in place', async () => {
    let dispose = () => {};
    let result: ReturnType<UseUnifiedResourcesModule['useUnifiedResources']> | undefined;
    let firstResourceEffectRuns = 0;
    createRoot((d) => {
      dispose = d;
      result = useUnifiedResources();
      createEffect(() => {
        const first = result!.resources()[0];
        if (first) {
          firstResourceEffectRuns += 1;
        }
      });
    });

    await flushAsync();
    expect(apiFetchMock).toHaveBeenCalledTimes(1);
    await waitForResourceCount(() => result!.resources().length);
    expect(apiFetchMock).toHaveBeenNthCalledWith(
      1,
      '/api/resources?type=host%2Cpbs%2Cpmg%2Ck8s_cluster%2Ck8s_node&page=1&limit=100',
      { cache: 'no-store' },
    );
    const originalResourceRef = result!.resources()[0];
    const effectsAfterInitialFetch = firstResourceEffectRuns;

    vi.advanceTimersByTime(800);
    await flushAsync();
    expect(apiFetchMock).toHaveBeenCalledTimes(1);

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
    expect(apiFetchMock).toHaveBeenCalledTimes(1);

    vi.advanceTimersByTime(2000);
    await flushAsync();
    expect(apiFetchMock).toHaveBeenCalledTimes(2);
    expect(result!.resources()[0]).toBe(originalResourceRef);
    expect(firstResourceEffectRuns).toBe(effectsAfterInitialFetch);
    expect(result!.resources()[0].discoveryTarget).toEqual({
      resourceType: 'host',
      hostId: 'host-1',
      resourceId: 'host-1',
      hostname: 'pve1',
    });

    dispose();
  });

  it('falls back to proxmox temperature when agent temperature is unavailable', async () => {
    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        data: [
          {
            ...v2Resource,
            sources: ['proxmox'],
            agent: undefined,
            proxmox: { nodeName: 'pve1', clusterName: 'mock-cluster', uptime: 1234, temperature: 58.4 },
          },
        ],
      }),
    });

    let dispose = () => {};
    let result: ReturnType<UseUnifiedResourcesModule['useUnifiedResources']> | undefined;
    createRoot((d) => {
      dispose = d;
      result = useUnifiedResources();
    });

    await result!.refetch();
    expect(result!.resources()[0].temperature).toBe(58.4);

    dispose();
  });

  it('uses only backend v2 resources for infrastructure even when websocket has kubernetes state', async () => {
    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        data: [
          {
            id: 'k8s-cluster-native',
            type: 'k8s-cluster',
            name: 'cluster-a',
            status: 'online',
            lastSeen: '2026-02-06T12:00:00Z',
            sources: ['kubernetes'],
          },
          {
            id: 'k8s-node-native',
            type: 'k8s-node',
            name: 'worker-1',
            status: 'online',
            lastSeen: '2026-02-06T12:00:00Z',
            parentId: 'k8s-cluster-native',
            sources: ['kubernetes'],
          },
        ],
      }),
    });

    batch(() => {
      setWsState('kubernetesClusters', [
        {
          id: 'legacy-cluster',
          name: 'legacy-cluster',
          hidden: false,
          status: 'online',
          lastSeen: 1738929600000,
          nodes: [{ uid: 'legacy-node', name: 'legacy-node', ready: true }],
        },
      ]);
      setWsState('lastUpdate', '2026-02-06T12:00:02Z');
    });

    let dispose = () => {};
    let result: ReturnType<UseUnifiedResourcesModule['useUnifiedResources']> | undefined;
    createRoot((d) => {
      dispose = d;
      result = useUnifiedResources();
    });

    await result!.refetch();

    const resources = result!.resources();
    expect(resources).toHaveLength(2);
    expect(resources.map((resource) => resource.id)).toEqual(['k8s-cluster-native', 'k8s-node-native']);

    dispose();
  });

  it('keeps using v2 infrastructure resources when websocket legacy infra arrays are populated', async () => {
    batch(() => {
      setWsState('hosts', [
        { id: 'legacy-host-1', hostname: 'legacy-host', status: 'online', lastSeen: 1738929600000 },
      ]);
      setWsState('dockerHosts', [
        { id: 'legacy-docker-1', hostname: 'legacy-docker', status: 'online', lastSeen: 1738929600000 },
      ]);
      setWsState('pbs', [{ id: 'legacy-pbs-1', name: 'legacy-pbs' }]);
      setWsState('pmg', [{ id: 'legacy-pmg-1', name: 'legacy-pmg' }]);
      setWsState('lastUpdate', '2026-02-06T12:00:04Z');
    });

    let dispose = () => {};
    let result: ReturnType<UseUnifiedResourcesModule['useUnifiedResources']> | undefined;
    createRoot((d) => {
      dispose = d;
      result = useUnifiedResources();
    });

    await result!.refetch();

    const resources = result!.resources();
    expect(resources).toHaveLength(1);
    expect(resources[0].id).toBe('node-1');
    expect(resources[0].type).toBe('host');

    dispose();
  });

  it('reuses fresh cache on remount without an extra network fetch', async () => {
    let disposeFirst = () => {};
    let first: ReturnType<UseUnifiedResourcesModule['useUnifiedResources']> | undefined;
    createRoot((d) => {
      disposeFirst = d;
      first = useUnifiedResources();
    });

    await flushAsync();
    await waitForResourceCount(() => first!.resources().length);
    expect(apiFetchMock).toHaveBeenCalledTimes(1);

    disposeFirst();

    let disposeSecond = () => {};
    let second: ReturnType<UseUnifiedResourcesModule['useUnifiedResources']> | undefined;
    createRoot((d) => {
      disposeSecond = d;
      second = useUnifiedResources();
    });

    await flushAsync();
    expect(apiFetchMock).toHaveBeenCalledTimes(1);
    expect(second!.resources().length).toBeGreaterThan(0);

    disposeSecond();
  });

  it('coalesces burst websocket updates into a single delayed refetch', async () => {
    let dispose = () => {};
    createRoot((d) => {
      dispose = d;
      useUnifiedResources();
    });

    await flushAsync();
    expect(apiFetchMock).toHaveBeenCalledTimes(1);

    setWsState('lastUpdate', '2026-02-06T12:00:01Z');
    await flushAsync();
    vi.advanceTimersByTime(100);

    setWsState('lastUpdate', '2026-02-06T12:00:02Z');
    await flushAsync();
    vi.advanceTimersByTime(100);

    setWsState('lastUpdate', '2026-02-06T12:00:03Z');
    await flushAsync();

    vi.advanceTimersByTime(2_500);
    await flushAsync();
    expect(apiFetchMock).toHaveBeenCalledTimes(2);

    vi.advanceTimersByTime(2_500);
    await flushAsync();
    expect(apiFetchMock).toHaveBeenCalledTimes(2);

    dispose();
  });

  it('uses the storage/backups query variant for storage-backups pages', async () => {
    let dispose = () => {};
    let result: ReturnType<UseUnifiedResourcesModule['useStorageBackupsResources']> | undefined;
    createRoot((d) => {
      dispose = d;
      result = useStorageBackupsResources();
    });

    await flushAsync();
    expect(apiFetchMock).toHaveBeenCalledTimes(1);
    expect(apiFetchMock).toHaveBeenNthCalledWith(
      1,
      '/api/resources?type=storage%2Cpbs%2Cpmg%2Cvm%2Clxc%2Ccontainer%2Cpod%2Chost%2Ck8s_cluster%2Ck8s_node%2Cphysical_disk%2Cceph&page=1&limit=100',
      { cache: 'no-store' },
    );
    expect(result!.resources().length).toBeGreaterThanOrEqual(0);

    dispose();
  });

  it('scopes unified resource caches by org and restores prior cache when switching back', async () => {
    let dispose = () => {};
    let result: ReturnType<UseUnifiedResourcesModule['useUnifiedResources']> | undefined;
    createRoot((d) => {
      dispose = d;
      result = useUnifiedResources();
    });

    await flushAsync();
    await waitForResourceCount(() => result!.resources().length);
    expect(apiFetchMock).toHaveBeenCalledTimes(1);
    expect(result!.resources()[0]?.id).toBe('node-1');

    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        data: [
          {
            ...v2Resource,
            id: 'node-2',
            name: 'node-2',
          },
        ],
      }),
    });

    eventBus.emit('org_switched', 'tenant-b');
    await flushAsync();
    await waitForResourceCount(() => result!.resources().length);

    expect(apiFetchMock).toHaveBeenCalledTimes(2);
    expect(result!.resources()[0]?.id).toBe('node-2');

    eventBus.emit('org_switched', 'default');
    await flushAsync();

    expect(apiFetchMock).toHaveBeenCalledTimes(2);
    expect(result!.resources()[0]?.id).toBe('node-1');

    dispose();
  });
});
