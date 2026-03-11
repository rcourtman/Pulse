import { formatPercent } from '@/utils/format';
import type { NormalizedHealth } from './models';
import type { StorageGroupedRecords } from '@/components/Storage/useStorageModel';
import { getStorageHealthPresentation } from './healthPresentation';

export interface StorageGroupHealthCountPresentation {
  health: NormalizedHealth;
  count: number;
  label: string;
  dotClass: string;
  countClass: string;
}

export interface StorageGroupRowPresentation {
  label: string;
  showUsage: boolean;
  usagePercentLabel: string;
  poolCountLabel: string;
  healthCounts: StorageGroupHealthCountPresentation[];
}

export const STORAGE_GROUP_ROW_CLASS =
  'cursor-pointer select-none bg-surface-alt hover:bg-surface-hover transition-colors border-b border-border';
export const STORAGE_GROUP_ROW_CELL_CLASS = 'px-1.5 sm:px-2 py-0.5';
export const STORAGE_GROUP_ROW_CONTENT_CLASS = 'flex items-center gap-3';
export const STORAGE_GROUP_ROW_LABEL_CLASS =
  'text-[11px] font-semibold text-base-content w-[140px] flex-shrink-0 truncate';
export const STORAGE_GROUP_ROW_USAGE_WRAP_CLASS = 'w-48 flex-shrink-0 hidden sm:block';
export const STORAGE_GROUP_ROW_USAGE_LABEL_CLASS = 'text-xs font-medium text-muted hidden sm:inline';
export const STORAGE_GROUP_ROW_POOL_COUNT_CLASS = 'text-xs text-muted whitespace-nowrap';
export const STORAGE_GROUP_ROW_HEALTH_WRAP_CLASS = 'flex items-center gap-1.5 ml-auto';
export const STORAGE_GROUP_ROW_HEALTH_ITEM_CLASS = 'flex items-center gap-0.5';
export const STORAGE_GROUP_ROW_HEALTH_COUNT_CLASS = 'text-[10px]';
export const STORAGE_GROUP_ROW_CHEVRON_BASE_CLASS =
  'w-3.5 h-3.5 text-muted transition-transform duration-150 flex-shrink-0';
export const STORAGE_GROUP_ROW_HEALTH_DOT_CLASS = 'w-2 h-2 rounded-full';

const STORAGE_GROUP_HEALTH_ORDER: NormalizedHealth[] = [
  'healthy',
  'warning',
  'critical',
  'offline',
  'unknown',
];

export const getStorageGroupPoolCountLabel = (count: number): string =>
  `${count} ${count === 1 ? 'pool' : 'pools'}`;

export const getStorageGroupUsagePercentLabel = (usagePercent: number): string =>
  formatPercent(usagePercent);

export const getStorageGroupHealthCountPresentation = (
  byHealth: Record<NormalizedHealth, number>,
): StorageGroupHealthCountPresentation[] =>
  STORAGE_GROUP_HEALTH_ORDER.flatMap((health) => {
    const count = byHealth[health] || 0;
    if (count <= 0) return [];
    const presentation = getStorageHealthPresentation(health);
    return [
      {
        health,
        count,
        label: presentation.label,
        dotClass: presentation.dotClass,
        countClass: presentation.countClass,
      },
    ];
  });

export const buildStorageGroupRowPresentation = (
  group: StorageGroupedRecords,
): StorageGroupRowPresentation => ({
  label: group.key,
  showUsage: group.stats.totalBytes > 0,
  usagePercentLabel: getStorageGroupUsagePercentLabel(group.stats.usagePercent),
  poolCountLabel: getStorageGroupPoolCountLabel(group.items.length),
  healthCounts: getStorageGroupHealthCountPresentation(group.stats.byHealth),
});

export const getStorageGroupChevronClass = (expanded: boolean): string =>
  `${STORAGE_GROUP_ROW_CHEVRON_BASE_CLASS} ${expanded ? 'rotate-90' : ''}`.trim();
