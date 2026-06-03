import type { StorageSortKey } from './storageModelCore';

export type StorageViewOption = {
  value: 'pools' | 'disks';
  label: string;
};

export type StoragePoolTableColumn = {
  label: string;
  compactLabel: string;
  sortKey: StorageSortKey;
  className: string;
  colClassName: string;
};

const STORAGE_POOL_TABLE_HEADER_CLASS =
  'overflow-hidden text-ellipsis whitespace-nowrap px-1 sm:px-1.5 lg:px-2 py-0.5 text-left text-[10px] sm:text-[11px] lg:text-xs font-medium uppercase tracking-wider';

export const STORAGE_VIEW_OPTIONS: readonly StorageViewOption[] = [
  { value: 'pools', label: 'Storage' },
  { value: 'disks', label: 'Physical Disks' },
];

export const getStoragePoolTableColumns = (
  growthColumnLabel: string,
): readonly StoragePoolTableColumn[] => [
  {
    label: 'Storage',
    compactLabel: 'Storage',
    sortKey: 'name',
    className: `${STORAGE_POOL_TABLE_HEADER_CLASS} w-[34%] sm:w-[26%] md:w-[23%] xl:w-[18%]`,
    colClassName: 'w-[34%] sm:w-[26%] md:w-[23%] xl:w-[18%]',
  },
  {
    label: 'State',
    compactLabel: 'State',
    sortKey: 'state',
    className: `${STORAGE_POOL_TABLE_HEADER_CLASS} w-[24%] sm:w-[20%] md:w-[17%] xl:w-[14%]`,
    colClassName: 'w-[24%] sm:w-[20%] md:w-[17%] xl:w-[14%]',
  },
  {
    label: 'Source',
    compactLabel: 'Src',
    sortKey: 'source',
    className: `${STORAGE_POOL_TABLE_HEADER_CLASS} w-[13%] sm:w-[10%] md:w-[8%]`,
    colClassName: 'w-[13%] sm:w-[10%] md:w-[8%]',
  },
  {
    label: 'Type',
    compactLabel: 'Type',
    sortKey: 'type',
    className: `${STORAGE_POOL_TABLE_HEADER_CLASS} hidden xl:table-cell xl:w-[8%]`,
    colClassName: 'hidden xl:table-column xl:w-[8%]',
  },
  {
    label: 'Host',
    compactLabel: 'Host',
    sortKey: 'host',
    className: `${STORAGE_POOL_TABLE_HEADER_CLASS} hidden sm:table-cell sm:w-[13%] md:w-[12%] xl:w-[10%]`,
    colClassName: 'hidden sm:table-column sm:w-[13%] md:w-[12%] xl:w-[10%]',
  },
  {
    label: 'Protection',
    compactLabel: 'Prot',
    sortKey: 'protection',
    className: `${STORAGE_POOL_TABLE_HEADER_CLASS} hidden sm:table-cell sm:w-[15%] md:w-[15%] xl:w-[13%]`,
    colClassName: 'hidden sm:table-column sm:w-[15%] md:w-[15%] xl:w-[13%]',
  },
  {
    label: 'Usage',
    compactLabel: 'Used',
    sortKey: 'usage',
    className: `${STORAGE_POOL_TABLE_HEADER_CLASS} w-[29%] sm:w-[16%] md:w-[25%] xl:w-[18%]`,
    colClassName: 'w-[29%] sm:w-[16%] md:w-[25%] xl:w-[18%]',
  },
  {
    label: growthColumnLabel,
    compactLabel: growthColumnLabel.replace(/^Growth\s*\((.+)\)$/i, '$1'),
    sortKey: 'growth',
    className: `${STORAGE_POOL_TABLE_HEADER_CLASS} hidden xl:table-cell xl:w-[11%]`,
    colClassName: 'hidden xl:table-column xl:w-[11%]',
  },
];
export const STORAGE_CONTENT_CARD_BODY_CLASS = 'p-2';

export const STORAGE_POOLS_EMPTY_STATE_CLASS = 'p-6 text-sm text-muted';
export const STORAGE_POOLS_LOADING_STATE_CLASS = 'p-6 text-sm text-muted';
export const STORAGE_POOLS_TABLE_CLASS = 'w-full table-fixed text-xs';
export const STORAGE_POOLS_HEADER_ROW_CLASS = 'bg-surface-alt text-muted border-b border-border';
export const STORAGE_POOLS_BODY_CLASS = 'divide-y divide-border';

export const getStorageTableHeading = (view: 'pools' | 'disks'): string =>
  view === 'pools' ? 'Storage' : 'Physical Disks';

export const getStorageLoadingMessage = (): string => 'Loading storage resources...';

export const getStorageEmptyStateMessage = (): string =>
  'No storage records match the current filters.';
