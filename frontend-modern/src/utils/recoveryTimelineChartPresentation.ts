export const RECOVERY_TIMELINE_LEGEND_ITEM_CLASS = 'flex items-center gap-1';
export const RECOVERY_TIMELINE_RANGE_GROUP_CLASS =
  'inline-flex rounded border border-border bg-surface p-0.5 text-xs';

export type RecoveryTimelineRangeDays = 7 | 30 | 90 | 365;

export interface RecoveryTimelineAxisTick {
  index: number;
  positionPct: number;
  align: 'start' | 'center' | 'end';
}

export function getRecoveryTimelineAxisLabelClass(selected: boolean): string {
  return selected ? 'font-semibold text-blue-700 dark:text-blue-300' : 'text-muted';
}

export function getRecoveryTimelineBarMinWidthClass(
  isMobile: boolean,
  days: 7 | 30 | 90 | 365,
): string {
  if (isMobile) return '';
  if (days === 7) return 'min-w-[28px]';
  if (days === 30) return 'min-w-[14px]';
  if (days === 365) return '';
  return 'min-w-[8px]';
}

export function getRecoveryTimelineChartMinWidthPx(
  isMobile: boolean,
  days: RecoveryTimelineRangeDays,
  pointCount: number = days,
): number {
  const effectiveDays = Math.max(days, pointCount);
  if (effectiveDays <= 7) return 0;
  if (effectiveDays <= 30) return isMobile ? 560 : 0;
  if (effectiveDays <= 90) return 900;
  return 2560;
}

export function getRecoveryTimelineChartGapPx(days: RecoveryTimelineRangeDays): number {
  if (days >= 365) return 1;
  if (days >= 90) return 2;
  return 3;
}

export function getRecoveryTimelineLabelEvery(dayCount: number, isMobile = false): number {
  if (dayCount <= 7) return 1;
  if (dayCount <= 30) return isMobile ? 7 : 5;
  if (dayCount <= 90) return isMobile ? 15 : 10;
  return 30;
}

export function getRecoveryTimelineAxisTicks(
  dayCount: number,
  isMobile = false,
  labelEveryOverride?: number,
): RecoveryTimelineAxisTick[] {
  if (dayCount <= 0) return [];

  const labelEvery =
    typeof labelEveryOverride === 'number' && Number.isFinite(labelEveryOverride)
      ? Math.max(1, Math.floor(labelEveryOverride))
      : getRecoveryTimelineLabelEvery(dayCount, isMobile);
  const lastIndex = dayCount - 1;
  const ticks: RecoveryTimelineAxisTick[] = [];

  for (let index = 0; index < dayCount; index += 1) {
    const isBoundary = index === 0 || index === lastIndex;
    if (!isBoundary && index % labelEvery !== 0) continue;
    if (!isBoundary && lastIndex - index < labelEvery) continue;

    const positionPct = dayCount === 1 ? 0 : (index / lastIndex) * 100;
    ticks.push({
      index,
      positionPct,
      align: index === 0 ? 'start' : index === lastIndex ? 'end' : 'center',
    });
  }

  return ticks;
}
