import { describe, it, expect } from 'vitest';
import type { TimeRange } from '@/api/charts';
import { timeRangeToMs } from '@/utils/timeRange';

describe('timeRangeToMs', () => {
  const cases: Array<[TimeRange, number]> = [
    ['5m', 5 * 60_000],
    ['15m', 15 * 60_000],
    ['30m', 30 * 60_000],
    ['1h', 60 * 60_000],
    ['4h', 4 * 60 * 60_000],
    ['12h', 12 * 60 * 60_000],
    ['24h', 24 * 60 * 60_000],
    ['7d', 7 * 24 * 60 * 60_000],
    ['30d', 30 * 24 * 60 * 60_000],
  ];

  it.each(cases)('converts %s to %d milliseconds', (range, expectedMs) => {
    expect(timeRangeToMs(range)).toBe(expectedMs);
  });
});
