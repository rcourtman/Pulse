import { describe, expect, it } from 'vitest';
import {
  getAlertHistoryResourceTypeBadgeClass,
  getAlertHistorySourcePresentation,
} from '@/utils/alertHistoryPresentation';

describe('alertHistoryPresentation', () => {
  it('returns canonical alert history source presentation', () => {
    expect(getAlertHistorySourcePresentation('ai')).toEqual({
      label: 'Patrol',
      className:
        'text-[10px] px-1.5 py-0.5 rounded font-medium bg-violet-100 dark:bg-violet-900 text-violet-700 dark:text-violet-300',
    });

    expect(getAlertHistorySourcePresentation('alert')).toEqual({
      label: 'Alert',
      className:
        'text-[10px] px-1.5 py-0.5 rounded font-medium bg-sky-100 dark:bg-sky-900 text-sky-700 dark:text-sky-300',
    });
  });

  it('returns canonical resource type badge classes for alert history rows', () => {
    expect(getAlertHistoryResourceTypeBadgeClass('VM')).toContain('bg-blue-100');
    expect(getAlertHistoryResourceTypeBadgeClass('Node')).toContain('bg-blue-100');
    expect(getAlertHistoryResourceTypeBadgeClass('Container')).toContain('bg-green-100');
    expect(getAlertHistoryResourceTypeBadgeClass('CT')).toContain('bg-green-100');
    expect(getAlertHistoryResourceTypeBadgeClass('Storage')).toContain('bg-orange-100');
    expect(getAlertHistoryResourceTypeBadgeClass('Unknown')).toContain('bg-surface-hover');
  });
});
