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
      });

      const { store, dispose } = await createStoreHarness();
      try {
        await waitForOpenTick();

        emitRawMessage(frameOfExactly(MAX_INBOUND_WEBSOCKET_MESSAGE_BYTES + 1));
        await flushMicrotasks();
        expect(store.state.resources).toHaveLength(1);

        // The server's withheld snapshot and this later REST response are not
        // guaranteed to be identical, so the delta must not patch the REST copy.
        emitMessage({
          type: 'rawData',
          data: {
            lastUpdate: 600,
            resourceDelta: {
              upserts: [{ id: 'agent-host-1', lastSeen: 200 }],
            },
          },
        });
        await flushMicrotasks();

        expect(store.state.resources).toHaveLength(1);
        expect(store.state.resources[0]?.lastSeen).toBe(100);
        expect(store.state.resources[0]?.name).toBe('host-1');
        expect(sentMessageTypes()).not.toContain('requestData');
      } finally {
        dispose();
      }
    });

    it('recovers immediately after a reconnect instead of inheriting the throttle', async () => {
      apiFetchJSONMock.mockResolvedValue({
        lastUpdate: 500,
        resources: [{ id: 'agent-host-1', type: 'agent', name: 'host-1', lastSeen: 100 }],
      });

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

        expect(apiFetchJSONMock).toHaveBeenCalledTimes(2);
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
        expect(apiFetchJSONMock).toHaveBeenCalledTimes(2);
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
