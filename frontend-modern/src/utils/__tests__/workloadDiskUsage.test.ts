import { describe, expect, it } from 'vitest';
import type { Disk } from '@/types/api';
import { getWorkloadDiskUsagePercent } from '../workloadDiskUsage';

describe('workload aggregate disk evidence', () => {
  it.each([0, 0.01, 0.5, 1, 1.01, 40, 95])(
    'keeps %s percentage points without guessing a fraction',
    (usage) => {
      expect(getWorkloadDiskUsagePercent({ usage })).toBe(usage);
      expect(getWorkloadDiskUsagePercent({ usage, total: 100 })).toBe(usage);
    },
  );

  it.each([
    undefined,
    null,
    {},
    { total: 100 },
    { free: 100 },
    { used: 0 },
    { used: 0, total: 0 },
    { used: -1, total: 100 },
    { used: NaN, total: 100 },
    { used: 0, total: Infinity },
    { used: 0, total: -1 },
    ...[-1, NaN, Infinity].map((usage) => ({ usage, total: 100, used: 0 })),
  ] satisfies Array<Disk | undefined | null>)(
    'does not manufacture a healthy zero from %j',
    (disk) => {
      expect(getWorkloadDiskUsagePercent(disk)).toBeNull();
    },
  );

  it('uses finite byte readings only when no percentage is supplied', () => {
    expect(getWorkloadDiskUsagePercent({ used: 0, total: 100 })).toBe(0);
    expect(getWorkloadDiskUsagePercent({ used: 0.5, total: 100 })).toBe(0.5);
    expect(getWorkloadDiskUsagePercent({ used: 50, total: 200 })).toBe(25);
    expect(getWorkloadDiskUsagePercent({ used: 50, total: 200, usage: 20 })).toBe(20);
    expect(getWorkloadDiskUsagePercent({ used: 150, total: 100 })).toBe(100);
  });
});
