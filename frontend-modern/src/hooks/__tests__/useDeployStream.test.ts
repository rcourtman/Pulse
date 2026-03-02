import { createRoot, createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const { loggerMock } = vi.hoisted(() => ({
  loggerMock: {
    info: vi.fn(),
    warn: vi.fn(),
  },
}));
vi.mock('@/utils/logger', () => ({ logger: loggerMock }));

import { useDeployStream, type DeployStreamState } from '@/hooks/useDeployStream';

class MockEventSource {
  static instances: MockEventSource[] = [];

  readonly url: string;
  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  readyState = 0;
  withCredentials = false;
  closed = false;

  constructor(url: string) {
    this.url = url;
    MockEventSource.instances.push(this);
  }

  close() {
    this.closed = true;
    this.readyState = 2;
  }

  emitOpen() {
    this.readyState = 1;
    this.onopen?.(new Event('open'));
  }

  emitMessage(payload: unknown, lastEventId = '') {
    const evt = { data: JSON.stringify(payload), lastEventId } as MessageEvent;
    this.onmessage?.(evt);
  }

  emitRawMessage(data: unknown, lastEventId = '') {
    const evt = { data, lastEventId } as MessageEvent;
    this.onmessage?.(evt);
  }

  emitError() {
    this.onerror?.(new Event('error'));
  }
}

describe('useDeployStream', () => {
  const originalEventSource = globalThis.EventSource;

  beforeEach(() => {
    vi.useFakeTimers();
    vi.clearAllMocks();
    MockEventSource.instances = [];
    (globalThis as unknown as { EventSource: typeof EventSource }).EventSource =
      MockEventSource as unknown as typeof EventSource;
  });

  afterEach(() => {
    vi.useRealTimers();
    (globalThis as unknown as { EventSource: typeof EventSource }).EventSource =
      originalEventSource;
  });

  it('opens SSE connection when eventsUrl is non-null', () => {
    let dispose!: () => void;
    createRoot((d) => {
      dispose = d;
      const [url] = createSignal<string | null>('/api/events');
      useDeployStream({ eventsUrl: url });
    });

    expect(MockEventSource.instances).toHaveLength(1);
    expect(MockEventSource.instances[0].url).toBe('/api/events');

    dispose();
  });

  it('does not open connection when eventsUrl is null', () => {
    let dispose!: () => void;
    createRoot((d) => {
      dispose = d;
      const [url] = createSignal<string | null>(null);
      useDeployStream({ eventsUrl: url });
    });

    expect(MockEventSource.instances).toHaveLength(0);

    dispose();
  });

  it('sets isStreaming to true on open', () => {
    let dispose!: () => void;
    let state!: DeployStreamState;
    createRoot((d) => {
      dispose = d;
      const [url] = createSignal<string | null>('/api/events');
      state = useDeployStream({ eventsUrl: url });
    });

    expect(state.isStreaming()).toBe(false);
    MockEventSource.instances[0].emitOpen();
    expect(state.isStreaming()).toBe(true);

    dispose();
  });

  it('appends events and calls onEvent callback', () => {
    const onEvent = vi.fn();
    let dispose!: () => void;
    let state!: DeployStreamState;
    createRoot((d) => {
      dispose = d;
      const [url] = createSignal<string | null>('/api/events');
      state = useDeployStream({ eventsUrl: url, onEvent });
    });

    const es = MockEventSource.instances[0];
    es.emitOpen();
    es.emitMessage({ id: 'e1', type: 'preflight_result', targetId: 't1', message: 'ok' });

    expect(state.events()).toHaveLength(1);
    expect(state.events()[0].id).toBe('e1');
    expect(onEvent).toHaveBeenCalledTimes(1);
    expect(onEvent).toHaveBeenCalledWith(expect.objectContaining({ id: 'e1' }));

    dispose();
  });

  it('deduplicates events by ID', () => {
    let dispose!: () => void;
    let state!: DeployStreamState;
    createRoot((d) => {
      dispose = d;
      const [url] = createSignal<string | null>('/api/events');
      state = useDeployStream({ eventsUrl: url });
    });

    const es = MockEventSource.instances[0];
    es.emitOpen();
    es.emitMessage({ id: 'e1', type: 'preflight_result', message: 'first' });
    es.emitMessage({ id: 'e1', type: 'preflight_result', message: 'duplicate' });
    es.emitMessage({ id: 'e2', type: 'install_output', message: 'second' });

    expect(state.events()).toHaveLength(2);
    expect(state.events()[0].id).toBe('e1');
    expect(state.events()[1].id).toBe('e2');

    dispose();
  });

  it('calls onComplete and closes on job_complete event', () => {
    const onComplete = vi.fn();
    let dispose!: () => void;
    let state!: DeployStreamState;
    createRoot((d) => {
      dispose = d;
      const [url] = createSignal<string | null>('/api/events');
      state = useDeployStream({ eventsUrl: url, onComplete });
    });

    const es = MockEventSource.instances[0];
    es.emitOpen();
    es.emitMessage({ type: 'job_complete', status: 'succeeded' });

    expect(onComplete).toHaveBeenCalledWith('succeeded');
    expect(es.closed).toBe(true);
    expect(state.isStreaming()).toBe(false);

    dispose();
  });

  it('calls onComplete with "unknown" when job_complete has no status', () => {
    const onComplete = vi.fn();
    let dispose!: () => void;
    createRoot((d) => {
      dispose = d;
      const [url] = createSignal<string | null>('/api/events');
      useDeployStream({ eventsUrl: url, onComplete });
    });

    const es = MockEventSource.instances[0];
    es.emitOpen();
    es.emitMessage({ type: 'job_complete' });

    expect(onComplete).toHaveBeenCalledWith('unknown');

    dispose();
  });

  it('reconnects with exponential backoff on error', () => {
    let dispose!: () => void;
    createRoot((d) => {
      dispose = d;
      const [url] = createSignal<string | null>('/api/events');
      useDeployStream({ eventsUrl: url });
    });

    expect(MockEventSource.instances).toHaveLength(1);

    // Error → reconnect after 1s
    MockEventSource.instances[0].emitError();
    vi.advanceTimersByTime(1000);
    expect(MockEventSource.instances).toHaveLength(2);

    // Error → reconnect after 2s
    MockEventSource.instances[1].emitError();
    vi.advanceTimersByTime(2000);
    expect(MockEventSource.instances).toHaveLength(3);

    // Error → reconnect after 4s
    MockEventSource.instances[2].emitError();
    vi.advanceTimersByTime(4000);
    expect(MockEventSource.instances).toHaveLength(4);

    dispose();
  });

  it('stops reconnecting after max attempts and calls onError', () => {
    const onError = vi.fn();
    let dispose!: () => void;
    createRoot((d) => {
      dispose = d;
      const [url] = createSignal<string | null>('/api/events');
      useDeployStream({ eventsUrl: url, onError });
    });

    const backoffMs = [1000, 2000, 4000, 8000, 15000];
    for (const delay of backoffMs) {
      const current = MockEventSource.instances[MockEventSource.instances.length - 1];
      current.emitError();
      vi.advanceTimersByTime(delay);
    }

    expect(MockEventSource.instances).toHaveLength(6); // initial + 5 reconnects

    // 6th error exceeds max
    MockEventSource.instances[5].emitError();
    vi.advanceTimersByTime(60000);

    expect(onError).toHaveBeenCalledTimes(1);
    expect(onError).toHaveBeenCalledWith('Connection lost after max retries');
    expect(MockEventSource.instances).toHaveLength(6);

    dispose();
  });

  it('resets reconnect counter on successful connection', () => {
    let dispose!: () => void;
    createRoot((d) => {
      dispose = d;
      const [url] = createSignal<string | null>('/api/events');
      useDeployStream({ eventsUrl: url });
    });

    // Fail twice
    MockEventSource.instances[0].emitError();
    vi.advanceTimersByTime(1000);
    MockEventSource.instances[1].emitError();
    vi.advanceTimersByTime(2000);

    // Succeed — resets counter
    MockEventSource.instances[2].emitOpen();

    // Error again — should use 1s delay (reset), not 4s
    MockEventSource.instances[2].emitError();
    vi.advanceTimersByTime(1000);
    expect(MockEventSource.instances).toHaveLength(4);

    dispose();
  });

  it('closes connection when eventsUrl changes to null', () => {
    let dispose!: () => void;
    let setUrl!: (url: string | null) => void;
    createRoot((d) => {
      dispose = d;
      const [url, _setUrl] = createSignal<string | null>('/api/events');
      setUrl = _setUrl;
      useDeployStream({ eventsUrl: url });
    });

    const es = MockEventSource.instances[0];
    es.emitOpen();
    expect(es.closed).toBe(false);

    setUrl(null);
    expect(es.closed).toBe(true);

    dispose();
  });

  it('opens new connection when eventsUrl changes to a new value', () => {
    let dispose!: () => void;
    let setUrl!: (url: string | null) => void;
    let state!: DeployStreamState;
    createRoot((d) => {
      dispose = d;
      const [url, _setUrl] = createSignal<string | null>('/api/events/1');
      setUrl = _setUrl;
      state = useDeployStream({ eventsUrl: url });
    });

    const first = MockEventSource.instances[0];
    first.emitOpen();
    first.emitMessage({ id: 'e1', type: 'preflight_result', message: 'msg' });
    expect(state.events()).toHaveLength(1);

    // Change URL — should close old, open new, reset events
    setUrl('/api/events/2');
    expect(first.closed).toBe(true);
    expect(MockEventSource.instances).toHaveLength(2);
    expect(MockEventSource.instances[1].url).toBe('/api/events/2');
    expect(state.events()).toHaveLength(0); // reset on new URL

    dispose();
  });

  it('drops oversized SSE payloads', () => {
    let dispose!: () => void;
    let state!: DeployStreamState;
    createRoot((d) => {
      dispose = d;
      const [url] = createSignal<string | null>('/api/events');
      state = useDeployStream({ eventsUrl: url });
    });

    const es = MockEventSource.instances[0];
    es.emitOpen();

    // Normal event
    es.emitMessage({ id: 'e1', type: 'preflight_result', message: 'ok' });
    expect(state.events()).toHaveLength(1);

    // Oversized event (>64KB)
    es.emitRawMessage('x'.repeat(65 * 1024));
    expect(state.events()).toHaveLength(1); // not added
    expect(loggerMock.warn).toHaveBeenCalledWith('[DeployStream] Dropping oversized SSE event');

    dispose();
  });

  it('ignores non-string SSE data', () => {
    let dispose!: () => void;
    let state!: DeployStreamState;
    createRoot((d) => {
      dispose = d;
      const [url] = createSignal<string | null>('/api/events');
      state = useDeployStream({ eventsUrl: url });
    });

    const es = MockEventSource.instances[0];
    es.emitOpen();
    es.emitRawMessage(42);

    expect(state.events()).toHaveLength(0);

    dispose();
  });

  it('ignores unparseable JSON', () => {
    let dispose!: () => void;
    let state!: DeployStreamState;
    createRoot((d) => {
      dispose = d;
      const [url] = createSignal<string | null>('/api/events');
      state = useDeployStream({ eventsUrl: url });
    });

    const es = MockEventSource.instances[0];
    es.emitOpen();
    es.emitRawMessage('not-valid-json');

    expect(state.events()).toHaveLength(0);
    expect(loggerMock.warn).toHaveBeenCalledWith('[DeployStream] Failed to parse SSE event');

    dispose();
  });

  it('cleans up on disposal', () => {
    let dispose!: () => void;
    createRoot((d) => {
      dispose = d;
      const [url] = createSignal<string | null>('/api/events');
      useDeployStream({ eventsUrl: url });
    });

    const es = MockEventSource.instances[0];
    es.emitOpen();
    expect(es.closed).toBe(false);

    dispose();
    expect(es.closed).toBe(true);
  });

  it('does not reconnect after intentional close', () => {
    let dispose!: () => void;
    let setUrl!: (url: string | null) => void;
    createRoot((d) => {
      dispose = d;
      const [url, _setUrl] = createSignal<string | null>('/api/events');
      setUrl = _setUrl;
      useDeployStream({ eventsUrl: url });
    });

    const es = MockEventSource.instances[0];
    es.emitOpen();

    // Close intentionally by setting url to null
    setUrl(null);
    expect(es.closed).toBe(true);

    // No reconnect should happen
    vi.advanceTimersByTime(60000);
    expect(MockEventSource.instances).toHaveLength(1);

    dispose();
  });
});
