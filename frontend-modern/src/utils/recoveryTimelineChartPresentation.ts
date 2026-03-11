export const RECOVERY_TIMELINE_LEGEND_ITEM_CLASS = 'flex items-center gap-1';
export const RECOVERY_TIMELINE_RANGE_GROUP_CLASS =
  'inline-flex rounded border border-border bg-surface p-0.5 text-xs';

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
  return 'min-w-[8px]';
}

export function getRecoveryTimelineLabelEvery(dayCount: number): number {
  if (dayCount <= 7) return 1;
  if (dayCount <= 30) return 3;
  return 10;
}
