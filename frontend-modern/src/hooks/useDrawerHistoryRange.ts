import type { HistoryTimeRange } from '@/api/charts';
import { usePersistentSignal } from './usePersistentSignal';

const DRAWER_HISTORY_RANGE_PREFIX = 'pulse.drawerHistoryRange';

const allowedHistoryRanges = new Set<HistoryTimeRange>([
  '1h',
  '6h',
  '12h',
  '24h',
  '7d',
  '30d',
  '90d',
]);

function normaliseHistoryRange(value: string): HistoryTimeRange {
  return allowedHistoryRanges.has(value as HistoryTimeRange) ? (value as HistoryTimeRange) : '1h';
}

export function useDrawerHistoryRange(resourceKey: string) {
  return usePersistentSignal<HistoryTimeRange>(
    `${DRAWER_HISTORY_RANGE_PREFIX}.${resourceKey}`,
    '1h',
    {
      deserialize: normaliseHistoryRange,
      serialize: (value) => value,
    },
  );
}
