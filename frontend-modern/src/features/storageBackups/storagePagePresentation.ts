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
  className: string;
};

export const STORAGE_VIEW_OPTIONS: readonly StorageViewOption[] = [
  { value: 'pools', label: 'Pools' },
  { value: 'disks', label: 'Physical Disks' },
];

export const STORAGE_POOL_TABLE_COLUMNS: readonly StoragePoolTableColumn[] = [
  {
    label: 'Storage',
    className: 'px-1.5 sm:px-2 py-0.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider',
  },
  {
    label: 'Source',
    className: 'px-1.5 sm:px-2 py-0.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider',
  },
  {
    label: 'Type',
    className:
      'hidden xl:table-cell px-1.5 sm:px-2 py-0.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider',
  },
  {
    label: 'Host',
    className: 'px-1.5 sm:px-2 py-0.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider',
  },
  {
    label: 'Protection',
    className:
      'px-1.5 sm:px-2 py-0.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider md:min-w-[180px]',
  },
  {
    label: 'Usage',
    className:
      'px-1.5 sm:px-2 py-0.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider md:min-w-[190px] xl:min-w-[220px]',
  },
  {
    label: 'Impact',
    className:
      'px-1.5 sm:px-2 py-0.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider hidden lg:table-cell',
  },
  {
    label: 'Primary Issue',
    className: 'px-1.5 sm:px-2 py-0.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider',
  },
  {
    label: '',
    className: 'px-1.5 sm:px-2 py-0.5 w-10',
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
export const STORAGE_POOLS_SCROLL_WRAP_CLASS = 'overflow-x-auto';
export const STORAGE_POOLS_TABLE_CLASS = 'w-full text-xs';
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

export const getStoragePageBannerActionLabel = (
  kind: StoragePageBannerKind,
): string | null => {
  if (kind === 'reconnecting') return 'Retry now';
  if (kind === 'disconnected') return 'Reconnect';
  return null;
};

export const getStorageTableHeading = (view: 'pools' | 'disks'): string =>
  view === 'pools' ? 'Storage Pools' : 'Physical Disks';

export const getStorageLoadingMessage = (): string => 'Loading storage resources...';

export const getStorageEmptyStateMessage = (): string =>
  'No storage records match the current filters.';
