import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createRoot } from 'solid-js';

// Mirrors MAX_INBOUND_WEBSOCKET_MESSAGE_BYTES in the store under test.
const MAX_INBOUND_WEBSOCKET_MESSAGE_BYTES = 32 * 1024 * 1024;

const apiFetchJSONMock = vi.fn();
vi.mock('@/utils/apiClient', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/utils/apiClient')>();
  return {
    ...actual,
    apiFetchJSON: (...args: unknown[]) => apiFetchJSONMock(...args),
  };
});

interface MockWebSocketInstance {
  url: string;
  readyState: number;
  send: ReturnType<typeof vi.fn>;
  close: ReturnType<typeof vi.fn>;
  onopen: ((event: Event) => void) | null;
  onmessage: ((event: MessageEvent) => void) | null;
  onclose: ((event: CloseEvent) => void) | null;
  onerror: ((event: Event) => void) | null;
}

let mockWsInstance: MockWebSocketInstance | null = null;

const MockWebSocket = vi.fn().mockImplementation((url: string): MockWebSocketInstance => {
  const instance: MockWebSocketInstance = {
    url,
    readyState: 1,
    send: vi.fn(),
    close: vi.fn(),
    onopen: null,
    onmessage: null,
    onclose: null,
    onerror: null,
  };

  mockWsInstance = instance;

  setTimeout(() => {
    instance.onopen?.({} as Event);
  }, 0);

  return instance;
});

const installWebSocketMock = () => {
  vi.stubGlobal(
    'WebSocket',
    Object.assign(MockWebSocket, {
      CONNECTING: 0,
      OPEN: 1,
      CLOSING: 2,
      CLOSED: 3,
    }),
  );
};

const waitForOpenTick = async () => {
  vi.advanceTimersByTime(1);
  await Promise.resolve();
};

const emitMessage = (payload: unknown) => {
  if (!mockWsInstance?.onmessage) {
    throw new Error('WebSocket onmessage handler is not initialized');
  }
  mockWsInstance.onmessage({ data: JSON.stringify(payload) } as MessageEvent);
};

const emitRawMessage = (raw: string) => {
  if (!mockWsInstance?.onmessage) {
    throw new Error('WebSocket onmessage handler is not initialized');
  }
  mockWsInstance.onmessage({ data: raw } as MessageEvent);
};

// Builds a valid all-ASCII rawData frame of exactly `totalBytes`, so the inbound
// size guard can be exercised right at its boundary without allocating a
// realistic multi-thousand-resource estate.
const frameOfExactly = (totalBytes: number): string => {
  const prefix = '{"type":"rawData","data":{"lastUpdate":1,"pad":"';
  const suffix = '"}}';
  const fillerLength = totalBytes - prefix.length - suffix.length;
  if (fillerLength < 0) throw new Error(`totalBytes ${totalBytes} is too small`);
  return `${prefix}${'x'.repeat(fillerLength)}${suffix}`;
};

// The REST recovery path awaits a fetch, so the store settles over several
// microtask turns rather than synchronously.
const flushMicrotasks = async (turns = 6) => {
  for (let i = 0; i < turns; i += 1) {
    await Promise.resolve();
  }
};

const sentMessageTypes = () =>
  (mockWsInstance?.send.mock.calls ?? []).map((call) => JSON.parse(call[0] as string).type);

const createStoreHarness = async () => {
  const { createWebSocketStore } = await import('@/stores/websocket');
  let dispose = () => {};
  const store = createRoot((d) => {
    dispose = d;
    return createWebSocketStore('ws://localhost/ws');
  });
  return { store, dispose };
};

