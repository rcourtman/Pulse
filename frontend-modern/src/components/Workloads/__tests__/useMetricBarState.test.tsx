import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render } from '@solidjs/testing-library';
import { useMetricBarState } from '@/components/Workloads/useMetricBarState';

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

describe('useMetricBarState', () => {
  it('centralizes metric bar derivations and resize observer cleanup', () => {
    const observers: MockResizeObserver[] = [];
    globalThis.ResizeObserver = class extends MockResizeObserver {
      constructor(callback: ResizeObserverCallback) {
        super(callback);
        observers.push(this);
      }
    } as unknown as typeof ResizeObserver;

    let captured: ReturnType<typeof useMetricBarState> | undefined;

    const Harness = () => {
      captured = useMetricBarState({
        value: 75,
        label: 'Memory',
        sublabel: '6 GB / 8 GB',
        type: 'memory',
      });
      return <div ref={captured.setContainerRef} />;
    };

    const { unmount } = render(() => <Harness />);

    expect(captured).toBeDefined();
    expect(observers).toHaveLength(1);
    expect(observers[0].observe).toHaveBeenCalled();
    expect(captured!.presentation().progressColorClass).toContain('warning');

    observers[0].trigger(320);
    expect(captured!.presentation().showSublabel).toBe(true);

    unmount();
    expect(observers[0].disconnect).toHaveBeenCalled();
  });

  it('shows the long label only at an observed width that fits it', () => {
    const observers: MockResizeObserver[] = [];
    globalThis.ResizeObserver = class extends MockResizeObserver {
      constructor(callback: ResizeObserverCallback) {
        super(callback);
        observers.push(this);
      }
    } as unknown as typeof ResizeObserver;

    let captured: ReturnType<typeof useMetricBarState> | undefined;

    const Harness = () => {
      captured = useMetricBarState({
        value: 60,
        label: '60%',
        sublabel: '265 GB/440 GB',
        type: 'memory',
      });
      return <div ref={captured.setContainerRef} />;
    };

    render(() => <Harness />);

    // '60% (265 GB/440 GB)' renders 110.1px of text plus 4px of padding. The
    // width arrives from the resize observer alone; the fit never reads the
    // bar's own layout.
    observers[0].trigger(113);
    expect(captured!.presentation().showSublabel).toBe(false);
    observers[0].trigger(115);
    expect(captured!.presentation().showSublabel).toBe(true);
    observers[0].trigger(109);
    expect(captured!.presentation().showSublabel).toBe(false);
  });
});
