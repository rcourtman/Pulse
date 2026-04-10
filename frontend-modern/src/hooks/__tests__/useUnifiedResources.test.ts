import { batch, createRoot, createSignal } from 'solid-js';
import { createStore, reconcile, type SetStoreFunction } from 'solid-js/store';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { State } from '@/types/api';
import type { Resource } from '@/types/resource';
import useUnifiedResourcesSource from '../useUnifiedResources.ts?raw';
import stringUtilsSource from '@/utils/stringUtils.ts?raw';

type UseUnifiedResourcesModule = typeof import('@/hooks/useUnifiedResources');

type TestWsState = State & Record<string, unknown>;

const v2Resource = {
  id: 'node-1',
  type: 'agent',
  name: 'node-1',
  status: 'online',
  lastSeen: '2026-02-06T12:00:00Z',
  sources: ['agent'],
  facetCounts: {
    recentChanges: 1,
  },
  metrics: {
    cpu: { percent: 15 },
    memory: { used: 4 * 1024 * 1024, total: 8 * 1024 * 1024, percent: 50 },
    disk: { used: 30 * 1024 * 1024, total: 100 * 1024 * 1024, percent: 30 },
  },
  discoveryTarget: {
    resourceType: 'agent',
    agentId: 'host-1',
    resourceId: 'host-1',
    hostname: 'pve1',
  },
};

const wsResource: Resource = {
  id: 'node-1',
  type: 'agent',
  name: 'node-1',
  displayName: 'node-1',
  platformId: 'node-1',
  platformType: 'agent',
  sourceType: 'agent',
  status: 'online',
  lastSeen: Date.parse('2026-02-06T12:00:00Z'),
  cpu: { current: 15 },
  memory: { current: 50, used: 4 * 1024 * 1024, total: 8 * 1024 * 1024, free: 4 * 1024 * 1024 },
  disk: {
    current: 30,
    used: 30 * 1024 * 1024,
    total: 100 * 1024 * 1024,
    free: 70 * 1024 * 1024,
  },
  discoveryTarget: {
    resourceType: 'agent',
    agentId: 'host-1',
    resourceId: 'host-1',
    hostname: 'pve1',
  },
  facetCounts: {
    recentChanges: 1,
  },
};

const createWsResource = (overrides: Partial<Resource> = {}): Resource => ({
  ...wsResource,
  ...overrides,
});

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

const waitForValue = async <T>(readValue: () => T, expected: T) => {
  for (let i = 0; i < 20; i++) {
    if (readValue() === expected) {
      return;
    }
    await flushAsync();
  }
  throw new Error(`Timed out waiting for expected value: ${String(expected)}`);
};

