import { cleanup, render, waitFor } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { useObservedElementWidth } from '../useObservedElementWidth';

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe('useObservedElementWidth', () => {
  it('tracks the rendered container and disconnects its observer', async () => {
    let measuredWidth = 640;
    let resizeCallback: ResizeObserverCallback | undefined;
    const disconnect = vi.fn();

    class ResizeObserverStub {
      constructor(callback: ResizeObserverCallback) {
        resizeCallback = callback;
      }
      observe = vi.fn();
      disconnect = () => disconnect();
      unobserve = vi.fn();
    }
    vi.stubGlobal('ResizeObserver', ResizeObserverStub);

    let width = () => null as number | null;
    const Harness = () => {
      const observed = useObservedElementWidth();
      width = observed.width;
      return (
        <div
          ref={(element) => {
            Object.defineProperty(element, 'clientWidth', {
              configurable: true,
              get: () => measuredWidth,
            });
            observed.setElement(element);
          }}
        />
      );
    };

    const { unmount } = render(() => <Harness />);
    await waitFor(() => expect(width()).toBe(640));

    measuredWidth = 880;
    resizeCallback?.([], {} as ResizeObserver);
    await waitFor(() => expect(width()).toBe(880));

    unmount();
    expect(disconnect).toHaveBeenCalledOnce();
  });
});
