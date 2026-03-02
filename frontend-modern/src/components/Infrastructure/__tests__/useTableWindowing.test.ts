import { createRoot, createSignal } from 'solid-js';
import { describe, it, expect, afterEach } from 'vitest';

import {
  useTableWindowing,
  type UseTableWindowingOptions,
  type UseTableWindowingResult,
} from '../useTableWindowing';

/** Helper: create the hook inside a reactive root, returning the result + dispose. */
function createHook(
  opts: Partial<UseTableWindowingOptions> & { totalCount: () => number },
): { result: UseTableWindowingResult; dispose: () => void } {
  let result!: UseTableWindowingResult;
  const dispose = createRoot((dispose) => {
    result = useTableWindowing({
      totalCount: opts.totalCount,
      windowSize: opts.windowSize,
      enabled: opts.enabled,
      revealIndex: opts.revealIndex,
    });
    return dispose;
  });
  return { result, dispose };
}

describe('useTableWindowing', () => {
  const disposers: (() => void)[] = [];

  afterEach(() => {
    disposers.forEach((d) => d());
    disposers.length = 0;
  });

  function setup(opts: Partial<UseTableWindowingOptions> & { totalCount: () => number }) {
    const { result, dispose } = createHook(opts);
    disposers.push(dispose);
    return result;
  }

  // ──────────────────────────────────────────────────────────────
  // isWindowed
  // ──────────────────────────────────────────────────────────────
  describe('isWindowed', () => {
    it('returns false when total count is below default threshold (500)', () => {
      const hook = setup({ totalCount: () => 200 });
      expect(hook.isWindowed()).toBe(false);
    });

    it('returns true when total count exceeds default threshold', () => {
      const hook = setup({ totalCount: () => 600 });
      expect(hook.isWindowed()).toBe(true);
    });

    it('returns false when total count equals threshold (not exceeded)', () => {
      const hook = setup({ totalCount: () => 500 });
      expect(hook.isWindowed()).toBe(false);
    });

    it('respects explicit enabled=true even below threshold', () => {
      const hook = setup({ totalCount: () => 50, enabled: () => true });
      expect(hook.isWindowed()).toBe(true);
    });

    it('respects explicit enabled=false even above threshold', () => {
      const hook = setup({ totalCount: () => 1000, enabled: () => false });
      expect(hook.isWindowed()).toBe(false);
    });

    it('returns false when total is 0 even if enabled=true', () => {
      const hook = setup({ totalCount: () => 0, enabled: () => true });
      expect(hook.isWindowed()).toBe(false);
    });
  });

  // ──────────────────────────────────────────────────────────────
  // startIndex / endIndex basics
  // ──────────────────────────────────────────────────────────────
  describe('startIndex / endIndex (not windowed)', () => {
    it('start is 0 and end equals total when not windowed', () => {
      const hook = setup({ totalCount: () => 100 });
      expect(hook.isWindowed()).toBe(false);
      expect(hook.startIndex()).toBe(0);
      expect(hook.endIndex()).toBe(100);
    });
  });

  describe('startIndex / endIndex (windowed)', () => {
    it('initial window starts at 0 with size capped to default (140)', () => {
      const hook = setup({ totalCount: () => 1000, enabled: () => true });
      expect(hook.startIndex()).toBe(0);
      expect(hook.endIndex()).toBe(140);
    });

    it('uses custom windowSize', () => {
      const hook = setup({
        totalCount: () => 1000,
        enabled: () => true,
        windowSize: 50,
      });
      expect(hook.endIndex()).toBe(50);
    });

    it('clamps window size to at least 1', () => {
      const hook = setup({
        totalCount: () => 1000,
        enabled: () => true,
        windowSize: 0,
      });
      expect(hook.endIndex()).toBe(1);
    });

    it('clamps negative windowSize to 1', () => {
      const hook = setup({
        totalCount: () => 1000,
        enabled: () => true,
        windowSize: -5,
      });
      expect(hook.endIndex()).toBe(1);
    });

    it('endIndex does not exceed totalCount', () => {
      const hook = setup({
        totalCount: () => 50,
        enabled: () => true,
        windowSize: 100,
      });
      expect(hook.endIndex()).toBe(50);
    });
  });

  // ──────────────────────────────────────────────────────────────
  // isVisible
  // ──────────────────────────────────────────────────────────────
  describe('isVisible', () => {
    it('returns true for all valid indices when not windowed', () => {
      const hook = setup({ totalCount: () => 100 });
      expect(hook.isVisible(0)).toBe(true);
      expect(hook.isVisible(50)).toBe(true);
      expect(hook.isVisible(99)).toBe(true);
    });

    it('returns false for negative indices when not windowed', () => {
      const hook = setup({ totalCount: () => 100 });
      expect(hook.isVisible(-1)).toBe(false);
    });

    it('returns false for index >= totalCount when not windowed', () => {
      const hook = setup({ totalCount: () => 100 });
      expect(hook.isVisible(100)).toBe(false);
      expect(hook.isVisible(200)).toBe(false);
    });

    it('returns true for indices within window when windowed', () => {
      const hook = setup({
        totalCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      // Window is [0, 100)
      expect(hook.isVisible(0)).toBe(true);
      expect(hook.isVisible(50)).toBe(true);
      expect(hook.isVisible(99)).toBe(true);
    });

    it('returns false for indices outside window when windowed', () => {
      const hook = setup({
        totalCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      // Window is [0, 100)
      expect(hook.isVisible(100)).toBe(false);
      expect(hook.isVisible(500)).toBe(false);
    });

    it('tracks window position after scroll', () => {
      const hook = setup({
        totalCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      // Scroll: firstVisible=50, overscan=20, start=30, end=130
      hook.onScroll(2000, 400, 40);
      expect(hook.isVisible(29)).toBe(false);
      expect(hook.isVisible(30)).toBe(true);
      expect(hook.isVisible(129)).toBe(true);
      expect(hook.isVisible(130)).toBe(false);
    });
  });

  // ──────────────────────────────────────────────────────────────
  // onScroll
  // ──────────────────────────────────────────────────────────────
  describe('onScroll', () => {
    it('moves window based on scroll position', () => {
      const hook = setup({
        totalCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });

      // scrollTop=2000, containerHeight=400, rowHeight=40
      // firstVisibleRow = floor(2000/40) = 50
      // overscan = min(20, max(0, 100 - ceil(400/40))) = min(20, 90) = 20
      // start = 50 - 20 = 30
      hook.onScroll(2000, 400, 40);
      expect(hook.startIndex()).toBe(30);
      expect(hook.endIndex()).toBe(130);
    });

    it('clamps start to 0 when scrolled near top', () => {
      const hook = setup({
        totalCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });

      // scrollTop=100 → firstVisible = 2, start = 2-20 = -18 → clamped to 0
      hook.onScroll(100, 400, 40);
      expect(hook.startIndex()).toBe(0);
    });

    it('clamps end to totalCount when scrolled near bottom', () => {
      const hook = setup({
        totalCount: () => 200,
        enabled: () => true,
        windowSize: 100,
      });

      // scrollTop=6000, firstVisible = 150, start = 150-20 = 130
      // maxStart = 200-100 = 100 → clamped to 100
      hook.onScroll(6000, 400, 40);
      expect(hook.startIndex()).toBe(100);
      expect(hook.endIndex()).toBe(200);
    });

    it('does nothing when not windowed', () => {
      const hook = setup({ totalCount: () => 100 });
      hook.onScroll(5000, 400, 40);
      expect(hook.startIndex()).toBe(0);
      expect(hook.endIndex()).toBe(100);
    });

    it('handles zero rowHeight gracefully (falls back to 40)', () => {
      const hook = setup({
        totalCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      // rowHeight=0 → safeRowHeight=40, same calc as normal
      hook.onScroll(2000, 400, 0);
      expect(hook.startIndex()).toBe(30);
    });

    it('handles negative scrollTop gracefully', () => {
      const hook = setup({
        totalCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      hook.onScroll(-100, 400, 40);
      expect(hook.startIndex()).toBe(0);
    });

    it('handles zero containerHeight gracefully', () => {
      const hook = setup({
        totalCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      // containerHeight=0 → safeContainerHeight = safeRowHeight = 40
      // rowsInView = ceil(40/40) = 1
      // overscan = min(20, max(0, 100-1)) = 20
      hook.onScroll(800, 0, 40);
      // firstVisible = 800/40 = 20, start = 20-20 = 0
      expect(hook.startIndex()).toBe(0);
    });

    it('handles negative rowHeight gracefully (falls back to 40)', () => {
      const hook = setup({
        totalCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      hook.onScroll(2000, 400, -10);
      // safeRowHeight = 40 (negative not > 0)
      expect(hook.startIndex()).toBe(30);
    });

    it('handles large scrollTop values without error', () => {
      const hook = setup({
        totalCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      // scrollTop far beyond total rows — should clamp to maxStart
      hook.onScroll(1_000_000, 400, 40);
      // maxStart = 1000-100 = 900
      expect(hook.startIndex()).toBe(900);
      expect(hook.endIndex()).toBe(1000);
    });
  });

  // ──────────────────────────────────────────────────────────────
  // revealIndex
  // ──────────────────────────────────────────────────────────────
  describe('revealIndex', () => {
    it('centers window on revealed index', () => {
      const hook = setup({
        totalCount: () => 1000,
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
        totalCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });

      // Initially window is [0, 100), row 50 is already visible
      hook.revealIndex(50);
      expect(hook.startIndex()).toBe(0);
    });

    it('clamps to start when revealing near beginning', () => {
      const hook = setup({
        totalCount: () => 1000,
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
        totalCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });

      hook.revealIndex(999);
      // centeredStart = 999 - 50 = 949, maxStart = 1000-100 = 900
      expect(hook.startIndex()).toBe(900);
      expect(hook.endIndex()).toBe(1000);
    });

    it('does nothing when not windowed', () => {
      const hook = setup({ totalCount: () => 100 });
      hook.revealIndex(50);
      expect(hook.startIndex()).toBe(0);
      expect(hook.endIndex()).toBe(100);
    });

    it('handles non-finite index (NaN)', () => {
      const hook = setup({
        totalCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      hook.revealIndex(NaN);
      expect(hook.startIndex()).toBe(0);
    });

    it('handles Infinity', () => {
      const hook = setup({
        totalCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      hook.revealIndex(Infinity);
      expect(hook.startIndex()).toBe(0);
    });

    it('handles negative index by clamping to 0', () => {
      const hook = setup({
        totalCount: () => 1000,
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
        totalCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      hook.revealIndex(500.7);
      // floor(500.7) = 500, centeredStart = 500 - 50 = 450
      expect(hook.startIndex()).toBe(450);
    });

    it('handles index beyond totalCount by clamping to last row', () => {
      const hook = setup({
        totalCount: () => 100,
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

  // ──────────────────────────────────────────────────────────────
  // revealIndex option (reactive signal)
  // ──────────────────────────────────────────────────────────────
  describe('revealIndex option (reactive signal)', () => {
    it('reveals target when revealIndex signal changes', () => {
      const [revealIdx, setRevealIdx] = createSignal<number | null>(null);
      let result!: UseTableWindowingResult;
      const dispose = createRoot((dispose) => {
        result = useTableWindowing({
          totalCount: () => 1000,
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
      let result!: UseTableWindowingResult;
      const dispose = createRoot((dispose) => {
        result = useTableWindowing({
          totalCount: () => 1000,
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

    it('ignores null revealIndex signal', () => {
      const [revealIdx, setRevealIdx] = createSignal<number | null>(null);
      let result!: UseTableWindowingResult;
      const dispose = createRoot((dispose) => {
        result = useTableWindowing({
          totalCount: () => 1000,
          enabled: () => true,
          windowSize: 100,
          revealIndex: revealIdx,
        });
        return dispose;
      });
      disposers.push(dispose);

      // Set to a value then back to null
      setRevealIdx(500);
      expect(result.startIndex()).toBe(450);
      setRevealIdx(null);
      // Window should stay where it was
      expect(result.startIndex()).toBe(450);
    });
  });

  // ──────────────────────────────────────────────────────────────
  // Edge cases
  // ──────────────────────────────────────────────────────────────
  describe('edge cases', () => {
    it('handles totalCount of 0 gracefully', () => {
      const hook = setup({ totalCount: () => 0, enabled: () => true });
      expect(hook.isWindowed()).toBe(false);
      expect(hook.startIndex()).toBe(0);
      expect(hook.endIndex()).toBe(0);
    });

    it('handles totalCount of 1', () => {
      const hook = setup({ totalCount: () => 1, enabled: () => true });
      expect(hook.isWindowed()).toBe(true);
      expect(hook.startIndex()).toBe(0);
      expect(hook.endIndex()).toBe(1);
    });

    it('window resets to 0 when windowing is disabled and re-enabled', () => {
      const [enabled, setEnabled] = createSignal(true);
      let result!: UseTableWindowingResult;
      const dispose = createRoot((dispose) => {
        result = useTableWindowing({
          totalCount: () => 1000,
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

      // Re-enable: internal windowStart should have been reset to 0
      setEnabled(true);
      expect(result.isWindowed()).toBe(true);
      expect(result.startIndex()).toBe(0);
      expect(result.endIndex()).toBe(100);
    });

    it('fractional windowSize is floored', () => {
      const hook = setup({
        totalCount: () => 1000,
        enabled: () => true,
        windowSize: 50.9,
      });
      expect(hook.endIndex()).toBe(50);
    });

    it('re-clamps when totalCount shrinks', () => {
      const [total, setTotal] = createSignal(1000);
      let result!: UseTableWindowingResult;
      const dispose = createRoot((dispose) => {
        result = useTableWindowing({
          totalCount: total,
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

    it('auto-enables windowing when totalCount crosses threshold upward', () => {
      const [total, setTotal] = createSignal(100);
      let result!: UseTableWindowingResult;
      const dispose = createRoot((dispose) => {
        result = useTableWindowing({ totalCount: total });
        return dispose;
      });
      disposers.push(dispose);

      expect(result.isWindowed()).toBe(false);
      setTotal(600);
      expect(result.isWindowed()).toBe(true);
    });

    it('auto-disables windowing when totalCount drops below threshold and resets start', () => {
      const [total, setTotal] = createSignal(600);
      let result!: UseTableWindowingResult;
      const dispose = createRoot((dispose) => {
        result = useTableWindowing({ totalCount: total });
        return dispose;
      });
      disposers.push(dispose);

      expect(result.isWindowed()).toBe(true);
      // Scroll to a non-zero position first
      result.onScroll(4000, 400, 40);
      expect(result.startIndex()).toBeGreaterThan(0);

      // Drop below threshold — windowing off
      setTotal(100);
      expect(result.isWindowed()).toBe(false);
      expect(result.startIndex()).toBe(0);
      expect(result.endIndex()).toBe(100);

      // Bring back above threshold — should restart at 0 (internal state was reset)
      setTotal(600);
      expect(result.isWindowed()).toBe(true);
      expect(result.startIndex()).toBe(0);
    });

    it('overscan becomes 0 when rowsInView >= windowSize', () => {
      const hook = setup({
        totalCount: () => 1000,
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

    it('window does not skip when setClampedStart gets same value', () => {
      const hook = setup({
        totalCount: () => 1000,
        enabled: () => true,
        windowSize: 100,
      });
      // Two identical scrolls should produce same result
      hook.onScroll(2000, 400, 40);
      const start1 = hook.startIndex();
      hook.onScroll(2000, 400, 40);
      expect(hook.startIndex()).toBe(start1);
    });

    it('totalCount exactly equals windowSize', () => {
      const hook = setup({
        totalCount: () => 100,
        enabled: () => true,
        windowSize: 100,
      });
      // maxStart = 100 - 100 = 0
      expect(hook.startIndex()).toBe(0);
      expect(hook.endIndex()).toBe(100);
      // Scrolling shouldn't move window since maxStart=0
      hook.onScroll(2000, 400, 40);
      expect(hook.startIndex()).toBe(0);
      expect(hook.endIndex()).toBe(100);
    });

    it('totalCount smaller than windowSize when windowed', () => {
      const hook = setup({
        totalCount: () => 30,
        enabled: () => true,
        windowSize: 100,
      });
      expect(hook.isWindowed()).toBe(true);
      expect(hook.startIndex()).toBe(0);
      expect(hook.endIndex()).toBe(30);
    });
  });
});
