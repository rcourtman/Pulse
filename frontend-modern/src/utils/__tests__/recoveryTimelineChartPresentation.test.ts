import { describe, expect, it } from 'vitest';
import {
  getRecoveryTimelineAxisLabelClass,
  getRecoveryTimelineAxisTicks,
  getRecoveryTimelineBarMinWidthClass,
  getRecoveryTimelineChartGapPx,
  getRecoveryTimelineChartMinWidthPx,
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
    expect(getRecoveryTimelineLabelEvery(15)).toBe(5);
    expect(getRecoveryTimelineLabelEvery(15, true)).toBe(7);
    expect(getRecoveryTimelineLabelEvery(60)).toBe(10);
    expect(getRecoveryTimelineLabelEvery(60, true)).toBe(15);
    expect(getRecoveryTimelineLabelEvery(365)).toBe(30);
  });

  it('derives stable range-aware chart sizing', () => {
    expect(getRecoveryTimelineChartMinWidthPx(false, 7)).toBe(0);
    expect(getRecoveryTimelineChartMinWidthPx(false, 30)).toBe(0);
    expect(getRecoveryTimelineChartMinWidthPx(true, 30)).toBe(560);
    expect(getRecoveryTimelineChartMinWidthPx(false, 90)).toBe(900);
    expect(getRecoveryTimelineChartMinWidthPx(false, 365)).toBe(2560);
    expect(getRecoveryTimelineChartGapPx(30)).toBe(3);
    expect(getRecoveryTimelineChartGapPx(90)).toBe(2);
    expect(getRecoveryTimelineChartGapPx(365)).toBe(1);
  });

  it('renders only visible axis ticks for long ranges', () => {
    expect(getRecoveryTimelineAxisTicks(0)).toEqual([]);
    expect(getRecoveryTimelineAxisTicks(7)).toHaveLength(7);
    expect(getRecoveryTimelineAxisTicks(30).map((tick) => tick.index)).toEqual([
      0, 5, 10, 15, 20, 29,
    ]);
    expect(getRecoveryTimelineAxisTicks(365).map((tick) => tick.index)).toEqual([
      0, 30, 60, 90, 120, 150, 180, 210, 240, 270, 300, 330, 364,
    ]);
    expect(getRecoveryTimelineAxisTicks(365).at(0)?.align).toBe('start');
    expect(getRecoveryTimelineAxisTicks(365).at(-1)?.align).toBe('end');
  });

  it('honors the timeline model cadence when supplied', () => {
    expect(getRecoveryTimelineAxisTicks(30, false, 10).map((tick) => tick.index)).toEqual([
      0, 10, 29,
    ]);
  });
});
