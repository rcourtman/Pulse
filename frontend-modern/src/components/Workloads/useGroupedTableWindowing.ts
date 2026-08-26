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
// A mutation frame costs a near-constant style/layout pass on the mounted
// table whatever the shift size, so runway top-ups run in dead-band-sized
// batches: most scroll frames stay compositor-only, and sub-row scroll jitter
// cannot thrash one-row shifts in alternating directions.
const TOP_UP_DEADBAND_ROWS = 8;

export const useGroupedTableWindowing = (
  options: UseGroupedTableWindowingOptions,
): UseGroupedTableWindowingResult => {
  const [windowStart, setWindowStart] = createSignal(0);

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
    const targetRunway = Math.floor(availableBuffer / 2);

    const leadingRunway = firstVisibleRow - startIndex();
    const trailingRunway = endIndex() - visibleEnd;
    if (leadingRunway < 0 || trailingRunway < 0) {
      // Teleport (scrollbar drag, deep jump): the viewport left the mounted
      // window entirely, so re-center the window on it.
      setClampedStart(firstVisibleRow - targetRunway);
      return;
    }

    // Top up only the depleted side, and only once its deficit clears the
    // dead-band, restoring that side to the full target. Leading and trailing
    // runway always sum to the spare buffer, so at most one side is below
    // target. Steady scrolling therefore pays one bounded mutation frame per
    // dead-band of travel instead of a slice update on every scroll event.
    const topUpThreshold = Math.min(TOP_UP_DEADBAND_ROWS, Math.max(1, Math.ceil(targetRunway / 2)));
    if (targetRunway - trailingRunway >= topUpThreshold) {
      setClampedStart(startIndex() + (targetRunway - trailingRunway));
    } else if (targetRunway - leadingRunway >= topUpThreshold) {
      setClampedStart(startIndex() - (targetRunway - leadingRunway));
    }
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
