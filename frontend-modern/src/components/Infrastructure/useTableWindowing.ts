import { createEffect, createMemo, createSignal } from 'solid-js';

export interface UseTableWindowingOptions {
  /** Total number of renderable items (rows + group headers) */
  totalCount: () => number;
  /** Maximum rows to mount at once */
  windowSize?: number;
  /** Whether windowing is enabled (disabled for small datasets) */
  enabled?: () => boolean;
  /** Row count above which the default bounded window activates */
  enableThreshold?: number;
  /** Index to ensure is visible (for deep-link/highlight reveal) */
  revealIndex?: () => number | null;
}

export interface UseTableWindowingResult {
  /** Start index of the visible window */
  startIndex: () => number;
  /** End index (exclusive) of the visible window */
  endIndex: () => number;
  /** Whether a given index is within the visible window */
  isVisible: (index: number) => boolean;
  /** Scroll handler to update the window position */
  onScroll: (scrollTop: number, containerHeight: number, rowHeight: number) => void;
  /** Whether windowing is active */
  isWindowed: () => boolean;
  /** Jump to make a specific index visible */
  revealIndex: (index: number) => void;
}

const DEFAULT_WINDOW_SIZE = 140;
// A few hundred rich platform rows are already enough to produce long style,
// layout, and paint tasks on phones. Keep the default aligned with the mounted
// row budget so consumers cannot accidentally leave a 200-500 row gap between
// "small table" and "large estate" behaviour.
const DEFAULT_ENABLE_THRESHOLD = DEFAULT_WINDOW_SIZE;
// A mutation frame costs a near-constant style/layout pass on the mounted
// table whatever the shift size, so runway top-ups run in dead-band-sized
// batches: most scroll frames stay compositor-only, and sub-row scroll jitter
// cannot thrash one-row shifts in alternating directions.
const TOP_UP_DEADBAND_ROWS = 8;

export const useTableWindowing = (options: UseTableWindowingOptions): UseTableWindowingResult => {
  const [windowStart, setWindowStart] = createSignal(0);

  const normalizedWindowSize = createMemo(() =>
    Math.max(1, Math.floor(options.windowSize ?? DEFAULT_WINDOW_SIZE)),
  );

  const isWindowed = createMemo(() => {
    const total = options.totalCount();
    const threshold = Math.max(0, Math.floor(options.enableThreshold ?? DEFAULT_ENABLE_THRESHOLD));
    const enabled = options.enabled?.() ?? total > threshold;
    return enabled && total > 0;
  });

  const maxStart = createMemo(() => Math.max(0, options.totalCount() - normalizedWindowSize()));

  const startIndex = createMemo(() => {
    if (!isWindowed()) return 0;
    return Math.max(0, Math.min(windowStart(), maxStart()));
  });

  const endIndex = createMemo(() => {
    if (!isWindowed()) return options.totalCount();
    return Math.min(options.totalCount(), startIndex() + normalizedWindowSize());
  });

  const setClampedStart = (nextStart: number) => {
    const clamped = Math.max(0, Math.min(Math.floor(nextStart), maxStart()));
    setWindowStart((current) => (current === clamped ? current : clamped));
  };

  const revealIndex = (index: number) => {
    if (!isWindowed()) return;
    if (!Number.isFinite(index)) return;
    const normalizedIndex = Math.max(0, Math.min(Math.floor(index), options.totalCount() - 1));
    if (normalizedIndex >= startIndex() && normalizedIndex < endIndex()) return;
    const centeredStart = normalizedIndex - Math.floor(normalizedWindowSize() / 2);
    setClampedStart(centeredStart);
  };

  const onScroll = (scrollTop: number, containerHeight: number, rowHeight: number) => {
    if (!isWindowed()) return;
    const safeRowHeight = rowHeight > 0 ? rowHeight : 40;
    const safeContainerHeight = containerHeight > 0 ? containerHeight : safeRowHeight;
    const rowsInView = Math.max(1, Math.ceil(safeContainerHeight / safeRowHeight));
    const firstVisibleRow = Math.floor(Math.max(0, scrollTop) / safeRowHeight);
    const visibleEnd = Math.min(options.totalCount(), firstVisibleRow + rowsInView);
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

  const isVisible = (index: number) => {
    if (!isWindowed()) return index >= 0 && index < options.totalCount();
    return index >= startIndex() && index < endIndex();
  };

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
    startIndex,
    endIndex,
    isVisible,
    onScroll,
    isWindowed,
    revealIndex,
  };
};
