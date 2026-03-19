import { describe, expect, it } from 'vitest';

import { formatThroughputRate } from '@/utils/throughputPresentation';

describe('throughputPresentation', () => {
  it('formats bytes per second with canonical throughput labels', () => {
    expect(formatThroughputRate(0)).toBe('0 B/s');
    expect(formatThroughputRate(999)).toBe('999 B/s');
    expect(formatThroughputRate(1_000)).toBe('1 KB/s');
    expect(formatThroughputRate(1_500_000)).toBe('1.5 MB/s');
    expect(formatThroughputRate(2_000_000_000)).toBe('2.0 GB/s');
  });
});
