import { describe, expect, it } from 'vitest';
import {
  resolveSummaryActiveSeriesId,
  resolveSummaryCardInteractionState,
} from '@/components/shared/summaryCardInteraction';

describe('summaryCardInteraction', () => {
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
});
