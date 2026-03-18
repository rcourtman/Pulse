import { describe, expect, it } from 'vitest';
import {
  computeDashboardStorageCapacityPercent,
  DASHBOARD_STORAGE_EMPTY_STATE,
  getDashboardStorageCapacityBarClass,
  getDashboardStorageIssueBadges,
} from '@/utils/dashboardStoragePresentation';

describe('dashboardStoragePresentation', () => {
  it('computes canonical capacity percentage', () => {
    expect(computeDashboardStorageCapacityPercent(50, 200)).toBe(25);
    expect(computeDashboardStorageCapacityPercent(0, 0)).toBe(0);
  });

  it('returns canonical issue badges and empty-state copy', () => {
    expect(DASHBOARD_STORAGE_EMPTY_STATE).toBe('No storage resources');
    expect(getDashboardStorageIssueBadges({ warningCount: 2, criticalCount: 1 })).toEqual([
      { label: '2 warnings', className: expect.stringContaining('amber-600') },
      { label: '1 critical', className: expect.stringContaining('red-600') },
    ]);
    expect(getDashboardStorageCapacityBarClass(90)).toContain('bg');
  });
});
