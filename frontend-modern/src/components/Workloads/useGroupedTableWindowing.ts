import { createEffect, createMemo, createSignal } from 'solid-js';
import type { WorkloadGuest } from '@/types/workloads';

export interface UseGroupedTableWindowingOptions {
  /** Total number of guest rows across all groups */
  totalRowCount: () => number;
  /** Maximum rows to mount */
  windowSize?: number;
  /** Whether windowing is enabled */
  enabled?: () => boolean;
  /** Index to ensure is visible (selection/deep-link reveal) */
  revealIndex?: () => number | null;
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
const DEFAULT_ENABLE_THRESHOLD = 500;
const DEFAULT_OVERSCAN_ROWS = 20;

export const useGroupedTableWindowing = (
  options: UseGroupedTableWindowingOptions,
): UseGroupedTableWindowingResult => {
  const [windowStart, setWindowStart] = createSignal(0);

  const normalizedWindowSize = createMemo(() =>
    Math.max(1, Math.floor(options.windowSize ?? DEFAULT_WINDOW_SIZE)),
  );

  const isWindowed = createMemo(() => {
    const total = options.totalRowCount();
    const enabled = options.enabled?.() ?? total > DEFAULT_ENABLE_THRESHOLD;
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
    const overscan = Math.min(
      DEFAULT_OVERSCAN_ROWS,
      Math.max(0, normalizedWindowSize() - rowsInView),
    );
    const firstVisibleRow = Math.floor(Math.max(0, scrollTop) / safeRowHeight);
    setClampedStart(firstVisibleRow - overscan);
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
    revealIndex(target);
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
