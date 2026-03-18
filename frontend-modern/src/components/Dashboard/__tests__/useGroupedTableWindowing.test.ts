import { createRoot, createSignal } from 'solid-js';
import { describe, it, expect, afterEach } from 'vitest';

import {
  useGroupedTableWindowing,
  type UseGroupedTableWindowingOptions,
  type UseGroupedTableWindowingResult,
} from '../useGroupedTableWindowing';
import type { WorkloadGuest } from '@/types/workloads';

/** Build a minimal WorkloadGuest stub for slicing tests. */
function makeGuest(id: string): WorkloadGuest {
  return { id, name: `guest-${id}`, status: 'running' } as unknown as WorkloadGuest;
}

/** Helper: create the hook inside a reactive root, returning the result + dispose. */
function createHook(
  opts: Partial<UseGroupedTableWindowingOptions> & { totalRowCount: () => number },
): { result: UseGroupedTableWindowingResult; dispose: () => void } {
  let result!: UseGroupedTableWindowingResult;
  const dispose = createRoot((dispose) => {
    result = useGroupedTableWindowing({
      totalRowCount: opts.totalRowCount,
      windowSize: opts.windowSize,
      enabled: opts.enabled,
      revealIndex: opts.revealIndex,
    });
    return dispose;
  });
  return { result, dispose };
}

