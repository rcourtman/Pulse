import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  findInlineDetailElement,
  revealInlineDetailInViewport,
} from '@/components/shared/contextualFocus';

// Branch-coverage tests for the DOM scroller helpers in contextualFocus.ts.
// The four private helpers (isWindowScroller, resolveVerticalScroller,
// getScrollerMetrics, scrollVerticalScroller) are only reachable through the
// exported `revealInlineDetailInViewport` entry point, so every test here
// drives them through real DOM elements (jsdom) with stubbed layout values.
// Every asserted value below is hand-computed against the source in
// src/components/shared/contextualFocus.ts — no snapshots, no source-string
// reads, no constant-equals-itself tautologies.

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

// Build a div whose scroll geometry is observable to resolveScrollableAncestor
// (overflow-y computed style + scrollHeight > clientHeight).
const buildScrollableAncestor = (
  overrides: {
    scrollHeight?: number;
    clientHeight?: number;
    scrollTop?: number;
    rectTop?: number;
  } = {},
): HTMLDivElement => {
  const scroller = document.createElement('div');
  scroller.style.overflowY = 'auto';
  Object.defineProperty(scroller, 'scrollHeight', {
    configurable: true,
    value: overrides.scrollHeight ?? 1000,
  });
  Object.defineProperty(scroller, 'clientHeight', {
    configurable: true,
    value: overrides.clientHeight ?? 400,
  });
  scroller.scrollTop = overrides.scrollTop ?? 0;
  Object.defineProperty(scroller, 'getBoundingClientRect', {
    configurable: true,
    value: () => buildRect(overrides.rectTop ?? 0, overrides.clientHeight ?? 400),
  });
  return scroller;
};

