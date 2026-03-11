import { describe, expect, it } from 'vitest';
import {
  DASHBOARD_MAX_ACTION_ITEMS,
  DASHBOARD_PRIORITY_ORDER,
  formatDashboardDelta,
  formatDashboardPercent,
  getDashboardDeltaColorClass,
  getDashboardPriorityBadgeClass,
  getDashboardStatusBadgeClass,
} from '@/utils/dashboardMetricPresentation';

describe('dashboardMetricPresentation', () => {
  it('returns canonical status badge classes', () => {
    expect(getDashboardStatusBadgeClass('online')).toContain('bg-emerald-100');
    expect(getDashboardStatusBadgeClass('offline')).toContain('bg-surface-alt');
    expect(getDashboardStatusBadgeClass('warning')).toContain('bg-amber-100');
    expect(getDashboardStatusBadgeClass('critical')).toContain('bg-red-100');
    expect(getDashboardStatusBadgeClass('unknown')).toContain('text-muted');
  });

  it('formats percent and deltas canonically', () => {
    expect(formatDashboardPercent(49.6)).toBe('50%');
    expect(formatDashboardDelta(null)).toBeNull();
    expect(formatDashboardDelta(5)).toBe('+5.0%');
    expect(formatDashboardDelta(-0.5)).toBe('-0.5%');
  });

  it('returns canonical delta and priority presentation', () => {
    expect(getDashboardDeltaColorClass(10)).toContain('red-500');
    expect(getDashboardDeltaColorClass(3)).toContain('amber-500');
    expect(getDashboardDeltaColorClass(-10)).toContain('emerald-500');
    expect(getDashboardDeltaColorClass(-3)).toContain('blue-500');
    expect(getDashboardDeltaColorClass(0)).toBe('text-muted');
    expect(getDashboardPriorityBadgeClass('critical')).toContain('bg-red-100');
    expect(getDashboardPriorityBadgeClass('high')).toContain('bg-orange-100');
    expect(getDashboardPriorityBadgeClass('medium')).toContain('bg-amber-100');
    expect(getDashboardPriorityBadgeClass('low')).toContain('bg-blue-100');
    expect(DASHBOARD_PRIORITY_ORDER.critical).toBe(0);
    expect(DASHBOARD_MAX_ACTION_ITEMS).toBe(5);
  });
});
