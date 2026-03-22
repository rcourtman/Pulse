import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';

import { useDashboardWorkloadViewportSync } from '@/components/Dashboard/useDashboardWorkloadViewportSync';
import type { UseGroupedTableWindowingResult } from '@/components/Dashboard/useGroupedTableWindowing';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe('useDashboardWorkloadViewportSync', () => {
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

      useDashboardWorkloadViewportSync({
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

    expect(addEventListenerSpy).toHaveBeenCalledWith(
      'scroll',
      expect.any(Function),
      { passive: true },
    );
    expect(addEventListenerSpy).toHaveBeenCalledWith('resize', expect.any(Function));

    window.dispatchEvent(new Event('scroll'));
    await waitFor(() => {
      expect(onScroll).toHaveBeenCalledTimes(2);
    });

    unmount();

    expect(removeEventListenerSpy).toHaveBeenCalledWith('scroll', expect.any(Function));
    expect(removeEventListenerSpy).toHaveBeenCalledWith('resize', expect.any(Function));
  });
});