describe('useGroupedTableWindowing', () => {
  const disposers: (() => void)[] = [];

  afterEach(() => {
    disposers.forEach((d) => d());
    disposers.length = 0;
  });

  function setup(opts: Partial<UseGroupedTableWindowingOptions> & { totalRowCount: () => number }) {
    const { result, dispose } = createHook(opts);
    disposers.push(dispose);
    return result;
  }

  // ──────────────────────────────────────────────────────────────
  // isWindowed
  // ──────────────────────────────────────────────────────────────
  describe('isWindowed', () => {
    it('returns false when total rows are below default threshold (500)', () => {
      const hook = setup({ totalRowCount: () => 200 });
      expect(hook.isWindowed()).toBe(false);
    });

    it('returns true when total rows exceed default threshold', () => {
      const hook = setup({ totalRowCount: () => 600 });
      expect(hook.isWindowed()).toBe(true);
    });

    it('returns false when total rows equal threshold (not exceeded)', () => {
      const hook = setup({ totalRowCount: () => 500 });
      expect(hook.isWindowed()).toBe(false);
    });

    it('respects explicit enabled=true even below threshold', () => {
      const hook = setup({ totalRowCount: () => 50, enabled: () => true });
      expect(hook.isWindowed()).toBe(true);
    });

    it('respects explicit enabled=false even above threshold', () => {
      const hook = setup({ totalRowCount: () => 1000, enabled: () => false });
      expect(hook.isWindowed()).toBe(false);
    });

    it('returns false when total is 0 even if enabled=true', () => {
      const hook = setup({ totalRowCount: () => 0, enabled: () => true });
      expect(hook.isWindowed()).toBe(false);
    });
  });

  // ──────────────────────────────────────────────────────────────
  // startIndex / endIndex basics
  // ──────────────────────────────────────────────────────────────
  describe('startIndex / endIndex (not windowed)', () => {
    it('start is 0 and end equals total when not windowed', () => {
      const hook = setup({ totalRowCount: () => 100 });
      expect(hook.isWindowed()).toBe(false);
      expect(hook.startIndex()).toBe(0);
      expect(hook.endIndex()).toBe(100);
    });
  });

  describe('startIndex / endIndex (windowed)', () => {
    it('initial window starts at 0 with size capped to default (140)', () => {
      const hook = setup({ totalRowCount: () => 1000, enabled: () => true });
      expect(hook.startIndex()).toBe(0);
      expect(hook.endIndex()).toBe(140);
    });

    it('uses custom windowSize', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: 50,
      });
      expect(hook.endIndex()).toBe(50);
    });

    it('clamps window size to at least 1', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: 0,
      });
      // windowSize floored to 1
      expect(hook.endIndex()).toBe(1);
    });

    it('clamps negative windowSize to 1', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: -5,
      });
      expect(hook.endIndex()).toBe(1);
    });

    it('endIndex does not exceed totalRowCount', () => {
      const hook = setup({
        totalRowCount: () => 50,
        enabled: () => true,
        windowSize: 100,
      });
      expect(hook.endIndex()).toBe(50);
    });
  });

  // ──────────────────────────────────────────────────────────────
  // mountedCount
  // ──────────────────────────────────────────────────────────────
  describe('mountedCount', () => {
    it('equals total when not windowed', () => {
      const hook = setup({ totalRowCount: () => 300 });
      expect(hook.mountedCount()).toBe(300);
    });

    it('equals window size when windowed and total > windowSize', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: 80,
      });
      expect(hook.mountedCount()).toBe(80);
    });

    it('equals total when windowed but total < windowSize', () => {
      const hook = setup({
        totalRowCount: () => 30,
        enabled: () => true,
        windowSize: 100,
      });
      expect(hook.mountedCount()).toBe(30);
    });
  });

  // ──────────────────────────────────────────────────────────────
  // onScroll
  // ──────────────────────────────────────────────────────────────
  describe('onScroll', () => {
    it('moves window based on scroll position', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });

      // Scroll down: scrollTop=2000, containerHeight=400, rowHeight=40
      // firstVisibleRow = floor(2000/40) = 50
      // overscan = min(20, max(0, 100 - ceil(400/40))) = min(20, max(0,90)) = 20
      // start = 50 - 20 = 30
      hook.onScroll(2000, 400, 40);
      expect(hook.startIndex()).toBe(30);
      expect(hook.endIndex()).toBe(130);
    });

    it('clamps start to 0 when scrolled near top', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });

      // scrollTop=100 → firstVisible = 2, start = 2-20 = -18 → clamped to 0
      hook.onScroll(100, 400, 40);
      expect(hook.startIndex()).toBe(0);
    });

    it('clamps end to totalRowCount when scrolled near bottom', () => {
      const hook = setup({
        totalRowCount: () => 200,
        enabled: () => true,
        windowSize: 100,
      });

      // scrollTop = 6000, firstVisible = 150, start = 150-20 = 130
      // maxStart = 200-100 = 100 → clamped to 100
      hook.onScroll(6000, 400, 40);
      expect(hook.startIndex()).toBe(100);
      expect(hook.endIndex()).toBe(200);
    });

    it('does nothing when not windowed', () => {
      const hook = setup({ totalRowCount: () => 100 });
      hook.onScroll(5000, 400, 40);
      expect(hook.startIndex()).toBe(0);
      expect(hook.endIndex()).toBe(100);
    });

    it('handles zero rowHeight gracefully (falls back to 40)', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      // rowHeight=0 → safeRowHeight=40, same calc as normal
      hook.onScroll(2000, 400, 0);
      expect(hook.startIndex()).toBe(30);
    });

    it('handles negative scrollTop gracefully', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      hook.onScroll(-100, 400, 40);
      expect(hook.startIndex()).toBe(0);
    });

    it('handles zero containerHeight gracefully', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      // containerHeight=0 → safeContainerHeight=40 (fallback to rowHeight)
      // rowsInView = ceil(40/40) = 1
      // overscan = min(20, max(0, 100-1)) = 20
      hook.onScroll(800, 0, 40);
      // firstVisible = 800/40 = 20, start = 20-20 = 0
      expect(hook.startIndex()).toBe(0);
    });
  });

  // ──────────────────────────────────────────────────────────────
  // revealIndex
  // ──────────────────────────────────────────────────────────────
  describe('revealIndex', () => {
    it('centers window on revealed index', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });

      // Reveal row 500 → centeredStart = 500 - 50 = 450
      hook.revealIndex(500);
      expect(hook.startIndex()).toBe(450);
      expect(hook.endIndex()).toBe(550);
    });

    it('does nothing if index is already visible', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });

      // Initially window is [0, 100), row 50 is already visible
      hook.revealIndex(50);
      expect(hook.startIndex()).toBe(0);
    });

    it('clamps to start when revealing near beginning', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });

      // First scroll away, then reveal index 10
      hook.onScroll(10000, 400, 40);
      hook.revealIndex(10);
      // centeredStart = 10 - 50 = -40 → clamped to 0
      expect(hook.startIndex()).toBe(0);
    });

    it('clamps to end when revealing near the last row', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });

      hook.revealIndex(999);
      // centeredStart = 999 - 50 = 949, maxStart = 1000-100 = 900
      expect(hook.startIndex()).toBe(900);
      expect(hook.endIndex()).toBe(1000);
    });

    it('does nothing when not windowed', () => {
      const hook = setup({ totalRowCount: () => 100 });
      hook.revealIndex(50);
      expect(hook.startIndex()).toBe(0);
      expect(hook.endIndex()).toBe(100);
    });

    it('handles non-finite index (NaN)', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      hook.revealIndex(NaN);
      // Should not move window
      expect(hook.startIndex()).toBe(0);
    });

    it('handles Infinity', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      hook.revealIndex(Infinity);
      expect(hook.startIndex()).toBe(0);
    });

    it('handles negative index by clamping to 0', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      // Scroll away first
      hook.onScroll(10000, 400, 40);
      hook.revealIndex(-5);
      // normalizedIndex = max(0, ...) = 0 → centeredStart = 0 - 50 = -50 → clamped to 0
      expect(hook.startIndex()).toBe(0);
    });

    it('handles fractional index by flooring', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      hook.revealIndex(500.7);
      // floor(500.7) = 500, centeredStart = 500 - 50 = 450
      expect(hook.startIndex()).toBe(450);
    });
  });

  // ──────────────────────────────────────────────────────────────
  // revealIndex option (reactive)
  // ──────────────────────────────────────────────────────────────
  describe('revealIndex option (reactive signal)', () => {
    it('reveals target when revealIndex signal changes', () => {
      const [revealIdx, setRevealIdx] = createSignal<number | null>(null);
      let result!: UseGroupedTableWindowingResult;
      const dispose = createRoot((dispose) => {
        result = useGroupedTableWindowing({
          totalRowCount: () => 1000,
          enabled: () => true,
          windowSize: 100,
          revealIndex: revealIdx,
        });
        return dispose;
      });
      disposers.push(dispose);

      expect(result.startIndex()).toBe(0);
      setRevealIdx(500);
      // centeredStart = 500 - 50 = 450
      expect(result.startIndex()).toBe(450);
    });

    it('ignores negative revealIndex signal values', () => {
      const [revealIdx, setRevealIdx] = createSignal<number | null>(null);
      let result!: UseGroupedTableWindowingResult;
      const dispose = createRoot((dispose) => {
        result = useGroupedTableWindowing({
          totalRowCount: () => 1000,
          enabled: () => true,
          windowSize: 100,
          revealIndex: revealIdx,
        });
        return dispose;
      });
      disposers.push(dispose);

      setRevealIdx(-1);
      expect(result.startIndex()).toBe(0);
    });
  });

  // ──────────────────────────────────────────────────────────────
  // getVisibleSlice
  // ──────────────────────────────────────────────────────────────
  describe('getVisibleSlice', () => {
    const guests = Array.from({ length: 50 }, (_, i) => makeGuest(String(i)));

    it('returns full array when not windowed', () => {
      const hook = setup({ totalRowCount: () => 100 });
      const slice = hook.getVisibleSlice('group-a', guests, 0);
      expect(slice).toBe(guests); // same reference — no copy
    });

    it('returns empty array for group entirely after window', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      // Window [0, 100). Group at index 200..250 is fully after the window.
      const slice = hook.getVisibleSlice('group-x', guests, 200);
      expect(slice).toEqual([]);
    });

    it('returns empty array for group entirely before window', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      // Scroll to [400, 500). Group at index 0..50 is fully before the window.
      hook.onScroll(16800, 400, 40);
      // firstVisible = 420, overscan=20, start=400
      const slice = hook.getVisibleSlice('group-early', guests, 0);
      expect(slice).toEqual([]);
    });

    it('returns full group when entirely within window', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      // Window [0, 100). Group at index 10..60 (50 items) is entirely within.
      const slice = hook.getVisibleSlice('group-inside', guests, 10);
      expect(slice).toHaveLength(50);
      expect(slice[0].id).toBe('0');
      expect(slice[49].id).toBe('49');
    });

    it('returns partial slice when group starts before window', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      // Scroll: window starts at 30
      hook.onScroll(2000, 400, 40);
      // startIndex=30, endIndex=130
      // Group at index 10..60 → sliceStart = max(0, 30-10) = 20, sliceEnd = min(50, 130-10) = 50
      const slice = hook.getVisibleSlice('group-partial', guests, 10);
      expect(slice).toHaveLength(30);
      expect(slice[0].id).toBe('20'); // guests[20]
    });

    it('returns partial slice when group extends past window end', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      // Window [0, 100). Group at index 80..130 → sliceStart=0, sliceEnd = min(50, 100-80) = 20
      const slice = hook.getVisibleSlice('group-overlap-end', guests, 80);
      expect(slice).toHaveLength(20);
      expect(slice[0].id).toBe('0');
      expect(slice[19].id).toBe('19');
    });

    it('handles group starting exactly at window start', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      // Window [0, 100). Group at 0..50
      const slice = hook.getVisibleSlice('group-at-start', guests, 0);
      expect(slice).toHaveLength(50);
    });

    it('handles group ending exactly at window end', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      // Window [0, 100). Group at 50..100 (50 guests starting at index 50)
      const slice = hook.getVisibleSlice('group-at-end', guests, 50);
      expect(slice).toHaveLength(50);
    });

    it('returns empty when group starts exactly at endIndex', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      // Window [0, 100). Group starts at 100 → fully outside.
      const slice = hook.getVisibleSlice('group-boundary', guests, 100);
      expect(slice).toEqual([]);
    });

    it('returns empty for empty guest array', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      const slice = hook.getVisibleSlice('empty', [], 0);
      expect(slice).toEqual([]);
    });
  });

  // ──────────────────────────────────────────────────────────────
  // Edge cases
  // ──────────────────────────────────────────────────────────────
  describe('edge cases', () => {
    it('handles totalRowCount of 0 gracefully', () => {
      const hook = setup({ totalRowCount: () => 0, enabled: () => true });
      expect(hook.isWindowed()).toBe(false);
      expect(hook.startIndex()).toBe(0);
      expect(hook.endIndex()).toBe(0);
      expect(hook.mountedCount()).toBe(0);
    });

    it('handles totalRowCount of 1', () => {
      const hook = setup({ totalRowCount: () => 1, enabled: () => true });
      expect(hook.isWindowed()).toBe(true);
      expect(hook.startIndex()).toBe(0);
      expect(hook.endIndex()).toBe(1);
      expect(hook.mountedCount()).toBe(1);
    });

    it('window resets to 0 when windowing is disabled', () => {
      const [enabled, setEnabled] = createSignal(true);
      let result!: UseGroupedTableWindowingResult;
      const dispose = createRoot((dispose) => {
        result = useGroupedTableWindowing({
          totalRowCount: () => 1000,
          enabled,
          windowSize: 100,
        });
        return dispose;
      });
      disposers.push(dispose);

      // Scroll somewhere
      result.onScroll(4000, 400, 40);
      expect(result.startIndex()).toBeGreaterThan(0);

      // Disable windowing
      setEnabled(false);
      expect(result.isWindowed()).toBe(false);
      expect(result.startIndex()).toBe(0);
      expect(result.endIndex()).toBe(1000);
    });

    it('fractional windowSize is floored', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: 50.9,
      });
      expect(hook.endIndex()).toBe(50);
    });

    it('re-clamps when totalRowCount shrinks', () => {
      const [total, setTotal] = createSignal(1000);
      let result!: UseGroupedTableWindowingResult;
      const dispose = createRoot((dispose) => {
        result = useGroupedTableWindowing({
          totalRowCount: total,
          enabled: () => true,
          windowSize: 100,
        });
        return dispose;
      });
      disposers.push(dispose);

      // Scroll to end
      result.onScroll(40000, 400, 40);
      expect(result.startIndex()).toBe(900);

      // Shrink total → maxStart = 200-100 = 100, so start clamps to 100
      setTotal(200);
      expect(result.startIndex()).toBe(100);
      expect(result.endIndex()).toBe(200);
    });

    it('auto-enables windowing when totalRowCount crosses threshold upward', () => {
      const [total, setTotal] = createSignal(100);
      let result!: UseGroupedTableWindowingResult;
      const dispose = createRoot((dispose) => {
        result = useGroupedTableWindowing({ totalRowCount: total });
        return dispose;
      });
      disposers.push(dispose);

      expect(result.isWindowed()).toBe(false);
      setTotal(600);
      expect(result.isWindowed()).toBe(true);
    });

    it('auto-disables windowing when totalRowCount drops below threshold', () => {
      const [total, setTotal] = createSignal(600);
      let result!: UseGroupedTableWindowingResult;
      const dispose = createRoot((dispose) => {
        result = useGroupedTableWindowing({ totalRowCount: total });
        return dispose;
      });
      disposers.push(dispose);

      expect(result.isWindowed()).toBe(true);
      setTotal(100);
      expect(result.isWindowed()).toBe(false);
      expect(result.startIndex()).toBe(0);
      expect(result.endIndex()).toBe(100);
    });

    it('overscan becomes 0 when rowsInView >= windowSize', () => {
      const hook = setup({
        totalRowCount: () => 1000,
        enabled: () => true,
        windowSize: 5,
      });
      // containerHeight=400, rowHeight=40 → rowsInView = ceil(400/40) = 10
      // overscan = min(20, max(0, 5 - 10)) = min(20, 0) = 0
      // firstVisible = floor(2000/40) = 50, start = 50 - 0 = 50
      hook.onScroll(2000, 400, 40);
      expect(hook.startIndex()).toBe(50);
      expect(hook.endIndex()).toBe(55);
    });

    it('revealIndex for index beyond totalRowCount clamps to last row', () => {
      const hook = setup({
        totalRowCount: () => 100,
        enabled: () => true,
        windowSize: 20,
      });
      // Index 200 → normalizedIndex = min(floor(200), 99) = 99
      // centeredStart = 99 - 10 = 89, maxStart = 100-20 = 80
      hook.revealIndex(200);
      expect(hook.startIndex()).toBe(80);
      expect(hook.endIndex()).toBe(100);
    });
  });
});
