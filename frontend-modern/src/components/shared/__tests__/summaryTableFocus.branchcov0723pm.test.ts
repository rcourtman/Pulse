import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  useSummaryPageInteractionState,
  useSummaryTableFocusBridge,
} from '@/components/shared/summaryTableFocus';

// Branch-coverage tests for the UNCOVERED guard arms of summaryTableFocus.ts.
// The existing spec (summaryTableFocus.test.tsx) exercises the happy paths;
// this file targets the null/empty/early-return arms that never fire there:
//   - scheduleViewportRefresh (scroll/resize -> viewportVersion tick) — the
//     primary measured gap; never invoked by the existing suite.
//   - activeRow / focusedGroupRow guards: missing container ref, empty id,
//     querySelector miss, attribute-selector escaping.
//   - jumpToActiveRow: no-active-id no-op + frame-exhaustion arm.
//   - shouldShowJumpToActiveRow: empty summary collections + already-visible.
//   - Escape handler: defaultPrevented, every modifier arm, dialog target.
// Every asserted value below is hand-computed against the source in
// src/components/shared/summaryTableFocus.ts — no snapshots, no constant-
// equals-itself tautologies.

const buildRect = (top: number, height = 32): DOMRect =>
  ({
    x: 0,
    y: top,
    width: 240,
    height,
    top,
    bottom: top + height,
    left: 0,
    right: 240,
    toJSON: () => ({}),
  }) as DOMRect;

