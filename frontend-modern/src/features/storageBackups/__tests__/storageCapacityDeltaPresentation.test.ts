import { describe, expect, it } from 'vitest';
import {
  buildStorageCapacityDeltaPresentation,
  computeStorageCapacityDelta,
  formatStorageCapacityDelta,
  getStorageCapacityDeltaToneClass,
} from '@/features/storageBackups/storageCapacityDeltaPresentation';

describe('storageCapacityDeltaPresentation', () => {
  it('computes a smoothed capacity delta from the start and end of the series', () => {
    const delta = computeStorageCapacityDelta([
      { timestamp: 1, value: 100 },
      { timestamp: 2, value: 110 },
      { timestamp: 3, value: 140 },
      { timestamp: 4, value: 150 },
    ]);

    expect(delta).toBe(40);
  });

  it('formats increase, decrease, and neutral deltas canonically', () => {
    expect(formatStorageCapacityDelta(2048)).toBe('+2.00 KB');
    expect(formatStorageCapacityDelta(-2048)).toBe('-2.00 KB');
    expect(formatStorageCapacityDelta(0)).toBe('0 B');
    expect(formatStorageCapacityDelta(null)).toBe('—');
  });

  it('builds tone and title metadata for operator-visible growth labels', () => {
    expect(getStorageCapacityDeltaToneClass(100)).toContain('text-amber-600');
    expect(getStorageCapacityDeltaToneClass(-100)).toContain('text-sky-600');
    expect(getStorageCapacityDeltaToneClass(0)).toBe('text-muted');

    expect(buildStorageCapacityDeltaPresentation([], '24h')).toEqual({
      deltaBytes: null,
      label: '—',
      title: 'No storage change history available for 24h.',
      toneClass: 'text-muted',
    });
    expect(buildStorageCapacityDeltaPresentation([{ timestamp: 1, value: 100 }, { timestamp: 2, value: 100 }], '24h')).toEqual({
      deltaBytes: 0,
      label: '0 B',
      title: 'No used-capacity change over 24h.',
      toneClass: 'text-muted',
    });
  });
});
