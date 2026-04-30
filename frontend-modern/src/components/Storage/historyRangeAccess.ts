import type { HistoryTimeRange } from '@/api/charts';

export type HistoryRangeOption = {
  value: HistoryTimeRange;
  label: string;
};

export function getHistoryRangeDays(range: HistoryTimeRange): number {
  const match = range.match(/^(\d+)(m|h|d)$/);
  if (!match) return 0;

  const value = Number.parseInt(match[1], 10);
  if (!Number.isFinite(value) || value <= 0) return 0;

  switch (match[2]) {
    case 'm':
      return value / (24 * 60);
    case 'h':
      return value / 24;
    case 'd':
      return value;
    default:
      return 0;
  }
}

export function getUnlockedHistoryRangeOptions<T extends HistoryRangeOption>(
  options: readonly T[],
  maxHistoryDays: number,
): T[] {
  return options.filter((option) => getHistoryRangeDays(option.value) <= maxHistoryDays);
}

export function resolveHistoryRangeWithinLimit<T extends HistoryRangeOption>(
  currentRange: HistoryTimeRange,
  options: readonly T[],
  maxHistoryDays: number,
): HistoryTimeRange {
  const available = getUnlockedHistoryRangeOptions(options, maxHistoryDays);
  if (available.some((option) => option.value === currentRange)) {
    return currentRange;
  }

  if (available.length > 0) {
    return available[available.length - 1].value;
  }

  return options.length > 0 ? options[0].value : '24h';
}
