import { describe, expect, it } from 'vitest';
import { getAlertHistoryResourceTypeBadgeClass } from '@/utils/alertHistoryPresentation';

describe('alertHistoryPresentation', () => {
  it('returns canonical resource type badge classes for alert history rows', () => {
    expect(getAlertHistoryResourceTypeBadgeClass('VM')).toContain('bg-blue-100');
    expect(getAlertHistoryResourceTypeBadgeClass('Node')).toContain('bg-blue-100');
    expect(getAlertHistoryResourceTypeBadgeClass('Container')).toContain('bg-green-100');
    expect(getAlertHistoryResourceTypeBadgeClass('CT')).toContain('bg-green-100');
    expect(getAlertHistoryResourceTypeBadgeClass('LXC')).toContain('bg-green-100');
    expect(getAlertHistoryResourceTypeBadgeClass('Storage')).toContain('bg-orange-100');
    expect(getAlertHistoryResourceTypeBadgeClass('Unknown')).toContain('bg-surface-hover');
  });
});
