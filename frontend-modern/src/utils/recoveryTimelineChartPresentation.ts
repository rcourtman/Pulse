export const RECOVERY_TIMELINE_LEGEND_ITEM_CLASS = 'flex items-center gap-1';
export const RECOVERY_TIMELINE_RANGE_GROUP_CLASS =
  'inline-flex rounded border border-border bg-surface p-0.5 text-xs';

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

export function getRecoveryTimelineLabelEvery(dayCount: number): number {
  if (dayCount <= 7) return 1;
  if (dayCount <= 30) return 3;
  return 10;
}

export function getRecoveryTimelineAxisTicks(dayCount: number): RecoveryTimelineAxisTick[] {
  if (dayCount <= 0) return [];

  const labelEvery = getRecoveryTimelineLabelEvery(dayCount);
  const lastIndex = dayCount - 1;
  const ticks: RecoveryTimelineAxisTick[] = [];

  for (let index = 0; index < dayCount; index += 1) {
    const isBoundary = index === 0 || index === lastIndex;
    if (!isBoundary && index % labelEvery !== 0) continue;

    const positionPct = dayCount === 1 ? 0 : (index / lastIndex) * 100;
    ticks.push({
      index,
      positionPct,
      align: index === 0 ? 'start' : index === lastIndex ? 'end' : 'center',
    });
  }

  return ticks;
}
