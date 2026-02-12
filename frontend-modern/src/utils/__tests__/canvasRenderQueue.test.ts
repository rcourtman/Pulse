import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

describe('canvasRenderQueue', () => {
  const frameCallbacks = new Map<number, FrameRequestCallback>();
  let nextFrameId = 1;

  beforeEach(() => {
    vi.resetModules();
    frameCallbacks.clear();
    nextFrameId = 1;

    vi.stubGlobal(
      'requestAnimationFrame',
      vi.fn((callback: FrameRequestCallback) => {
        const id = nextFrameId++;
        frameCallbacks.set(id, callback);
        return id;
      }),
    );

    vi.stubGlobal(
      'cancelAnimationFrame',
      vi.fn((id: number) => {
        frameCallbacks.delete(id);
      }),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('cancels a pending frame when the last callback is unregistered', async () => {
    const { scheduleSparkline } = await import('../canvasRenderQueue');
    const draw = vi.fn();

    const unregister = scheduleSparkline(draw);
    expect(requestAnimationFrame).toHaveBeenCalledTimes(1);

    unregister();

    expect(cancelAnimationFrame).toHaveBeenCalledWith(1);
    expect(frameCallbacks.size).toBe(0);
    expect(draw).not.toHaveBeenCalled();
  });

  it('defers callbacks queued during flush to the next animation frame', async () => {
    const { scheduleSparkline } = await import('../canvasRenderQueue');
    const second = vi.fn();
    const first = vi.fn(() => {
      scheduleSparkline(second);
    });

    scheduleSparkline(first);
    expect(requestAnimationFrame).toHaveBeenCalledTimes(1);

    frameCallbacks.get(1)?.(0);

    expect(first).toHaveBeenCalledTimes(1);
    expect(second).not.toHaveBeenCalled();
    expect(requestAnimationFrame).toHaveBeenCalledTimes(2);

    frameCallbacks.get(2)?.(16);

    expect(second).toHaveBeenCalledTimes(1);
  });
});
