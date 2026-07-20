import { afterEach, describe, expect, it, vi } from 'vitest';
import { consumeJSONEventStream } from '@/api/streaming';

const makeEventStreamResponse = (body: string) =>
  ({
    body: {
      getReader: () => ({
        read: vi
          .fn()
          .mockResolvedValueOnce({ done: false, value: new TextEncoder().encode(body) })
          .mockResolvedValueOnce({ done: true, value: undefined }),
        releaseLock: vi.fn(),
      }),
    },
  }) as unknown as Response;

const makeChunkedEventStreamResponse = (chunks: string[]) => {
  const read = vi.fn();
  for (const chunk of chunks) {
    read.mockResolvedValueOnce({ done: false, value: new TextEncoder().encode(chunk) });
  }
  read.mockResolvedValueOnce({ done: true, value: undefined });

  return {
    body: {
      getReader: () => ({
        read,
        releaseLock: vi.fn(),
      }),
    },
  } as unknown as Response;
};

const flushMicrotasks = async () => {
  for (let attempt = 0; attempt < 20; attempt += 1) {
    await Promise.resolve();
  }
};

const originalRequestAnimationFrame = window.requestAnimationFrame;
const originalCancelAnimationFrame = window.cancelAnimationFrame;

describe('consumeJSONEventStream', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    Object.defineProperty(window, 'requestAnimationFrame', {
      configurable: true,
      value: originalRequestAnimationFrame,
    });
    Object.defineProperty(window, 'cancelAnimationFrame', {
      configurable: true,
      value: originalCancelAnimationFrame,
    });
    vi.useRealTimers();
  });

  const installAnimationFrame = () => {
    const requestAnimationFrame = vi.fn((callback: FrameRequestCallback) => {
      return window.setTimeout(() => callback(performance.now()), 16);
    });
    const cancelAnimationFrame = vi.fn((id: number) => window.clearTimeout(id));
    vi.stubGlobal('requestAnimationFrame', requestAnimationFrame);
    vi.stubGlobal('cancelAnimationFrame', cancelAnimationFrame);
    Object.defineProperty(window, 'requestAnimationFrame', {
      configurable: true,
      value: requestAnimationFrame,
    });
    Object.defineProperty(window, 'cancelAnimationFrame', {
      configurable: true,
      value: cancelAnimationFrame,
    });
    return requestAnimationFrame;
  };

  const advancePaintCheckpoint = async () => {
    await vi.advanceTimersToNextTimerAsync();
    await vi.advanceTimersToNextTimerAsync();
    await flushMicrotasks();
  };

  it('yields to the browser between coalesced stream events when requested', async () => {
    vi.useFakeTimers();
    const requestAnimationFrame = installAnimationFrame();

    const events: string[] = [];
    const streamPromise = consumeJSONEventStream<{ type: string }>(
      makeEventStreamResponse(
        [
          'data: {"type":"tool_start"}',
          '',
          'data: {"type":"tool_progress"}',
          '',
          'data: {"type":"done"}',
          '',
          '',
        ].join('\n'),
      ),
      {
        onEvent: (event) => {
          events.push(event.type);
          return event.type === 'done';
        },
        yieldBetweenEvents: true,
      },
    );

    await flushMicrotasks();
    expect(events).toEqual(['tool_start']);
    expect(requestAnimationFrame).toHaveBeenCalledTimes(1);
    expect(vi.getTimerCount()).toBeGreaterThan(0);

    await advancePaintCheckpoint();
    expect(events).toEqual(['tool_start', 'tool_progress']);
    expect(requestAnimationFrame).toHaveBeenCalledTimes(2);
    expect(vi.getTimerCount()).toBeGreaterThan(0);

    await advancePaintCheckpoint();
    await streamPromise;
    expect(events).toEqual(['tool_start', 'tool_progress', 'done']);
    expect(vi.getTimerCount()).toBe(0);
  });

  it('does not yield for coalesced events rejected by the event predicate', async () => {
    vi.useFakeTimers();
    const requestAnimationFrame = installAnimationFrame();

    const events: string[] = [];
    const streamPromise = consumeJSONEventStream<{ type: string }>(
      makeEventStreamResponse(
        [
          'data: {"type":"content"}',
          '',
          'data: {"type":"tool_progress"}',
          '',
          'data: {"type":"done"}',
          '',
          '',
        ].join('\n'),
      ),
      {
        onEvent: (event) => {
          events.push(event.type);
          return event.type === 'done';
        },
        yieldBetweenEvents: (event) => event.type !== 'content',
      },
    );

    await flushMicrotasks();
    expect(events).toEqual(['content', 'tool_progress']);
    expect(requestAnimationFrame).toHaveBeenCalledTimes(1);
    expect(vi.getTimerCount()).toBeGreaterThan(0);

    await advancePaintCheckpoint();
    await streamPromise;
    expect(events).toEqual(['content', 'tool_progress', 'done']);
    expect(vi.getTimerCount()).toBe(0);
  });

  it('yields to the browser between matching events that arrive in separate queued reads', async () => {
    vi.useFakeTimers();
    const requestAnimationFrame = installAnimationFrame();

    const events: string[] = [];
    const streamPromise = consumeJSONEventStream<{ type: string }>(
      makeChunkedEventStreamResponse([
        'data: {"type":"tool_start"}\n\n',
        'data: {"type":"tool_progress"}\n\n',
        'data: {"type":"done"}\n\n',
      ]),
      {
        onEvent: (event) => {
          events.push(event.type);
          return event.type === 'done';
        },
        yieldBetweenEvents: (event) => event.type !== 'content',
      },
    );

    await flushMicrotasks();
    expect(events).toEqual(['tool_start']);
    expect(requestAnimationFrame).toHaveBeenCalledTimes(1);
    expect(vi.getTimerCount()).toBeGreaterThan(0);

    await advancePaintCheckpoint();
    expect(events).toEqual(['tool_start', 'tool_progress']);
    expect(requestAnimationFrame).toHaveBeenCalledTimes(2);
    expect(vi.getTimerCount()).toBeGreaterThan(0);

    await advancePaintCheckpoint();
    await streamPromise;
    expect(events).toEqual(['tool_start', 'tool_progress', 'done']);
    expect(vi.getTimerCount()).toBe(0);
  });

  it('falls back to a short timer when animation frames are unavailable', async () => {
    vi.useFakeTimers();
    vi.stubGlobal('requestAnimationFrame', undefined);
    Object.defineProperty(window, 'requestAnimationFrame', {
      configurable: true,
      value: undefined,
    });

    const events: string[] = [];
    const streamPromise = consumeJSONEventStream<{ type: string }>(
      makeEventStreamResponse(
        [
          'data: {"type":"tool_start"}',
          '',
          'data: {"type":"tool_progress"}',
          '',
          'data: {"type":"done"}',
          '',
          '',
        ].join('\n'),
      ),
      {
        onEvent: (event) => {
          events.push(event.type);
          return event.type === 'done';
        },
        yieldBetweenEvents: true,
      },
    );

    await flushMicrotasks();
    expect(events).toEqual(['tool_start']);

    await vi.advanceTimersByTimeAsync(1);
    await flushMicrotasks();
    expect(events).toEqual(['tool_start', 'tool_progress']);

    await vi.advanceTimersByTimeAsync(1);
    await streamPromise;
    expect(events).toEqual(['tool_start', 'tool_progress', 'done']);
  });

  it('surfaces stalled read timeouts without reporting normal completion', async () => {
    vi.useFakeTimers();
    const read = vi.fn(() => new Promise<ReadableStreamReadResult<Uint8Array>>(() => undefined));
    const releaseLock = vi.fn();
    const onEvent = vi.fn();
    const onTimeout = vi.fn();
    const onComplete = vi.fn();

    const streamPromise = consumeJSONEventStream<{ type: string }>(
      {
        body: {
          getReader: () => ({ read, releaseLock }),
        },
      } as unknown as Response,
      {
        onEvent,
        onTimeout,
        onComplete,
        timeoutMs: 1000,
      },
    );

    await flushMicrotasks();
    await vi.advanceTimersByTimeAsync(1000);
    await streamPromise;

    expect(read).toHaveBeenCalledTimes(1);
    expect(onTimeout).toHaveBeenCalledTimes(1);
    expect(onComplete).not.toHaveBeenCalled();
    expect(onEvent).not.toHaveBeenCalled();
    expect(releaseLock).toHaveBeenCalledTimes(1);
  });
});
