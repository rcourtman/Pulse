import { cleanup, renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  preserveScrollableAncestorVerticalOffset,
  useSummaryContextualFocusState,
} from '@/components/shared/contextualFocus';
import {
  resolveSummaryActiveSeriesId,
  resolveSummaryCardInteractionState,
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

  it('filters contextual focus down to interactive series ids through one shared hook', () => {
    const [hoveredSeriesId] = createSignal<string | null>('gamma');
    const [focusedSeriesId] = createSignal<string | null>('alpha');

    const { result } = renderHook(() =>
      useSummaryContextualFocusState({
        interactiveSeries: () => [
          { id: 'alpha', name: 'Alpha', interactive: true },
          { id: 'beta', name: 'Beta', interactive: false },
        ],
        hoveredSeriesId,
        focusedSeriesId,
        isSeriesInteractive: (series) => series.interactive,
      }),
    );

    expect(result.hasInteractiveSeriesId('alpha')).toBe(true);
    expect(result.hasInteractiveSeriesId('beta')).toBe(false);
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
