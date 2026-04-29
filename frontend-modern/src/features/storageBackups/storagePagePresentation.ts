import { formatBytes, formatPercent } from '@/utils/format';
import type { StorageRecord } from './models';
import type { CephSummaryStats } from './cephSummaryPresentation';

export type StoragePageBannerKind =
  | 'reconnecting'
  | 'fetch-error'
  | 'disconnected'
  | 'waiting-for-data';

export type StorageViewOption = {
  value: 'pools' | 'disks';
  label: string;
};

export type StoragePoolTableColumn = {
  label: string;
  compactLabel: string;
  className: string;
  colClassName: string;
};

const STORAGE_POOL_TABLE_HEADER_CLASS =
  'overflow-hidden text-ellipsis whitespace-nowrap px-1 sm:px-1.5 lg:px-2 py-0.5 text-left text-[10px] sm:text-[11px] lg:text-xs font-medium uppercase tracking-wider';

export const STORAGE_VIEW_OPTIONS: readonly StorageViewOption[] = [
  { value: 'pools', label: 'Pools' },
  { value: 'disks', label: 'Physical Disks' },
];

export const getStoragePoolTableColumns = (
  growthColumnLabel: string,
): readonly StoragePoolTableColumn[] => [
  {
    label: 'Storage',
    compactLabel: 'Storage',
    className: `${STORAGE_POOL_TABLE_HEADER_CLASS} w-[34%] sm:w-[26%] md:w-[23%] xl:w-[18%]`,
    colClassName: 'w-[34%] sm:w-[26%] md:w-[23%] xl:w-[18%]',
  },
  {
    label: 'Primary Issue',
    compactLabel: 'Issue',
    className: `${STORAGE_POOL_TABLE_HEADER_CLASS} w-[24%] sm:w-[20%] md:w-[17%] xl:w-[14%]`,
    colClassName: 'w-[24%] sm:w-[20%] md:w-[17%] xl:w-[14%]',
  },
  {
    label: 'Source',
    compactLabel: 'Src',
    className: `${STORAGE_POOL_TABLE_HEADER_CLASS} w-[13%] sm:w-[10%] md:w-[8%]`,
    colClassName: 'w-[13%] sm:w-[10%] md:w-[8%]',
  },
  {
    label: 'Type',
    compactLabel: 'Type',
    className: `${STORAGE_POOL_TABLE_HEADER_CLASS} hidden xl:table-cell xl:w-[8%]`,
    colClassName: 'hidden xl:table-column xl:w-[8%]',
  },
  {
    label: 'Host',
    compactLabel: 'Host',
    className: `${STORAGE_POOL_TABLE_HEADER_CLASS} hidden sm:table-cell sm:w-[13%] md:w-[12%] xl:w-[10%]`,
    colClassName: 'hidden sm:table-column sm:w-[13%] md:w-[12%] xl:w-[10%]',
  },
  {
    label: 'Protection',
    compactLabel: 'Prot',
    className: `${STORAGE_POOL_TABLE_HEADER_CLASS} hidden sm:table-cell sm:w-[15%] md:w-[15%] xl:w-[13%]`,
    colClassName: 'hidden sm:table-column sm:w-[15%] md:w-[15%] xl:w-[13%]',
  },
  {
    label: 'Usage',
    compactLabel: 'Used',
    className: `${STORAGE_POOL_TABLE_HEADER_CLASS} w-[29%] sm:w-[16%] md:w-[25%] xl:w-[18%]`,
    colClassName: 'w-[29%] sm:w-[16%] md:w-[25%] xl:w-[18%]',
  },
  {
    label: growthColumnLabel,
    compactLabel: growthColumnLabel.replace(/^Growth\s*\((.+)\)$/i, '$1'),
    className: `${STORAGE_POOL_TABLE_HEADER_CLASS} hidden xl:table-cell xl:w-[11%]`,
    colClassName: 'hidden xl:table-column xl:w-[11%]',
  },
];

export const STORAGE_BANNER_ACTION_BUTTON_CLASS =
  'rounded border border-amber-300 bg-amber-100 px-2 py-1 text-xs font-medium text-amber-800 hover:bg-amber-200 dark:border-amber-700 dark:bg-amber-900 dark:text-amber-200 dark:hover:bg-amber-900';
export const STORAGE_PAGE_BANNER_ROW_CLASS = 'flex items-center justify-between gap-3';
export const STORAGE_PAGE_BANNER_TEXT_CLASS = 'text-xs text-amber-800 dark:text-amber-200';

export const STORAGE_CONTROLS_NODE_SELECT_CLASS =
  'px-2 py-1 text-xs border border-border rounded-md bg-surface text-base-content focus:ring-2 focus:ring-blue-500 focus:border-blue-500';

export const STORAGE_CONTROLS_NODE_DIVIDER_CLASS = 'h-5 w-px bg-surface-hover hidden sm:block';

export const STORAGE_CONTENT_CARD_HEADER_CLASS =
  'border-b border-border bg-surface-hover px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-muted';
export const STORAGE_CONTENT_CARD_BODY_CLASS = 'p-2';

export const STORAGE_POOLS_EMPTY_STATE_CLASS = 'p-6 text-sm text-muted';
export const STORAGE_POOLS_LOADING_STATE_CLASS = 'p-6 text-sm text-muted';
export const STORAGE_POOLS_TABLE_CLASS = 'w-full table-fixed text-xs';
export const STORAGE_POOLS_HEADER_ROW_CLASS = 'bg-surface-alt text-muted border-b border-border';
export const STORAGE_POOLS_BODY_CLASS = 'divide-y divide-border';

export const shouldShowCephSummaryCard = (
  view: 'pools' | 'disks',
  cephSummary: CephSummaryStats,
  filteredRecords: StorageRecord[],
  isCephRecord: (record: StorageRecord) => boolean,
): boolean =>
  view === 'pools' &&
  cephSummary.clusters.length > 0 &&
  filteredRecords.some((record) => isCephRecord(record));

export const getCephSummaryClusterCountLabel = (count: number): string =>
  `${count} cluster${count !== 1 ? 's' : ''} detected`;

export const getCephSummaryHeading = (): string => 'Ceph Summary';

export const getCephSummaryTotalLabel = (totalBytes: number): string => formatBytes(totalBytes);

export const getCephSummaryUsageLabel = (usagePercent: number): string =>
  `${formatPercent(usagePercent)} used`;

export const getCephClusterCardTitle = (name: string): string => name || 'Ceph Cluster';

export const getStoragePageBannerMessage = (kind: StoragePageBannerKind): string => {
  switch (kind) {
    case 'reconnecting':
      return 'Reconnecting to backend data stream…';
    case 'fetch-error':
      return 'Unable to refresh storage resources. Showing latest available data.';
    case 'disconnected':
      return 'Storage data stream disconnected. Data may be stale.';
    case 'waiting-for-data':
      return 'Waiting for storage data from connected platforms.';
  }
};

export const getStoragePageBannerActionLabel = (kind: StoragePageBannerKind): string | null => {
  if (kind === 'reconnecting') return 'Retry now';
  if (kind === 'disconnected') return 'Reconnect';
  return null;
};

export const getStorageTableHeading = (view: 'pools' | 'disks'): string =>
  view === 'pools' ? 'Storage Pools' : 'Physical Disks';

export const getStorageLoadingMessage = (): string => 'Loading storage resources...';

export const getStorageEmptyStateMessage = (): string =>
  'No storage records match the current filters.';
