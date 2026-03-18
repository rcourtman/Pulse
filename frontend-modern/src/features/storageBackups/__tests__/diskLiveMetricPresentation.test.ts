import { describe, expect, it } from 'vitest';
import {
  getDiskLiveMetricFormattedValue,
  getDiskLiveMetricTextClass,
} from '@/features/storageBackups/diskLiveMetricPresentation';

describe('diskLiveMetricPresentation', () => {
  it('returns canonical ioTime urgency classes', () => {
    expect(getDiskLiveMetricTextClass(95, 'ioTime')).toBe(
      'text-red-600 dark:text-red-400 font-bold',
    );
    expect(getDiskLiveMetricTextClass(60, 'ioTime')).toBe(
      'text-yellow-600 dark:text-yellow-400 font-semibold',
    );
    expect(getDiskLiveMetricTextClass(20, 'ioTime')).toBe('text-muted');
  });

  it('returns canonical throughput classes', () => {
    expect(getDiskLiveMetricTextClass(120 * 1024 * 1024, 'read')).toBe(
      'text-blue-600 dark:text-blue-400 font-semibold',
    );
    expect(getDiskLiveMetricTextClass(1024, 'write')).toBe('text-muted');
  });

  it('formats canonical live metric values', () => {
    expect(getDiskLiveMetricFormattedValue(72, 'ioTime')).toBe('72%');
    expect(getDiskLiveMetricFormattedValue(1024, 'read')).toBe('1.00 KB/s');
  });
});
