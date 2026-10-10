import { cleanup } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { preserveScrollableAncestorVerticalOffset } from '@/components/shared/contextualFocus';
import {
  resolveSummaryActiveSeriesId,
  resolveSummaryScopeState,
} from '@/components/shared/summaryCardInteraction';

describe('summaryCardInteraction', () => {
  beforeEach(() => {
    vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
      callback(0);
      return 1;
    });
  });

  afterEach(() => {
    cleanup();
    document.body.innerHTML = '';
    vi.unstubAllGlobals();
  });

  it('prefers hovered series ids and falls back to focused ids', () => {
    expect(
      resolveSummaryActiveSeriesId({
        hoveredSeriesId: 'beta',
        focusedSeriesId: 'alpha',
      }),
    ).toBe('beta');

    expect(
      resolveSummaryActiveSeriesId({
        hoveredSeriesId: '',
        focusedSeriesId: 'alpha',
      }),
    ).toBe('alpha');
  });

  it('resolves preview, pinned and page scope state through one shared precedence helper', () => {
    expect(
      resolveSummaryScopeState({
        hoveredSeriesId: 'beta',
        focusedSeriesId: 'alpha',
      }),
    ).toEqual({
      kind: 'entity',
      seriesId: 'beta',
      source: 'preview',
    });

    expect(
      resolveSummaryScopeState({
        hoveredSeriesId: '  ',
        focusedSeriesId: ' alpha ',
      }),
    ).toEqual({
      kind: 'entity',
      seriesId: 'alpha',
      source: 'pinned',
    });

    expect(resolveSummaryScopeState({})).toEqual({
      kind: 'page',
      seriesId: null,
      source: 'page',
    });
  });

  it('preserves the nearest scrollable ancestor when contextual focus changes locally', () => {
    const scroller = document.createElement('div');
    scroller.style.overflowY = 'auto';
    Object.defineProperty(scroller, 'scrollHeight', {
      configurable: true,
      value: 400,
    });
    Object.defineProperty(scroller, 'clientHeight', {
      configurable: true,
      value: 200,
    });
    scroller.scrollTop = 120;
    const child = document.createElement('div');
    scroller.appendChild(child);
    document.body.appendChild(scroller);

    preserveScrollableAncestorVerticalOffset(child, () => {
      scroller.scrollTop = 0;
    });

    expect(scroller.scrollTop).toBe(120);
  });
});
