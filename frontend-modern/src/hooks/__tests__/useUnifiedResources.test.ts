import { batch, createRoot, createSignal } from 'solid-js';
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

describe('useUnifiedResources', () => {
  let apiFetchMock: ReturnType<typeof vi.fn>;
  let setWsState: SetStoreFunction<TestWsState>;
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
    expect(apiFetchMock).toHaveBeenNthCalledWith(
      1,
      '/api/v2/resources?type=host,pbs,pmg,k8s_cluster,k8s_node',
      { cache: 'no-store' },
    );
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

    await flushAsync();
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

    await flushAsync();

    const resources = result!.resources();
    expect(resources).toHaveLength(2);
    expect(resources.map((resource) => resource.id)).toEqual(['k8s-cluster-native', 'k8s-node-native']);

    dispose();
  });
});
