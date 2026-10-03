import type { JSX } from 'solid-js';
import type { StorageSortKey } from './storageModelCore';
import type { PlatformTableColumnKind } from '@/features/platformPage/columnAlignment';
import { getPlatformTableWeightedColumnWidthStyle } from '@/features/platformPage/sharedPlatformPage';

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
  kind: PlatformTableColumnKind;
};

export type StoragePoolTableLayoutMode = 'narrow' | 'compact' | 'operational' | 'full';

// The compact/operational boundary matches the shared phone projection of the
// table container (`@container (max-width: 33.999rem)` = 544px), so the
// columns and the in-cell labels always change projection together.
export const getStoragePoolTableLayoutModeForContainer = (
  containerWidth: number,
): StoragePoolTableLayoutMode => {
  if (containerWidth >= 1_040) return 'full';
  if (containerWidth >= 544) return 'operational';
  if (containerWidth > 0 && containerWidth < 360) return 'narrow';
  return 'compact';
};

const STORAGE_POOL_VISIBLE_COLUMNS: Record<
  StoragePoolTableLayoutMode,
  readonly StoragePoolTableColumnId[]
> = {
  // Phones prioritize the four questions needed for an operational scan:
  // what, health, where, and capacity. Type and protection remain available
  // in the expanded detail instead of being squeezed into unusable tracks.
  narrow: ['name', 'state', 'host', 'usage'],
  compact: ['name', 'state', 'host', 'usage'],
  operational: ['name', 'state', 'host', 'protection', 'usage'],
  full: ['name', 'state', 'type', 'host', 'protection', 'usage', 'growth'],
};

// Relative weights per layout, resolved through the canonical weighted-width
// helper. On phones the usage bar carries only its percentage, so most of the
// row goes to the two identity strings (pool name and host); a state word such
// as "Degraded" must still fit whole. The shared 30% identity anchor is not
// used here because a four-column row cannot keep the host readable with it.
const STORAGE_POOL_COLUMN_WEIGHTS: Record<
  StoragePoolTableLayoutMode,
  Partial<Record<StoragePoolTableColumnId, number>>
> = {
  narrow: { name: 35, state: 21, host: 31, usage: 13 },
  compact: { name: 37, state: 18.5, host: 31.5, usage: 13 },
  // The full "59% (1.18 TB/2.00 TB)" bar label returns at 544px, so the usage
  // track must hold ~120px of text from the first operational width.
  operational: { name: 27, state: 18, host: 15, protection: 15, usage: 25 },
  full: { name: 20, state: 14, type: 10, host: 12, protection: 13, usage: 20, growth: 11 },
};

export const isStoragePoolColumnVisible = (
  layout: StoragePoolTableLayoutMode,
  columnId: StoragePoolTableColumnId,
): boolean => STORAGE_POOL_VISIBLE_COLUMNS[layout].includes(columnId);

// On phones the pool row sheds the desktop cell gutter the same way its
// headers (via the shared container query) and the physical-disk rows do:
// four cells at 12px each were 48px of a 361px row, and the measured values
// fit the tracks with only a pixel or two to spare.
export const STORAGE_POOL_PHONE_CELL_PADDING_CLASS = '!px-1';

export const getStoragePoolCellPaddingClass = (layout: StoragePoolTableLayoutMode): string =>
  layout === 'narrow' || layout === 'compact' ? STORAGE_POOL_PHONE_CELL_PADDING_CLASS : '';

export const getStoragePoolColumnWidthStyle = (
  layout: StoragePoolTableLayoutMode,
  columnId: StoragePoolTableColumnId,
): JSX.CSSProperties =>
  getPlatformTableWeightedColumnWidthStyle(
    columnId,
    STORAGE_POOL_COLUMN_WEIGHTS[layout],
    STORAGE_POOL_VISIBLE_COLUMNS[layout],
  );

const STORAGE_POOL_TABLE_HEADER_CLASS =
  'overflow-hidden text-ellipsis whitespace-nowrap text-[10px] lg:text-xs uppercase tracking-wider';

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
    kind: 'name',
  },
  {
    id: 'state',
    label: 'State',
    compactLabel: 'State',
    sortKey: 'state',
    className: STORAGE_POOL_TABLE_HEADER_CLASS,
    colClassName: '',
    kind: 'text',
  },
  {
    id: 'type',
    label: 'Type',
    compactLabel: 'Type',
    sortKey: 'type',
    className: STORAGE_POOL_TABLE_HEADER_CLASS,
    colClassName: '',
    kind: 'text',
  },
  {
    id: 'host',
    label: 'Host',
    compactLabel: 'Host',
    sortKey: 'host',
    className: STORAGE_POOL_TABLE_HEADER_CLASS,
    colClassName: '',
    kind: 'text',
  },
  {
    id: 'protection',
    label: 'Protection',
    compactLabel: 'Prot',
    sortKey: 'protection',
    className: STORAGE_POOL_TABLE_HEADER_CLASS,
    colClassName: '',
    kind: 'badge',
  },
  {
    id: 'usage',
    label: 'Usage',
    compactLabel: 'Used',
    sortKey: 'usage',
    className: STORAGE_POOL_TABLE_HEADER_CLASS,
    colClassName: '',
    kind: 'metric-bar',
  },
  {
    id: 'growth',
    label: growthColumnLabel,
    compactLabel: growthColumnLabel.replace(/^Growth\s*\((.+)\)$/i, '$1'),
    sortKey: 'growth',
    className: STORAGE_POOL_TABLE_HEADER_CLASS,
    colClassName: '',
    kind: 'numeric-value',
  },
];
export const STORAGE_CONTENT_CARD_BODY_CLASS = 'p-2';

export const STORAGE_POOLS_EMPTY_STATE_CLASS = 'p-6 text-sm text-muted';
export const STORAGE_POOLS_LOADING_STATE_CLASS = 'p-6 text-sm text-muted';
export const STORAGE_POOLS_TABLE_CLASS = 'platform-table w-full table-fixed text-xs';
export const STORAGE_POOLS_HEADER_ROW_CLASS = 'bg-surface-alt text-muted border-b border-border';
export const STORAGE_POOLS_BODY_CLASS = 'divide-y divide-border';

export const getStorageTableHeading = (view: 'pools' | 'disks'): string =>
  view === 'pools' ? 'Storage' : 'Physical Disks';

export const getStorageLoadingMessage = (): string => 'Loading storage resources...';

export const getStorageEmptyStateMessage = (): string =>
  'No storage records match the current filters.';
