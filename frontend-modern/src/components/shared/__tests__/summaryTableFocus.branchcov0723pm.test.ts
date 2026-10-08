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
//   - focusedGroupRow guards: missing container ref, empty id, querySelector
//     miss, attribute-selector escaping.
//   - page-default and hover-only active series resolution.
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
  // focusedGroupRow guard arms: missing container ref, empty id, querySelector
  // miss, attribute-selector escaping.
  // -------------------------------------------------------------------------
  describe('focusedGroupRow guard arms', () => {
    it('does not attempt a reveal when setTableRootRef receives undefined (setter `element ?? null` right arm)', () => {
      const rafSpy = vi.fn((cb: FrameRequestCallback) => {
        cb(0);
        return 1;
      });
      vi.stubGlobal('requestAnimationFrame', rafSpy);

      const [focusedGroupId] = createSignal<string | null>('group-a');
      const { result } = renderHook(() => useSummaryTableFocusBridge({ focusedGroupId }));
      result.setTableRootRef(undefined);

      // tableRoot stays null -> the reveal effect returns before scheduling.
      expect(rafSpy).not.toHaveBeenCalled();
    });

    it('does not attempt a reveal for a whitespace-only focused group id (normalizeSeriesId guard)', () => {
      const rafSpy = vi.fn((cb: FrameRequestCallback) => {
        cb(0);
        return 1;
      });
      vi.stubGlobal('requestAnimationFrame', rafSpy);

      const root = document.createElement('div');
      document.body.appendChild(root);

      const [focusedGroupId] = createSignal<string | null>('   ');
      const { result } = renderHook(() => useSummaryTableFocusBridge({ focusedGroupId }));
      result.setTableRootRef(root);

      expect(rafSpy).not.toHaveBeenCalled();
    });

    it('does not scroll a decoy row when no group row carries the matching id (querySelector miss)', () => {
      const scrollIntoView = vi.fn();
      const root = document.createElement('div');
      const decoy = document.createElement('div');
      decoy.setAttribute('data-summary-group-id', 'other');
      Object.defineProperty(decoy, 'getBoundingClientRect', {
        configurable: true,
        value: () => buildRect(1200, 40),
      });
      Object.defineProperty(decoy, 'scrollIntoView', {
        configurable: true,
        value: scrollIntoView,
      });
      root.appendChild(decoy);
      document.body.appendChild(root);

      const [focusedGroupId] = createSignal<string | null>('group-a');
      const { result } = renderHook(() => useSummaryTableFocusBridge({ focusedGroupId }));
      result.setTableRootRef(root);

      expect(scrollIntoView).not.toHaveBeenCalled();
    });

    it('escapes backslashes and double quotes in the group id before injecting it into the attribute selector', () => {
      const scrollIntoView = vi.fn();
      const root = document.createElement('div');
      const row = document.createElement('div');
      // Literal id containing both a double-quote and a backslash — the exact
      // characters escapeAttributeSelectorValue rewrites.
      row.setAttribute('data-summary-group-id', 'id"with\\special');
      Object.defineProperty(row, 'getBoundingClientRect', {
        configurable: true,
        value: () => buildRect(1200, 40),
      });
      Object.defineProperty(row, 'scrollIntoView', {
        configurable: true,
        value: scrollIntoView,
      });
      root.appendChild(row);
      document.body.appendChild(root);

      const [focusedGroupId] = createSignal<string | null>('id"with\\special');
      const { result } = renderHook(() => useSummaryTableFocusBridge({ focusedGroupId }));
      result.setTableRootRef(root);

      // If escaping were broken the selector would not match the literal id.
      expect(scrollIntoView).toHaveBeenCalledWith({ behavior: 'smooth', block: 'nearest' });
    });
  });

  // -------------------------------------------------------------------------
  // Active series resolution — page default and transient hover
  // -------------------------------------------------------------------------
  describe('active series resolution arms', () => {
    it('resolves no active series or group when no hover/focus/group signal is set (page-default state)', () => {
      const root = document.createElement('div');
      document.body.appendChild(root);

      const { result } = renderHook(() => useSummaryPageInteractionState({}));
      result.setTableRootRef(root);

      expect(result.activeSeriesId()).toBeNull();
      expect(result.activeGroupScope()).toBeNull();
    });

    it('highlights a hovered off-screen row without moving the page (hover is not a reveal)', () => {
      const scrollIntoView = vi.fn();
      const scrollTo = vi.fn();
      Object.defineProperty(window, 'scrollTo', {
        configurable: true,
        value: scrollTo,
      });
      const root = document.createElement('div');
      const row = document.createElement('div');
      row.setAttribute('data-summary-series-id', 'workload-a');
      // top=1200 > innerHeight(800) -> off-screen.
      Object.defineProperty(row, 'getBoundingClientRect', {
        configurable: true,
        value: () => buildRect(1200, 40),
      });
      Object.defineProperty(row, 'scrollIntoView', {
        configurable: true,
        value: scrollIntoView,
      });
      root.appendChild(row);
      document.body.appendChild(root);

      const revealActiveSeries = vi.fn();
      const [hoveredSeriesId] = createSignal<string | null>('workload-a');
      const { result } = renderHook(() =>
        useSummaryPageInteractionState({ hoveredSeriesId, revealActiveSeries }),
      );
      result.setTableRootRef(root);

      expect(result.activeSeriesId()).toBe('workload-a');
      expect(revealActiveSeries).not.toHaveBeenCalled();
      expect(scrollIntoView).not.toHaveBeenCalled();
      expect(scrollTo).not.toHaveBeenCalled();
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
      const { result } = renderHook(() => useSummaryTableFocusBridge({ focusedGroupId }));
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
      const { result } = renderHook(() => useSummaryTableFocusBridge({ focusedGroupId }));
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
      const { result } = renderHook(() => useSummaryTableFocusBridge({ focusedGroupId }));
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
