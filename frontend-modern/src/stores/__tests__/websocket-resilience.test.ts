import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createRoot } from 'solid-js';

// Mirrors MAX_INBOUND_WEBSOCKET_MESSAGE_BYTES in the store under test.
const MAX_INBOUND_WEBSOCKET_MESSAGE_BYTES = 32 * 1024 * 1024;

const notificationMocks = vi.hoisted(() => ({
  success: vi.fn(),
  error: vi.fn(),
  info: vi.fn(),
  warning: vi.fn(),
}));
const apiFetchJSONMock = vi.hoisted(() => vi.fn());

vi.mock('@/stores/notifications', () => ({
  notificationStore: notificationMocks,
}));
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

let currentInstance: MockWebSocketInstance | null = null;
const instances: MockWebSocketInstance[] = [];
let autoOpenSockets = true;

class MockWebSocketClass implements MockWebSocketInstance {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;

  url: string;
  readyState: number;
  send = vi.fn();
  close = vi.fn();
  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;

  constructor(url: string) {
    this.url = url;
    this.readyState = autoOpenSockets ? MockWebSocketClass.OPEN : MockWebSocketClass.CONNECTING;

    instances.push(this);
    currentInstance = this; // eslint-disable-line @typescript-eslint/no-this-alias -- test mock needs instance capture

    if (autoOpenSockets) {
      setTimeout(() => {
        this.readyState = MockWebSocketClass.OPEN;
        this.onopen?.({} as Event);
      }, 0);
    }
  }
}

const installWebSocketMock = () => {
  vi.stubGlobal('WebSocket', MockWebSocketClass);
};

const flushMicrotasks = async (turns = 6) => {
  for (let i = 0; i < turns; i += 1) {
    await Promise.resolve();
  }
};

const createStoreHarness = async () => {
  const { createWebSocketStore } = await import('@/stores/websocket');
  let dispose = () => {};
  const store = createRoot((d) => {
    dispose = d;
    return createWebSocketStore('ws://localhost/ws');
  });
  return { store, dispose };
};

