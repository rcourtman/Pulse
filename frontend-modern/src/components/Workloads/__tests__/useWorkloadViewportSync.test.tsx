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
        rowHeight: () => 32,
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
    expect(addEventListenerSpy).toHaveBeenCalledWith('wheel', expect.any(Function), {
      passive: true,
    });
    expect(addEventListenerSpy.mock.calls.some(([type]) => String(type).startsWith('touch'))).toBe(
      false,
    );
    expect(addEventListenerSpy).toHaveBeenCalledWith('resize', expect.any(Function));

    window.dispatchEvent(new Event('scroll'));
    await waitFor(() => {
      expect(onScroll).toHaveBeenCalledTimes(2);
    });

    unmount();

    expect(removeEventListenerSpy).toHaveBeenCalledWith('scroll', expect.any(Function));
    expect(removeEventListenerSpy).toHaveBeenCalledWith('wheel', expect.any(Function));
    expect(
      removeEventListenerSpy.mock.calls.some(([type]) => String(type).startsWith('touch')),
    ).toBe(false);
    expect(removeEventListenerSpy).toHaveBeenCalledWith('resize', expect.any(Function));
  });

  it('captures expanded detail geometry when the drawer mounts before scrolling', async () => {
    const onExpandedDetailHeightChange = vi.fn();
    const groupedWindowing: UseGroupedTableWindowingResult = {
      endIndex: () => 36,
      getVisibleSlice: (_groupKey, guests) => guests,
      isWindowed: () => true,
      mountedCount: () => 36,
      onScroll: vi.fn(),
      revealIndex: vi.fn(),
      startIndex: () => 0,
    };

    const Harness = () => {
      const [bodyRef, setBodyRef] = createSignal<HTMLTableSectionElement | null>(null);

      useWorkloadViewportSync({
        filteredGuestCount: () => 640,
        groupedWindowing,
        onExpandedDetailHeightChange,
        rowHeight: () => 37,
        selectedGuestId: () => 'guest-1',
        tableBodyRef: bodyRef,
      });

      return (
        <table>
          <tbody ref={setBodyRef}>
            <tr
              data-inline-detail-for="guest-1"
              ref={(element) => {
                vi.spyOn(element, 'getBoundingClientRect').mockReturnValue({
                  bottom: 900,
                  height: 900,
                  left: 0,
                  right: 800,
                  toJSON: () => ({}),
                  top: 0,
                  width: 800,
                  x: 0,
                  y: 0,
                } as DOMRect);
              }}
            >
              <td>Expanded guest details</td>
            </tr>
          </tbody>
        </table>
      );
    };

    render(() => <Harness />);

    await waitFor(() => {
      expect(onExpandedDetailHeightChange).toHaveBeenCalledWith(900);
    });
  });

  it('tracks the app scroll container instead of leaving the initial spacer in place', async () => {
    let expandedDetailActive = false;
    let appScrollContainer!: HTMLDivElement;
    const onScroll = vi.fn();
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
        expandedDetailActive: () => expandedDetailActive,
        filteredGuestCount: () => 640,
        groupedWindowing,
        rowHeight: () => 32,
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

    const wheelEvent = new WheelEvent('wheel', {
      cancelable: true,
      deltaY: 120,
    });
    appScrollContainer.scrollTop = 300;
    appScrollContainer.dispatchEvent(wheelEvent);
    expect(onScroll).toHaveBeenLastCalledWith(680, 400, 32);
    expect(wheelEvent.defaultPrevented).toBe(false);

    expandedDetailActive = true;
    const callsBeforeExpandedInput = onScroll.mock.calls.length;
    appScrollContainer.dispatchEvent(wheelEvent);
    expect(onScroll).toHaveBeenCalledTimes(callsBeforeExpandedInput);
  });

  it('exposes one app-shell back-to-top action after sustained workload scrolling', async () => {
    let appScrollContainer!: HTMLDivElement;
    let viewportSync!: ReturnType<typeof useWorkloadViewportSync>;
    const scrollTo = vi.fn();
    const groupedWindowing: UseGroupedTableWindowingResult = {
      endIndex: () => 20,
      getVisibleSlice: (_groupKey, guests) => guests,
      isWindowed: () => false,
      mountedCount: () => 20,
      onScroll: vi.fn(),
      revealIndex: vi.fn(),
      startIndex: () => 0,
    };

    const Harness = () => {
      const [bodyRef, setBodyRef] = createSignal<HTMLTableSectionElement | null>(null);
      viewportSync = useWorkloadViewportSync({
        filteredGuestCount: () => 20,
        groupedWindowing,
        rowHeight: () => 32,
        tableBodyRef: bodyRef,
      });

      return (
        <div
          ref={(element) => {
            appScrollContainer = element;
            Object.defineProperty(element, 'clientHeight', { configurable: true, value: 400 });
            Object.defineProperty(element, 'scrollHeight', { configurable: true, value: 1600 });
            Object.defineProperty(element, 'scrollTo', { configurable: true, value: scrollTo });
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
          <table>
            <tbody ref={setBodyRef} />
          </table>
        </div>
      );
    };

    render(() => <Harness />);
    expect(viewportSync.isScrollToTopVisible()).toBe(false);

    appScrollContainer.scrollTop = 700;
    appScrollContainer.dispatchEvent(new Event('scroll'));
    await waitFor(() => expect(viewportSync.isScrollToTopVisible()).toBe(true));

    viewportSync.scrollToTop();
    expect(scrollTo).toHaveBeenCalledWith({ top: 0, behavior: 'smooth' });

    appScrollContainer.scrollTop = 0;
    appScrollContainer.dispatchEvent(new Event('scroll'));
    await waitFor(() => expect(viewportSync.isScrollToTopVisible()).toBe(false));
  });
});
