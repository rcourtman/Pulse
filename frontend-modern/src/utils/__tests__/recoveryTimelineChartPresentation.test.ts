import { describe, expect, it } from 'vitest';
import {
  getRecoveryTimelineAxisLabelClass,
  getRecoveryTimelineBarMinWidthClass,
  getRecoveryTimelineLabelEvery,
  RECOVERY_TIMELINE_LEGEND_ITEM_CLASS,
  RECOVERY_TIMELINE_RANGE_GROUP_CLASS,
} from '@/utils/recoveryTimelineChartPresentation';

describe('recoveryTimelineChartPresentation', () => {
  it('exposes shared legend and range-group classes', () => {
    expect(RECOVERY_TIMELINE_LEGEND_ITEM_CLASS).toContain('items-center');
    expect(RECOVERY_TIMELINE_RANGE_GROUP_CLASS).toContain('border-border');
  });

  it('derives axis label and bar width classes', () => {
    expect(getRecoveryTimelineAxisLabelClass(true)).toContain('text-blue-700');
    expect(getRecoveryTimelineAxisLabelClass(false)).toBe('text-muted');
    expect(getRecoveryTimelineBarMinWidthClass(true, 30)).toBe('');
    expect(getRecoveryTimelineBarMinWidthClass(false, 7)).toBe('min-w-[28px]');
    expect(getRecoveryTimelineBarMinWidthClass(false, 30)).toBe('min-w-[14px]');
    expect(getRecoveryTimelineBarMinWidthClass(false, 90)).toBe('min-w-[8px]');
  });

  it('derives label cadence from day count', () => {
    expect(getRecoveryTimelineLabelEvery(7)).toBe(1);
    expect(getRecoveryTimelineLabelEvery(15)).toBe(3);
    expect(getRecoveryTimelineLabelEvery(60)).toBe(10);
  });
});