describe('summaryTableFocus.branchcov0723pm', () => {
  beforeEach(() => {
    vi.stubGlobal('innerHeight', 800);
    // Default synchronous rAF (matches the existing spec convention).
    vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => {
      cb(0);
      return 1;
    });
    vi.stubGlobal('cancelAnimationFrame', () => {});
  });

  afterEach(() => {
    document.body.innerHTML = '';
    vi.unstubAllGlobals();
  });

  // -------------------------------------------------------------------------
  // PRIMARY TARGET: scheduleViewportRefresh
  // (createEffect at summaryTableFocus.ts:226). The existing spec never
  // dispatches scroll/resize after mounting a table root, so the entire
  // effect body — listener registration, the rAF callback that ticks
  // viewportVersion, the dedup guard, and the cancel-on-cleanup arm — is
  // uncovered.
  // -------------------------------------------------------------------------
  describe('scheduleViewportRefresh', () => {
    it('ticks viewportVersion via the rAF callback after a scroll event, recomputing isActiveRowVisible', () => {
      // Deferred rAF: capture the callback so we control when viewportVersion
      // increments (the synchronous default stub would tick instantly and hide
      // the dedup arm tested below).
      const scheduled: FrameRequestCallback[] = [];
      let nextId = 7;
      vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => {
        scheduled.push(cb);
        return nextId++;
      });

      const root = document.createElement('div');
      const row = document.createElement('div');
      row.setAttribute('data-summary-series-id', 'workload-a');
      let rowTop = 1200; // below innerHeight(800) -> not visible
      Object.defineProperty(row, 'getBoundingClientRect', {
        configurable: true,
        value: () => buildRect(rowTop),
      });
      root.appendChild(row);
      document.body.appendChild(root);

      const [hoveredSeriesId] = createSignal<string | null>('workload-a');
      const { result } = renderHook(() => useSummaryPageInteractionState({ hoveredSeriesId }));
      result.setTableRootRef(root);

      // Row below the viewport -> jump affordance shown.
      expect(result.shouldShowJumpToActiveRow()).toBe(true);

      // Slide the row into the viewport. The memo must NOT pick this up yet
      // because viewportVersion has not ticked (no rAF has fired).
      rowTop = 64;
      expect(result.shouldShowJumpToActiveRow()).toBe(true);

      // scroll -> scheduleViewportRefresh schedules exactly one rAF.
      window.dispatchEvent(new Event('scroll'));
      expect(scheduled).toHaveLength(1);

      // Fire the rAF -> viewportVersion++ -> isActiveRowVisible recomputes ->
      // row is now in-viewport -> affordance hidden.
      scheduled[0](0);
      expect(result.shouldShowJumpToActiveRow()).toBe(false);
    });

    it('coalesces back-to-back scroll events into a single rAF (rafId !== undefined dedup arm)', () => {
      const scheduled: FrameRequestCallback[] = [];
      let nextId = 3;
      vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => {
        scheduled.push(cb);
        return nextId++;
      });

      const root = document.createElement('div');
      document.body.appendChild(root);

      const { result } = renderHook(() =>
        useSummaryTableFocusBridge({ activeSeriesId: () => null }),
      );
      result.setTableRootRef(root);

      window.dispatchEvent(new Event('scroll'));
      window.dispatchEvent(new Event('scroll'));
      window.dispatchEvent(new Event('scroll'));

      // Only the first scroll schedules an rAF; subsequent ones hit
      // `if (rafId !== undefined) return;` while the callback is pending.
      expect(scheduled).toHaveLength(1);
    });

    it('refreshes viewportVersion on window resize as well as scroll (resize listener arm)', () => {
      const scheduled: FrameRequestCallback[] = [];
      let nextId = 11;
      vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => {
        scheduled.push(cb);
        return nextId++;
      });

      const root = document.createElement('div');
      const row = document.createElement('div');
      row.setAttribute('data-summary-series-id', 'workload-a');
      let rowTop = 900;
      Object.defineProperty(row, 'getBoundingClientRect', {
        configurable: true,
        value: () => buildRect(rowTop),
      });
      root.appendChild(row);
      document.body.appendChild(root);

      const [hoveredSeriesId] = createSignal<string | null>('workload-a');
      const { result } = renderHook(() => useSummaryPageInteractionState({ hoveredSeriesId }));
      result.setTableRootRef(root);
      expect(result.shouldShowJumpToActiveRow()).toBe(true);

      rowTop = 50;
      window.dispatchEvent(new Event('resize'));
      expect(scheduled).toHaveLength(1);
      scheduled[0](0);
      expect(result.shouldShowJumpToActiveRow()).toBe(false);
    });

    it('cancels the pending rAF id when the owner is disposed (cancelAnimationFrame cleanup arm)', () => {
      const scheduled: FrameRequestCallback[] = [];
      const cancelSpy = vi.fn();
      const pendingId = 42;
      vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => {
        scheduled.push(cb);
        return pendingId;
      });
      vi.stubGlobal('cancelAnimationFrame', cancelSpy);

      const root = document.createElement('div');
      document.body.appendChild(root);

      const { result, cleanup } = renderHook(() =>
        useSummaryTableFocusBridge({ activeSeriesId: () => null }),
      );
      result.setTableRootRef(root);

      window.dispatchEvent(new Event('scroll'));
      expect(cancelSpy).not.toHaveBeenCalled();

      cleanup();

      // The rAF id returned by the deferred stub (42) is cancelled on dispose.
      expect(cancelSpy).toHaveBeenCalledWith(pendingId);
    });

    it('does not attach scroll/resize listeners when tableRoot ref is missing (missing container ref arm)', () => {
      const rafSpy = vi.fn((cb: FrameRequestCallback) => {
        cb(0);
        return 1;
      });
      vi.stubGlobal('requestAnimationFrame', rafSpy);

      renderHook(() => useSummaryTableFocusBridge({ activeSeriesId: () => null }));

      // No setTableRootRef -> effect hits `if (!root ...) return;` -> no
      // listeners -> scroll/resize schedule no rAF.
      window.dispatchEvent(new Event('scroll'));
      window.dispatchEvent(new Event('resize'));
      expect(rafSpy).not.toHaveBeenCalled();
    });
  });

  // -------------------------------------------------------------------------
  // activeRow guard arms: missing container ref, empty id, querySelector miss,
  // attribute-selector escaping.
  // -------------------------------------------------------------------------
  describe('activeRow guard arms', () => {
    it('returns null when the tableRoot ref has never been set (missing container ref)', () => {
      const { result } = renderHook(() =>
        useSummaryTableFocusBridge({ activeSeriesId: () => 'workload-a' }),
      );
      expect(result.activeRow()).toBeNull();
    });

    it('returns null when setTableRootRef receives undefined (setter `element ?? null` right arm)', () => {
      const { result } = renderHook(() =>
        useSummaryTableFocusBridge({ activeSeriesId: () => 'workload-a' }),
      );
      result.setTableRootRef(undefined);
      expect(result.activeRow()).toBeNull();
    });

    it('returns null for null / whitespace-only active ids and the row for a real id (normalizedActiveSeriesId guard)', () => {
      const root = document.createElement('div');
      const row = document.createElement('div');
      row.setAttribute('data-summary-series-id', 'workload-a');
      root.appendChild(row);
      document.body.appendChild(root);

      const [activeSeriesId, setActiveSeriesId] = createSignal<string | null>(null);
      const { result } = renderHook(() => useSummaryTableFocusBridge({ activeSeriesId }));
      result.setTableRootRef(root);

      expect(result.activeRow()).toBeNull();
      setActiveSeriesId('   ');
      expect(result.activeRow()).toBeNull();
      setActiveSeriesId('workload-a');
      expect(result.activeRow()).toBe(row);
    });

    it('returns null when no descendant carries the matching data attribute (querySelector miss)', () => {
      const root = document.createElement('div');
      const decoy = document.createElement('div');
      decoy.setAttribute('data-summary-series-id', 'other');
      root.appendChild(decoy);
      document.body.appendChild(root);

      const { result } = renderHook(() =>
        useSummaryTableFocusBridge({ activeSeriesId: () => 'workload-a' }),
      );
      result.setTableRootRef(root);

      expect(result.activeRow()).toBeNull();
    });

    it('escapes backslashes and double quotes in the series id before injecting it into the attribute selector', () => {
      const root = document.createElement('div');
      const row = document.createElement('div');
      // Literal id containing both a double-quote and a backslash — the exact
      // characters escapeAttributeSelectorValue rewrites.
      row.setAttribute('data-summary-series-id', 'id"with\\special');
      root.appendChild(row);
      document.body.appendChild(root);

      const { result } = renderHook(() =>
        useSummaryTableFocusBridge({ activeSeriesId: () => 'id"with\\special' }),
      );
      result.setTableRootRef(root);

      // If escaping were broken the selector would not match the literal id.
      expect(result.activeRow()).toBe(row);
    });
  });

  // -------------------------------------------------------------------------
  // jumpToActiveRow guard arms
  // -------------------------------------------------------------------------
  describe('jumpToActiveRow guard arms', () => {
    it('is a no-op when there is no active series id (normalizedActiveSeriesId falsy -> early return)', () => {
      const revealActiveSeries = vi.fn();
      const rafSpy = vi.fn((cb: FrameRequestCallback) => {
        cb(0);
        return 1;
      });
      vi.stubGlobal('requestAnimationFrame', rafSpy);

      const root = document.createElement('div');
      document.body.appendChild(root);

      const { result } = renderHook(() =>
        useSummaryTableFocusBridge({
          activeSeriesId: () => null,
          revealActiveSeries,
        }),
      );
      result.setTableRootRef(root);

      result.jumpToActiveRow();

      expect(revealActiveSeries).not.toHaveBeenCalled();
      expect(rafSpy).not.toHaveBeenCalled();
    });

    it('exhausts the 6-frame retry budget without scrolling when the active row never mounts (no matching row)', () => {
      const revealActiveSeries = vi.fn();
      const rafSpy = vi.fn((cb: FrameRequestCallback) => {
        cb(0);
        return 1;
      });
      vi.stubGlobal('requestAnimationFrame', rafSpy);

      const root = document.createElement('div');
      document.body.appendChild(root);

      const { result } = renderHook(() =>
        useSummaryTableFocusBridge({
          activeSeriesId: () => 'workload-a',
          revealActiveSeries,
        }),
      );
      result.setTableRootRef(root);

      result.jumpToActiveRow();

      // revealActiveSeries fires once before the scroll retry loop.
      expect(revealActiveSeries).toHaveBeenCalledWith('workload-a');
      expect(revealActiveSeries).toHaveBeenCalledTimes(1);
      // attemptScroll(6) recurses once per synchronous rAF until remainingHits
      // 0 -> 6 rAF calls total, then the `remainingFrames <= 0` arm returns.
      expect(rafSpy).toHaveBeenCalledTimes(6);
    });
  });

  // -------------------------------------------------------------------------
  // shouldShowJumpToActiveRow — empty summary collections + already-visible row
  // -------------------------------------------------------------------------
  describe('shouldShowJumpToActiveRow arms', () => {
    it('is false when no hover/focus/group signal resolves an active series (empty summary collections / page-default state)', () => {
      const root = document.createElement('div');
      document.body.appendChild(root);

      const { result } = renderHook(() => useSummaryPageInteractionState({}));
      result.setTableRootRef(root);

      // resolveSummaryScopeState returns { kind: 'page', seriesId: null }.
      expect(result.activeSeriesId()).toBeNull();
      expect(result.activeGroupScope()).toBeNull();
      // Boolean(null) && !isActiveRowVisible() -> false && ... -> false.
      expect(result.shouldShowJumpToActiveRow()).toBe(false);
    });

    it('is false when the active row is already inside the viewport (already-focused / visible arm)', () => {
      const root = document.createElement('div');
      const row = document.createElement('div');
      row.setAttribute('data-summary-series-id', 'workload-a');
      // top=200, height=40, innerHeight=800 -> 0 < 200 and 240 < 800 -> visible.
      Object.defineProperty(row, 'getBoundingClientRect', {
        configurable: true,
        value: () => buildRect(200, 40),
      });
      root.appendChild(row);
      document.body.appendChild(root);

      const [hoveredSeriesId] = createSignal<string | null>('workload-a');
      const { result } = renderHook(() => useSummaryPageInteractionState({ hoveredSeriesId }));
      result.setTableRootRef(root);

      expect(result.activeSeriesId()).toBe('workload-a');
      // Boolean('workload-a') && !true -> false.
      expect(result.shouldShowJumpToActiveRow()).toBe(false);
    });
  });

  // -------------------------------------------------------------------------
  // Focused-group reveal effect (summaryTableFocus.ts:350) — the
  // already-visible arm skips scrollIntoView; the out-of-viewport arm scrolls.
  // -------------------------------------------------------------------------
  describe('focused-group reveal arms', () => {
    it('does not call scrollIntoView when the focused group row is already visible (already-focused arm)', () => {
      const scrollIntoView = vi.fn();
      const root = document.createElement('div');
      const row = document.createElement('div');
      row.setAttribute('data-summary-group-id', 'group-a');
      Object.defineProperty(row, 'getBoundingClientRect', {
        configurable: true,
        value: () => buildRect(120, 40), // inside viewport
      });
      Object.defineProperty(row, 'scrollIntoView', {
        configurable: true,
        value: scrollIntoView,
      });
      root.appendChild(row);
      document.body.appendChild(root);

      const [focusedGroupId] = createSignal<string | null>('group-a');
      const { result } = renderHook(() =>
        useSummaryTableFocusBridge({ activeSeriesId: () => null, focusedGroupId }),
      );
      result.setTableRootRef(root);

      // `!isElementVisibleWithinViewport(row) && ...` short-circuits on the
      // left operand when the row is already on-screen.
      expect(scrollIntoView).not.toHaveBeenCalled();
    });

    it('calls scrollIntoView with nearest-block when the focused group row sits below the viewport', () => {
      const scrollIntoView = vi.fn();
      const root = document.createElement('div');
      const row = document.createElement('div');
      row.setAttribute('data-summary-group-id', 'group-a');
      Object.defineProperty(row, 'getBoundingClientRect', {
        configurable: true,
        value: () => buildRect(1200, 40), // below innerHeight(800)
      });
      Object.defineProperty(row, 'scrollIntoView', {
        configurable: true,
        value: scrollIntoView,
      });
      root.appendChild(row);
      document.body.appendChild(root);

      const [focusedGroupId] = createSignal<string | null>('group-a');
      const { result } = renderHook(() =>
        useSummaryTableFocusBridge({ activeSeriesId: () => null, focusedGroupId }),
      );
      result.setTableRootRef(root);

      expect(scrollIntoView).toHaveBeenCalledWith({
        behavior: 'smooth',
        block: 'nearest',
      });
    });

    it('retries via rAF then stops when the focused group row never mounts (no matching group row -> remainingFrames exhausted)', () => {
      const rafSpy = vi.fn((cb: FrameRequestCallback) => {
        cb(0);
        return 1;
      });
      vi.stubGlobal('requestAnimationFrame', rafSpy);

      const root = document.createElement('div');
      document.body.appendChild(root);

      const [focusedGroupId] = createSignal<string | null>('group-a');
      const { result } = renderHook(() =>
        useSummaryTableFocusBridge({ activeSeriesId: () => null, focusedGroupId }),
      );
      result.setTableRootRef(root);

      // Initial rAF + one per decrement: remainingFrames 12->0 = 13 calls.
      expect(rafSpy).toHaveBeenCalledTimes(13);
    });
  });

  // -------------------------------------------------------------------------
  // Escape handler guard arms (summaryTableFocus.ts:174). The existing spec
  // only covers the happy path; every short-circuit in the `||` chain and the
  // dialog-target guard are uncovered.
  // -------------------------------------------------------------------------
  describe('escape handler guard arms', () => {
    it('does not invoke onEscapeClear when the event default is already prevented', () => {
      const onEscapeClear = vi.fn();
      renderHook(() => useSummaryPageInteractionState({ onEscapeClear }));

      const event = new KeyboardEvent('keydown', {
        key: 'Escape',
        bubbles: true,
        cancelable: true,
      });
      event.preventDefault();
      document.dispatchEvent(event);

      expect(onEscapeClear).not.toHaveBeenCalled();
    });

    it.each([
      ['altKey', { altKey: true }],
      ['ctrlKey', { ctrlKey: true }],
      ['metaKey', { metaKey: true }],
      ['shiftKey', { shiftKey: true }],
    ] as const)(
      'ignores Escape pressed while %s is held (modifier short-circuit arm)',
      (_label, modifiers) => {
        const onEscapeClear = vi.fn();
        renderHook(() => useSummaryPageInteractionState({ onEscapeClear }));
        document.dispatchEvent(
          new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, ...modifiers }),
        );
        expect(onEscapeClear).not.toHaveBeenCalled();
      },
    );

    it('ignores Escape dispatched from inside an open dialog (dialog-target guard)', () => {
      const onEscapeClear = vi.fn();
      renderHook(() => useSummaryPageInteractionState({ onEscapeClear }));

      const dialog = document.createElement('div');
      dialog.setAttribute('role', 'dialog');
      document.body.appendChild(dialog);
      dialog.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));

      expect(onEscapeClear).not.toHaveBeenCalled();
    });

    it('ignores non-Escape keys (event.key !== "Escape" arm)', () => {
      const onEscapeClear = vi.fn();
      renderHook(() => useSummaryPageInteractionState({ onEscapeClear }));
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
      expect(onEscapeClear).not.toHaveBeenCalled();
    });
  });
});