describe('websocket store unified resource contract', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.resetModules();
    mockWsInstance = null;
    MockWebSocket.mockClear();
    apiFetchJSONMock.mockReset();
    installWebSocketMock();
  });

  afterEach(() => {
    vi.clearAllTimers();
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('initializes with empty resources array only', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      expect(store.state.resources).toEqual([]);
      expect((store.state as unknown as Record<string, unknown>).nodes).toBeUndefined();
    } finally {
      dispose();
    }
  });

  it('processes unified-only payload (resources populated, no legacy arrays)', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();

      emitMessage({
        type: 'initialState',
        data: {
          connectedInfrastructure: [],
          resources: [
            { id: 'node-1', type: 'agent', name: 'pve1', status: 'online' },
            { id: 'vm-101', type: 'vm', name: 'web-server', status: 'running' },
          ],
          lastUpdate: 1739059200000,
          activeAlerts: [],
          recentlyResolved: [],
        },
      });

      expect(store.resourceChange().changedIds).toBeNull();

      expect(store.state.resources).toHaveLength(2);
      expect((store.state as unknown as Record<string, unknown>).nodes).toBeUndefined();
    } finally {
      dispose();
    }
  });

  it('processes mixed payload (resources + legacy arrays) with resources as canonical state', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();

      emitMessage({
        type: 'initialState',
        data: {
          connectedInfrastructure: [],
          resources: [{ id: 'node-1', type: 'agent', name: 'pve1', status: 'online' }],
          nodes: [{ id: 'node-1', name: 'pve1' }],
          lastUpdate: 1739059200000,
          activeAlerts: [],
          recentlyResolved: [],
        },
      });

      expect(store.state.resources).toHaveLength(1);
      expect((store.state as unknown as Record<string, unknown>).nodes).toBeUndefined();
    } finally {
      dispose();
    }
  });

  it('coalesces split host identities from realtime resource snapshots', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();

      emitMessage({
        type: 'initialState',
        data: {
          connectedInfrastructure: [],
          resources: [
            {
              id: 'agent-proxmox-delly',
              type: 'agent',
              name: 'delly',
              displayName: 'delly',
              platformId: 'delly',
              platformType: 'proxmox-pve',
              sourceType: 'api',
              sources: ['proxmox'],
              status: 'online',
              lastSeen: Date.now() - 1000,
              proxmox: {
                nodeName: 'delly',
                clusterName: 'homelab',
              },
              platformData: {
                sources: ['proxmox'],
                proxmox: {
                  nodeName: 'delly',
                  clusterName: 'homelab',
                },
              },
            },
            {
              id: 'agent-runtime-delly',
              type: 'agent',
              name: 'delly',
              displayName: 'delly',
              platformId: 'delly',
              platformType: 'agent',
              sourceType: 'agent',
              sources: ['agent'],
              status: 'online',
              lastSeen: Date.now(),
              agent: {
                hostname: 'delly',
                osName: 'Debian GNU/Linux',
              },
              platformData: {
                sources: ['agent'],
                agent: {
                  hostname: 'delly',
                  osName: 'Debian GNU/Linux',
                },
              },
            },
          ],
          lastUpdate: 1739059200000,
          activeAlerts: [],
          recentlyResolved: [],
        },
      });

      expect(store.state.resources).toHaveLength(1);
      expect(store.state.resources[0]?.id).toBe('agent-runtime-delly');
      expect(store.state.resources[0]?.sourceType).toBe('hybrid');
      expect(store.state.resources[0]?.proxmox).toMatchObject({ clusterName: 'homelab' });
      expect(store.state.resources[0]?.agent).toMatchObject({ osName: 'Debian GNU/Linux' });
    } finally {
      dispose();
    }
  });

  it('incremental update adds to resources without creating legacy fields', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();

      emitMessage({
        type: 'initialState',
        data: {
          connectedInfrastructure: [],
          resources: [{ id: 'node-1', type: 'agent', name: 'pve1', status: 'online' }],
          lastUpdate: 1739059200000,
          activeAlerts: [],
          recentlyResolved: [],
        },
      });

      emitMessage({
        type: 'rawData',
        data: {
          connectedInfrastructure: [],
          resources: [
            { id: 'node-1', type: 'agent', name: 'pve1', status: 'online' },
            { id: 'vm-101', type: 'vm', name: 'web-server', status: 'running' },
          ],
          lastUpdate: 1739059260000,
          activeAlerts: [],
          recentlyResolved: [],
        },
      });

      expect(store.state.resources).toHaveLength(2);
      expect((store.state as unknown as Record<string, unknown>).nodes).toBeUndefined();
    } finally {
      dispose();
    }
  });

  it('applies resource merge patches without replacing unchanged resource data', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();

      emitMessage({
        type: 'initialState',
        data: {
          connectedInfrastructure: [],
          resources: [
            {
              id: 'agent-1',
              type: 'agent',
              name: 'agent-1',
              status: 'online',
              lastSeen: 100,
              cpu: { current: 10 },
              platformData: {
                agent: { osName: 'Debian', agentVersion: '6.2.0' },
                disks: [{ name: 'sda', usage: 10 }],
              },
            },
            { id: 'vm-1', type: 'vm', name: 'vm-1', status: 'running' },
          ],
          lastUpdate: 100,
          activeAlerts: [],
          recentlyResolved: [],
        },
      });

      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 200,
          resourceDelta: {
            upserts: [
              {
                id: 'agent-1',
                lastSeen: 200,
                cpu: { current: 42 },
                status: null,
                platformData: { disks: [{ name: 'sda', usage: 20 }] },
              },
              { id: 'vm-2', type: 'vm', name: 'vm-2', status: 'running' },
            ],
            removed: ['vm-1'],
            order: ['vm-2', 'agent-1'],
          },
        },
      });

      expect(store.state.resources.map((resource) => resource.id)).toEqual(['vm-2', 'agent-1']);
      const agent = store.state.resources[1];
      expect(agent?.cpu?.current).toBe(42);
      expect(agent?.lastSeen).toBe(200);
      expect(agent?.platformData).toMatchObject({
        agent: { osName: 'Debian', agentVersion: '6.2.0' },
        disks: [{ name: 'sda', usage: 20 }],
      });
      // Deltas must land exactly where a full snapshot would: the canonical
      // merge keeps the previous status when the server payload omits it, so
      // the delta path has to agree with the full-snapshot path here.
      expect(agent?.status).toBe('online');
      expect(store.state.lastUpdate).toBe(200);
      expect(store.resourceChange().changedIds).toEqual(new Set(['agent-1', 'vm-2', 'vm-1']));
    } finally {
      dispose();
    }
  });

  it('commits metrics-only deltas with per-key change shapes and a catch-up meta history', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();

      emitMessage({
        type: 'initialState',
        data: {
          connectedInfrastructure: [],
          resources: [
            {
              id: 'vm-1',
              type: 'vm',
              name: 'vm-1',
              status: 'running',
              lastSeen: 100,
              cpu: { current: 10 },
              memory: { current: 20, total: 1024, used: 256 },
              tags: ['prod'],
              platformData: {
                sources: ['proxmox-pve'],
                vmid: 100,
                diskRead: 5,
                networkIn: 100,
              },
            },
            { id: 'vm-2', type: 'vm', name: 'vm-2', status: 'running', lastSeen: 100 },
          ],
          lastUpdate: 100,
          activeAlerts: [],
          recentlyResolved: [],
        },
      });
      const baseVersion = store.resourceChange().version;

      // Metrics-only tick: aligned commit, per-key change shape recorded with
      // platformData expanded into its leaves.
      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 200,
          resourceDelta: {
            upserts: [
              {
                id: 'vm-1',
                lastSeen: 200,
                cpu: { current: 55 },
                platformData: { diskRead: 9, networkIn: 300 },
              },
            ],
          },
        },
      });

      const row = store.state.resources[0];
      expect(row?.cpu?.current).toBe(55);
      expect(row?.lastSeen).toBe(200);
      expect(row?.tags).toEqual(['prod']);
      expect(row?.platformData).toMatchObject({ vmid: 100, diskRead: 9, networkIn: 300 });
      expect(store.resourceChange().changedKeys?.get('vm-1')).toEqual([
        'lastSeen',
        'cpu',
        'platformData.diskRead',
        'platformData.networkIn',
      ]);

      // Second tick with a structural key; the catch-up meta must union both
      // ticks per id.
      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 300,
          resourceDelta: {
            upserts: [
              { id: 'vm-1', cpu: { current: 60 } },
              { id: 'vm-2', tags: ['edge'] },
            ],
          },
        },
      });

      const meta = store.changedResourceMetaSince(baseVersion);
      expect(meta).not.toBeNull();
      expect(meta?.changedIds).toEqual(new Set(['vm-1', 'vm-2']));
      expect(meta?.changedKeys.get('vm-1')).toEqual([
        'lastSeen',
        'cpu',
        'platformData.diskRead',
        'platformData.networkIn',
      ]);
      expect(meta?.changedKeys.get('vm-2')).toEqual(['tags']);

      // A removal marks the row's change shape unknown.
      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 400,
          resourceDelta: { removed: ['vm-2'] },
        },
      });
      expect(store.changedResourceMetaSince(baseVersion)?.changedKeys.get('vm-2')).toBeNull();
      expect(store.state.resources.map((resource) => resource.id)).toEqual(['vm-1']);
    } finally {
      dispose();
    }
  });

  it('expands capabilitiesRef through the catalog and synthesizes omitted default policies', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();

      emitMessage({
        type: 'initialState',
        data: {
          connectedInfrastructure: [],
          capabilityCatalog: {
            cap1: [{ name: 'restart', type: 'common', description: 'Restart the guest' }],
          },
          resources: [
            {
              id: 'vm-1',
              type: 'vm',
              name: 'vm-1',
              status: 'running',
              capabilitiesRef: 'cap1',
            },
            {
              id: 'storage-1',
              type: 'storage',
              name: 'tank',
              status: 'online',
              policy: { sensitivity: 'sensitive', routing: { scope: 'local-first', redact: [] } },
            },
          ],
          lastUpdate: 100,
          activeAlerts: [],
          recentlyResolved: [],
        },
      });

      const vm = store.state.resources.find((resource) => resource.id === 'vm-1');
      expect(vm?.capabilities?.map((capability) => capability.name)).toEqual(['restart']);
      // Omitted policy means default posture; ingestion synthesizes it so every
      // policy consumer keeps seeing an inline policy.
      expect(vm?.policy).toEqual({
        sensitivity: 'internal',
        routing: { scope: 'cloud-summary', redact: [] },
      });
      const storage = store.state.resources.find((resource) => resource.id === 'storage-1');
      expect(storage?.policy?.sensitivity).toBe('sensitive');

      // A later delta can move a resource's ref; the catalog change rides the
      // same frame and the patched row re-expands.
      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 200,
          capabilityCatalog: {
            cap1: [{ name: 'restart', type: 'common', description: 'Restart the guest' }],
            cap2: [{ name: 'stop', type: 'common', description: 'Stop the guest' }],
          },
          resourceDelta: {
            upserts: [{ id: 'vm-1', capabilitiesRef: 'cap2' }],
            removed: [],
            order: ['vm-1', 'storage-1'],
          },
        },
      });
      const vmAfter = store.state.resources.find((resource) => resource.id === 'vm-1');
      expect(vmAfter?.capabilities?.map((capability) => capability.name)).toEqual(['stop']);

      // A posture transition back to default nulls the policy in the patch;
      // ingestion re-synthesizes the default instead of keeping the stale one.
      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 300,
          resourceDelta: {
            upserts: [{ id: 'storage-1', policy: null }],
            removed: [],
            order: ['vm-1', 'storage-1'],
          },
        },
      });
      const storageAfter = store.state.resources.find((resource) => resource.id === 'storage-1');
      expect(storageAfter?.policy).toEqual({
        sensitivity: 'internal',
        routing: { scope: 'cloud-summary', redact: [] },
      });
    } finally {
      dispose();
    }
  });

  it('serves unions from the bounded changed-id history so late consumers catch up incrementally', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();

      emitMessage({
        type: 'initialState',
        data: {
          connectedInfrastructure: [],
          resources: [
            { id: 'agent-1', type: 'agent', name: 'agent-1', status: 'online' },
            { id: 'vm-1', type: 'vm', name: 'vm-1', status: 'running' },
            { id: 'vm-2', type: 'vm', name: 'vm-2', status: 'running' },
          ],
          lastUpdate: 100,
          activeAlerts: [],
          recentlyResolved: [],
        },
      });
      // version 1: full snapshot — not coverable by the history.
      expect(store.changedResourceIdsSince(0)).toBeNull();

      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 200,
          resourceDelta: {
            upserts: [{ id: 'vm-1', lastSeen: 200 }],
            removed: [],
            order: ['agent-1', 'vm-1', 'vm-2'],
          },
        },
      });
      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 300,
          resourceDelta: {
            upserts: [{ id: 'vm-2', lastSeen: 300 }],
            removed: [],
            order: ['agent-1', 'vm-1', 'vm-2'],
          },
        },
      });

      // versions 2 and 3 are delta commits: a consumer still on version 1 gets
      // the union, one on version 2 gets just the last delta, and a current
      // consumer gets null (nothing to catch up).
      expect(store.changedResourceIdsSince(1)).toEqual(new Set(['vm-1', 'vm-2']));
      expect(store.changedResourceIdsSince(2)).toEqual(new Set(['vm-2']));
      expect(store.changedResourceIdsSince(3)).toBeNull();
      // version 1 was a full snapshot, so a span reaching before it is not
      // coverable and must force the caller onto the full-merge path.
      expect(store.changedResourceIdsSince(0)).toBeNull();

      emitMessage({
        type: 'initialState',
        data: {
          connectedInfrastructure: [],
          resources: [{ id: 'agent-1', type: 'agent', name: 'agent-1', status: 'online' }],
          lastUpdate: 400,
          activeAlerts: [],
          recentlyResolved: [],
        },
      });
      // A later full snapshot invalidates the accumulated history.
      expect(store.changedResourceIdsSince(3)).toBeNull();
      expect(store.changedResourceIdsSince(2)).toBeNull();
    } finally {
      dispose();
    }
  });

  it('keeps canonically merged hosts consistent with full snapshots across deltas (#1601)', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();

      // Two server resources that the client coalesces into ONE host: an
      // agent host and a docker host sharing a hostname. The server's
      // per-client delta baseline keeps both IDs, so deltas reference IDs
      // the merged client view no longer holds.
      emitMessage({
        type: 'initialState',
        data: {
          connectedInfrastructure: [],
          resources: [
            {
              id: 'agent-host-1',
              type: 'agent',
              name: 'docker-01',
              status: 'online',
              lastSeen: 100,
              sources: ['agent'],
            },
            {
              id: 'docker-host-1',
              type: 'agent',
              name: 'docker-01',
              status: 'online',
              lastSeen: 100,
              sources: ['docker'],
            },
          ],
          lastUpdate: 100,
          activeAlerts: [],
          recentlyResolved: [],
        },
      });

      expect(store.state.resources).toHaveLength(1);
      expect(store.state.resources[0]?.id).toBe('agent-host-1');

      // A merge patch for the coalesced-away docker ID must not surface a
      // typeless stub resource: the delta applies to the raw server baseline
      // and the canonical merge runs again on the result.
      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 200,
          resourceDelta: {
            upserts: [{ id: 'docker-host-1', lastSeen: 200 }],
          },
        },
      });

      expect(store.state.resources).toHaveLength(1);
      expect(store.state.resources[0]?.id).toBe('agent-host-1');
      expect(store.state.resources.every((resource) => Boolean(resource.type))).toBe(true);

      // Removing the agent side server-side must leave the surviving docker
      // host visible. Before the raw-baseline fix this removed the single
      // merged client resource outright and the docker host never came back.
      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 300,
          resourceDelta: {
            removed: ['agent-host-1'],
            order: ['docker-host-1'],
          },
        },
      });

      expect(store.state.resources).toHaveLength(1);
      expect(store.state.resources[0]?.id).toBe('docker-host-1');
      expect(store.state.resources[0]?.type).toBe('agent');
      expect(store.state.resources[0]?.lastSeen).toBe(200);
    } finally {
      dispose();
    }
  });

  it('coalesces hidden-tab resource deltas into one visible-state reconciliation', async () => {
    const visibility = vi.spyOn(document, 'visibilityState', 'get');
    visibility.mockReturnValue('visible');
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();
      emitMessage({
        type: 'initialState',
        data: {
          resources: [
            {
              id: 'vm-1',
              type: 'vm',
              name: 'vm-1',
              status: 'running',
              cpu: { current: 10 },
            },
          ],
          lastUpdate: 100,
          activeAlerts: [],
          recentlyResolved: [],
        },
      });

      visibility.mockReturnValue('hidden');
      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 200,
          resourceDelta: { upserts: [{ id: 'vm-1', cpu: { current: 20 } }] },
        },
      });
      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 300,
          resourceDelta: { upserts: [{ id: 'vm-1', cpu: { current: 30 } }] },
        },
      });

      expect(store.state.resources[0]?.cpu?.current).toBe(10);
      expect(store.state.lastUpdate).toBe(300);

      visibility.mockReturnValue('visible');
      document.dispatchEvent(new Event('visibilitychange'));

      expect(store.state.resources[0]?.cpu?.current).toBe(30);
    } finally {
      dispose();
      visibility.mockRestore();
    }
  });

  it('defers resource deltas while the operator scrolls and flushes at scroll idle', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();
      emitMessage({
        type: 'initialState',
        data: {
          resources: [
            {
              id: 'vm-1',
              type: 'vm',
              name: 'vm-1',
              status: 'running',
              cpu: { current: 10 },
            },
          ],
          lastUpdate: 100,
          activeAlerts: [],
          recentlyResolved: [],
        },
      });

      // A tick arriving mid-gesture must not reconcile the estate.
      window.dispatchEvent(new Event('scroll'));
      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 200,
          resourceDelta: { upserts: [{ id: 'vm-1', cpu: { current: 20 } }] },
        },
      });
      expect(store.state.resources[0]?.cpu?.current).toBe(10);

      // Scrolling continues: the idle recheck re-arms instead of flushing.
      vi.advanceTimersByTime(200);
      window.dispatchEvent(new Event('scroll'));
      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 300,
          resourceDelta: { upserts: [{ id: 'vm-1', cpu: { current: 30 } }] },
        },
      });
      vi.advanceTimersByTime(200);
      expect(store.state.resources[0]?.cpu?.current).toBe(10);
      // The realtime tick token defers too, so downstream consumers keyed on
      // it stay quiescent through the gesture.
      expect(store.state.lastUpdate).toBe(100);

      // Gesture idle: both deferred ticks land as one coalesced reconciliation.
      vi.advanceTimersByTime(600);
      expect(store.state.resources[0]?.cpu?.current).toBe(30);
      expect(store.state.lastUpdate).toBe(300);
    } finally {
      dispose();
    }
  });

  it('defers resource deltas while the operator types and flushes at input idle', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();
      emitMessage({
        type: 'initialState',
        data: {
          resources: [
            {
              id: 'vm-1',
              type: 'vm',
              name: 'vm-1',
              status: 'running',
              cpu: { current: 10 },
            },
          ],
          lastUpdate: 100,
          activeAlerts: [],
          recentlyResolved: [],
        },
      });

      // A tick landing mid-keystroke must not delay the input's response.
      window.dispatchEvent(new Event('keydown'));
      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 200,
          resourceDelta: { upserts: [{ id: 'vm-1', cpu: { current: 20 } }] },
        },
      });
      expect(store.state.resources[0]?.cpu?.current).toBe(10);

      // A pointer press extends the same input-active window.
      vi.advanceTimersByTime(200);
      window.dispatchEvent(new Event('pointerdown'));
      vi.advanceTimersByTime(200);
      expect(store.state.resources[0]?.cpu?.current).toBe(10);

      vi.advanceTimersByTime(600);
      expect(store.state.resources[0]?.cpu?.current).toBe(20);
      expect(store.state.lastUpdate).toBe(200);
    } finally {
      dispose();
    }
  });

  it('holds hidden-tab deferrals through an active scroll and lands them at idle', async () => {
    const visibility = vi.spyOn(document, 'visibilityState', 'get');
    visibility.mockReturnValue('visible');
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();
      emitMessage({
        type: 'initialState',
        data: {
          resources: [
            {
              id: 'vm-1',
              type: 'vm',
              name: 'vm-1',
              status: 'running',
              cpu: { current: 10 },
            },
          ],
          lastUpdate: 100,
          activeAlerts: [],
          recentlyResolved: [],
        },
      });

      visibility.mockReturnValue('hidden');
      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 200,
          resourceDelta: { upserts: [{ id: 'vm-1', cpu: { current: 20 } }] },
        },
      });

      // The tab returns while the operator is already scrolling: the deferred
      // tick must wait for scroll idle rather than landing mid-gesture.
      window.dispatchEvent(new Event('scroll'));
      visibility.mockReturnValue('visible');
      document.dispatchEvent(new Event('visibilitychange'));
      expect(store.state.resources[0]?.cpu?.current).toBe(10);

      vi.advanceTimersByTime(600);
      expect(store.state.resources[0]?.cpu?.current).toBe(20);
    } finally {
      dispose();
      visibility.mockRestore();
    }
  });

  it('drains scroll-queued deltas in arrival order before a hidden-tab tick applies', async () => {
    const visibility = vi.spyOn(document, 'visibilityState', 'get');
    visibility.mockReturnValue('visible');
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();
      emitMessage({
        type: 'initialState',
        data: {
          resources: [
            {
              id: 'vm-1',
              type: 'vm',
              name: 'vm-1',
              status: 'running',
              cpu: { current: 10 },
              memory: { used: 1 },
            },
          ],
          lastUpdate: 100,
          activeAlerts: [],
          recentlyResolved: [],
        },
      });

      // Tick A queues mid-scroll with a memory change tick B does not carry.
      window.dispatchEvent(new Event('scroll'));
      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 200,
          resourceDelta: { upserts: [{ id: 'vm-1', cpu: { current: 20 }, memory: { used: 5 } }] },
        },
      });

      // Tick B arrives hidden and applies in place — A must drain first so the
      // baseline never sees B's older sibling fields overwrite A's.
      visibility.mockReturnValue('hidden');
      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 300,
          resourceDelta: { upserts: [{ id: 'vm-1', cpu: { current: 30 } }] },
        },
      });
      expect(store.state.resources[0]?.cpu?.current).toBe(10);

      vi.advanceTimersByTime(600);
      visibility.mockReturnValue('visible');
      document.dispatchEvent(new Event('visibilitychange'));

      expect(store.state.resources[0]?.cpu?.current).toBe(30);
      expect(store.state.resources[0]?.memory?.used).toBe(5);
    } finally {
      dispose();
      visibility.mockRestore();
    }
  });

  it('defers the reporting projection during scroll and applies only the latest payload', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();
      emitMessage({
        type: 'initialState',
        data: {
          connectedInfrastructure: [{ id: 'infra-1', name: 'old' }],
          resources: [],
          lastUpdate: 100,
          activeAlerts: [],
          recentlyResolved: [],
        },
      });
      expect(store.state.connectedInfrastructure[0]?.name).toBe('old');

      window.dispatchEvent(new Event('scroll'));
      emitMessage({
        type: 'rawData',
        data: { lastUpdate: 200, connectedInfrastructure: [{ id: 'infra-1', name: 'mid' }] },
      });
      vi.advanceTimersByTime(200);
      window.dispatchEvent(new Event('scroll'));
      emitMessage({
        type: 'rawData',
        data: { lastUpdate: 300, connectedInfrastructure: [{ id: 'infra-1', name: 'new' }] },
      });
      expect(store.state.connectedInfrastructure[0]?.name).toBe('old');

      vi.advanceTimersByTime(800);
      expect(store.state.connectedInfrastructure[0]?.name).toBe('new');
    } finally {
      dispose();
    }
  });

  it('expands policyRef and aiSafeSummaryRef from the state catalogs at ingestion', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();
      emitMessage({
        type: 'initialState',
        data: {
          connectedInfrastructure: [],
          policyCatalog: {
            pol1: {
              sensitivity: 'sensitive',
              routing: { scope: 'local-first', redact: ['hostname'] },
            },
          },
          aiSafeSummaryCatalog: {
            sum1: 'storage resource; status online; redacted for cloud summary',
          },
          resources: [
            {
              id: 'storage-1',
              type: 'storage',
              name: 'tank',
              status: 'online',
              policyRef: 'pol1',
              aiSafeSummaryRef: 'sum1',
            },
            { id: 'vm-1', type: 'vm', name: 'vm-1', status: 'running' },
          ],
          lastUpdate: 100,
          activeAlerts: [],
          recentlyResolved: [],
        },
      });

      const storage = store.state.resources.find((resource) => resource.id === 'storage-1');
      expect(storage?.policy?.sensitivity).toBe('sensitive');
      expect(storage?.policy?.routing?.redact).toEqual(['hostname']);
      expect(storage?.aiSafeSummary).toBe(
        'storage resource; status online; redacted for cloud summary',
      );
      // A row without refs still synthesizes the default posture.
      const vm = store.state.resources.find((resource) => resource.id === 'vm-1');
      expect(vm?.policy?.sensitivity).toBe('internal');

      // A posture change moves the ref; the catalog change rides the same
      // frame and the patched row re-expands from the new entry.
      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 200,
          policyCatalog: {
            pol1: {
              sensitivity: 'sensitive',
              routing: { scope: 'local-first', redact: ['hostname'] },
            },
            pol2: { sensitivity: 'public', routing: { scope: 'cloud-summary' } },
          },
          resourceDelta: { upserts: [{ id: 'storage-1', policyRef: 'pol2' }] },
        },
      });
      const storageAfter = store.state.resources.find((resource) => resource.id === 'storage-1');
      expect(storageAfter?.policy?.sensitivity).toBe('public');
    } finally {
      dispose();
    }
  });

  it('applies infrastructure merge-patch deltas touching only changed items', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();
      emitMessage({
        type: 'initialState',
        data: {
          connectedInfrastructure: [
            { id: 'infra-1', name: 'host-1', status: 'online', lastSeen: 100 },
            { id: 'infra-2', name: 'host-2', status: 'online', lastSeen: 100 },
          ],
          resources: [],
          lastUpdate: 100,
          activeAlerts: [],
          recentlyResolved: [],
        },
      });
      const { unwrap } = await import('solid-js/store');
      const untouchedBefore = unwrap(store.state.connectedInfrastructure)[1];

      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 200,
          connectedInfrastructureDelta: { upserts: [{ id: 'infra-1', lastSeen: 200 }] },
        },
      });

      expect(store.state.connectedInfrastructure[0]?.lastSeen).toBe(200);
      expect(store.state.connectedInfrastructure[0]?.name).toBe('host-1');
      // The untouched item keeps its store identity: the sync hands reconcile
      // reference-equal objects so it never deep-walks the rest.
      expect(unwrap(store.state.connectedInfrastructure)[1]).toBe(untouchedBefore);

      // Removal and reorder ride the same keyed payload.
      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 300,
          connectedInfrastructureDelta: {
            upserts: [{ id: 'infra-3', name: 'host-3', status: 'online', lastSeen: 300 }],
            removed: ['infra-1'],
            order: ['infra-3', 'infra-2'],
          },
        },
      });
      expect(store.state.connectedInfrastructure.map((item) => item.id)).toEqual([
        'infra-3',
        'infra-2',
      ]);
    } finally {
      dispose();
    }
  });

  it('requests recovery for infrastructure deltas without a projection baseline', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();
      // No full payload has delivered connectedInfrastructure on this socket.
      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 200,
          connectedInfrastructureDelta: { upserts: [{ id: 'infra-1', lastSeen: 200 }] },
        },
      });
      expect(store.state.connectedInfrastructure).toEqual([]);
      expect(sentMessageTypes()).toContain('requestData');
    } finally {
      dispose();
    }
  });

  it('defers infrastructure deltas during operator input and coalesces them at idle', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();
      emitMessage({
        type: 'initialState',
        data: {
          connectedInfrastructure: [
            { id: 'infra-1', name: 'host-1', status: 'online', lastSeen: 100 },
          ],
          resources: [],
          lastUpdate: 100,
          activeAlerts: [],
          recentlyResolved: [],
        },
      });

      window.dispatchEvent(new Event('scroll'));
      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 200,
          connectedInfrastructureDelta: { upserts: [{ id: 'infra-1', lastSeen: 200 }] },
        },
      });
      vi.advanceTimersByTime(200);
      window.dispatchEvent(new Event('scroll'));
      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 300,
          connectedInfrastructureDelta: { upserts: [{ id: 'infra-1', lastSeen: 300 }] },
        },
      });
      expect(store.state.connectedInfrastructure[0]?.lastSeen).toBe(100);

      vi.advanceTimersByTime(800);
      expect(store.state.connectedInfrastructure[0]?.lastSeen).toBe(300);
    } finally {
      dispose();
    }
  });

  it('applies alert deltas immediately even while operator input is active', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();
      emitMessage({
        type: 'initialState',
        data: {
          resources: [],
          lastUpdate: 100,
          activeAlerts: [
            { id: 'alert-1', type: 'cpu', level: 'warning', resourceId: 'vm-1', value: 85 },
          ],
          recentlyResolved: [],
        },
      });
      expect(store.state.activeAlerts).toHaveLength(1);
      expect(store.state.activeAlerts[0]?.level).toBe('warning');

      // Alert lifecycle truth does not wait for input idle: a delta arriving
      // mid-gesture escalates and adds alerts immediately.
      window.dispatchEvent(new Event('scroll'));
      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 200,
          activeAlertsDelta: {
            upserts: [
              { id: 'alert-1', level: 'critical', value: 97 },
              { id: 'alert-2', type: 'memory', level: 'warning', resourceId: 'vm-2', value: 91 },
            ],
          },
        },
      });
      const byId = new Map(store.state.activeAlerts.map((alert) => [alert.id, alert]));
      expect(byId.get('alert-1')?.level).toBe('critical');
      expect(byId.get('alert-1')?.value).toBe(97);
      expect(byId.get('alert-1')?.resourceId).toBe('vm-1');
      expect(byId.get('alert-2')?.level).toBe('warning');

      // Resolution rides the same payload and is equally immediate.
      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 300,
          activeAlertsDelta: { removed: ['alert-1'] },
        },
      });
      expect(store.state.activeAlerts.map((alert) => alert.id)).toEqual(['alert-2']);
    } finally {
      dispose();
    }
  });

  it('requests recovery for alert deltas without an alert baseline', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();
      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 200,
          activeAlertsDelta: { upserts: [{ id: 'alert-1', level: 'warning' }] },
        },
      });
      expect(store.state.activeAlerts).toEqual([]);
      expect(sentMessageTypes()).toContain('requestData');
    } finally {
      dispose();
    }
  });

  it('drops scroll-queued deltas when a full snapshot supersedes their baseline', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();
      emitMessage({
        type: 'initialState',
        data: {
          resources: [
            {
              id: 'vm-1',
              type: 'vm',
              name: 'vm-1',
              status: 'running',
              cpu: { current: 10 },
            },
          ],
          lastUpdate: 100,
          activeAlerts: [],
          recentlyResolved: [],
        },
      });

      window.dispatchEvent(new Event('scroll'));
      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 200,
          resourceDelta: { upserts: [{ id: 'vm-1', cpu: { current: 20 } }] },
        },
      });
      expect(store.state.resources[0]?.cpu?.current).toBe(10);

      // A full snapshot replaces the baseline the queued delta referenced.
      emitMessage({
        type: 'rawData',
        data: {
          resources: [
            {
              id: 'vm-1',
              type: 'vm',
              name: 'vm-1',
              status: 'running',
              cpu: { current: 40 },
            },
          ],
          lastUpdate: 300,
        },
      });
      expect(store.state.resources[0]?.cpu?.current).toBe(40);

      // The idle flush must not resurrect the stale queued delta.
      vi.advanceTimersByTime(600);
      expect(store.state.resources[0]?.cpu?.current).toBe(40);
    } finally {
      dispose();
    }
  });

  // Regression: on estates large enough that the full snapshot exceeds the
  // inbound guard (~3100 resources at the ~2.7 KB/resource measured against a
  // real /api/state payload; ~12.8 MB at 5000), the client dropped initialState
  // and asked the server for another one over the same socket. That response is
  // the same oversized frame, so it was dropped too and the UI never hydrated.
  describe('oversized snapshot recovery', () => {
    it('processes a frame sitting exactly on the inbound byte limit', async () => {
      const { store, dispose } = await createStoreHarness();
      try {
        await waitForOpenTick();

        emitRawMessage(frameOfExactly(MAX_INBOUND_WEBSOCKET_MESSAGE_BYTES));
        await flushMicrotasks();

        // Accepted and parsed: no recovery of any kind was triggered.
        expect(apiFetchJSONMock).not.toHaveBeenCalled();
        expect(sentMessageTypes()).not.toContain('requestData');
        expect(store.state.lastUpdate).toBe(1);
      } finally {
        dispose();
      }
    });

    it('hydrates over REST when the snapshot exceeds the inbound byte limit by one byte', async () => {
      apiFetchJSONMock.mockResolvedValue({
        lastUpdate: 500,
        resources: [
          { id: 'agent-host-1', type: 'agent', name: 'host-1', lastSeen: 100 },
          { id: 'agent-host-2', type: 'agent', name: 'host-2', lastSeen: 100 },
        ],
      });

      const { store, dispose } = await createStoreHarness();
      try {
        await waitForOpenTick();

        emitRawMessage(frameOfExactly(MAX_INBOUND_WEBSOCKET_MESSAGE_BYTES + 1));
        await flushMicrotasks();

        expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/state');
        expect(store.state.resources).toHaveLength(2);
        expect(store.state.resources.map((resource) => resource.id)).toEqual([
          'agent-host-1',
          'agent-host-2',
        ]);
        expect(store.initialDataReceived()).toBe(true);
      } finally {
        dispose();
      }
    });

    it('does not treat an independently built REST snapshot as a delta baseline', async () => {
      apiFetchJSONMock.mockResolvedValue({
        lastUpdate: 500,
        resources: [{ id: 'agent-host-1', type: 'agent', name: 'host-1', lastSeen: 100 }],
        connectedInfrastructure: [
          { id: 'infra-1', name: 'host-1', status: 'online', lastSeen: 100 },
        ],
        activeAlerts: [
          { id: 'alert-1', type: 'cpu', level: 'warning', resourceId: 'agent-host-1', value: 85 },
        ],
      });

      const { store, dispose } = await createStoreHarness();
      try {
        await waitForOpenTick();

        emitRawMessage(frameOfExactly(MAX_INBOUND_WEBSOCKET_MESSAGE_BYTES + 1));
        await flushMicrotasks();
        expect(store.state.resources).toHaveLength(1);
        expect(store.state.connectedInfrastructure[0]?.lastSeen).toBe(100);
        expect(store.state.activeAlerts[0]?.level).toBe('warning');

        // The server's withheld snapshot and this later REST response are not
        // guaranteed to be identical, so none of the socket deltas may patch
        // their corresponding REST copies.
        emitMessage({
          type: 'rawData',
          data: {
            lastUpdate: 600,
            resourceDelta: {
              upserts: [{ id: 'agent-host-1', lastSeen: 200 }],
            },
            connectedInfrastructureDelta: {
              upserts: [{ id: 'infra-1', lastSeen: 200 }],
            },
            activeAlertsDelta: {
              upserts: [{ id: 'alert-1', level: 'critical', value: 99 }],
            },
          },
        });
        await flushMicrotasks();

        expect(store.state.resources).toHaveLength(1);
        expect(store.state.resources[0]?.lastSeen).toBe(100);
        expect(store.state.resources[0]?.name).toBe('host-1');
        expect(store.state.connectedInfrastructure[0]?.lastSeen).toBe(100);
        expect(store.state.activeAlerts[0]?.level).toBe('warning');
        expect(store.state.activeAlerts[0]?.value).toBe(85);
        expect(sentMessageTypes()).not.toContain('requestData');
      } finally {
        dispose();
      }
    });

    it('recovers immediately after a reconnect instead of inheriting the throttle', async () => {
      apiFetchJSONMock.mockImplementation((path: string) =>
        path === '/api/alerts/active'
          ? Promise.resolve([])
          : Promise.resolve({
              lastUpdate: 500,
              resources: [{ id: 'agent-host-1', type: 'agent', name: 'host-1', lastSeen: 100 }],
            }),
      );

      const { store, dispose } = await createStoreHarness();
      try {
        await waitForOpenTick();

        emitRawMessage(frameOfExactly(MAX_INBOUND_WEBSOCKET_MESSAGE_BYTES + 1));
        await flushMicrotasks();
        expect(apiFetchJSONMock).toHaveBeenCalledTimes(1);
        expect(store.state.resources).toHaveLength(1);

        // Drop the connection and let the reconnect timer fire.
        mockWsInstance?.onclose?.({ code: 1011, reason: 'network blip' } as CloseEvent);
        vi.advanceTimersByTime(5000);
        await flushMicrotasks();
        await waitForOpenTick();

        // The fresh connection re-sends the same oversized snapshot. Recovery
        // must not be suppressed by the previous connection's throttle window.
        emitRawMessage(frameOfExactly(MAX_INBOUND_WEBSOCKET_MESSAGE_BYTES + 1));
        await flushMicrotasks();

        expect(apiFetchJSONMock.mock.calls.filter(([path]) => path === '/api/state')).toHaveLength(
          2,
        );
        expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/alerts/active');
        expect(sentMessageTypes()).not.toContain('requestData');
        expect(store.state.resources).toHaveLength(1);
      } finally {
        dispose();
      }
    });

    it('never re-requests the snapshot over the socket once one was dropped', async () => {
      apiFetchJSONMock.mockRejectedValue(new Error('state endpoint unavailable'));

      const { store, dispose } = await createStoreHarness();
      try {
        await waitForOpenTick();

        emitRawMessage(frameOfExactly(MAX_INBOUND_WEBSOCKET_MESSAGE_BYTES + 1));
        await flushMicrotasks();
        expect(apiFetchJSONMock).toHaveBeenCalledTimes(1);

        // A baseline-less delta arrives while the estate is still unhydrated.
        // The old behaviour asked the socket for another full snapshot, which
        // the guard drops again — an endless 30s retry loop on an empty UI.
        emitMessage({
          type: 'rawData',
          data: {
            lastUpdate: 600,
            resourceDelta: { upserts: [{ id: 'agent-host-1', lastSeen: 200 }] },
          },
        });
        await flushMicrotasks();

        expect(sentMessageTypes()).not.toContain('requestData');
        expect(store.state.resources).toHaveLength(0);

        // Recovery stays throttled rather than hammering the endpoint.
        expect(apiFetchJSONMock).toHaveBeenCalledTimes(1);

        vi.advanceTimersByTime(30001);
        emitMessage({
          type: 'rawData',
          data: {
            lastUpdate: 700,
            resourceDelta: { upserts: [{ id: 'agent-host-1', lastSeen: 300 }] },
          },
        });
        await flushMicrotasks();

        expect(sentMessageTypes()).not.toContain('requestData');
        expect(apiFetchJSONMock.mock.calls.filter(([path]) => path === '/api/state')).toHaveLength(
          2,
        );
        expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/alerts/active');
      } finally {
        dispose();
      }
    });
  });

  it('requests a full snapshot instead of applying a delta without a baseline', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();

      emitMessage({
        type: 'rawData',
        data: {
          lastUpdate: 100,
          resourceDelta: {
            upserts: [{ id: 'agent-1', cpu: { current: 42 } }],
          },
        },
      });

      expect(store.state.resources).toHaveLength(0);
      const sentTypes = (mockWsInstance?.send.mock.calls ?? []).map(
        (call) => JSON.parse(call[0] as string).type,
      );
      expect(sentTypes).toContain('requestData');
    } finally {
      dispose();
    }
  });

  it('preserves connected infrastructure when raw updates omit that projection', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();

      emitMessage({
        type: 'initialState',
        data: {
          connectedInfrastructure: [
            {
              id: 'primary:agent:pve1',
              name: 'pve1',
              displayName: 'pve1',
              status: 'active',
              healthStatus: 'online',
              surfaces: [{ id: 'pve:pve1', kind: 'proxmox', label: 'PVE data' }],
            },
          ],
          resources: [{ id: 'node-1', type: 'agent', name: 'pve1', status: 'online' }],
          lastUpdate: 1739059200000,
          activeAlerts: [],
          recentlyResolved: [],
        },
      });

      emitMessage({
        type: 'rawData',
        data: {
          resources: [
            { id: 'node-1', type: 'agent', name: 'pve1', status: 'online' },
            { id: 'vm-101', type: 'vm', name: 'web-server', status: 'running' },
          ],
          lastUpdate: 1739059260000,
          activeAlerts: [],
          recentlyResolved: [],
        },
      });

      expect(store.state.connectedInfrastructure).toHaveLength(1);
      expect(store.state.connectedInfrastructure[0]?.id).toBe('primary:agent:pve1');
      expect(store.state.resources).toHaveLength(2);
    } finally {
      dispose();
    }
  });

  it('preserves canonical resource details when realtime payloads are thinner', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();

      emitMessage({
        type: 'initialState',
        data: {
          connectedInfrastructure: [],
          resources: [
            {
              id: 'node-1',
              type: 'agent',
              name: 'West Production A',
              displayName: 'West Production A',
              platformId: 'west-production-a',
              platformType: 'proxmox-pve',
              sourceType: 'hybrid',
              status: 'online',
              lastSeen: Date.now(),
              cpu: { current: 10 },
              diskIO: { readRate: 1250000, writeRate: 640000 },
              platformData: {
                proxmox: {
                  clusterName: 'Core Fabric',
                },
              },
            },
            {
              id: 'pbs-1',
              type: 'pbs',
              name: 'backup-vault',
              displayName: 'backup-vault',
              platformId: 'pbs-main',
              platformType: 'proxmox-pbs',
              sourceType: 'api',
              status: 'online',
              lastSeen: Date.now(),
              platformData: {
                pbs: {
                  hostname: '198.51.100.10',
                  version: '3.2.1',
                  connectionHealth: 'healthy',
                  datastoreCount: 2,
                },
              },
            },
          ],
          lastUpdate: 1739059200000,
          activeAlerts: [],
          recentlyResolved: [],
        },
      });

      emitMessage({
        type: 'rawData',
        data: {
          connectedInfrastructure: [],
          resources: [
            {
              id: 'node-1',
              type: 'agent',
              name: 'West Production A',
              displayName: 'West Production A',
              platformId: 'west-production-a',
              platformType: 'proxmox-pve',
              sourceType: 'hybrid',
              status: 'online',
              lastSeen: Date.now(),
              cpu: { current: 42 },
              platformData: {},
            },
            {
              id: 'pbs-1',
              type: 'pbs',
              name: 'backup-vault',
              displayName: 'backup-vault',
              platformId: 'pbs-main',
              platformType: 'proxmox-pbs',
              sourceType: 'api',
              status: 'online',
              lastSeen: Date.now(),
              platformData: {
                host: '198.51.100.10',
                version: '3.2.1',
                connectionHealth: 'healthy',
                numDatastores: 2,
              },
            },
          ],
          lastUpdate: 1739059260000,
          activeAlerts: [],
          recentlyResolved: [],
        },
      });

      const agent = store.state.resources.find((resource) => resource.id === 'node-1');
      const pbs = store.state.resources.find((resource) => resource.id === 'pbs-1');
      expect(agent?.cpu?.current).toBe(42);
      expect(agent?.diskIO).toEqual({ readRate: 1250000, writeRate: 640000 });
      expect(agent?.clusterId).toBe('Core Fabric');
      expect((pbs?.platformData as Record<string, unknown>)?.pbs).toMatchObject({
        hostname: '198.51.100.10',
        version: '3.2.1',
        connectionHealth: 'healthy',
        datastoreCount: 2,
      });
    } finally {
      dispose();
    }
  });

  it('accepts canonical alertResolved websocket payloads', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();

      emitMessage({
        type: 'alertResolved',
        data: {
          alertIdentifier: 'instance:node:100::metric/cpu',
        },
      });

      expect(store.state.resources).toEqual([]);
    } finally {
      dispose();
    }
  });

  it('keeps state.recentlyResolved aligned with the resolved-alert index', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();

      emitMessage({
        type: 'initialState',
        data: {
          connectedInfrastructure: [],
          resources: [],
          lastUpdate: 1739059200000,
          activeAlerts: [],
          recentlyResolved: [
            {
              id: 'resolved-1',
              type: 'cpu',
              level: 'warning',
              resourceId: 'node-1',
              resourceName: 'pve1',
              node: 'pve1',
              instance: 'default',
              message: 'CPU normalized',
              value: 25,
              threshold: 80,
              startTime: '2026-03-15T08:00:00.000Z',
              ackTime: '2026-03-15T08:01:00.000Z',
              ackUser: 'system',
              acknowledged: true,
              resolvedAt: '2026-03-15T08:02:00.000Z',
            },
          ],
        },
      });

      expect(store.state.recentlyResolved).toHaveLength(1);
      expect(store.state.recentlyResolved[0]?.id).toBe('resolved-1');
      expect(store.recentlyResolved['resolved-1']?.id).toBe('resolved-1');
    } finally {
      dispose();
    }
  });

  it('does not create a delayed reconnect socket after store disposal', async () => {
    const { dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();
      expect(MockWebSocket).toHaveBeenCalledTimes(1);

      mockWsInstance?.onclose?.({ code: 1011, reason: 'test disconnect' } as CloseEvent);

      // Dispose before the reconnect timer fires — should cancel reconnection
      dispose();

      vi.advanceTimersByTime(60000);
      await Promise.resolve();

      expect(MockWebSocket).toHaveBeenCalledTimes(1);
    } finally {
      // dispose is idempotent and keeps the test resilient if assertions fail.
      dispose();
    }
  });
});