describe('contextualFocus.branchcov0723pm', () => {
  let originalScrollTo: PropertyDescriptor | undefined;
  let originalDocumentScrollTop: PropertyDescriptor | undefined;
  let originalBodyScrollTop: PropertyDescriptor | undefined;

  beforeEach(() => {
    // innerHeight=800 → minTopMargin=96, preferredTop=224, detailPeek=192 for
    // the window-scroller path. Keeps expected offsets stable per scenario.
    vi.stubGlobal('innerHeight', 800);
    vi.stubGlobal('scrollY', 0);
    vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => {
      cb(0);
      return 1;
    });
    originalScrollTo = Object.getOwnPropertyDescriptor(window, 'scrollTo');
    originalDocumentScrollTop = Object.getOwnPropertyDescriptor(
      document.documentElement,
      'scrollTop',
    );
    originalBodyScrollTop = Object.getOwnPropertyDescriptor(document.body, 'scrollTop');
  });

  afterEach(() => {
    document.body.innerHTML = '';
    document.documentElement.scrollTop = 0;
    document.body.scrollTop = 0;
    if (originalScrollTo) {
      Object.defineProperty(window, 'scrollTo', originalScrollTo);
    }
    if (originalDocumentScrollTop) {
      Object.defineProperty(document.documentElement, 'scrollTop', originalDocumentScrollTop);
    }
    if (originalBodyScrollTop) {
      Object.defineProperty(document.body, 'scrollTop', originalBodyScrollTop);
    }
    vi.unstubAllGlobals();
  });

  describe('findInlineDetailElement', () => {
    // `if (!root || !normalizedId) return null;` — every falsy combination of
    // the two guards collapses to null before the querySelector runs.

    it('returns null when root is null (root guard, left operand)', () => {
      expect(findInlineDetailElement(null, 'workload-a')).toBeNull();
    });

    it('returns null when seriesId is null (normalizedId guard, ?? right arm)', () => {
      const root = document.createElement('div');
      expect(findInlineDetailElement(root, null)).toBeNull();
    });

    it('returns null when seriesId is the empty string (normalized truthiness guard)', () => {
      const root = document.createElement('div');
      expect(findInlineDetailElement(root, '')).toBeNull();
    });

    it('returns null when seriesId is whitespace-only (trim() collapses to empty)', () => {
      const root = document.createElement('div');
      expect(findInlineDetailElement(root, '   ')).toBeNull();
    });

    it('returns null when no descendant matches the data attribute (querySelector miss)', () => {
      const root = document.createElement('div');
      const decoy = document.createElement('div');
      decoy.setAttribute('data-inline-detail-for', 'other');
      root.appendChild(decoy);
      expect(findInlineDetailElement(root, 'workload-a')).toBeNull();
    });

    it('returns the matching element when one is present (querySelector hit)', () => {
      const root = document.createElement('div');
      const detail = document.createElement('div');
      detail.setAttribute('data-inline-detail-for', 'workload-a');
      root.appendChild(detail);
      expect(findInlineDetailElement(root, 'workload-a')).toBe(detail);
    });

    it('trims surrounding whitespace from seriesId before querying (trim() before selector)', () => {
      const root = document.createElement('div');
      const detail = document.createElement('div');
      detail.setAttribute('data-inline-detail-for', 'workload-a');
      root.appendChild(detail);
      expect(findInlineDetailElement(root, '\t workload-a \n')).toBe(detail);
    });

    it('matches against a Document root, not just Element roots (ParentNode overload)', () => {
      const detail = document.createElement('div');
      detail.setAttribute('data-inline-detail-for', 'workload-a');
      document.body.appendChild(detail);
      expect(findInlineDetailElement(document, 'workload-a')).toBe(detail);
    });
  });

  describe('revealInlineDetailInViewport — window-scroller resolution', () => {
    // resolveVerticalScroller falls back to `window` whenever the row has no
    // scrollable ancestor (no element with overflow-y auto/scroll AND
    // scrollHeight>clientHeight up the parent chain). isWindowScroller then
    // steers metrics + scrollVerticalScroller down the window arms.

    it('returns false without scrolling when the row already has breathing room and detail peeks (early-return false arm)', () => {
      const row = document.createElement('div');
      // rowRect.top=200 → currentRowOffset=200 >= minTopMargin(96) → breathing room true.
      row.getBoundingClientRect = vi.fn(() => buildRect(200, 40));
      // detail.top=300 → 300 <= height(800)-peek(192)=608 → detailHasPeek true.
      const detail = document.createElement('div');
      detail.getBoundingClientRect = vi.fn(() => buildRect(300, 220));
      document.body.append(row, detail);

      expect(revealInlineDetailInViewport({ row, detail, behavior: 'smooth' })).toBe(false);
    });

    it('returns false without scrolling when detail is omitted (?? null → detailHasPeek forced true)', () => {
      const row = document.createElement('div');
      row.getBoundingClientRect = vi.fn(() => buildRect(200, 40));
      document.body.appendChild(row);
      expect(revealInlineDetailInViewport({ row })).toBe(false);
    });

    it('scrolls the window via window.scrollTo when the row sits above minTopMargin (canUseScrollTo=true, window arm)', () => {
      const scrollTo = vi.fn();
      Object.defineProperty(window, 'scrollTo', { configurable: true, value: scrollTo });
      vi.stubGlobal('scrollY', 50);

      const row = document.createElement('div');
      // rowRect.top=80 → currentRowOffset=80 < minTopMargin(96) → breathing room false.
      row.getBoundingClientRect = vi.fn(() => buildRect(80, 40));
      document.body.appendChild(row);

      // desiredRowOffset = 80 > preferredTop(224)? no → max(96, 80)=96
      // nextScrollTop = 50 + 80 - 96 = 34
      expect(revealInlineDetailInViewport({ row, detail: null })).toBe(true);
      expect(scrollTo).toHaveBeenCalledWith({ top: 34, behavior: 'smooth' });
    });

    it('uses the explicit behavior override when scrolling the window (behavior ?? right arm)', () => {
      const scrollTo = vi.fn();
      Object.defineProperty(window, 'scrollTo', { configurable: true, value: scrollTo });
      vi.stubGlobal('scrollY', 50);

      const row = document.createElement('div');
      row.getBoundingClientRect = vi.fn(() => buildRect(80, 40));
      document.body.appendChild(row);

      expect(revealInlineDetailInViewport({ row, detail: null, behavior: 'auto' })).toBe(true);
      expect(scrollTo).toHaveBeenCalledWith({ top: 34, behavior: 'auto' });
    });

    it('falls back to document.documentElement.scrollTop when window.scrollTo lacks a `.mock` marker (jsdom fallback arm)', () => {
      // A plain function with no `.mock` property → canUseScrollTo=false for the
      // window scroller → the code sets documentElement/body.scrollTop instead
      // of calling window.scrollTo.
      Object.defineProperty(window, 'scrollTo', {
        configurable: true,
        value: function notImplemented() {},
      });
      vi.stubGlobal('scrollY', 50);

      const row = document.createElement('div');
      row.getBoundingClientRect = vi.fn(() => buildRect(80, 40));
      document.body.appendChild(row);

      let observedDocumentTop = -1;
      let observedBodyTop = -1;
      Object.defineProperty(document.documentElement, 'scrollTop', {
        configurable: true,
        get: () => observedDocumentTop,
        set: (v: number) => {
          observedDocumentTop = v;
        },
      });
      Object.defineProperty(document.body, 'scrollTop', {
        configurable: true,
        get: () => observedBodyTop,
        set: (v: number) => {
          observedBodyTop = v;
        },
      });

      expect(revealInlineDetailInViewport({ row, detail: null })).toBe(true);
      // nextScrollTop = 50 + 80 - 96 = 34; both documentElement and body receive it.
      expect(observedDocumentTop).toBe(34);
      expect(observedBodyTop).toBe(34);
    });

    it('reads window.scrollY through getScrollerMetrics when computing nextScrollTop (window metrics branch)', () => {
      const scrollTo = vi.fn();
      Object.defineProperty(window, 'scrollTo', { configurable: true, value: scrollTo });
      vi.stubGlobal('scrollY', 150);

      const row = document.createElement('div');
      row.getBoundingClientRect = vi.fn(() => buildRect(20, 40));
      document.body.appendChild(row);

      // currentRowOffset = 20 (rowRect.top - metrics.top(=0))
      // desiredRowOffset = max(96, 20) = 96
      // nextScrollTop = 150 + 20 - 96 = 74
      expect(revealInlineDetailInViewport({ row, detail: null })).toBe(true);
      expect(scrollTo).toHaveBeenCalledWith({ top: 74, behavior: 'smooth' });
    });
  });

  describe('revealInlineDetailInViewport — element-scroller resolution', () => {
    // resolveScrollableAncestor returns a real HTMLElement when an ancestor
    // has overflow-y auto/scroll AND scrollHeight>clientHeight. That drives
    // getScrollerMetrics down the rect-based arm and scrollVerticalScroller
    // down the HTMLElement arm.

    it('scrolls the nearest scrollable ancestor via scrollTo with the rect-derived offset (HTMLElement canUseScrollTo=true)', () => {
      // scroller: clientHeight=400 → minTopMargin=max(72,48)=72,
      // preferredTop=max(72,112)=112, detailPeek=max(160,96)=160.
      const scroller = buildScrollableAncestor({
        scrollHeight: 1000,
        clientHeight: 400,
        scrollTop: 200,
        rectTop: 0,
      });
      const scrollTo = vi.fn();
      Object.defineProperty(scroller, 'scrollTo', { configurable: true, value: scrollTo });

      const row = document.createElement('div');
      // rowRect.top=40 → currentRowOffset = 40 - metrics.top(=0) = 40
      // rowHasBreathingRoom = 40 >= 72? false → trigger scroll.
      // desiredRowOffset = 40 > 112? no → max(72, 40) = 72
      // nextScrollTop = 200 + 40 - 72 = 168
      row.getBoundingClientRect = vi.fn(() => buildRect(40, 40));
      scroller.appendChild(row);
      document.body.appendChild(scroller);

      expect(revealInlineDetailInViewport({ row, detail: null })).toBe(true);
      expect(scrollTo).toHaveBeenCalledWith({ top: 168, behavior: 'smooth' });
    });

    it('takes the preferredTop arm of desiredRowOffset when the row sits below preferredTop (ternary left arm)', () => {
      // Same scroller geometry (clientHeight=400 → preferredTop=112).
      const scroller = buildScrollableAncestor({
        scrollHeight: 1000,
        clientHeight: 400,
        scrollTop: 0,
        rectTop: 0,
      });
      const scrollTo = vi.fn();
      Object.defineProperty(scroller, 'scrollTo', { configurable: true, value: scrollTo });

      const row = document.createElement('div');
      row.getBoundingClientRect = vi.fn(() => buildRect(400, 40));
      // rowRect.top=400 → currentRowOffset=400 > preferredTop(112) → left arm.
      const detail = document.createElement('div');
      // detail.top=700 → 700 > height(400)-peek(160)=240 → detailHasPeek false → trigger.
      detail.getBoundingClientRect = vi.fn(() => buildRect(700, 220));
      scroller.append(row, detail);
      document.body.appendChild(scroller);

      // desiredRowOffset = 400 > 112 ? 112 : ... → 112
      // nextScrollTop = 0 + 400 - 112 = 288
      expect(revealInlineDetailInViewport({ row, detail })).toBe(true);
      expect(scrollTo).toHaveBeenCalledWith({ top: 288, behavior: 'smooth' });
    });

    it('takes the max(minTopMargin, currentRowOffset) arm when row sits between minTopMargin and preferredTop and detail is below the peek (ternary right arm)', () => {
      const scroller = buildScrollableAncestor({
        scrollHeight: 1000,
        clientHeight: 400,
        scrollTop: 20,
        rectTop: 0,
      });
      const scrollTo = vi.fn();
      Object.defineProperty(scroller, 'scrollTo', { configurable: true, value: scrollTo });

      const row = document.createElement('div');
      // currentRowOffset=100, between minTopMargin(72) and preferredTop(112).
      row.getBoundingClientRect = vi.fn(() => buildRect(100, 40));
      const detail = document.createElement('div');
      // detail.top=700 → detailHasPeek false → trigger scroll.
      detail.getBoundingClientRect = vi.fn(() => buildRect(700, 220));
      scroller.append(row, detail);
      document.body.appendChild(scroller);

      // rowRect.top=100 → currentRowOffset=100; rowHasBreathingRoom = 100>=72 = true.
      // detailHasPeek = 700<=240 = false → scroll path.
      // desiredRowOffset = 100 > 112 ? 112 : max(72, 100) = 100  (right arm)
      // nextScrollTop = 20 + 100 - 100 = 20
      expect(revealInlineDetailInViewport({ row, detail })).toBe(true);
      expect(scrollTo).toHaveBeenCalledWith({ top: 20, behavior: 'smooth' });
    });

    it('clamps a deeply negative nextScrollTop to 0 when the row is at the very top (Math.max(0, top))', () => {
      const scroller = buildScrollableAncestor({
        scrollHeight: 1000,
        clientHeight: 400,
        scrollTop: 0,
        rectTop: 0,
      });
      const scrollTo = vi.fn();
      Object.defineProperty(scroller, 'scrollTo', { configurable: true, value: scrollTo });

      const row = document.createElement('div');
      // rowRect.top=5 → currentRowOffset=5 → desiredRowOffset=max(72,5)=72
      // nextScrollTop = 0 + 5 - 72 = -67 → clamped 0 (NOT -67).
      row.getBoundingClientRect = vi.fn(() => buildRect(5, 40));
      scroller.appendChild(row);
      document.body.appendChild(scroller);

      expect(revealInlineDetailInViewport({ row, detail: null })).toBe(true);
      expect(scrollTo).toHaveBeenCalledWith({ top: 0, behavior: 'smooth' });
    });

    it('assigns scroller.scrollTop directly when the scroller has no callable scrollTo (HTMLElement canUseScrollTo=false arm)', () => {
      // Deliberately malformed: a scroller whose scrollTo is undefined, cast
      // back through `unknown` to satisfy TS while breaking the runtime guard
      // `typeof scroller.scrollTo !== 'function'`.
      const scroller = buildScrollableAncestor({
        scrollHeight: 1000,
        clientHeight: 400,
        scrollTop: 50,
        rectTop: 0,
      });
      Object.defineProperty(scroller, 'scrollTo', {
        configurable: true,
        value: undefined,
      });
      const malformedScroller = scroller as unknown as HTMLElement & {
        scrollTo?: unknown;
      };

      const row = document.createElement('div');
      // rowRect.top=60 → currentRowOffset=60 → rowHasBreathingRoom = 60>=72 = false.
      // desiredRowOffset = max(72, 60) = 72 → nextScrollTop = 50 + 60 - 72 = 38.
      row.getBoundingClientRect = vi.fn(() => buildRect(60, 40));
      scroller.appendChild(row);
      document.body.appendChild(scroller);

      expect(
        revealInlineDetailInViewport({
          row: malformedScroller.firstElementChild as HTMLElement,
          detail: null,
        }),
      ).toBe(true);
      // scroller.scrollTop is the natural writable property (jsdom supports it)
      // and gets reassigned by the `scroller.scrollTop = clampedTop` arm.
      expect(scroller.scrollTop).toBe(38);
    });

    it('does not throw when scroller.scrollTo throws inside scrollVerticalScroller (outer try/catch swallow arm)', () => {
      const scroller = buildScrollableAncestor({
        scrollHeight: 1000,
        clientHeight: 400,
        scrollTop: 0,
        rectTop: 0,
      });
      const throwingScrollTo = vi.fn(() => {
        throw new Error('scroll boom');
      });
      Object.defineProperty(scroller, 'scrollTo', {
        configurable: true,
        value: throwingScrollTo,
      });

      const row = document.createElement('div');
      row.getBoundingClientRect = vi.fn(() => buildRect(5, 40));
      scroller.appendChild(row);
      document.body.appendChild(scroller);

      // revealInlineDetailInViewport must still return true; the thrown error
      // is swallowed by the outer try/catch in scrollVerticalScroller.
      expect(() => revealInlineDetailInViewport({ row, detail: null })).not.toThrow();
      expect(revealInlineDetailInViewport({ row, detail: null })).toBe(true);
      expect(throwingScrollTo).toHaveBeenCalled();
    });

    it('treats an HTMLElement scroller whose scrollTo source contains "notImplemented" as canUseScrollTo=false (toString.includes arm)', () => {
      // For an HTMLElement scroller the jsdom-specific shortcut is skipped
      // (isWindowScroller is false), so the code reaches the
      // Function.prototype.toString.call(...).includes('notImplemented')
      // check. A function whose own source contains that string makes
      // canUseScrollTo return false → scroller.scrollTop is assigned directly.
      const scroller = buildScrollableAncestor({
        scrollHeight: 1000,
        clientHeight: 400,
        scrollTop: 50,
        rectTop: 0,
      });
      Object.defineProperty(scroller, 'scrollTo', {
        configurable: true,
        // The literal name in the source is what the includes() check matches.
        value: function notImplementedScrollTo() {},
      });

      const row = document.createElement('div');
      // rowRect.top=60 → currentRowOffset=60 → rowHasBreathingRoom false.
      // desiredRowOffset = max(72, 60) = 72 → nextScrollTop = 50 + 60 - 72 = 38.
      row.getBoundingClientRect = vi.fn(() => buildRect(60, 40));
      scroller.appendChild(row);
      document.body.appendChild(scroller);

      expect(revealInlineDetailInViewport({ row, detail: null })).toBe(true);
      // The "notImplemented" arm routes through `scroller.scrollTop = clampedTop`.
      expect(scroller.scrollTop).toBe(38);
    });

    it('falls back to the window scroller when no ancestor is scrollable', () => {
      // resolveScrollableAncestor walks parentElement, so it never sees the
      // Document. A row attached directly to body finds no overflow-y:auto
      // oversized ancestor, so resolveVerticalScroller returns window.
      const scrollTo = vi.fn();
      Object.defineProperty(window, 'scrollTo', { configurable: true, value: scrollTo });
      vi.stubGlobal('scrollY', 50);

      const row = document.createElement('div');
      // rowRect.top=80 → currentRowOffset=80 < minTopMargin(96) → trigger.
      // desiredRowOffset = max(96, 80) = 96 → nextScrollTop = 50 + 80 - 96 = 34.
      row.getBoundingClientRect = vi.fn(() => buildRect(80, 40));
      document.body.appendChild(row);

      expect(revealInlineDetailInViewport({ row, detail: null })).toBe(true);
      // Window-scroller path: window.scrollTo receives the offset.
      expect(scrollTo).toHaveBeenCalledWith({ top: 34, behavior: 'smooth' });
    });
  });
});
