import { describe, expect, it } from 'vitest';
import {
  getRecoveryTimelineAxisLabelClass,
  getRecoveryTimelineAxisTicks,
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
    expect(getRecoveryTimelineBarMinWidthClass(false, 365)).toBe('');
  });

  it('derives label cadence from day count', () => {
    expect(getRecoveryTimelineLabelEvery(7)).toBe(1);
    expect(getRecoveryTimelineLabelEvery(15)).toBe(3);
    expect(getRecoveryTimelineLabelEvery(60)).toBe(10);
  });

  it('renders only visible axis ticks for long ranges', () => {
    expect(getRecoveryTimelineAxisTicks(0)).toEqual([]);
    expect(getRecoveryTimelineAxisTicks(7)).toHaveLength(7);
    expect(getRecoveryTimelineAxisTicks(30).map((tick) => tick.index)).toEqual([
      0, 3, 6, 9, 12, 15, 18, 21, 24, 27, 29,
    ]);
    expect(getRecoveryTimelineAxisTicks(365).map((tick) => tick.index)).toEqual([
      0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160, 170,
      180, 190, 200, 210, 220, 230, 240, 250, 260, 270, 280, 290, 300, 310, 320, 330,
      340, 350, 360, 364,
    ]);
    expect(getRecoveryTimelineAxisTicks(365).at(0)?.align).toBe('start');
    expect(getRecoveryTimelineAxisTicks(365).at(-1)?.align).toBe('end');
  });
});
