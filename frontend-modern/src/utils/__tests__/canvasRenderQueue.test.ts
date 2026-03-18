import { afterEach, describe, expect, it, vi } from 'vitest';

describe('canvasRenderQueue', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.resetModules();
  });

  it('cancels scheduled frame when last callback unregisters', async () => {
    const callbacks = new Map<number, FrameRequestCallback>();
    let nextId = 1;

    const requestAnimationFrameMock = vi.fn((cb: FrameRequestCallback) => {
      const id = nextId++;
      callbacks.set(id, cb);
      return id;
    });
    const cancelAnimationFrameMock = vi.fn((id: number) => {
      callbacks.delete(id);
    });

    vi.stubGlobal('requestAnimationFrame', requestAnimationFrameMock);
    vi.stubGlobal('cancelAnimationFrame', cancelAnimationFrameMock);

    const { scheduleSparkline } = await import('@/utils/canvasRenderQueue');

    const cleanup = scheduleSparkline(vi.fn());
    expect(requestAnimationFrameMock).toHaveBeenCalledTimes(1);
    expect(callbacks.size).toBe(1);

    cleanup();

    expect(cancelAnimationFrameMock).toHaveBeenCalledTimes(1);
    expect(callbacks.size).toBe(0);
  });

  it('flushes pending draw callbacks on animation frame', async () => {
    const callbacks = new Map<number, FrameRequestCallback>();
    let nextId = 1;

    vi.stubGlobal(
      'requestAnimationFrame',
      vi.fn((cb: FrameRequestCallback) => {
        const id = nextId++;
        callbacks.set(id, cb);
        return id;
      }),
    );
    vi.stubGlobal(
      'cancelAnimationFrame',
      vi.fn((id: number) => {
        callbacks.delete(id);
      }),
    );

    const { scheduleSparkline } = await import('@/utils/canvasRenderQueue');

    const draw = vi.fn();
    const cleanup = scheduleSparkline(draw);
    const frameCallback = callbacks.values().next().value as FrameRequestCallback;

    frameCallback(0);
    cleanup();

    expect(draw).toHaveBeenCalledTimes(1);
  });
});
