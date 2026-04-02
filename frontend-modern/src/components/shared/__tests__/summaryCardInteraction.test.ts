import { cleanup, renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  preserveScrollableAncestorVerticalOffset,
  useSummaryContextualFocusState,
} from '@/components/shared/contextualFocus';
import {
  filterSummarySeriesByGroupScope,
  resolveSummaryActiveSeriesId,
  resolveSummaryCardInteractionState,
  resolveSummaryGroupMemberInteractionState,
  resolveSummaryScopeState,
  type SummarySeriesGroupScope,
} from '@/components/shared/summaryCardInteraction';
import { buildSummaryScopePresentation } from '@/components/shared/summaryScopePresentation';

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
        chartHoveredSeriesId: 'gamma',
        hoveredSeriesId: 'beta',
        focusedSeriesId: 'alpha',
      }),
    ).toBe('gamma');

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

  it('returns default without an active hover or focus context', () => {
    expect(
      resolveSummaryCardInteractionState({
        series: [{ id: 'alpha' }],
      }),
    ).toBe('default');
  });

  it('marks cards active when the hovered series is present', () => {
    expect(
      resolveSummaryCardInteractionState({
        series: [{ id: 'alpha' }, { id: 'beta' }],
        chartHoveredSeriesId: 'beta',
        hoveredSeriesId: 'beta',
        focusedSeriesId: 'alpha',
      }),
    ).toBe('active');
  });

  it('marks cards inactive when another summary series owns the current interaction', () => {
    expect(
      resolveSummaryCardInteractionState({
        series: [{ id: 'pool:alpha' }, { id: 'pool:beta' }],
        hoveredSeriesId: 'SERIAL-123',
        focusedSeriesId: 'pool:alpha',
      }),
    ).toBe('inactive');
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

    expect(
      resolveSummaryCardInteractionState({
        series: [{ id: 'alpha' }, { id: 'beta' }],
        hoveredGroupScope: groupScope,
      }),
    ).toBe('active');

    expect(
      resolveSummaryCardInteractionState({
        series: [{ id: 'gamma' }],
        hoveredGroupScope: groupScope,
      }),
    ).toBe('inactive');

    expect(
      filterSummarySeriesByGroupScope(
        [{ id: 'alpha' }, { id: 'beta' }, { id: 'gamma' }],
        groupScope,
      ).map((series) => series.id),
    ).toEqual(['alpha', 'beta']);
  });

  it('resolves preview and pinned scope state through one shared precedence helper', () => {
    const groupScope: SummarySeriesGroupScope = {
      id: 'cluster-a',
      label: 'Cluster A (2 workloads)',
      seriesIds: ['alpha', 'beta'],
    };

    expect(
      resolveSummaryScopeState({
        chartHoveredSeriesId: 'beta',
        hoveredSeriesId: 'alpha',
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

  it('builds consistent scope-bar presentation for page, group, and entity states', () => {
    const groupScope: SummarySeriesGroupScope = {
      id: 'cluster-a',
      label: 'Cluster A (2 workloads)',
      seriesIds: ['alpha', 'beta'],
    };

    expect(
      buildSummaryScopePresentation({
        allLabel: 'All workloads',
        resolveEntityLabel: (seriesId) => ({ alpha: 'Alpha VM' })[seriesId] ?? seriesId,
        state: resolveSummaryScopeState({}),
      }),
    ).toEqual({
      contextLabel: null,
      kind: 'page',
      label: 'All workloads',
      mode: 'all',
    });

    expect(
      buildSummaryScopePresentation({
        allLabel: 'All workloads',
        state: resolveSummaryScopeState({
          hoveredGroupScope: groupScope,
        }),
      }),
    ).toEqual({
      contextLabel: null,
      kind: 'group',
      label: 'Cluster A (2 workloads)',
      mode: 'preview',
    });

    expect(
      buildSummaryScopePresentation({
        allLabel: 'All workloads',
        resolveEntityLabel: (seriesId) => ({ alpha: 'Alpha VM' })[seriesId] ?? seriesId,
        state: resolveSummaryScopeState({
          focusedSeriesId: 'alpha',
          focusedGroupScope: groupScope,
        }),
      }),
    ).toEqual({
      contextLabel: 'Cluster A (2 workloads)',
      kind: 'entity',
      label: 'Alpha VM',
      mode: 'pinned',
    });
  });

  it('filters contextual focus down to interactive series ids through one shared hook', () => {
    const [hoveredSeriesId] = createSignal<string | null>('gamma');
    const [focusedSeriesId] = createSignal<string | null>('alpha');
    const [chartHoveredSeriesId, setChartHoveredSeriesId] = createSignal<string | null>(null);
    const [hoveredGroupScope, setHoveredGroupScope] = createSignal<SummarySeriesGroupScope | null>({
      id: 'cluster-a',
      label: 'Cluster A (2 workloads)',
      seriesIds: ['alpha', 'beta'],
    });

    const { result } = renderHook(() =>
      useSummaryContextualFocusState({
        chartHoveredSeriesId,
        interactiveSeries: () => [
          { id: 'alpha', name: 'Alpha', interactive: true },
          { id: 'beta', name: 'Beta', interactive: false },
        ],
        hoveredGroupScope,
        hoveredSeriesId,
        focusedSeriesId,
        isSeriesInteractive: (series) => series.interactive,
      }),
    );

    expect(result.hasInteractiveSeriesId('alpha')).toBe(true);
    expect(result.hasInteractiveSeriesId('beta')).toBe(false);
    expect(result.activeGroupScope()).toEqual({
      id: 'cluster-a',
      label: 'Cluster A (2 workloads)',
      seriesIds: ['alpha'],
    });
    expect(result.effectiveHoveredSeriesId()).toBeNull();
    expect(result.effectiveFocusedSeriesId()).toBe('alpha');
    expect(result.activeSeriesId()).toBe('alpha');
    expect(
      result.getFocusedSeriesName([
        { id: 'alpha', name: 'Alpha', interactive: true },
        { id: 'beta', name: 'Beta', interactive: false },
      ]),
    ).toBe('Alpha');
    expect(result.interactionStateFor([{ id: 'alpha' }])).toBe('active');
    expect(result.interactionStateFor([{ id: 'beta' }])).toBe('inactive');

    setChartHoveredSeriesId('alpha');
    expect(result.effectiveChartHoveredSeriesId()).toBe('alpha');
    expect(result.activeSeriesId()).toBe('alpha');
    expect(result.getActiveSeriesName([{ id: 'alpha', name: 'Alpha', interactive: true }])).toBe(
      'Alpha',
    );

    setChartHoveredSeriesId(null);
    setHoveredGroupScope({
      id: 'cluster-a',
      label: 'Cluster A (2 workloads)',
      seriesIds: ['alpha'],
    });
    expect(result.activeGroupScope()?.id).toBe('cluster-a');
    expect(result.activeSeriesId()).toBe('alpha');
    expect(
      result.filterSeriesForActiveScope([
        { id: 'alpha', name: 'Alpha', interactive: true },
        { id: 'gamma', name: 'Gamma', interactive: true },
      ]),
    ).toEqual([{ id: 'alpha', name: 'Alpha', interactive: true }]);
    expect(result.isSeriesIdVisibleInActiveScope('alpha')).toBe(true);
    expect(result.isSeriesIdVisibleInActiveScope('gamma')).toBe(false);
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
