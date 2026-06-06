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

describe('consumeJSONEventStream', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  it('yields to the browser between coalesced stream events when requested', async () => {
    vi.useFakeTimers();

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
    expect(vi.getTimerCount()).toBeGreaterThan(0);

    await vi.advanceTimersByTimeAsync(1);
    await flushMicrotasks();
    expect(events).toEqual(['tool_start', 'tool_progress']);
    expect(vi.getTimerCount()).toBeGreaterThan(0);

    await vi.advanceTimersByTimeAsync(1);
    await streamPromise;
    expect(events).toEqual(['tool_start', 'tool_progress', 'done']);
    expect(vi.getTimerCount()).toBe(0);
  });

  it('does not yield for coalesced events rejected by the event predicate', async () => {
    vi.useFakeTimers();

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
    expect(vi.getTimerCount()).toBeGreaterThan(0);

    await vi.advanceTimersByTimeAsync(1);
    await streamPromise;
    expect(events).toEqual(['content', 'tool_progress', 'done']);
    expect(vi.getTimerCount()).toBe(0);
  });

  it('yields to the browser between matching events that arrive in separate queued reads', async () => {
    vi.useFakeTimers();

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
    expect(vi.getTimerCount()).toBeGreaterThan(0);

    await vi.advanceTimersByTimeAsync(1);
    await flushMicrotasks();
    expect(events).toEqual(['tool_start', 'tool_progress']);
    expect(vi.getTimerCount()).toBeGreaterThan(0);

    await vi.advanceTimersByTimeAsync(1);
    await streamPromise;
    expect(events).toEqual(['tool_start', 'tool_progress', 'done']);
    expect(vi.getTimerCount()).toBe(0);
  });
});
