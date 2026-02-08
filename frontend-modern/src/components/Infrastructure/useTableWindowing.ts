import { createEffect, createMemo, createSignal } from 'solid-js';

export interface UseTableWindowingOptions {
  /** Total number of renderable items (rows + group headers) */
  totalCount: () => number;
  /** Maximum rows to mount at once */
  windowSize?: number;
  /** Whether windowing is enabled (disabled for small datasets) */
  enabled?: () => boolean;
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
const DEFAULT_ENABLE_THRESHOLD = 500;
const DEFAULT_OVERSCAN_ROWS = 20;

export const useTableWindowing = (options: UseTableWindowingOptions): UseTableWindowingResult => {
  const [windowStart, setWindowStart] = createSignal(0);

  const normalizedWindowSize = createMemo(() =>
    Math.max(1, Math.floor(options.windowSize ?? DEFAULT_WINDOW_SIZE)),
  );

  const isWindowed = createMemo(() => {
    const total = options.totalCount();
    const enabled = options.enabled?.() ?? total > DEFAULT_ENABLE_THRESHOLD;
    return enabled && total > 0;
  });

  const maxStart = createMemo(() =>
    Math.max(0, options.totalCount() - normalizedWindowSize()),
  );

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
    const overscan = Math.min(DEFAULT_OVERSCAN_ROWS, Math.max(0, normalizedWindowSize() - rowsInView));
    const firstVisibleRow = Math.floor(Math.max(0, scrollTop) / safeRowHeight);
    setClampedStart(firstVisibleRow - overscan);
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
