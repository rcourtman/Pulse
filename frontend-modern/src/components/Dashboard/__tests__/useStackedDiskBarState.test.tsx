import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render } from '@solidjs/testing-library';
import type { Disk } from '@/types/api';
import { useStackedDiskBarState } from '@/components/Dashboard/useStackedDiskBarState';

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

function makeDisk(overrides: Partial<Disk> = {}): Disk {
  return {
    total: 107374182400,
    used: 53687091200,
    free: 53687091200,
    usage: 50,
    mountpoint: '/',
    type: 'ext4',
    device: '/dev/sda1',
    ...overrides,
  };
}

afterEach(() => {
  cleanup();
  globalThis.ResizeObserver = originalResizeObserver;
});

describe('useStackedDiskBarState', () => {
  it('centralizes stacked disk derivations and resize observer cleanup', () => {
    const observers: MockResizeObserver[] = [];
    globalThis.ResizeObserver = class extends MockResizeObserver {
      constructor(callback: ResizeObserverCallback) {
        super(callback);
        observers.push(this);
      }
    } as unknown as typeof ResizeObserver;

    let captured: ReturnType<typeof useStackedDiskBarState> | undefined;

    const Harness = () => {
      captured = useStackedDiskBarState({
        disks: [makeDisk({ mountpoint: '/boot' }), makeDisk({ mountpoint: '/data' })],
        mode: 'aggregate',
      });
      return <div ref={captured.setContainerRef} />;
    };

    const { unmount } = render(() => <Harness />);

    expect(captured).toBeDefined();
    expect(observers).toHaveLength(1);
    expect(observers[0].observe).toHaveBeenCalled();
    expect(captured!.presentation().hasMultipleDisks).toBe(true);
    expect(captured!.presentation().aggregateMode).toBe(true);

    observers[0].trigger(320);
    expect(captured!.presentation().showSublabel).toBe(true);

    unmount();
    expect(observers[0].disconnect).toHaveBeenCalled();
  });
});
