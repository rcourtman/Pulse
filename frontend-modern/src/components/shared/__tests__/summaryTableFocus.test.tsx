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
});
