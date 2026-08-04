import type { StorageSortKey } from './storageModelCore';

export type StorageViewOption = {
  value: 'pools' | 'disks';
  label: string;
};

export type StoragePoolTableColumnId =
  'name' | 'state' | 'type' | 'host' | 'protection' | 'usage' | 'growth';

export type StoragePoolTableColumn = {
  id: StoragePoolTableColumnId;
  label: string;
  compactLabel: string;
  sortKey: StorageSortKey;
  className: string;
  colClassName: string;
};

export type StoragePoolTableLayoutMode = 'compact' | 'operational' | 'full';

export const getStoragePoolTableLayoutModeForContainer = (
  containerWidth: number,
): StoragePoolTableLayoutMode => {
  if (containerWidth >= 1_040) return 'full';
  if (containerWidth >= 560) return 'operational';
  return 'compact';
};

const STORAGE_POOL_VISIBLE_COLUMNS: Record<
  StoragePoolTableLayoutMode,
  readonly StoragePoolTableColumnId[]
> = {
  compact: ['name', 'state', 'usage'],
  operational: ['name', 'state', 'host', 'protection', 'usage'],
  full: ['name', 'state', 'type', 'host', 'protection', 'usage', 'growth'],
};

const STORAGE_POOL_COLUMN_WIDTHS: Record<
  StoragePoolTableLayoutMode,
  Partial<Record<StoragePoolTableColumnId, number>>
> = {
  compact: { name: 39, state: 28, usage: 33 },
  operational: { name: 29, state: 20, host: 15, protection: 15, usage: 21 },
  full: { name: 20, state: 14, type: 10, host: 12, protection: 13, usage: 20, growth: 11 },
};

export const isStoragePoolColumnVisible = (
  layout: StoragePoolTableLayoutMode,
  columnId: StoragePoolTableColumnId,
): boolean => STORAGE_POOL_VISIBLE_COLUMNS[layout].includes(columnId);

export const getStoragePoolColumnWidthPercent = (
  layout: StoragePoolTableLayoutMode,
  columnId: StoragePoolTableColumnId,
): number => STORAGE_POOL_COLUMN_WIDTHS[layout][columnId] ?? 0;

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
    id: 'name',
    label: 'Storage',
    compactLabel: 'Storage',
    sortKey: 'name',
    className: STORAGE_POOL_TABLE_HEADER_CLASS,
    colClassName: '',
  },
  {
    id: 'state',
    label: 'State',
    compactLabel: 'State',
    sortKey: 'state',
    className: STORAGE_POOL_TABLE_HEADER_CLASS,
    colClassName: '',
  },
  {
    id: 'type',
    label: 'Type',
    compactLabel: 'Type',
    sortKey: 'type',
    className: STORAGE_POOL_TABLE_HEADER_CLASS,
    colClassName: '',
  },
  {
    id: 'host',
    label: 'Host',
    compactLabel: 'Host',
    sortKey: 'host',
    className: STORAGE_POOL_TABLE_HEADER_CLASS,
    colClassName: '',
  },
  {
    id: 'protection',
    label: 'Protection',
    compactLabel: 'Prot',
    sortKey: 'protection',
    className: STORAGE_POOL_TABLE_HEADER_CLASS,
    colClassName: '',
  },
  {
    id: 'usage',
    label: 'Usage',
    compactLabel: 'Used',
    sortKey: 'usage',
    className: STORAGE_POOL_TABLE_HEADER_CLASS,
    colClassName: '',
  },
  {
    id: 'growth',
    label: growthColumnLabel,
    compactLabel: growthColumnLabel.replace(/^Growth\s*\((.+)\)$/i, '$1'),
    sortKey: 'growth',
    className: STORAGE_POOL_TABLE_HEADER_CLASS,
    colClassName: '',
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
