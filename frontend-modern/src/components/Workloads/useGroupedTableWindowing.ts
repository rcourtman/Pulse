import { createEffect, createMemo, createSignal, untrack } from 'solid-js';
import type { WorkloadGuest } from '@/types/workloads';

export interface UseGroupedTableWindowingOptions {
  /** Total number of guest rows across all groups */
  totalRowCount: () => number;
  /** Maximum rows to mount */
  windowSize?: number;
  /** Row count above which the bounded window activates */
  enableThreshold?: number;
  /** Whether windowing is enabled */
  enabled?: () => boolean;
  /** Index to ensure is visible (selection/deep-link reveal) */
  revealIndex?: () => number | null;
  /** Resolve a row index from a virtual offset when grouped decorations add height. */
  rowIndexAtOffset?: (offset: number, rowHeight: number) => number;
}

export interface UseGroupedTableWindowingResult {
  /** Whether windowing is active */
  isWindowed: () => boolean;
  /** Start index of the visible window (inclusive) */
  startIndex: () => number;
  /** End index of the visible window (exclusive) */
  endIndex: () => number;
  /** Get the visible slice of guests for a given group */
  getVisibleSlice: (
    groupKey: string,
    guests: WorkloadGuest[],
    groupStartIndex: number,
  ) => WorkloadGuest[];
  /** Total count of currently mounted guest rows */
  mountedCount: () => number;
  /** Scroll handler to move the window */
  onScroll: (scrollTop: number, containerHeight: number, rowHeight: number) => void;
  /** Jump window to include a specific global row index */
  revealIndex: (index: number) => void;
}

const DEFAULT_WINDOW_SIZE = 140;
// Keep even medium estates virtualized. On mobile, mounting a few hundred
// metric-heavy rows is already enough to cause long layout and paint tasks.
const DEFAULT_ENABLE_THRESHOLD = DEFAULT_WINDOW_SIZE;
const DEFAULT_OVERSCAN_ROWS = 20;
const DEFAULT_EDGE_RUNWAY_ROWS = 24;

export const useGroupedTableWindowing = (
  options: UseGroupedTableWindowingOptions,
): UseGroupedTableWindowingResult => {
  const [windowStart, setWindowStart] = createSignal(0);
  let lastFirstVisibleRow = 0;

  const normalizedWindowSize = createMemo(() =>
    Math.max(1, Math.floor(options.windowSize ?? DEFAULT_WINDOW_SIZE)),
  );

  const isWindowed = createMemo(() => {
    const total = options.totalRowCount();
    const threshold = Math.max(0, Math.floor(options.enableThreshold ?? DEFAULT_ENABLE_THRESHOLD));
    const enabled = options.enabled?.() ?? total > threshold;
    return enabled && total > 0;
  });

  const maxStart = createMemo(() => Math.max(0, options.totalRowCount() - normalizedWindowSize()));

  const startIndex = createMemo(() => {
    if (!isWindowed()) return 0;
    return Math.max(0, Math.min(windowStart(), maxStart()));
  });

  const endIndex = createMemo(() => {
    if (!isWindowed()) return options.totalRowCount();
    return Math.min(options.totalRowCount(), startIndex() + normalizedWindowSize());
  });

  const setClampedStart = (nextStart: number) => {
    const clamped = Math.max(0, Math.min(Math.floor(nextStart), maxStart()));
    setWindowStart((current) => (current === clamped ? current : clamped));
  };

  const revealIndex = (index: number) => {
    if (!isWindowed()) return;
    if (!Number.isFinite(index)) return;

    const normalizedIndex = Math.max(0, Math.min(Math.floor(index), options.totalRowCount() - 1));
    if (normalizedIndex >= startIndex() && normalizedIndex < endIndex()) return;

    const centeredStart = normalizedIndex - Math.floor(normalizedWindowSize() / 2);
    setClampedStart(centeredStart);
  };

  const onScroll = (scrollTop: number, containerHeight: number, rowHeight: number) => {
    if (!isWindowed()) return;

    const safeRowHeight = rowHeight > 0 ? rowHeight : 40;
    const safeContainerHeight = containerHeight > 0 ? containerHeight : safeRowHeight;
    const rowsInView = Math.max(1, Math.ceil(safeContainerHeight / safeRowHeight));
    const resolvedFirstVisibleRow = options.rowIndexAtOffset?.(
      Math.max(0, scrollTop),
      safeRowHeight,
    );
    const firstVisibleRow = Number.isFinite(resolvedFirstVisibleRow)
      ? Math.max(0, Math.floor(resolvedFirstVisibleRow!))
      : Math.floor(Math.max(0, scrollTop) / safeRowHeight);
    const visibleEnd = Math.min(options.totalRowCount(), firstVisibleRow + rowsInView);
    const availableBuffer = Math.max(0, normalizedWindowSize() - rowsInView);
    const edgeRunway = Math.min(
      Math.floor(availableBuffer / 2),
      Math.max(DEFAULT_EDGE_RUNWAY_ROWS, rowsInView),
    );
    const direction = Math.sign(firstVisibleRow - lastFirstVisibleRow);
    lastFirstVisibleRow = firstVisibleRow;

    const leadingRunway = firstVisibleRow - startIndex();
    const trailingRunway = endIndex() - visibleEnd;
    const viewportIsMounted = leadingRunway >= 0 && trailingRunway >= 0;
    if (
      viewportIsMounted &&
      ((direction > 0 && trailingRunway > edgeRunway) ||
        (direction < 0 && leadingRunway > edgeRunway) ||
        (direction === 0 && leadingRunway >= edgeRunway && trailingRunway >= edgeRunway))
    ) {
      return;
    }

    // Keep most of the spare window in the active scroll direction. Unlike
    // tracking every visible row, this runway only moves when the viewport
    // approaches an edge, avoiding a full reactive slice update per wheel tick.
    const directionalRunway = Math.max(edgeRunway, availableBuffer - edgeRunway);
    const rowsBeforeViewport =
      direction < 0
        ? directionalRunway
        : direction > 0
          ? edgeRunway
          : Math.min(DEFAULT_OVERSCAN_ROWS, availableBuffer);
    setClampedStart(firstVisibleRow - rowsBeforeViewport);
  };

  const getVisibleSlice = (
    _groupKey: string,
    guests: WorkloadGuest[],
    groupStartIndex: number,
  ): WorkloadGuest[] => {
    if (!isWindowed()) return guests;

    const groupEndIndex = groupStartIndex + guests.length;
    if (groupEndIndex <= startIndex() || groupStartIndex >= endIndex()) return [];

    const sliceStart = Math.max(0, startIndex() - groupStartIndex);
    const sliceEnd = Math.min(guests.length, endIndex() - groupStartIndex);
    return guests.slice(sliceStart, sliceEnd);
  };

  const mountedCount = createMemo(() => {
    if (!isWindowed()) return options.totalRowCount();
    return Math.max(0, endIndex() - startIndex());
  });

  createEffect(() => {
    if (!isWindowed()) {
      setWindowStart(0);
      return;
    }
    setClampedStart(windowStart());
  });

  createEffect(() => {
    if (!isWindowed()) return;
    const target = options.revealIndex?.();
    if (target == null || target < 0) return;
    // A reveal target is an initial positioning request, not a permanent pin.
    // Reading the current window inside this effect would otherwise make every
    // scroll-driven window shift snap back to an expanded or deep-linked row.
    untrack(() => revealIndex(target));
  });

  return {
    isWindowed,
    startIndex,
    endIndex,
    getVisibleSlice,
    mountedCount,
    onScroll,
    revealIndex,
  };
};
