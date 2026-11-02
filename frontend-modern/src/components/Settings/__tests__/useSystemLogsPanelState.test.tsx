import { For, createEffect, createSignal } from 'solid-js';
import { cleanup, render } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useSystemLogsPanelState, type SystemLogsPanelProps } from '../useSystemLogsPanelState';
import { eventBus } from '@/stores/events';

const mocks = vi.hoisted(() => ({
  api: vi.fn(),
  success: vi.fn(),
  error: vi.fn(),
  org: 'default',
}));
vi.mock('@/utils/apiClient', () => ({ apiFetchJSON: mocks.api, getOrgID: () => mocks.org }));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: mocks.success, error: mocks.error },
}));
vi.mock('@/utils/logger', () => ({ logger: { debug: vi.fn(), error: vi.fn(), org: 'default' } }));

class FixtureEventSource {
  static instances: FixtureEventSource[] = [];
  static CLOSED = 2;
  readyState = 0;
  onopen: (() => void) | null = null;
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

async function mount(props: SystemLogsPanelProps = {}) {
  let state!: ReturnType<typeof useSystemLogsPanelState>;
  const publish = vi.fn();
  const view = render(() => {
    state = useSystemLogsPanelState(props);
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
  mocks.org = 'default';
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
  it('withdraws private lines on transient loss and requires explicit recovery after terminal loss', async () => {
    const { state, stream, view } = await mount();
    expect(state.connectionState()).toBe('connecting');
    stream.onopen?.();
    stream.send('accepted before disconnect');
    flushFrame();
    stream.send('pending before disconnect');
    state.togglePaused();
    stream.readyState = 0;
    stream.onerror?.();
    expect(state.connectionState()).toBe('reconnecting');
    expect(state.isPaused()).toBe(true);
    expect(state.logs()).toEqual([]);
    expect(frames.size).toBe(0);
    expect(state.canUseControls()).toBe(false);
    expect(stream.close).not.toHaveBeenCalled();
    stream.onopen?.();
    expect(state.connectionState()).toBe('streaming');
    stream.send('not collected while paused');
    flushFrame();
    expect(state.logs()).toEqual([]);
    stream.readyState = 2;
    stream.onerror?.();
    expect(state.connectionState()).toBe('disconnected');
    expect(stream.close).toHaveBeenCalledTimes(1);
    stream.onopen?.();
    stream.send('obsolete queued callback');
    flushFrame();
    expect(state.connectionState()).toBe('disconnected');
    expect(state.logs()).toEqual([]);
    await state.reconnect();
    expect(FixtureEventSource.instances).toHaveLength(2);
    const next = FixtureEventSource.instances[1];
    next.onopen?.();
    state.togglePaused();
    next.send('fresh reader line');
    flushFrame();
    expect(state.logs()).toEqual(['fresh reader line']);
    view.unmount();
    expect(next.close).toHaveBeenCalledTimes(1);
    expect(state.logs()).toEqual([]);
  });

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
    stream.onopen?.();
    await state.handleLevelChange('warn');
    expect(mocks.api).toHaveBeenLastCalledWith('/api/logs/level', {
      method: 'POST',
      body: JSON.stringify({ level: 'warn' }),
      signal: expect.any(AbortSignal),
      expectedOrgID: 'default',
      retry: false,
    });
    expect(state.notice()).toBe('Log level set to warn');
    mocks.api.mockRejectedValueOnce(new Error('fixture unavailable'));
    await state.handleLevelChange('error');
    expect(state.level()).toBe('warn');
    expect(state.error()).toMatch(/change could not be confirmed/);
    expect(state.logs()).toEqual([]);
    expect(state.canUseControls()).toBe(false);
    expect(mocks.success).not.toHaveBeenCalled();
    expect(mocks.error).not.toHaveBeenCalled();
  });
  it('binds initial reads to the workspace, disables actions until verified, and never starts a stream after a failed or malformed check', async () => {
    for (const value of [null, {}, { level: 'not-a-level' }]) {
      mocks.api.mockResolvedValueOnce(value);
      const { state, view } = await mount();
      expect(FixtureEventSource.instances).toHaveLength(0);
      expect(state.canUseControls()).toBe(false);
      expect(state.error()).toMatch(/Log access could not be confirmed/);
      await state.handleLevelChange('debug');
      view.unmount();
    }
    expect(mocks.api).toHaveBeenCalledTimes(3);
    for (const call of mocks.api.mock.calls)
      expect(call).toEqual([
        '/api/logs/level',
        {
          signal: expect.any(AbortSignal),
          expectedOrgID: 'default',
          retry: false,
        },
      ]);
  });

  it('withdraws visible and frame-starved buffers on workspace change, including a return to identical reader names', async () => {
    const { state, stream } = await mount();
    stream.send('visible-private');
    flushFrame();
    stream.send('pending-private');
    const oldFrame = [...frames.values()][0];
    mocks.org = 'tenant-b';
    eventBus.emit('org_switched', 'tenant-b');
    expect(state.logs()).toEqual([]);
    expect(state.canUseControls()).toBe(false);
    expect(stream.close).toHaveBeenCalledTimes(1);
    expect(FixtureEventSource.instances).toHaveLength(1);
    mocks.org = 'default';
    eventBus.emit('org_switched', 'default');
    await state.reconnect();
    const next = FixtureEventSource.instances[1];
    next.send('current-private');
    oldFrame(0);
    stream.send('late-private');
    stream.onopen?.();
    expect(state.logs()).toEqual([]);
    flushFrame();
    expect(state.logs()).toEqual(['current-private']);
  });

  it('ignores equal access facts but withdraws on principal, credential and capability changes', async () => {
    const [user, setUser] = createSignal('operator');
    const [key, setKey] = createSignal('credential-a');
    const [allowed, setAllowed] = createSignal(true);
    const { state, stream } = await mount({
      get currentUser() {
        return user();
      },
      get accessKey() {
        return key();
      },
      get canRead() {
        return allowed();
      },
    });
    stream.send('private');
    flushFrame();
    setUser('operator');
    setKey('credential-a');
    expect(state.logs()).toEqual(['private']);
    setUser('different-operator');
    expect(state.logs()).toEqual([]);
    expect(stream.close).toHaveBeenCalledTimes(1);
    expect(FixtureEventSource.instances).toHaveLength(1);
    await state.reconnect();
    FixtureEventSource.instances[1].send('current');
    flushFrame();
    setKey('credential-b');
    expect(state.logs()).toEqual([]);
    await state.reconnect();
    FixtureEventSource.instances[2].send('current');
    flushFrame();
    setAllowed(false);
    expect(state.logs()).toEqual([]);
    expect(state.canReconnect()).toBe(false);
    await state.reconnect();
    await state.handleLevelChange('debug');
    expect(FixtureEventSource.instances).toHaveLength(3);
    setAllowed(true);
    expect(state.canReconnect()).toBe(true);
    expect(FixtureEventSource.instances).toHaveLength(3);
  });

  it('cancels obsolete level reads and rejects their late completion even after returning to the same workspace', async () => {
    let finish!: (value: { level: string }) => void;
    mocks.api.mockReturnValueOnce(
      new Promise((resolve) => {
        finish = resolve;
      }),
    );
    const { state } = await mount();
    const options = mocks.api.mock.calls[0][1];
    mocks.org = 'tenant-b';
    eventBus.emit('org_switched', 'tenant-b');
    mocks.org = 'default';
    eventBus.emit('org_switched', 'default');
    expect(options.signal.aborted).toBe(true);
    finish({ level: 'debug' });
    await flushAsync();
    expect(FixtureEventSource.instances).toHaveLength(0);
    expect(state.level()).toBe('info');
    await state.reconnect();
    expect(FixtureEventSource.instances).toHaveLength(1);
  });

  it('single-submits level changes and withholds late success after disposal', async () => {
    const { state, stream, view } = await mount();
    stream.onopen?.();
    let finish!: () => void;
    mocks.api.mockReturnValueOnce(
      new Promise<void>((resolve) => {
        finish = resolve;
      }),
    );
    const changing = state.handleLevelChange('warn');
    await state.handleLevelChange('debug');
    expect(mocks.api).toHaveBeenCalledTimes(2);
    const options = mocks.api.mock.calls[1][1];
    view.unmount();
    expect(options.signal.aborted).toBe(true);
    finish();
    await changing;
    expect(state.notice()).toBe('');
    expect(state.error()).toBe('');
    expect(mocks.success).not.toHaveBeenCalled();
  });
  it('starts support downloads without navigating the live stream and disables them when access is unknown', async () => {
    const { state, stream } = await mount();
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (
      this: HTMLAnchorElement,
    ) {
      expect(this.isConnected).toBe(true);
      expect(this.getAttribute('href')).toBe('/api/logs/download');
      expect(this.hasAttribute('download')).toBe(true);
    });
    state.handleDownload();
    expect(click).not.toHaveBeenCalled();
    stream.onopen?.();
    state.handleDownload();
    expect(click).toHaveBeenCalledTimes(1);
    expect(document.querySelector('a[download]')).toBeNull();
    stream.readyState = 2;
    stream.onerror?.();
    state.handleDownload();
    expect(click).toHaveBeenCalledTimes(1);
    click.mockRestore();
  });
});
