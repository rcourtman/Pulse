import { createEffect, onCleanup, type Accessor } from 'solid-js';

import type { UseGroupedTableWindowingResult } from './useGroupedTableWindowing';

const SCROLLABLE_OVERFLOW_PATTERN = /(?:auto|scroll|overlay)/;
const MIN_VERTICAL_SCROLL_RANGE_PX = 1;

const findScrollContainer = (element: HTMLElement): HTMLElement | null => {
  let parent = element.parentElement;
  while (parent && parent !== document.body && parent !== document.documentElement) {
    const styles = getComputedStyle(parent);
    const hasVerticalScrollRange =
      parent.scrollHeight - parent.clientHeight > MIN_VERTICAL_SCROLL_RANGE_PX;
    const ownsVerticalScrollBeforeOverflow = styles.overflowY === 'scroll';
    if (
      SCROLLABLE_OVERFLOW_PATTERN.test(styles.overflowY) &&
      (ownsVerticalScrollBeforeOverflow || hasVerticalScrollRange)
    ) {
      return parent;
    }
    parent = parent.parentElement;
  }
  return null;
};

interface WorkloadsWorkloadViewportSyncOptions {
  filteredGuestCount: Accessor<number>;
  groupedWindowing: UseGroupedTableWindowingResult;
  rowHeight: number;
  tableBodyRef: Accessor<HTMLTableSectionElement | null>;
}

export function useWorkloadViewportSync(options: WorkloadsWorkloadViewportSyncOptions) {
  const syncGuestWindowToViewport = () => {
    if (!options.groupedWindowing.isWindowed() || typeof window === 'undefined') return;
    const body = options.tableBodyRef();
    if (!body) return;
    const rect = body.getBoundingClientRect();
    const scrollContainer = findScrollContainer(body);
    if (scrollContainer) {
      const containerRect = scrollContainer.getBoundingClientRect();
      const scrollTop = Math.max(0, containerRect.top - rect.top);
      options.groupedWindowing.onScroll(
        scrollTop,
        scrollContainer.clientHeight || window.innerHeight,
        options.rowHeight,
      );
      return;
    }

    options.groupedWindowing.onScroll(
      Math.max(0, -rect.top),
      window.innerHeight,
      options.rowHeight,
    );
  };

  createEffect(() => {
    if (typeof window === 'undefined') return;
    options.filteredGuestCount();
    if (!options.groupedWindowing.isWindowed()) return;
    if (!options.tableBodyRef()) return;

    const handleViewportChange = () => {
      syncGuestWindowToViewport();
    };

    handleViewportChange();
    const scrollContainer = findScrollContainer(options.tableBodyRef()!);
    const scrollTarget = scrollContainer ?? window;
    scrollTarget.addEventListener('scroll', handleViewportChange, { passive: true });
    window.addEventListener('resize', handleViewportChange);
    onCleanup(() => {
      scrollTarget.removeEventListener('scroll', handleViewportChange);
      window.removeEventListener('resize', handleViewportChange);
    });
  });
}
