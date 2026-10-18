import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useSummaryPageInteractionState } from '@/components/shared/summaryTableFocus';

// Branch-coverage tests for the UNCOVERED guard arms of summaryTableFocus.ts.
// The existing spec (summaryTableFocus.test.tsx) exercises the happy paths;
// this file targets the null/empty/early-return arms that never fire there:
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
  // Active series resolution — page default and transient hover
  // -------------------------------------------------------------------------
  describe('active series resolution arms', () => {
    it('resolves no active series when no hover or focus signal is set (page-default state)', () => {
      const root = document.createElement('div');
      document.body.appendChild(root);

      const { result } = renderHook(() => useSummaryPageInteractionState({}));
      result.setTableRootRef(root);

      expect(result.activeSeriesId()).toBeNull();
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
