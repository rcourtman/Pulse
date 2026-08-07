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
    this.readyState = MockWebSocketClass.OPEN;
    instances.push(this);
    currentInstance = this; // eslint-disable-line @typescript-eslint/no-this-alias -- test mock needs instance capture
    setTimeout(() => {
      this.readyState = MockWebSocketClass.OPEN;
      this.onopen?.({} as Event);
    }, 0);
  }
}

const makeResource = (id: string, cpu = 10) => ({
  id,
  type: 'agent',
  name: id,
  displayName: id,
  status: 'online',
  cpu: { current: cpu },
});

const createStoreHarness = async () => {
  const { createWebSocketStore } = await import('@/stores/websocket');
  let dispose = () => {};
  const store = createRoot((d) => {
    dispose = d;
    return createWebSocketStore('ws://localhost/ws');
  });
  return { store, dispose };
};

const deliver = (payload: unknown) => {
  const data = typeof payload === 'string' ? payload : JSON.stringify(payload);
  currentInstance!.onmessage?.({ data } as MessageEvent);
};

// Lets queued promise callbacks run under fake timers.
const flush = async () => {
  await vi.advanceTimersByTimeAsync(0);
  await vi.advanceTimersByTimeAsync(0);
};

describe('oversized state payload recovery', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.resetModules();
    currentInstance = null;
    instances.length = 0;
    vi.setSystemTime(new Date('2026-08-07T08:00:00.000Z'));
    apiClientMocks.apiFetchJSON.mockReset();
    vi.stubGlobal('WebSocket', MockWebSocketClass);
  });

  afterEach(() => {
    vi.clearAllTimers();
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('advertises its inbound frame limit on the handshake URL', async () => {
    const { dispose } = await createStoreHarness();
    try {
      // The limit has to reach the server before it builds the first snapshot,
      // so it rides the upgrade URL rather than a post-connect message.
      expect(currentInstance).not.toBeNull();
      const socketUrl = new URL(currentInstance!.url);
      expect(socketUrl.searchParams.get('max_message_bytes')).toBe(String(8 * 1024 * 1024));
    } finally {
      dispose();
    }
  });

  it('keeps advertising the limit across reconnects', async () => {
    const { dispose } = await createStoreHarness();
    try {
      vi.advanceTimersByTime(1);
      currentInstance!.onclose?.({ code: 1006, reason: '' } as CloseEvent);
      vi.advanceTimersByTime(5000);

      expect(instances.length).toBeGreaterThan(1);
      for (const instance of instances) {
        expect(new URL(instance.url).searchParams.get('max_message_bytes')).toBe(
          String(8 * 1024 * 1024),
        );
      }
    } finally {
      dispose();
    }
  });

  it('hydrates over REST when the server withholds an oversized snapshot', async () => {
    apiClientMocks.apiFetchJSON.mockResolvedValue({
      resources: [makeResource('agent:host-1'), makeResource('agent:host-2')],
      lastUpdate: 100,
    });

    const { store, dispose } = await createStoreHarness();
    try {
      vi.advanceTimersByTime(1);

      deliver({
        type: 'stateTooLarge',
        data: {
          supersedes: 'initialState',
          bytes: 13_411_000,
          maxBytes: 8 * 1024 * 1024,
          resourceCount: 5000,
          hydrateFrom: '/api/state',
        },
      });
      await flush();

      expect(apiClientMocks.apiFetchJSON).toHaveBeenCalledWith('/api/state', expect.anything());
      expect(store.state.resources).toHaveLength(2);
      expect(store.initialDataReceived()).toBe(true);
    } finally {
      dispose();
    }
  });

  it('hydrates over REST when an oversized frame is dropped by the guard', async () => {
    apiClientMocks.apiFetchJSON.mockResolvedValue({
      resources: [makeResource('agent:host-1')],
      lastUpdate: 100,
    });

    const { store, dispose } = await createStoreHarness();
    try {
      vi.advanceTimersByTime(1);

      // A server too old to honor the advertised limit still sends the snapshot.
      deliver('x'.repeat(9 * 1024 * 1024));
      await flush();

      expect(apiClientMocks.apiFetchJSON).toHaveBeenCalledWith('/api/state', expect.anything());
      expect(store.state.resources).toHaveLength(1);
    } finally {
      dispose();
    }
  });

  it('applies later deltas onto the REST-hydrated baseline', async () => {
    apiClientMocks.apiFetchJSON.mockResolvedValue({
      resources: [makeResource('agent:host-1', 10), makeResource('agent:host-2', 20)],
      lastUpdate: 100,
    });

    const { store, dispose } = await createStoreHarness();
    try {
      vi.advanceTimersByTime(1);
      deliver({
        type: 'stateTooLarge',
        data: {
          supersedes: 'initialState',
          bytes: 13_411_000,
          maxBytes: 8 * 1024 * 1024,
          resourceCount: 2,
          hydrateFrom: '/api/state',
        },
      });
      await flush();

      // The server keeps diffing against the snapshot it withheld, which is the
      // same payload REST just served, so this delta must land.
      deliver({
        type: 'rawData',
        data: {
          resourceDelta: { upserts: [{ id: 'agent:host-1', cpu: { current: 77 } }] },
          lastUpdate: 200,
        },
      });
      await flush();

      const hydrated = store.state.resources.find((r) => r.id === 'agent:host-1');
      expect(hydrated?.cpu?.current).toBe(77);
      expect(store.state.resources).toHaveLength(2);
    } finally {
      dispose();
    }
  });

  it('keeps the app on the bootstrap snapshot when a delta arrives with no baseline', async () => {
    // Never resolves: hydration is still in flight when the delta lands.
    apiClientMocks.apiFetchJSON.mockReturnValue(new Promise(() => {}));

    const { store, dispose } = await createStoreHarness();
    try {
      vi.advanceTimersByTime(1);

      deliver({
        type: 'rawData',
        data: {
          resourceDelta: { upserts: [{ id: 'agent:host-1', cpu: { current: 77 } }] },
          lastUpdate: 200,
        },
      });
      await flush();

      // Flagging this as usable data would make the app shell drop the populated
      // REST bootstrap state for this empty live store and blank the dashboard.
      expect(store.state.resources).toHaveLength(0);
      expect(store.initialDataReceived()).toBe(false);
    } finally {
      dispose();
    }
  });

  it('marks data received once a real snapshot arrives', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      vi.advanceTimersByTime(1);

      deliver({
        type: 'initialState',
        data: { resources: [makeResource('agent:host-1')], lastUpdate: 100 },
      });
      await flush();

      expect(store.initialDataReceived()).toBe(true);
      expect(store.state.resources).toHaveLength(1);
    } finally {
      dispose();
    }
  });

  it('retries hydration immediately after a failed REST fetch', async () => {
    apiClientMocks.apiFetchJSON.mockRejectedValueOnce(new Error('network down'));
    apiClientMocks.apiFetchJSON.mockResolvedValueOnce({
      resources: [makeResource('agent:host-1')],
      lastUpdate: 100,
    });

    const { store, dispose } = await createStoreHarness();
    try {
      vi.advanceTimersByTime(1);

      const marker = {
        type: 'stateTooLarge',
        data: {
          supersedes: 'initialState',
          bytes: 13_411_000,
          maxBytes: 8 * 1024 * 1024,
          resourceCount: 1,
          hydrateFrom: '/api/state',
        },
      };

      deliver(marker);
      await flush();
      expect(store.state.resources).toHaveLength(0);

      // A failed hydration must not be rate-limited into a stall: without a
      // baseline every subsequent delta is unusable.
      deliver(marker);
      await flush();

      expect(apiClientMocks.apiFetchJSON).toHaveBeenCalledTimes(2);
      expect(store.state.resources).toHaveLength(1);
    } finally {
      dispose();
    }
  });
});