describe('useUnifiedResources', () => {
  let apiFetchMock: ReturnType<typeof vi.fn>;
  let setWsState: SetStoreFunction<TestWsState>;
  let setWsConnected: (value: boolean) => boolean;
  let setWsInitialDataReceived: (value: boolean) => boolean;
  let useUnifiedResources: UseUnifiedResourcesModule['useUnifiedResources'];
  let useStorageRecoveryResources: UseUnifiedResourcesModule['useStorageRecoveryResources'];
  let resetUnifiedResourcesCacheForTests: UseUnifiedResourcesModule['__resetUnifiedResourcesCacheForTests'];
  let eventBus: (typeof import('@/stores/events'))['eventBus'];

  beforeEach(async () => {
    vi.useFakeTimers();
    vi.resetModules();

    apiFetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ data: [v2Resource] }),
    });

    const [connected, _setConnected] = createSignal(true);
    const [initialDataReceived, _setInitialDataReceived] = createSignal(true);
    setWsConnected = _setConnected;
    setWsInitialDataReceived = _setInitialDataReceived;
    const [state, _setWsState] = createStore<TestWsState>({
      connectedInfrastructure: [],
      metrics: [],
      performance: {
        apiCallDuration: {},
        lastPollDuration: 0,
        pollingStartTime: '',
        totalApiCalls: 0,
        failedApiCalls: 0,
        cacheHits: 0,
        cacheMisses: 0,
      },
      connectionHealth: {},
      stats: {
        startTime: '',
        uptime: 0,
        pollingCycles: 0,
        webSocketClients: 0,
        version: '2.0.0',
      },
      activeAlerts: [],
      recentlyResolved: [],
      resources: [wsResource],
      lastUpdate: 0,
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
      useStorageRecoveryResources,
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

  it('uses canonical websocket resource updates without a REST refetch for supported queries', async () => {
    let dispose = () => {};
    let result: ReturnType<UseUnifiedResourcesModule['useUnifiedResources']> | undefined;
    createRoot((d) => {
      dispose = d;
      result = useUnifiedResources();
    });

    await flushAsync();
    expect(apiFetchMock).toHaveBeenCalledTimes(1);
    await waitForResourceCount(() => result!.resources().length);
    expect(apiFetchMock).toHaveBeenNthCalledWith(
      1,
      '/api/resources?type=agent%2Cdocker-host%2Cpbs%2Cpmg%2Ck8s-cluster%2Ck8s-node&page=1&limit=100',
      { cache: 'no-store' },
    );
    await flushAsync();
    await flushAsync();

    // Simulate a metric update where reconcile keeps array identity stable.
    batch(() => {
      setWsState(
        'resources',
        reconcile(
          [
            createWsResource({
              cpu: { current: 42 },
            }),
          ],
          { key: 'id' },
        ),
      );
      setWsState('lastUpdate', 1738843201000);
    });

    await flushAsync();
    expect(apiFetchMock).toHaveBeenCalledTimes(1);
    expect(result!.resources()[0]?.cpu?.current).toBe(42);
    expect(result!.resources()[0].discoveryTarget).toEqual({
      resourceType: 'agent',
      agentId: 'host-1',
      resourceId: 'host-1',
      hostname: 'pve1',
    });
    expect(result!.resources()[0].facetCounts).toEqual({
      recentChanges: 1,
    });

    dispose();
  });

  it('prefers websocket initial hydration over an immediate REST fetch for dashboard snapshots', async () => {
    setWsConnected(false);
    setWsInitialDataReceived(false);
    setWsState('resources', []);

    let dispose = () => {};
    let result: ReturnType<UseUnifiedResourcesModule['useUnifiedResources']> | undefined;
    createRoot((d) => {
      dispose = d;
      result = useUnifiedResources({
        query: '',
        cacheKey: 'all-resources',
        initialHydration: 'prefer-ws',
      });
    });

    await flushAsync();
    expect(apiFetchMock).not.toHaveBeenCalled();
    expect(result!.loading()).toBe(true);

    batch(() => {
      setWsConnected(true);
      setWsState(
        'resources',
        reconcile(
          [
            createWsResource({ id: 'node-1', type: 'agent' }),
            createWsResource({ id: 'storage-1', type: 'storage', name: 'storage-1' }),
          ],
          { key: 'id' },
        ),
      );
      setWsState('lastUpdate', 1738843202000);
      setWsInitialDataReceived(true);
    });

    await waitForResourceCount(() => result!.resources().length, 2);
    expect(apiFetchMock).not.toHaveBeenCalled();
    expect(result!.loading()).toBe(false);

    await vi.advanceTimersByTimeAsync(2_000);
    expect(apiFetchMock).not.toHaveBeenCalled();

    dispose();
  });

  it('skips unified resource hydration while disabled and resumes when enabled', async () => {
    let dispose = () => {};
    let result: ReturnType<UseUnifiedResourcesModule['useUnifiedResources']> | undefined;
    const [enabled, setEnabled] = createSignal(false);

    createRoot((d) => {
      dispose = d;
      result = useUnifiedResources({
        query: '',
        cacheKey: 'all-resources',
        enabled,
      });
    });

    await flushAsync();
    expect(apiFetchMock).not.toHaveBeenCalled();
    expect(result!.loading()).toBe(false);

    setEnabled(true);
    await flushAsync();
    expect(apiFetchMock).toHaveBeenCalledTimes(1);
    await waitForResourceCount(() => result!.resources().length);

    dispose();
  });

  it('falls back to REST when websocket initial hydration does not arrive in time', async () => {
    setWsConnected(false);
    setWsInitialDataReceived(false);
    setWsState('resources', []);

    let dispose = () => {};
    let result: ReturnType<UseUnifiedResourcesModule['useUnifiedResources']> | undefined;
    createRoot((d) => {
      dispose = d;
      result = useUnifiedResources({
        query: '',
        cacheKey: 'all-resources',
        initialHydration: 'prefer-ws',
      });
    });

    await flushAsync();
    expect(apiFetchMock).not.toHaveBeenCalled();

    await vi.advanceTimersByTimeAsync(1_250);
    await flushAsync();

    expect(apiFetchMock).toHaveBeenCalledTimes(1);
    expect(result!.resources().length).toBeGreaterThanOrEqual(1);

    dispose();
  });

  it('projects kubernetes clusterId from the canonical context prefix', async () => {
    apiFetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({
        data: [
          {
            ...v2Resource,
            type: 'pod',
            kubernetes: {
              clusterName: 'cluster-a',
              context: 'cluster-context',
              clusterId: 'cluster-a-id',
              podUid: 'pod-uid-1',
            },
          },
        ],
      }),
    });

    let dispose = () => {};
    let result: ReturnType<UseUnifiedResourcesModule['useUnifiedResources']> | undefined;
    createRoot((d) => {
      dispose = d;
      result = useUnifiedResources({ query: 'type=pod', cacheKey: 'pods' });
    });

    await result!.refetch();

    expect(result!.resources()[0]?.clusterId).toBe('cluster-a');

    dispose();
  });

  it('projects proxmox clusterId from the shared cluster helper', async () => {
    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        data: [
          {
            ...v2Resource,
            type: 'vm',
            proxmox: {
              nodeName: 'pve1',
              clusterName: 'cluster-b',
            },
          },
        ],
      }),
    });

    let dispose = () => {};
    let result: ReturnType<UseUnifiedResourcesModule['useUnifiedResources']> | undefined;
    createRoot((d) => {
      dispose = d;
      result = useUnifiedResources({ query: 'type=vm', cacheKey: 'vms' });
    });

    await result!.refetch();

    expect(result!.resources()[0]?.clusterId).toBe('cluster-b');

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
            proxmox: {
              nodeName: 'pve1',
              clusterName: 'mock-cluster',
              uptime: 1234,
              temperature: 58.4,
            },
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

  it('projects raw VMware sources onto the canonical vSphere platform model', async () => {
    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        data: [
          {
            ...v2Resource,
            id: 'vmware-host-1',
            name: 'esxi-01.lab.local',
            sources: ['vmware'],
            canonicalIdentity: {
              displayName: 'ESXi 01',
              hostname: 'esxi-01.lab.local',
              platformId: 'vmware-host-1',
            },
            vmware: {
              connectionId: 'vc-1',
              connectionName: 'Lab VC',
              vCenterHost: 'vc.lab.local',
              managedObjectId: 'host-101',
              entityType: 'host',
              datacenterName: 'DC1',
              clusterName: 'Compute-A',
            },
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
    expect(result!.resources()[0].platformType).toBe('vmware-vsphere');
    expect(result!.resources()[0].sourceType).toBe('api');
    expect(result!.resources()[0].platformData?.sources).toEqual(['vmware']);
    expect(result!.resources()[0].vmware).toMatchObject({
      connectionId: 'vc-1',
      managedObjectId: 'host-101',
      entityType: 'host',
      datacenterName: 'DC1',
      clusterName: 'Compute-A',
    });

    dispose();
  });

  it('maps discoveryTarget.agentId into canonical discovery agentId', async () => {
    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        data: [
          {
            ...v2Resource,
            discoveryTarget: {
              resourceType: 'agent',
              agentId: 'agent-discovery-1',
              resourceId: 'agent-discovery-1',
              hostname: 'pve1',
            },
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
    expect(result!.resources()[0].discoveryTarget).toEqual({
      resourceType: 'agent',
      agentId: 'agent-discovery-1',
      resourceId: 'agent-discovery-1',
      hostname: 'pve1',
    });

    dispose();
  });

  it('passes canonical policy metadata and aiSafeSummary through unchanged', async () => {
    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        data: [
          {
            ...v2Resource,
            policy: {
              sensitivity: 'restricted',
              routing: {
                scope: 'local-only',
                redact: ['hostname', 'ip-address', 'platform-id', 'alias'],
              },
            },
            aiSafeSummary: 'resource summary safe for remote AI use',
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
    expect(result!.resources()[0].policy).toEqual({
      sensitivity: 'restricted',
      routing: {
        scope: 'local-only',
        redact: ['hostname', 'ip-address', 'platform-id', 'alias'],
      },
    });
    expect(result!.resources()[0].aiSafeSummary).toBe('resource summary safe for remote AI use');

    dispose();
  });

  it('normalizes legacy k8s discovery targets to pod for frontend consumers', async () => {
    apiFetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({
        data: [
          {
            ...v2Resource,
            type: 'pod',
            discoveryTarget: {
              resourceType: 'k8s',
              agentId: 'cluster-agent-1',
              resourceId: 'pod/default/nginx',
              hostname: 'cluster-a',
            },
          },
        ],
      }),
    });

    let dispose = () => {};
    let result: ReturnType<UseUnifiedResourcesModule['useUnifiedResources']> | undefined;
    createRoot((d) => {
      dispose = d;
      result = useUnifiedResources({ query: 'type=pod', cacheKey: 'pods' });
    });

    await result!.refetch();
    expect(result!.resources()[0].discoveryTarget).toEqual({
      resourceType: 'pod',
      agentId: 'cluster-agent-1',
      resourceId: 'pod/default/nginx',
      hostname: 'cluster-a',
    });

    dispose();
  });

  it('normalizes legacy node metrics targets at the API load boundary', async () => {
    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        data: [
          {
            ...v2Resource,
            metricsTarget: {
              resourceType: 'node',
              resourceId: 'pve-node-1',
            },
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
    expect(result!.resources()[0].metricsTarget).toEqual({
      resourceType: 'agent',
      resourceId: 'pve-node-1',
    });

    dispose();
  });

  it('derives normalized platformId through the shared identity helper precedence', async () => {
    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        data: [
          {
            ...v2Resource,
            id: 'docker-runtime-1',
            type: 'agent',
            name: 'fallback-name',
            proxmox: undefined,
            agent: { hostname: 'agent-host.local' },
            docker: { hostname: 'docker-host.local' },
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
    expect(result!.resources()[0].platformId).toBe('agent-host.local');

    dispose();
  });

  it('prefers backend canonical identity fields over frontend fallback inference', async () => {
    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        data: [
          {
            ...v2Resource,
            id: 'hybrid-host-1',
            name: 'fallback-name',
            identity: {
              hostnames: ['legacy-host.local'],
            },
            proxmox: { nodeName: 'legacy-node' },
            canonicalIdentity: {
              displayName: 'Tower',
              hostname: 'tower.local',
              platformId: 'pve1',
              primaryId: 'node:instance-pve1',
              aliases: ['node:instance-pve1', 'instance-pve1', 'tower.local'],
            },
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
    expect(result!.resources()[0].name).toBe('Tower');
    expect(result!.resources()[0].displayName).toBe('Tower');
    expect(result!.resources()[0].platformId).toBe('pve1');
    expect(result!.resources()[0].identity?.hostname).toBe('tower.local');
    expect(result!.resources()[0].canonicalIdentity).toMatchObject({
      primaryId: 'node:instance-pve1',
      hostname: 'tower.local',
    });

    dispose();
  });

  it('preserves parentName from backend resources', async () => {
    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        data: [
          {
            ...v2Resource,
            id: 'storage-1',
            type: 'storage',
            name: 'local-zfs',
            parentId: 'agent-123',
            parentName: 'pve1',
          },
        ],
      }),
    });

    let dispose = () => {};
    let result: ReturnType<UseUnifiedResourcesModule['useUnifiedResources']> | undefined;
    createRoot((d) => {
      dispose = d;
      result = useUnifiedResources({ query: 'type=storage' });
    });

    await result!.refetch();
    expect(result!.resources()[0].parentId).toBe('agent-123');
    expect(result!.resources()[0].parentName).toBe('pve1');

    dispose();
  });

  it('uses backend resources as canonical infrastructure state even with non-canonical websocket fields', async () => {
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
      setWsState('legacy_clusters', [
        {
          id: 'legacy-cluster',
          name: 'legacy-cluster',
          hidden: false,
          status: 'online',
          lastSeen: 1738929600000,
          nodes: [{ uid: 'legacy-node', name: 'legacy-node', ready: true }],
        },
      ]);
      setWsState('lastUpdate', 1738843202000);
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
    expect(resources.map((resource) => resource.id)).toEqual([
      'k8s-cluster-native',
      'k8s-node-native',
    ]);

    dispose();
  });

  it('ignores non-canonical websocket payload fields for infrastructure resources', async () => {
    batch(() => {
      setWsState('legacy_hosts', [
        { id: 'legacy-host-1', hostname: 'legacy-host', status: 'online', lastSeen: 1738929600000 },
      ]);
      setWsState('legacy_docker_hosts', [
        {
          id: 'legacy-docker-1',
          hostname: 'legacy-docker',
          status: 'online',
          lastSeen: 1738929600000,
        },
      ]);
      setWsState('legacy_pbs', [{ id: 'legacy-pbs-1', name: 'legacy-pbs' }]);
      setWsState('legacy_pmg', [{ id: 'legacy-pmg-1', name: 'legacy-pmg' }]);
      setWsState('lastUpdate', 1738843204000);
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
    expect(resources[0].type).toBe('agent');

    dispose();
  });

  it('reuses fresh cache on remount without an extra network fetch', async () => {
    let disposeFirst = () => {};
    let first: ReturnType<UseUnifiedResourcesModule['useUnifiedResources']> | undefined;
    createRoot((d) => {
      disposeFirst = d;
      first = useUnifiedResources({ query: '', cacheKey: 'all-resources' });
    });

    await flushAsync();
    await waitForResourceCount(() => first!.resources().length);
    expect(apiFetchMock).toHaveBeenCalledTimes(1);
    await flushAsync();
    await flushAsync();

    disposeFirst();

    let disposeSecond = () => {};
    let second: ReturnType<UseUnifiedResourcesModule['useUnifiedResources']> | undefined;
    createRoot((d) => {
      disposeSecond = d;
      second = useUnifiedResources({ query: '', cacheKey: 'all-resources' });
    });

    await waitForResourceCount(() => second!.resources().length);
    expect(apiFetchMock).toHaveBeenCalledTimes(1);
    expect(second!.resources().length).toBeGreaterThan(0);

    disposeSecond();
  });

  it('seeds a narrower type-filtered cache from a fresh all-resources snapshot', async () => {
    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        data: [
          v2Resource,
          {
            ...v2Resource,
            id: 'storage-1',
            type: 'storage',
            name: 'tank',
          },
        ],
      }),
    });

    let disposeAll = () => {};
    let allResources: ReturnType<UseUnifiedResourcesModule['useUnifiedResources']> | undefined;
    createRoot((d) => {
      disposeAll = d;
      allResources = useUnifiedResources({ query: '', cacheKey: 'all-resources' });
    });

    await flushAsync();
    await waitForResourceCount(() => allResources!.resources().length, 2);
    expect(apiFetchMock).toHaveBeenCalledTimes(1);
    batch(() => {
      setWsState('resources', [
        wsResource,
        createWsResource({
          id: 'disk-1',
          type: 'physical_disk',
          name: 'nvme0n1',
          displayName: 'nvme0n1',
          platformId: 'disk-1',
        }),
      ]);
      setWsState('lastUpdate', 1738843202500);
    });
    await flushAsync();

    disposeAll();

    let disposeFiltered = () => {};
    let filtered: ReturnType<UseUnifiedResourcesModule['useUnifiedResources']> | undefined;
    createRoot((d) => {
      disposeFiltered = d;
      filtered = useUnifiedResources();
    });

    await flushAsync();
    expect(apiFetchMock).toHaveBeenCalledTimes(1);
    expect(filtered!.loading()).toBe(false);
    expect(filtered!.resources().map((resource) => resource.id)).toEqual(['node-1']);

    disposeFiltered();
  });

  it('seeds a physical-disk filtered cache from a fresh all-resources snapshot', async () => {
    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        data: [
          v2Resource,
          {
            ...v2Resource,
            id: 'disk-1',
            type: 'physical_disk',
            name: 'nvme0n1',
          },
        ],
      }),
    });

    let disposeAll = () => {};
    let allResources: ReturnType<UseUnifiedResourcesModule['useUnifiedResources']> | undefined;
    createRoot((d) => {
      disposeAll = d;
      allResources = useUnifiedResources({ query: '', cacheKey: 'all-resources' });
    });

    await flushAsync();
    await waitForResourceCount(() => allResources!.resources().length, 2);
    expect(apiFetchMock).toHaveBeenCalledTimes(1);
    batch(() => {
      setWsState('resources', [
        wsResource,
        createWsResource({
          id: 'disk-1',
          type: 'physical_disk',
          name: 'nvme0n1',
          displayName: 'nvme0n1',
          platformId: 'disk-1',
        }),
      ]);
      setWsState('lastUpdate', 1738843202501);
    });
    await flushAsync();
    await flushAsync();

    disposeAll();

    let disposeFiltered = () => {};
    let filtered: ReturnType<UseUnifiedResourcesModule['useUnifiedResources']> | undefined;
    createRoot((d) => {
      disposeFiltered = d;
      filtered = useUnifiedResources({ query: 'type=physical_disk' });
    });

    await waitForValue(() => filtered!.resources().length, 1);
    expect(apiFetchMock).toHaveBeenCalledTimes(1);
    expect(filtered!.loading()).toBe(false);
    expect(filtered!.resources().map((resource) => resource.id)).toEqual(['disk-1']);

    disposeFiltered();
  });

  it('treats an empty fresh snapshot as cacheable on remount', async () => {
    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ data: [] }),
    });
    batch(() => {
      setWsState('resources', []);
      setWsState('lastUpdate', 1738843202600);
    });

    let disposeFirst = () => {};
    let first: ReturnType<UseUnifiedResourcesModule['useUnifiedResources']> | undefined;
    createRoot((d) => {
      disposeFirst = d;
      first = useUnifiedResources();
    });

    await flushAsync();
    await waitForValue(() => first!.loading(), false);
    expect(apiFetchMock).toHaveBeenCalledTimes(1);
    expect(first!.resources()).toEqual([]);

    disposeFirst();

    let disposeSecond = () => {};
    let second: ReturnType<UseUnifiedResourcesModule['useUnifiedResources']> | undefined;
    createRoot((d) => {
      disposeSecond = d;
      second = useUnifiedResources();
    });

    await flushAsync();
    expect(apiFetchMock).toHaveBeenCalledTimes(1);
    expect(second!.loading()).toBe(false);
    expect(second!.resources()).toEqual([]);

    disposeSecond();
  });

  it('coalesces burst websocket updates into a single delayed refetch for unsupported queries', async () => {
    let dispose = () => {};
    createRoot((d) => {
      dispose = d;
      useUnifiedResources({ query: 'status=online', cacheKey: 'status-online' });
    });

    await flushAsync();
    expect(apiFetchMock).toHaveBeenCalledTimes(1);

    setWsState('lastUpdate', 1738843201000);
    await flushAsync();
    vi.advanceTimersByTime(100);

    setWsState('lastUpdate', 1738843202000);
    await flushAsync();
    vi.advanceTimersByTime(100);

    setWsState('lastUpdate', 1738843203000);
    await flushAsync();

    vi.advanceTimersByTime(2_500);
    await flushAsync();
    expect(apiFetchMock).toHaveBeenCalledTimes(2);

    vi.advanceTimersByTime(2_500);
    await flushAsync();
    expect(apiFetchMock).toHaveBeenCalledTimes(2);

    dispose();
  });

  it('replaces stale canonical resources through fallback refetches for unsupported queries', async () => {
    apiFetchMock
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          data: [
            {
              ...v2Resource,
              id: 'agent-old',
              name: 'seeded-agent-old',
            },
          ],
        }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          data: [
            {
              ...v2Resource,
              id: 'agent-new',
              name: 'seeded-agent-new',
              lastSeen: '2026-02-06T12:01:00Z',
            },
          ],
        }),
      });

    let dispose = () => {};
    let result: ReturnType<UseUnifiedResourcesModule['useUnifiedResources']> | undefined;
    createRoot((d) => {
      dispose = d;
      result = useUnifiedResources({ query: 'status=online', cacheKey: 'status-online' });
    });

    await flushAsync();
    await waitForResourceCount(() => result!.resources().length);
    expect(apiFetchMock).toHaveBeenCalledTimes(1);
    expect(result!.resources()[0]?.id).toBe('agent-old');
    expect(result!.resources()[0]?.name).toBe('seeded-agent-old');

    setWsState('lastUpdate', 1738843209000);
    await flushAsync();

    vi.advanceTimersByTime(2_500);
    await flushAsync();

    expect(apiFetchMock).toHaveBeenCalledTimes(2);
    await waitForValue(() => result!.resources()[0]?.id, 'agent-new');
    expect(result!.resources()[0]?.id).toBe('agent-new');
    expect(result!.resources()[0]?.name).toBe('seeded-agent-new');

    dispose();
  });

  it('uses the storage/recovery query variant for storage pages', async () => {
    let dispose = () => {};
    let result: ReturnType<UseUnifiedResourcesModule['useStorageRecoveryResources']> | undefined;
    createRoot((d) => {
      dispose = d;
      result = useStorageRecoveryResources();
    });

    await flushAsync();
    expect(apiFetchMock).toHaveBeenCalledTimes(1);
    expect(apiFetchMock).toHaveBeenNthCalledWith(
      1,
      '/api/resources?type=storage%2Cpbs%2Cpmg%2Cvm%2Csystem-container%2Cpod%2Cagent%2Ck8s-cluster%2Ck8s-node%2Cphysical_disk%2Cceph&page=1&limit=100',
      { cache: 'no-store' },
    );
    expect(result!.resources().length).toBeGreaterThanOrEqual(0);

    dispose();
  });

  it('normalizes legacy truenas resources to canonical agent records at ingest', async () => {
    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        data: [
          {
            ...v2Resource,
            id: 'truenas-main',
            type: 'truenas',
            name: 'truenas-main',
            sources: ['agent', 'truenas'],
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

    await waitForValue(() => result!.resources()[0]?.platformType, 'truenas');

    expect(result!.resources()[0]?.type).toBe('agent');
    expect(result!.resources()[0]?.platformType).toBe('truenas');

    dispose();
  });

  it('preserves resource facets from backend payloads', async () => {
    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        data: [
          {
            ...v2Resource,
            id: 'node-facets',
            capabilities: [
              {
                name: 'restart',
                type: 'common',
                description: 'Restart the resource safely.',
                minimumApprovalLevel: 'admin',
                platform: 'proxmox',
              },
            ],
            relationships: [
              {
                sourceId: 'node:pve-1',
                targetId: 'node-facets',
                type: 'runs_on',
                confidence: 0.98,
                active: true,
                discoverer: 'proxmox_adapter',
                observedAt: '2026-03-18T12:00:00Z',
                lastSeenAt: '2026-03-18T12:05:00Z',
              },
            ],
            recentChanges: [
              {
                id: 'change-1',
                observedAt: '2026-03-18T12:06:00Z',
                resourceId: 'node-facets',
                kind: 'capability_change',
                from: 'none',
                to: 'restart',
                sourceType: 'platform_event',
                sourceAdapter: 'proxmox_adapter',
                confidence: 'high',
              },
            ],
            facetCounts: {
              recentChanges: 1,
            },
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

    await waitForValue(() => result!.resources()[0]?.recentChanges?.[0]?.id, 'change-1');

    expect(result!.resources()[0]?.recentChanges).toEqual([
      {
        id: 'change-1',
        observedAt: '2026-03-18T12:06:00Z',
        resourceId: 'node-facets',
        kind: 'capability_change',
        from: 'none',
        to: 'restart',
        sourceType: 'platform_event',
        sourceAdapter: 'proxmox_adapter',
        confidence: 'high',
      },
    ]);
    expect(result!.resources()[0]?.facetCounts).toEqual({
      recentChanges: 1,
    });

    dispose();
  });

  it('filters malformed source entries from backend payloads', async () => {
    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        data: [
          {
            ...v2Resource,
            sources: ['agent', 42, null, { bad: true }, 'proxmox'],
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

    await waitForValue(() => result!.resources()[0]?.platformType, 'proxmox-pve');
    expect(apiFetchMock).toHaveBeenCalledTimes(1);
    expect(result!.resources()[0]?.id).toBe('node-1');
    expect(result!.resources()[0]?.platformType).toBe('proxmox-pve');

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
    await waitForValue(() => result!.resources()[0]?.id, 'node-2');

    expect(apiFetchMock).toHaveBeenCalledTimes(2);
    expect(result!.resources()[0]?.id).toBe('node-2');

    eventBus.emit('org_switched', 'default');
    await flushAsync();

    expect(apiFetchMock).toHaveBeenCalledTimes(2);
    expect(result!.resources()[0]?.id).toBe('node-1');

    dispose();
  });

  it('normalizes proxmox service aliases into canonical platform types', async () => {
    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        data: [
          { ...v2Resource, id: 'pve-1', name: 'pve-1', sources: ['proxmox'] },
          { ...v2Resource, id: 'pbs-1', name: 'pbs-1', sources: ['pbs'] },
          { ...v2Resource, id: 'pmg-1', name: 'pmg-1', sources: ['pmg'] },
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
    await waitForResourceCount(() => result!.resources().length, 3);

    const byId = new Map(result!.resources().map((resource) => [resource.id, resource]));
    expect(byId.get('pve-1')?.platformType).toBe('proxmox-pve');
    expect(byId.get('pbs-1')?.platformType).toBe('proxmox-pbs');
    expect(byId.get('pmg-1')?.platformType).toBe('proxmox-pmg');

    dispose();
  });

  it('uses the shared org scope helper for cache and fetch state', () => {
    expect(useUnifiedResourcesSource).toContain('normalizeOrgScope(getOrgID())');
    expect(useUnifiedResourcesSource).toContain('supportsCanonicalWsHydration');
    expect(useUnifiedResourcesSource).toContain("initialHydration === 'prefer-ws'");
    expect(useUnifiedResourcesSource).toContain('wsStore.state.resources');
    expect(useUnifiedResourcesSource).not.toContain('const DEFAULT_ORG_SCOPE = \'default\'');
    expect(useUnifiedResourcesSource).not.toContain('const normalizeOrgScope =');
    expect(useUnifiedResourcesSource).toContain('asTrimmedString');
    expect(useUnifiedResourcesSource).not.toContain(
      'const asTrimmedString = (value: unknown): string | undefined => {',
    );
    expect(stringUtilsSource).toContain('export const asTrimmedString');
  });
});
