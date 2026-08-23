import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';

import { useWorkloadViewportSync } from '@/components/Workloads/useWorkloadViewportSync';
import type { UseGroupedTableWindowingResult } from '@/components/Workloads/useGroupedTableWindowing';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe('useWorkloadViewportSync', () => {
  it('owns grouped workload viewport sync and listener cleanup', async () => {
    const onScroll = vi.fn();
    const addEventListenerSpy = vi.spyOn(window, 'addEventListener');
    const removeEventListenerSpy = vi.spyOn(window, 'removeEventListener');
    const groupedWindowing: UseGroupedTableWindowingResult = {
      endIndex: () => 10,
      getVisibleSlice: (_groupKey, guests) => guests,
      isWindowed: () => true,
      mountedCount: () => 10,
      onScroll,
      revealIndex: vi.fn(),
      startIndex: () => 0,
    };

    const Harness = () => {
      const [bodyRef, setBodyRef] = createSignal<HTMLTableSectionElement | null>(null);

      useWorkloadViewportSync({
        filteredGuestCount: () => 640,
        groupedWindowing,
        rowHeight: 32,
        tableBodyRef: bodyRef,
      });

      return (
        <table>
          <tbody
            ref={(element) => {
              vi.spyOn(element, 'getBoundingClientRect').mockReturnValue({
                bottom: 400,
                height: 320,
                left: 0,
                right: 0,
                toJSON: () => ({}),
                top: -96,
                width: 800,
                x: 0,
                y: -96,
              } as DOMRect);
              setBodyRef(element);
            }}
          />
        </table>
      );
    };

    const { unmount } = render(() => <Harness />);

    await waitFor(() => {
      expect(onScroll).toHaveBeenCalledWith(96, window.innerHeight, 32);
    });

    expect(addEventListenerSpy).toHaveBeenCalledWith('scroll', expect.any(Function), {
      passive: true,
    });
    expect(addEventListenerSpy).toHaveBeenCalledWith('resize', expect.any(Function));

    window.dispatchEvent(new Event('scroll'));
    await waitFor(() => {
      expect(onScroll).toHaveBeenCalledTimes(2);
    });

    unmount();

    expect(removeEventListenerSpy).toHaveBeenCalledWith('scroll', expect.any(Function));
    expect(removeEventListenerSpy).toHaveBeenCalledWith('resize', expect.any(Function));
  });

  it('tracks the app scroll container instead of leaving the initial spacer in place', async () => {
    const onScroll = vi.fn();
    let appScrollContainer!: HTMLDivElement;
    let horizontalTableWrapper!: HTMLDivElement;
    const groupedWindowing: UseGroupedTableWindowingResult = {
      endIndex: () => 150,
      getVisibleSlice: (_groupKey, guests) => guests,
      isWindowed: () => true,
      mountedCount: () => 140,
      onScroll,
      revealIndex: vi.fn(),
      startIndex: () => 0,
    };

    const Harness = () => {
      const [bodyRef, setBodyRef] = createSignal<HTMLTableSectionElement | null>(null);

      useWorkloadViewportSync({
        filteredGuestCount: () => 640,
        groupedWindowing,
        rowHeight: 32,
        tableBodyRef: bodyRef,
      });

      return (
        <div
          ref={(element) => {
            appScrollContainer = element;
            Object.defineProperty(element, 'clientHeight', { configurable: true, value: 400 });
            // The explicit app scroll shell owns future vertical overflow even
            // if the table rows have not contributed their final height yet.
            Object.defineProperty(element, 'scrollHeight', { configurable: true, value: 400 });
            vi.spyOn(element, 'getBoundingClientRect').mockReturnValue({
              bottom: 400,
              height: 400,
              left: 0,
              right: 800,
              toJSON: () => ({}),
              top: 0,
              width: 800,
              x: 0,
              y: 0,
            } as DOMRect);
          }}
          style={{ 'overflow-y': 'scroll' }}
        >
          <div
            ref={(element) => {
              horizontalTableWrapper = element;
              Object.defineProperty(element, 'clientHeight', { configurable: true, value: 2800 });
              Object.defineProperty(element, 'scrollHeight', { configurable: true, value: 2800 });
            }}
            style={{ 'overflow-x': 'auto' }}
          >
            <table>
              <tbody
                ref={(element) => {
                  vi.spyOn(element, 'getBoundingClientRect').mockReturnValue({
                    bottom: -240,
                    height: 320,
                    left: 0,
                    right: 800,
                    toJSON: () => ({}),
                    top: -560,
                    width: 800,
                    x: 0,
                    y: -560,
                  } as DOMRect);
                  setBodyRef(element);
                }}
              />
            </table>
          </div>
        </div>
      );
    };

    render(() => <Harness />);

    await waitFor(() => {
      expect(onScroll).toHaveBeenCalledWith(560, 400, 32);
    });

    appScrollContainer.dispatchEvent(new Event('scroll'));
    await waitFor(() => {
      expect(onScroll).toHaveBeenCalledTimes(2);
    });

    horizontalTableWrapper.dispatchEvent(new Event('scroll'));
    expect(onScroll).toHaveBeenCalledTimes(2);
  });
});
