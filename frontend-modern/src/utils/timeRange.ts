import type { TimeRange } from '@/api/charts';

/**
 * Convert chart time range tokens to milliseconds.
 */
export function timeRangeToMs(range: TimeRange): number {
  switch (range) {
    case '5m':
      return 5 * 60_000;
    case '15m':
      return 15 * 60_000;
    case '30m':
      return 30 * 60_000;
    case '1h':
      return 60 * 60_000;
    case '4h':
      return 4 * 60 * 60_000;
    case '12h':
      return 12 * 60 * 60_000;
    case '24h':
      return 24 * 60 * 60_000;
    case '7d':
      return 7 * 24 * 60 * 60_000;
    case '30d':
      return 30 * 24 * 60 * 60_000;
    default:
      return 60 * 60_000;
  }
}

