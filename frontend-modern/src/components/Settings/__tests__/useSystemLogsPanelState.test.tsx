import { For, createEffect } from 'solid-js';
import { cleanup, render } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useSystemLogsPanelState } from '../useSystemLogsPanelState';

const mocks = vi.hoisted(() => ({ api: vi.fn(), success: vi.fn(), error: vi.fn() }));
vi.mock('@/utils/apiClient', () => ({ apiFetchJSON: mocks.api }));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: mocks.success, error: mocks.error },
}));
vi.mock('@/utils/logger', () => ({ logger: { debug: vi.fn(), error: vi.fn() } }));

class FixtureEventSource {
  static instances: FixtureEventSource[] = [];
  onmessage: ((event: MessageEvent<string>) => void) | null = null;
  onerror: (() => void) | null = null;
  close = vi.fn();
  constructor(readonly url: string) {
    FixtureEventSource.instances.push(this);
  }
  send(data: string) {
    this.onmessage?.(new MessageEvent('message', { data }));
  }
}

let frames: Map<number, FrameRequestCallback>;
let nextFrame: number;
const flushAsync = async () => {
  await Promise.resolve();
  await Promise.resolve();
  await Promise.resolve();
};
const flushFrame = () => {
  const callbacks = [...frames.values()];
  frames.clear();
  callbacks.forEach((callback) => callback(0));
};

async function mount() {
  let state!: ReturnType<typeof useSystemLogsPanelState>;
  const publish = vi.fn();
  const view = render(() => {
    state = useSystemLogsPanelState();
    createEffect(() => {
      publish(state.logs());
    });
    return (
      <div ref={state.setLogContainer}>
        <For each={state.logs()}>{(line) => <div>{line}</div>}</For>
      </div>
    );
  });
  const container = view.container.firstElementChild as HTMLDivElement;
  const measure = vi.fn(() => container.childElementCount * 20);
  const scroll = vi.fn();
  Object.defineProperty(container, 'scrollHeight', { get: measure });
  Object.defineProperty(container, 'scrollTop', { set: scroll });
  await flushAsync();
  publish.mockClear();
  return {
    state,
    view,
    container,
    measure,
    scroll,
    publish,
    stream: FixtureEventSource.instances[0],
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  FixtureEventSource.instances = [];
  frames = new Map();
  nextFrame = 0;
  vi.stubGlobal('EventSource', FixtureEventSource);
  vi.stubGlobal(
    'requestAnimationFrame',
    vi.fn((callback: FrameRequestCallback) => {
      frames.set(++nextFrame, callback);
      return nextFrame;
    }),
  );
  vi.stubGlobal(
    'cancelAnimationFrame',
    vi.fn((id: number) => {
      frames.delete(id);
    }),
  );
  mocks.api.mockResolvedValue({ level: 'info' });
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe('live system log rendering', () => {
  it('publishes and scrolls once per frame, after the accepted rows render', async () => {
    const { state, stream, measure, scroll, publish, container } = await mount();
    stream.send('first');
    stream.send('second');
    stream.send('third');
    expect(state.logs()).toEqual([]);
    expect(requestAnimationFrame).toHaveBeenCalledTimes(1);
    expect(measure).not.toHaveBeenCalled();
    expect(publish).not.toHaveBeenCalled();
    flushFrame();
    expect(state.logs()).toEqual(['first', 'second', 'third']);
    expect(container.childElementCount).toBe(3);
    expect(publish).toHaveBeenCalledTimes(1);
    expect(measure).toHaveBeenCalledTimes(1);
    expect(scroll).toHaveBeenCalledWith(60);
    stream.send('next frame');
    flushFrame();
    expect(publish).toHaveBeenCalledTimes(2);
    expect(state.logs().at(-1)).toBe('next frame');
  });

  it('bounds a frame-starved pending buffer and retains the newest 1,000 in arrival order', async () => {
    const { state, stream, measure, publish } = await mount();
    for (let index = 0; index < 1007; index += 1) stream.send(`line-${index}`);
    expect(frames.size).toBe(1);
    expect(measure).not.toHaveBeenCalled();
    flushFrame();
    expect(state.logs()).toEqual(Array.from({ length: 1000 }, (_, index) => `line-${index + 7}`));
    expect(publish).toHaveBeenCalledTimes(1);
    stream.send('last-a');
    stream.send('last-b');
    flushFrame();
    expect(state.logs()).toEqual([
      ...Array.from({ length: 998 }, (_, index) => `line-${index + 9}`),
      'last-a',
      'last-b',
    ]);
  });

  it('keeps already accepted lines on pause, drops paused arrivals and resumes without replay', async () => {
    const { state, stream } = await mount();
    stream.send('before pause');
    state.togglePaused();
    expect(state.logs()).toEqual(['before pause']);
    expect(state.isPaused()).toBe(true);
    expect(frames.size).toBe(0);
    stream.send('discarded while paused');
    flushFrame();
    expect(state.logs()).toEqual(['before pause']);
    state.togglePaused();
    stream.send('after resume');
    flushFrame();
    expect(state.logs()).toEqual(['before pause', 'after resume']);
    expect(stream.close).not.toHaveBeenCalled();
  });

  it('clears both visible and pending lines without closing the stream or resurrecting them', async () => {
    const { state, stream } = await mount();
    stream.send('visible');
    flushFrame();
    stream.send('pending');
    state.clearLogs();
    flushFrame();
    expect(state.logs()).toEqual([]);
    expect(frames.size).toBe(0);
    stream.send('after clear');
    flushFrame();
    expect(state.logs()).toEqual(['after clear']);
    state.togglePaused();
    state.clearLogs();
    expect(state.isPaused()).toBe(true);
    expect(state.logs()).toEqual([]);
    expect(stream.close).not.toHaveBeenCalled();
  });

  it('cancels pending rendering and closes the stream when the panel unmounts', async () => {
    const { state, stream, view, measure } = await mount();
    stream.send('pending at unmount');
    const staleCallback = [...frames.values()][0];
    view.unmount();
    expect(frames.size).toBe(0);
    expect(stream.close).toHaveBeenCalledTimes(1);
    staleCallback(0);
    stream.send('late message');
    expect(state.logs()).toEqual([]);
    expect(measure).not.toHaveBeenCalled();
    expect(requestAnimationFrame).toHaveBeenCalledTimes(1);
  });

  it('does not open a stream if the panel unmounts during its initial level request', async () => {
    let finish!: (value: { level: string }) => void;
    mocks.api.mockReturnValue(
      new Promise((resolve) => {
        finish = resolve;
      }),
    );
    const { view, state } = await mount();
    expect(state.isLoading()).toBe(true);
    view.unmount();
    finish({ level: 'info' });
    await flushAsync();
    expect(FixtureEventSource.instances).toHaveLength(0);
  });

  it('keeps level fetch/change and their error handling independent of render scheduling', async () => {
    const { state, stream } = await mount();
    expect(stream.url).toBe('/api/logs/stream');
    expect(state.level()).toBe('info');
    expect(state.isLoading()).toBe(false);
    await state.handleLevelChange('warn');
    expect(mocks.api).toHaveBeenLastCalledWith('/api/logs/level', {
      method: 'POST',
      body: JSON.stringify({ level: 'warn' }),
    });
    expect(mocks.success).toHaveBeenCalledWith('Log level set to warn');
    mocks.api.mockRejectedValueOnce(new Error('fixture unavailable'));
    await state.handleLevelChange('error');
    expect(state.level()).toBe('warn');
    expect(mocks.error).toHaveBeenCalledWith('Failed to set log level');
  });
});
