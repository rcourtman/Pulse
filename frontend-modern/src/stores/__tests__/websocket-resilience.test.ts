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

  it('manual reconnect avoids duplicate reconnect scheduling', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      vi.advanceTimersByTime(1); // run onopen tick
      const previous = currentInstance;
      expect(previous).not.toBeNull();

      store.reconnect();
      expect(previous!.close).toHaveBeenCalledWith(1000, 'Reconnecting');
      expect(instances).toHaveLength(2);

      previous!.onclose?.({ code: 1000, reason: 'Reconnecting' } as CloseEvent);
      vi.advanceTimersByTime(60000);
      expect(instances).toHaveLength(2);
    } finally {
      dispose();
    }
  });
});