describe('websocket store resilience', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.resetModules();
    autoOpenSockets = true;
    currentInstance = null;
    instances.length = 0;
    vi.setSystemTime(new Date('2026-05-14T08:00:00.000Z'));
    notificationMocks.success.mockClear();
    apiFetchJSONMock.mockReset();
    installWebSocketMock();
  });

  afterEach(() => {
    vi.clearAllTimers();
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('uses exponential reconnect backoff with deterministic jitter', async () => {
    autoOpenSockets = false;
    vi.spyOn(Math, 'random').mockReturnValue(0.5);

    const { dispose } = await createStoreHarness();
    try {
      const first = currentInstance;
      expect(first).not.toBeNull();
      first!.onclose?.({ code: 1006, reason: '' } as CloseEvent);

      vi.advanceTimersByTime(999);
      expect(instances).toHaveLength(1);
      vi.advanceTimersByTime(1);
      expect(instances).toHaveLength(2);

      const second = currentInstance;
      expect(second).not.toBeNull();
      second!.onclose?.({ code: 1006, reason: '' } as CloseEvent);

      vi.advanceTimersByTime(1999);
      expect(instances).toHaveLength(2);
      vi.advanceTimersByTime(1);
      expect(instances).toHaveLength(3);
    } finally {
      dispose();
    }
  });

  it('does not reconnect after cleanup', async () => {
    const { dispose } = await createStoreHarness();
    expect(currentInstance).not.toBeNull();

    dispose();
    currentInstance!.onclose?.({ code: 1006, reason: '' } as CloseEvent);
    vi.advanceTimersByTime(60000);

    expect(instances).toHaveLength(1);
  });

  it('forces reconnect on heartbeat timeout when server is silent', async () => {
    const { dispose } = await createStoreHarness();
    try {
      vi.advanceTimersByTime(1); // run onopen tick
      expect(currentInstance).not.toBeNull();

      vi.advanceTimersByTime(90000);
      expect(currentInstance!.close).toHaveBeenCalledWith(4000, 'Heartbeat timeout');
    } finally {
      dispose();
    }
  });

  it('recovers after a heartbeat close without letting retired callbacks disconnect the new socket', async () => {
    vi.spyOn(Math, 'random').mockReturnValue(0.5);
    apiFetchJSONMock.mockResolvedValue([]);
    const { store, dispose } = await createStoreHarness();
    try {
      vi.advanceTimersByTime(1);
      const silentSocket = currentInstance!;
      expect(store.connected()).toBe(true);

      vi.advanceTimersByTime(90_000);
      expect(silentSocket.close).toHaveBeenCalledWith(4000, 'Heartbeat timeout');
      // Model the browser completing the close handshake; the mock does not
      // emit close events automatically.
      silentSocket.readyState = MockWebSocketClass.CLOSED;
      silentSocket.onclose?.({ code: 4000, reason: 'Heartbeat timeout' } as CloseEvent);
      expect(store.connected()).toBe(false);
      expect(store.reconnecting()).toBe(true);

      vi.advanceTimersByTime(1_001);
      expect(instances).toHaveLength(2);
      expect(store.connected()).toBe(true);
      expect(store.reconnecting()).toBe(false);

      silentSocket.onclose?.({ code: 1006, reason: '' } as CloseEvent);
      expect(store.connected()).toBe(true);
      expect(store.reconnecting()).toBe(false);
      vi.advanceTimersByTime(2_000);
      expect(instances).toHaveLength(2);
    } finally {
      dispose();
    }
  });

  it('does not let malformed messages postpone detection of a silent server', async () => {
    apiFetchJSONMock.mockResolvedValue([]);
    const { dispose } = await createStoreHarness();
    try {
      vi.advanceTimersByTime(1);
      const socket = currentInstance!;
      vi.advanceTimersByTime(89_000);
      socket.onmessage?.({ data: '{invalid JSON' } as MessageEvent);
      socket.onmessage?.({ data: new Blob(['pong']) } as MessageEvent);
      vi.advanceTimersByTime(1_000);
      expect(socket.close).toHaveBeenCalledWith(4000, 'Heartbeat timeout');
    } finally {
      dispose();
    }
  });

  it('keeps the websocket open when server activity arrives before heartbeat timeout', async () => {
    const { dispose } = await createStoreHarness();
    try {
      vi.advanceTimersByTime(1); // run onopen tick
      expect(currentInstance).not.toBeNull();

      vi.advanceTimersByTime(89_000);
      currentInstance!.onmessage?.({
        data: JSON.stringify({ type: 'pong', data: { timestamp: Date.now() } }),
      } as MessageEvent);
      vi.advanceTimersByTime(2_000);

      expect(currentInstance!.close).not.toHaveBeenCalledWith(4000, 'Heartbeat timeout');
      expect(currentInstance!.close).not.toHaveBeenCalled();
    } finally {
      dispose();
    }
  });

  it('bounds cold alert uncertainty when an open socket never sends its snapshot', async () => {
    apiFetchJSONMock.mockResolvedValueOnce([]);
    const { store, dispose } = await createStoreHarness();
    try {
      vi.advanceTimersByTime(1);
      expect(store.activeAlertsHydrationStatus()).toBe('pending');

      vi.advanceTimersByTime(4_998);
      expect(apiFetchJSONMock).not.toHaveBeenCalled();
      vi.advanceTimersByTime(2);
      await flushMicrotasks();

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/alerts/active');
      expect(store.activeAlertsHydrationStatus()).toBe('ready');
      expect(store.state.activeAlerts).toEqual([]);
    } finally {
      dispose();
    }
  });

  it('recovers canonical active alert truth over REST when the socket closes before hydration', async () => {
    autoOpenSockets = false;
    const recoveredAlert = {
      id: 'agent:host-1-cpu',
      type: 'cpu',
      level: 'critical',
      resourceId: 'agent:host-1',
      resourceName: 'host-1',
      node: 'host-1',
      instance: 'host-1',
      message: 'CPU above threshold',
      value: 96,
      threshold: 90,
      startTime: '2026-05-14T07:59:00Z',
      acknowledged: false,
    };
    apiFetchJSONMock.mockResolvedValueOnce([recoveredAlert]);

    const { store, dispose } = await createStoreHarness();
    try {
      expect(store.activeAlertsHydrationStatus()).toBe('pending');
      currentInstance!.onclose?.({ code: 1006, reason: '' } as CloseEvent);
      await flushMicrotasks();

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/alerts/active');
      expect(store.activeAlertsHydrationStatus()).toBe('ready');
      expect(store.activeAlerts[recoveredAlert.id]).toMatchObject(recoveredAlert);
      expect(store.state.activeAlerts).toHaveLength(1);
    } finally {
      dispose();
    }
  });

  it('exposes an honest unavailable state and supports an operator retry', async () => {
    autoOpenSockets = false;
    const recoveredAlert = {
      id: 'agent:host-2-memory',
      type: 'memory',
      level: 'warning',
      resourceId: 'agent:host-2',
      resourceName: 'host-2',
      node: 'host-2',
      instance: 'host-2',
      message: 'Memory above threshold',
      value: 88,
      threshold: 85,
      startTime: '2026-05-14T07:58:00Z',
      acknowledged: false,
    };
    apiFetchJSONMock
      .mockRejectedValueOnce(new Error('backend unavailable'))
      .mockResolvedValueOnce([recoveredAlert]);

    const { store, dispose } = await createStoreHarness();
    try {
      currentInstance!.onclose?.({ code: 1006, reason: '' } as CloseEvent);
      await flushMicrotasks();
      expect(store.activeAlertsHydrationStatus()).toBe('unavailable');
      expect(Object.keys(store.activeAlerts)).toHaveLength(0);

      await expect(store.refreshActiveAlerts()).resolves.toBe(true);
      expect(store.activeAlertsHydrationStatus()).toBe('ready');
      expect(store.activeAlerts[recoveredAlert.id]).toMatchObject(recoveredAlert);
      expect(apiFetchJSONMock).toHaveBeenCalledTimes(2);
    } finally {
      dispose();
    }
  });

  it('discards a late REST recovery after newer socket alert truth arrives', async () => {
    vi.spyOn(Math, 'random').mockReturnValue(0.5);
    let resolveRecovery!: (alerts: unknown[]) => void;
    apiFetchJSONMock.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveRecovery = resolve;
        }),
    );

    const { store, dispose } = await createStoreHarness();
    try {
      vi.advanceTimersByTime(1);
      const firstConnection = currentInstance;
      firstConnection!.onclose?.({ code: 1006, reason: '' } as CloseEvent);

      vi.advanceTimersByTime(1_000);
      vi.advanceTimersByTime(1);
      const secondConnection = currentInstance;
      const socketAlert = {
        id: 'agent:host-3-disk',
        type: 'disk',
        level: 'critical',
        resourceId: 'agent:host-3',
        resourceName: 'host-3',
        node: 'host-3',
        instance: 'host-3',
        message: 'Disk above threshold',
        value: 97,
        threshold: 90,
        startTime: '2026-05-14T07:57:00Z',
        acknowledged: false,
      };
      secondConnection!.onmessage?.({
        data: JSON.stringify({
          type: 'initialState',
          data: {
            resources: [],
            activeAlerts: [socketAlert],
            recentlyResolved: [],
            lastUpdate: Date.now(),
          },
        }),
      } as MessageEvent);

      resolveRecovery([
        {
          ...socketAlert,
          id: 'stale-rest-alert',
          message: 'Older recovery response',
        },
      ]);
      await flushMicrotasks();

      expect(store.activeAlertsHydrationStatus()).toBe('ready');
      expect(store.activeAlerts[socketAlert.id]).toMatchObject(socketAlert);
      expect(store.activeAlerts['stale-rest-alert']).toBeUndefined();
    } finally {
      dispose();
    }
  });

  it('preserves active alert state when a resource-only delta arrives', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      vi.advanceTimersByTime(1);
      const alert = {
        id: 'agent:host-1-cpu',
        type: 'cpu',
        level: 'warning',
        resourceId: 'agent:host-1',
        resourceName: 'host-1',
        node: 'host-1',
        instance: 'host-1',
        message: 'CPU above threshold',
        value: 90,
        threshold: 80,
        startTime: '2026-05-14T07:59:00Z',
        acknowledged: false,
      };
      currentInstance!.onmessage?.({
        data: JSON.stringify({
          type: 'initialState',
          data: {
            connectedInfrastructure: [],
            resources: [{ id: 'agent:host-1', type: 'agent', name: 'host-1' }],
            activeAlerts: [alert],
            recentlyResolved: [],
            lastUpdate: Date.now(),
          },
        }),
      } as MessageEvent);

      currentInstance!.onmessage?.({
        data: JSON.stringify({
          type: 'rawData',
          data: {
            resourceDelta: {
              upserts: [{ id: 'agent:host-1', cpu: { current: 91 } }],
            },
            lastUpdate: Date.now() + 1_000,
          },
        }),
      } as MessageEvent);

      expect(store.activeAlerts[alert.id]).toMatchObject(alert);
      expect(store.state.activeAlerts).toHaveLength(1);
      expect(store.state.resources[0]?.cpu?.current).toBe(91);
    } finally {
      dispose();
    }
  });

  it('waits for the reconnect snapshot before applying resource deltas', async () => {
    vi.spyOn(Math, 'random').mockReturnValue(0.5);
    let resolveRestSnapshot!: (snapshot: unknown) => void;
    apiFetchJSONMock.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveRestSnapshot = resolve;
        }),
    );
    apiFetchJSONMock.mockResolvedValueOnce({
      resources: [{ id: 'new-host', type: 'agent', name: 'new-host', lastSeen: 400 }],
      activeAlerts: [],
      recentlyResolved: [],
      lastUpdate: 400,
    });

    const { store, dispose } = await createStoreHarness();
    try {
      vi.advanceTimersByTime(1);
      const firstConnection = currentInstance;
      expect(firstConnection).not.toBeNull();

      firstConnection!.onmessage?.({
        data: JSON.stringify({
          type: 'initialState',
          data: {
            resources: [{ id: 'old-host', type: 'agent', name: 'old-host', lastSeen: 100 }],
            activeAlerts: [],
            recentlyResolved: [],
            lastUpdate: 100,
          },
        }),
      } as MessageEvent);
      expect(store.state.resources[0]?.lastSeen).toBe(100);

      firstConnection!.onclose?.({ code: 1011, reason: 'network blip' } as CloseEvent);
      expect(store.initialDataReceived()).toBe(false);
      vi.advanceTimersByTime(1_000);
      vi.advanceTimersByTime(1);
      await flushMicrotasks();

      const secondConnection = currentInstance;
      expect(secondConnection).not.toBe(firstConnection);
      secondConnection!.onmessage?.({
        data: JSON.stringify({
          type: 'stateTooLarge',
          data: {
            supersedes: 'initialState',
            bytes: MAX_INBOUND_WEBSOCKET_MESSAGE_BYTES + 1,
            maxBytes: MAX_INBOUND_WEBSOCKET_MESSAGE_BYTES,
            resourceCount: 1,
            hydrateFrom: '/api/state',
          },
        }),
      } as MessageEvent);
      expect(apiFetchJSONMock).toHaveBeenCalledTimes(1);

      // The new connection's delta is based on its own dropped snapshot. It
      // must not patch the previous connection's raw resource array while the
      // replacement snapshot is still pending over REST.
      secondConnection!.onmessage?.({
        data: JSON.stringify({
          type: 'rawData',
          data: {
            resourceDelta: { upserts: [{ id: 'old-host', lastSeen: 999 }] },
            lastUpdate: 200,
          },
        }),
      } as MessageEvent);
      expect(store.state.resources[0]?.lastSeen).toBe(100);
      expect(store.initialDataReceived()).toBe(false);

      resolveRestSnapshot({
        resources: [{ id: 'new-host', type: 'agent', name: 'new-host', lastSeen: 300 }],
        activeAlerts: [],
        recentlyResolved: [],
        lastUpdate: 300,
      });
      await flushMicrotasks();

      expect(store.state.resources.map((resource) => resource.id)).toEqual(['new-host']);
      expect(store.initialDataReceived()).toBe(true);

      // REST is a valid display snapshot, but it is not the exact state the
      // server withheld earlier. Deltas remain disabled for this connection.
      secondConnection!.onmessage?.({
        data: JSON.stringify({
          type: 'rawData',
          data: {
            resourceDelta: { upserts: [{ id: 'new-host', lastSeen: 400 }] },
            lastUpdate: 400,
          },
        }),
      } as MessageEvent);
      expect(store.state.resources[0]?.lastSeen).toBe(300);

      // The delta that arrived during/after hydration is coalesced into one
      // trailing REST refresh once the shared throttle expires. It cannot be
      // lost merely because the first request was still in flight.
      vi.advanceTimersByTime(30_000);
      await flushMicrotasks();
      expect(apiFetchJSONMock).toHaveBeenCalledTimes(2);
      expect(store.state.resources[0]?.lastSeen).toBe(400);

      // If the estate later fits, a delivered socket snapshot establishes the
      // connection's delta baseline and normal incremental updates resume.
      secondConnection!.onmessage?.({
        data: JSON.stringify({
          type: 'rawData',
          data: {
            resources: [{ id: 'new-host', type: 'agent', name: 'new-host', lastSeen: 500 }],
            lastUpdate: 500,
          },
        }),
      } as MessageEvent);
      secondConnection!.onmessage?.({
        data: JSON.stringify({
          type: 'rawData',
          data: {
            resourceDelta: { upserts: [{ id: 'new-host', lastSeen: 600 }] },
            lastUpdate: 600,
          },
        }),
      } as MessageEvent);
      expect(store.state.resources[0]?.lastSeen).toBe(600);
    } finally {
      dispose();
    }
  });

  it('keeps URL-switched sockets isolated until their own snapshot arrives', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      vi.advanceTimersByTime(1);
      const firstConnection = currentInstance;
      firstConnection!.onmessage?.({
        data: JSON.stringify({
          type: 'initialState',
          data: {
            resources: [{ id: 'first-host', type: 'agent', name: 'first-host' }],
            activeAlerts: [],
            recentlyResolved: [],
          },
        }),
      } as MessageEvent);

      store.switchUrl('ws://other-backend/ws');
      const secondConnection = currentInstance;
      expect(secondConnection).not.toBe(firstConnection);
      const switchedUrl = new URL(secondConnection!.url);
      expect(`${switchedUrl.protocol}//${switchedUrl.host}${switchedUrl.pathname}`).toBe(
        'ws://other-backend/ws',
      );
      expect(switchedUrl.searchParams.get('max_message_bytes')).toBe(
        String(MAX_INBOUND_WEBSOCKET_MESSAGE_BYTES),
      );
      expect(store.state.resources).toEqual([]);
      expect(store.initialDataReceived()).toBe(false);

      // A late callback from the retired backend must not repopulate the reset
      // store after URL switching.
      firstConnection!.onmessage?.({
        data: JSON.stringify({
          type: 'rawData',
          data: {
            resources: [{ id: 'late-first-host', type: 'agent', name: 'late-first-host' }],
          },
        }),
      } as MessageEvent);
      expect(store.state.resources).toEqual([]);

      secondConnection!.onmessage?.({
        data: JSON.stringify({
          type: 'rawData',
          data: {
            resourceDelta: { upserts: [{ id: 'second-host', lastSeen: 200 }] },
          },
        }),
      } as MessageEvent);
      expect(store.state.resources).toEqual([]);
      expect(store.initialDataReceived()).toBe(false);
      expect(secondConnection!.send).toHaveBeenCalledWith(JSON.stringify({ type: 'requestData' }));

      secondConnection!.onmessage?.({
        data: JSON.stringify({
          type: 'initialState',
          data: {
            resources: [{ id: 'second-host', type: 'agent', name: 'second-host' }],
            activeAlerts: [],
            recentlyResolved: [],
          },
        }),
      } as MessageEvent);
      expect(store.state.resources.map((resource) => resource.id)).toEqual(['second-host']);
      expect(store.initialDataReceived()).toBe(true);
    } finally {
      dispose();
    }
  });

  it('manual reconnect avoids duplicate reconnect scheduling', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      vi.advanceTimersByTime(1); // run onopen tick
      const previous = currentInstance;
      expect(previous).not.toBeNull();

      store.reconnect();
      expect(previous!.close).toHaveBeenCalledWith(1000, 'Reconnecting');
      expect(instances).toHaveLength(2);
      expect(store.connected()).toBe(false);
      expect(store.reconnecting()).toBe(true);

      previous!.onclose?.({ code: 1000, reason: 'Reconnecting' } as CloseEvent);
      vi.advanceTimersByTime(60000);
      expect(instances).toHaveLength(2);
    } finally {
      dispose();
    }
  });

  it('shows auto-registration success once for a fresh event and suppresses replayed copies', async () => {
    const { dispose } = await createStoreHarness();
    try {
      vi.advanceTimersByTime(1);
      expect(currentInstance).not.toBeNull();

      const event = {
        type: 'node_auto_registered',
        timestamp: '2026-05-14T08:00:00.000Z',
        data: {
          type: 'pve',
          source: 'script',
          host: 'https://minipc:8006',
          name: 'minipc',
          nodeId: 'minipc',
          tokenId: 'pulse-monitor@pve!pulse-minipc',
          hasToken: true,
        },
      };

      currentInstance!.onmessage?.({ data: JSON.stringify(event) } as MessageEvent);
      currentInstance!.onmessage?.({ data: JSON.stringify(event) } as MessageEvent);

      expect(notificationMocks.success).toHaveBeenCalledTimes(1);
      expect(notificationMocks.success).toHaveBeenCalledWith(
        'Proxmox VE node "minipc" was successfully auto-registered and is now being monitored!',
        8000,
      );
    } finally {
      dispose();
    }
  });

  it('suppresses stale auto-registration success notifications', async () => {
    const { dispose } = await createStoreHarness();
    try {
      vi.advanceTimersByTime(1);
      expect(currentInstance).not.toBeNull();

      currentInstance!.onmessage?.({
        data: JSON.stringify({
          type: 'node_auto_registered',
          timestamp: '2026-05-14T07:50:00.000Z',
          data: {
            type: 'pve',
            host: 'https://minipc:8006',
            name: 'minipc',
            nodeId: 'minipc',
            tokenId: 'pulse-monitor@pve!pulse-minipc',
            hasToken: true,
          },
        }),
      } as MessageEvent);

      expect(notificationMocks.success).not.toHaveBeenCalled();
    } finally {
      dispose();
    }
  });

  it('suppresses auto-registration notifications that predate the websocket session', async () => {
    const { dispose } = await createStoreHarness();
    try {
      vi.advanceTimersByTime(1);
      expect(currentInstance).not.toBeNull();

      currentInstance!.onmessage?.({
        data: JSON.stringify({
          type: 'node_auto_registered',
          timestamp: '2026-05-14T07:59:30.000Z',
          data: {
            type: 'pve',
            source: 'script',
            host: 'https://minipc:8006',
            name: 'minipc',
            nodeId: 'minipc',
            tokenId: 'pulse-monitor@pve!pulse-minipc',
            hasToken: true,
          },
        }),
      } as MessageEvent);

      expect(notificationMocks.success).not.toHaveBeenCalled();
    } finally {
      dispose();
    }
  });

  it('suppresses background agent auto-registration success notifications', async () => {
    const { dispose } = await createStoreHarness();
    try {
      vi.advanceTimersByTime(1);
      expect(currentInstance).not.toBeNull();

      currentInstance!.onmessage?.({
        data: JSON.stringify({
          type: 'node_auto_registered',
          timestamp: '2026-05-14T08:00:00.000Z',
          data: {
            type: 'pve',
            source: 'agent',
            host: 'https://minipc:8006',
            name: 'minipc',
            nodeId: 'minipc',
            tokenId: 'pulse-monitor@pve!pulse-minipc',
            hasToken: true,
          },
        }),
      } as MessageEvent);

      expect(notificationMocks.success).not.toHaveBeenCalled();
    } finally {
      dispose();
    }
  });
});
