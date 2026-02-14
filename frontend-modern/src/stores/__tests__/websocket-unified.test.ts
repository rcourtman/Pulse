import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createRoot } from 'solid-js';

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
    installWebSocketMock();
  });

  afterEach(() => {
    vi.clearAllTimers();
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('initializes with empty resources array and empty legacy arrays', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      expect(store.state.resources).toEqual([]);
      expect(store.state.nodes).toEqual([]);
      expect(store.state.vms).toEqual([]);
      expect(store.state.containers).toEqual([]);
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
          resources: [
            { id: 'node-1', type: 'node', name: 'pve1', status: 'online' },
            { id: 'vm-101', type: 'vm', name: 'web-server', status: 'running' },
          ],
          lastUpdate: '2026-02-09T00:00:00Z',
          activeAlerts: [],
          recentlyResolved: [],
        },
      });

      expect(store.state.resources).toHaveLength(2);
      expect(store.state.nodes).toEqual([]);
      expect(store.state.vms).toEqual([]);
      expect(store.state.containers).toEqual([]);
    } finally {
      dispose();
    }
  });

  it('processes mixed payload (resources + legacy arrays)', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();

      emitMessage({
        type: 'initialState',
        data: {
          resources: [{ id: 'node-1', type: 'node', name: 'pve1', status: 'online' }],
          nodes: [{ id: 'node-1', name: 'pve1' }],
          lastUpdate: '2026-02-09T00:00:00Z',
          activeAlerts: [],
          recentlyResolved: [],
        },
      });

      expect(store.state.resources).toHaveLength(1);
      expect(store.state.nodes).toHaveLength(1);
    } finally {
      dispose();
    }
  });

  it('incremental update adds to resources without affecting legacy arrays', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();

      emitMessage({
        type: 'initialState',
        data: {
          resources: [{ id: 'node-1', type: 'node', name: 'pve1', status: 'online' }],
          lastUpdate: '2026-02-09T00:00:00Z',
          activeAlerts: [],
          recentlyResolved: [],
        },
      });

      emitMessage({
        type: 'rawData',
        data: {
          resources: [
            { id: 'node-1', type: 'node', name: 'pve1', status: 'online' },
            { id: 'vm-101', type: 'vm', name: 'web-server', status: 'running' },
          ],
          lastUpdate: '2026-02-09T00:01:00Z',
          activeAlerts: [],
          recentlyResolved: [],
        },
      });

      expect(store.state.resources).toHaveLength(2);
      expect(store.state.nodes).toEqual([]);
      expect(store.state.vms).toEqual([]);
      expect(store.state.containers).toEqual([]);
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
