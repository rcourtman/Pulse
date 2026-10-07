import { renderHook, waitFor } from '@solidjs/testing-library';
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

  it('prefers row hover over route focus for the active series id', () => {
    const [hoveredSeriesId, setHoveredSeriesId] = createSignal<string | null>('row-hovered');
    const [focusedSeriesId] = createSignal<string | null>('route-focused');

    const { result } = renderHook(() =>
      useSummaryPageInteractionState({
        hoveredSeriesId,
        focusedSeriesId,
      }),
    );

    expect(result.activeSeriesId()).toBe('row-hovered');

    setHoveredSeriesId(null);

    expect(result.activeSeriesId()).toBe('route-focused');
  });

  it('does not subscribe to window scroll or resize', () => {
    const addEventListener = vi.spyOn(window, 'addEventListener');
    const [hoveredSeriesId] = createSignal<string | null>('workload-a');
    const [focusedSeriesId] = createSignal<string | null>('workload-b');
    const root = document.createElement('div');
    document.body.appendChild(root);

    const { result } = renderHook(() =>
      useSummaryPageInteractionState({
        hoveredSeriesId,
        focusedSeriesId,
      }),
    );

    result.setTableRootRef(root);

    const subscribed = addEventListener.mock.calls.map(([type]) => type);
    addEventListener.mockRestore();
    expect(subscribed).not.toContain('scroll');
    expect(subscribed).not.toContain('resize');
  });

  it('reveals a focused row that only mounts once its owning view opens', async () => {
    const [focusedSeriesId] = createSignal<string | null>('pool-alpha');
    const scrollTo = vi.fn();
    const root = document.createElement('div');
    const row = document.createElement('div');
    const detail = document.createElement('div');

    Object.defineProperty(window, 'scrollTo', {
      configurable: true,
      value: scrollTo,
    });
    vi.stubGlobal('scrollY', 0);

    row.setAttribute('data-summary-series-id', 'pool-alpha');
    row.getBoundingClientRect = vi.fn(() => buildRect(680, 40));
    detail.setAttribute('data-inline-detail-for', 'pool-alpha');
    detail.getBoundingClientRect = vi.fn(() => buildRect(724, 220));
    document.body.appendChild(root);

    // The owning view renders the row after the reveal request returns, so the
    // bridge must pick it up through its mutation observer.
    const revealActiveSeries = vi.fn(() => {
      window.setTimeout(() => root.append(row, detail), 0);
    });

    const { result } = renderHook(() =>
      useSummaryPageInteractionState({
        focusedSeriesId,
        revealActiveSeries,
      }),
    );

    result.setTableRootRef(root);

    expect(revealActiveSeries).toHaveBeenCalledWith('pool-alpha');
    expect(scrollTo).not.toHaveBeenCalled();
    await waitFor(() => {
      expect(scrollTo).toHaveBeenCalledWith({ top: 456, behavior: 'smooth' });
    });
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

  it('clears pinned scope when operators click table whitespace on a clear surface', () => {
    const [focusedSeriesId] = createSignal<string | null>('workload-a');
    const clearPinnedScope = vi.fn();
    const clearRoot = document.createElement('div');
    const root = document.createElement('div');
    root.setAttribute('data-summary-clear-surface', '');
    const filler = document.createElement('div');
    clearRoot.append(root, filler);
    document.body.appendChild(clearRoot);

    const { result } = renderHook(() =>
      useSummaryPageInteractionState({
        clearPinnedScope,
        focusedSeriesId,
      }),
    );

    result.setTableRootRef(root);
    result.setClearSurfaceRootRef(clearRoot);
    filler.click();

    expect(clearPinnedScope).toHaveBeenCalledTimes(1);
  });

  it('does not clear pinned scope when operators click an active summary row', () => {
    const [focusedSeriesId] = createSignal<string | null>('workload-a');
    const clearPinnedScope = vi.fn();
    const clearRoot = document.createElement('div');
    const root = document.createElement('div');
    root.setAttribute('data-summary-clear-surface', '');
    const row = document.createElement('div');
    row.setAttribute('data-summary-series-id', 'workload-a');
    clearRoot.append(root);
    root.appendChild(row);
    document.body.appendChild(clearRoot);

    const { result } = renderHook(() =>
      useSummaryPageInteractionState({
        clearPinnedScope,
        focusedSeriesId,
      }),
    );

    result.setTableRootRef(root);
    result.setClearSurfaceRootRef(clearRoot);
    row.click();

    expect(clearPinnedScope).not.toHaveBeenCalled();
  });

  it('does not clear pinned scope when operators click ignored controls inside the clear root', () => {
    const [focusedGroupId] = createSignal<string | null>('group-a');
    const clearPinnedScope = vi.fn();
    const clearRoot = document.createElement('div');
    const root = document.createElement('div');
    const controls = document.createElement('div');
    const input = document.createElement('input');
    controls.setAttribute('data-summary-clear-ignore', '');
    controls.appendChild(input);
    clearRoot.append(controls, root);
    document.body.appendChild(clearRoot);

    const { result } = renderHook(() =>
      useSummaryPageInteractionState({
        clearPinnedScope,
        focusedGroupId,
      }),
    );

    result.setTableRootRef(root);
    result.setClearSurfaceRootRef(clearRoot);
    input.click();

    expect(clearPinnedScope).not.toHaveBeenCalled();
  });

  it('clears pinned scope when operators click neutral whitespace inside the table root', () => {
    const [focusedGroupId] = createSignal<string | null>('group-a');
    const clearPinnedScope = vi.fn();
    const clearRoot = document.createElement('div');
    const root = document.createElement('div');
    const neutral = document.createElement('div');
    root.appendChild(neutral);
    clearRoot.appendChild(root);
    document.body.appendChild(clearRoot);

    const { result } = renderHook(() =>
      useSummaryPageInteractionState({
        clearPinnedScope,
        focusedGroupId,
      }),
    );

    result.setTableRootRef(root);
    result.setClearSurfaceRootRef(clearRoot);
    neutral.click();

    expect(clearPinnedScope).toHaveBeenCalledTimes(1);
  });

  it('does not clear pinned scope when page-shell clicks land above the table root', () => {
    const [focusedGroupId] = createSignal<string | null>('group-a');
    const clearPinnedScope = vi.fn();
    const clearRoot = document.createElement('div');
    const root = document.createElement('div');
    const summary = document.createElement('div');
    clearRoot.append(summary, root);
    document.body.appendChild(clearRoot);

    Object.defineProperty(root, 'getBoundingClientRect', {
      configurable: true,
      value: () => buildRect(200, 120),
    });
    Object.defineProperty(summary, 'getBoundingClientRect', {
      configurable: true,
      value: () => buildRect(40, 80),
    });

    const { result } = renderHook(() =>
      useSummaryPageInteractionState({
        clearPinnedScope,
        focusedGroupId,
      }),
    );

    result.setTableRootRef(root);
    result.setClearSurfaceRootRef(clearRoot);
    summary.dispatchEvent(new MouseEvent('click', { bubbles: true, clientY: 80 }));

    expect(clearPinnedScope).not.toHaveBeenCalled();
  });

  it('routes Escape through the shared page-clear owner', () => {
    const onEscapeClear = vi.fn();

    renderHook(() =>
      useSummaryPageInteractionState({
        onEscapeClear,
      }),
    );

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));

    expect(onEscapeClear).toHaveBeenCalledTimes(1);
  });
});
