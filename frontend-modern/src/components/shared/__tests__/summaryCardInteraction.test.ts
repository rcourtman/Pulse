import { cleanup } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { preserveScrollableAncestorVerticalOffset } from '@/components/shared/contextualFocus';
import {
  resolveSummaryActiveSeriesId,
  resolveSummaryGroupMemberInteractionState,
  resolveSummaryScopeState,
  type SummarySeriesGroupScope,
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

  it('keeps group scope separate from active entity focus', () => {
    const groupScope: SummarySeriesGroupScope = {
      id: 'cluster-a',
      label: 'Cluster A (2 workloads)',
      seriesIds: ['alpha', 'beta'],
    };

    expect(
      resolveSummaryActiveSeriesId({
        hoveredSeriesId: 'gamma',
        focusedSeriesId: 'alpha',
        groupScope,
      }),
    ).toBe('alpha');
  });

  it('resolves preview and pinned scope state through one shared precedence helper', () => {
    const groupScope: SummarySeriesGroupScope = {
      id: 'cluster-a',
      label: 'Cluster A (2 workloads)',
      seriesIds: ['alpha', 'beta'],
    };

    expect(
      resolveSummaryScopeState({
        hoveredSeriesId: 'beta',
        focusedSeriesId: 'alpha',
        hoveredGroupScope: groupScope,
        focusedGroupScope: groupScope,
      }),
    ).toEqual({
      groupScope,
      kind: 'entity',
      seriesId: 'beta',
      source: 'preview',
    });

    expect(
      resolveSummaryScopeState({
        focusedGroupScope: groupScope,
      }),
    ).toEqual({
      groupScope,
      kind: 'group',
      seriesId: null,
      source: 'pinned',
    });

    expect(
      resolveSummaryScopeState({
        hoveredSeriesId: 'gamma',
        focusedGroupScope: groupScope,
      }),
    ).toEqual({
      groupScope,
      kind: 'group',
      seriesId: null,
      source: 'pinned',
    });
  });

  it('resolves group-member emphasis from hovered and pinned group scope', () => {
    const hoveredGroupScope: SummarySeriesGroupScope = {
      id: 'cluster-a',
      label: 'Cluster A (2 workloads)',
      seriesIds: ['alpha', 'beta'],
    };
    const focusedGroupScope: SummarySeriesGroupScope = {
      id: 'cluster-b',
      label: 'Cluster B (2 workloads)',
      seriesIds: ['gamma', 'delta'],
    };

    expect(
      resolveSummaryGroupMemberInteractionState({
        seriesId: 'alpha',
        hoveredGroupScope,
        focusedGroupScope,
      }),
    ).toBe('preview');

    expect(
      resolveSummaryGroupMemberInteractionState({
        seriesId: 'gamma',
        hoveredGroupScope,
        focusedGroupScope,
      }),
    ).toBe('pinned');

    expect(
      resolveSummaryGroupMemberInteractionState({
        seriesId: 'omega',
        hoveredGroupScope,
        focusedGroupScope,
      }),
    ).toBe('default');
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
