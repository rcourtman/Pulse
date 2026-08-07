import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createRoot } from 'solid-js';

const notificationMocks = vi.hoisted(() => ({
  success: vi.fn(),
  error: vi.fn(),
  info: vi.fn(),
  warning: vi.fn(),
}));

const apiClientMocks = vi.hoisted(() => ({
  apiFetchJSON: vi.fn(),
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: notificationMocks,
}));

vi.mock('@/utils/apiClient', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/utils/apiClient')>();
  return { ...actual, apiFetchJSON: apiClientMocks.apiFetchJSON };
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
    apiClientMocks.apiFetchJSON.mockReset();
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

  it('keeps alert state when the snapshot is withheld and hydrated over REST', async () => {
    // activeAlerts ships inside the same payload as resources, so a snapshot the
    // server withholds for exceeding the client's frame limit withholds alert
    // state too. The REST hydration path must restore both.
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
    apiClientMocks.apiFetchJSON.mockResolvedValue({
      connectedInfrastructure: [],
      resources: [{ id: 'agent:host-1', type: 'agent', name: 'host-1' }],
      activeAlerts: [alert],
      recentlyResolved: [],
      lastUpdate: Date.now(),
    });

    const { store, dispose } = await createStoreHarness();
    try {
      vi.advanceTimersByTime(1);
      expect(currentInstance).not.toBeNull();

      currentInstance!.onmessage?.({
        data: JSON.stringify({
          type: 'stateTooLarge',
          data: {
            supersedes: 'initialState',
            bytes: 13_411_000,
            maxBytes: 8 * 1024 * 1024,
            resourceCount: 1,
            hydrateFrom: '/api/state',
          },
        }),
      } as MessageEvent);
      await vi.advanceTimersByTimeAsync(0);
      await vi.advanceTimersByTimeAsync(0);

      expect(store.activeAlerts[alert.id]).toMatchObject(alert);
      expect(store.state.activeAlerts).toHaveLength(1);

      // Deltas keep applying to the REST-hydrated baseline, because the server
      // set its delta baseline to the snapshot it withheld.
      currentInstance!.onmessage?.({
        data: JSON.stringify({
          type: 'rawData',
          data: {
            resourceDelta: { upserts: [{ id: 'agent:host-1', cpu: { current: 91 } }] },
            lastUpdate: Date.now() + 1_000,
          },
        }),
      } as MessageEvent);

      expect(store.state.resources[0]?.cpu?.current).toBe(91);
      expect(store.state.activeAlerts).toHaveLength(1);
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
