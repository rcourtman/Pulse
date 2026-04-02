import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useSummaryPageInteractionState } from '@/components/shared/summaryTableFocus';

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

describe('useSummaryPageInteractionState', () => {
  beforeEach(() => {
    vi.stubGlobal('innerHeight', 800);
    vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => {
      cb(0);
      return 1;
    });
  });

  afterEach(() => {
    document.body.innerHTML = '';
    vi.unstubAllGlobals();
  });

  it('prefers chart hover over row hover and route focus for the active series id', () => {
    const [hoveredSeriesId] = createSignal<string | null>('row-hovered');
    const [focusedSeriesId] = createSignal<string | null>('route-focused');

    const { result } = renderHook(() =>
      useSummaryPageInteractionState({
        hoveredSeriesId,
        focusedSeriesId,
      }),
    );

    expect(result.activeSeriesId()).toBe('row-hovered');

    result.setChartHoverSync({
      sourceKey: 'cpu',
      seriesId: 'chart-hovered',
      timestamp: 123,
    });

    expect(result.activeSeriesId()).toBe('chart-hovered');
  });

  it('shows a jump affordance when the active row is mounted but outside the viewport', () => {
    const [hoveredSeriesId] = createSignal<string | null>('workload-a');
    const scrollIntoView = vi.fn();
    const root = document.createElement('div');
    const row = document.createElement('div');
    row.setAttribute('data-summary-series-id', 'workload-a');
    Object.defineProperty(row, 'getBoundingClientRect', {
      configurable: true,
      value: () => buildRect(1200),
    });
    Object.defineProperty(row, 'scrollIntoView', {
      configurable: true,
      value: scrollIntoView,
    });
    root.appendChild(row);
    document.body.appendChild(root);

    const { result } = renderHook(() =>
      useSummaryPageInteractionState({
        hoveredSeriesId,
      }),
    );

    result.setTableRootRef(root);

    expect(result.shouldShowJumpToActiveRow()).toBe(true);

    result.jumpToActiveRow();

    expect(scrollIntoView).toHaveBeenCalledWith({ behavior: 'smooth', block: 'center' });
  });

  it('reveals an unmapped row before attempting to scroll to it', () => {
    const [hoveredSeriesId] = createSignal<string | null>('pool-alpha');
    const scrollIntoView = vi.fn();
    const root = document.createElement('div');
    const row = document.createElement('div');
    row.setAttribute('data-summary-series-id', 'pool-alpha');
    Object.defineProperty(row, 'getBoundingClientRect', {
      configurable: true,
      value: () => buildRect(64),
    });
    Object.defineProperty(row, 'scrollIntoView', {
      configurable: true,
      value: scrollIntoView,
    });

    const revealActiveSeries = vi.fn(() => {
      root.appendChild(row);
    });

    const { result } = renderHook(() =>
      useSummaryPageInteractionState({
        hoveredSeriesId,
        revealActiveSeries,
      }),
    );

    result.setTableRootRef(root);
    expect(result.shouldShowJumpToActiveRow()).toBe(true);

    result.jumpToActiveRow();

    expect(revealActiveSeries).toHaveBeenCalledWith('pool-alpha');
    expect(scrollIntoView).toHaveBeenCalledWith({ behavior: 'smooth', block: 'center' });
  });

  it('reveals focused inline detail without hard-centering the row', () => {
    const [focusedSeriesId] = createSignal<string | null>('workload-a');
    const revealActiveSeries = vi.fn();
    const scrollTo = vi.fn();
    const root = document.createElement('div');
    const row = document.createElement('div');
    const detail = document.createElement('div');

    Object.defineProperty(window, 'scrollTo', {
      configurable: true,
      value: scrollTo,
    });
    vi.stubGlobal('scrollY', 0);

    row.setAttribute('data-summary-series-id', 'workload-a');
    row.getBoundingClientRect = vi.fn(() => buildRect(680, 40));
    detail.setAttribute('data-inline-detail-for', 'workload-a');
    detail.getBoundingClientRect = vi.fn(() => buildRect(724, 220));
    root.append(row, detail);
    document.body.appendChild(root);

    const { result } = renderHook(() =>
      useSummaryPageInteractionState({
        focusedSeriesId,
        revealActiveSeries,
      }),
    );

    result.setTableRootRef(root);

    expect(revealActiveSeries).toHaveBeenCalledWith('workload-a');
    expect(scrollTo).toHaveBeenCalledWith({ top: 456, behavior: 'smooth' });
  });

  it('shows the pinned-scope fallback when a focused row is mounted outside the viewport', () => {
    const [focusedSeriesId] = createSignal<string | null>('workload-a');
    const root = document.createElement('div');
    const row = document.createElement('div');
    row.setAttribute('data-summary-series-id', 'workload-a');
    Object.defineProperty(row, 'getBoundingClientRect', {
      configurable: true,
      value: () => buildRect(1200),
    });
    root.appendChild(row);
    document.body.appendChild(root);

    const { result } = renderHook(() =>
      useSummaryPageInteractionState({
        focusedSeriesId,
      }),
    );

    result.setTableRootRef(root);

    expect(result.shouldShowPinnedScopeFallback()).toBe(true);
  });

  it('keeps the pinned-scope fallback hidden while the focused group row is visible', () => {
    const [focusedGroupId] = createSignal<string | null>('group-a');
    const root = document.createElement('div');
    const row = document.createElement('div');
    row.setAttribute('data-summary-group-id', 'group-a');
    Object.defineProperty(row, 'getBoundingClientRect', {
      configurable: true,
      value: () => buildRect(120),
    });
    root.appendChild(row);
    document.body.appendChild(root);

    const { result } = renderHook(() =>
      useSummaryPageInteractionState({
        focusedGroupId,
      }),
    );

    result.setTableRootRef(root);

    expect(result.shouldShowPinnedScopeFallback()).toBe(false);
  });

  it('recomputes pinned-scope fallback visibility when scrolling moves the pinned row off-screen', () => {
    const [focusedSeriesId] = createSignal<string | null>('workload-a');
    let top = 120;
    const root = document.createElement('div');
    const row = document.createElement('div');
    row.setAttribute('data-summary-series-id', 'workload-a');
    Object.defineProperty(row, 'getBoundingClientRect', {
      configurable: true,
      value: () => buildRect(top),
    });
    root.appendChild(row);
    document.body.appendChild(root);

    const { result } = renderHook(() =>
      useSummaryPageInteractionState({
        focusedSeriesId,
      }),
    );

    result.setTableRootRef(root);
    expect(result.shouldShowPinnedScopeFallback()).toBe(false);

    top = 1200;
    window.dispatchEvent(new Event('scroll'));

    expect(result.shouldShowPinnedScopeFallback()).toBe(true);
  });

  it('clears pinned scope when operators click table whitespace on a clear surface', () => {
    const [focusedSeriesId] = createSignal<string | null>('workload-a');
    const clearPinnedScope = vi.fn();
    const root = document.createElement('div');
    root.setAttribute('data-summary-clear-surface', '');
    const filler = document.createElement('div');
    root.appendChild(filler);
    document.body.appendChild(root);

    const { result } = renderHook(() =>
      useSummaryPageInteractionState({
        clearPinnedScope,
        focusedSeriesId,
      }),
    );

    result.setTableRootRef(root);
    filler.click();

    expect(clearPinnedScope).toHaveBeenCalledTimes(1);
  });

  it('does not clear pinned scope when operators click an active summary row', () => {
    const [focusedSeriesId] = createSignal<string | null>('workload-a');
    const clearPinnedScope = vi.fn();
    const root = document.createElement('div');
    root.setAttribute('data-summary-clear-surface', '');
    const row = document.createElement('div');
    row.setAttribute('data-summary-series-id', 'workload-a');
    root.appendChild(row);
    document.body.appendChild(root);

    const { result } = renderHook(() =>
      useSummaryPageInteractionState({
        clearPinnedScope,
        focusedSeriesId,
      }),
    );

    result.setTableRootRef(root);
    row.click();

    expect(clearPinnedScope).not.toHaveBeenCalled();
  });
});
