import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render } from '@solidjs/testing-library';
import { useStackedMemoryBarState } from '@/components/Dashboard/useStackedMemoryBarState';

class MockResizeObserver {
  callback: ResizeObserverCallback;
  disconnect = vi.fn();
  observe = vi.fn();

  constructor(callback: ResizeObserverCallback) {
    this.callback = callback;
  }

  trigger(width: number) {
    this.callback(
      [
        {
          contentRect: { width } as DOMRectReadOnly,
        } as ResizeObserverEntry,
      ],
      this as unknown as ResizeObserver,
    );
  }
}

const originalResizeObserver = globalThis.ResizeObserver;

afterEach(() => {
  cleanup();
  globalThis.ResizeObserver = originalResizeObserver;
});

describe('useStackedMemoryBarState', () => {
  it('centralizes stacked memory derivations and resize observer cleanup', () => {
    const observers: MockResizeObserver[] = [];
    globalThis.ResizeObserver = class extends MockResizeObserver {
      constructor(callback: ResizeObserverCallback) {
        super(callback);
        observers.push(this);
      }
    } as unknown as typeof ResizeObserver;

    let captured: ReturnType<typeof useStackedMemoryBarState> | undefined;

    const Harness = () => {
      captured = useStackedMemoryBarState({
        used: 4 * 1024 ** 3,
        total: 8 * 1024 ** 3,
        balloon: 6 * 1024 ** 3,
        swapUsed: 1 * 1024 ** 3,
        swapTotal: 2 * 1024 ** 3,
      });
      return <div ref={captured.setContainerRef} />;
    };

    const { unmount } = render(() => <Harness />);

    expect(captured).toBeDefined();
    expect(observers).toHaveLength(1);
    expect(observers[0].observe).toHaveBeenCalled();
    expect(captured!.presentation().segments).toHaveLength(2);
    expect(captured!.presentation().showSwapBar).toBe(true);

    observers[0].trigger(320);
    expect(captured!.presentation().showSublabel).toBe(true);

    unmount();
    expect(observers[0].disconnect).toHaveBeenCalled();
  });
});
