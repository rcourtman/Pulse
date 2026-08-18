import type { JSX } from 'solid-js';

import type { WorkloadTableLayoutMode } from '@/components/Workloads/guestRowModel';
import type { PlatformTableColumnKind } from '@/features/platformPage/columnAlignment';
import {
  getPlatformTableWeightedColumnWidthStyle,
  PLATFORM_TABLE_PHONE_IDENTITY_WIDTH_PERCENT,
} from '@/features/platformPage/sharedPlatformPage';

export type ProxmoxHostTableColumnId =
  'node' | 'version' | 'uptime' | 'cpu' | 'memory' | 'disk' | 'temp' | 'vms' | 'cts' | 'cluster';

export type ProxmoxHostTableColumn = {
  id: ProxmoxHostTableColumnId;
  label: string;
  kind: PlatformTableColumnKind;
};

const HOST_TABLE_LAYOUT_ORDER: Record<WorkloadTableLayoutMode, number> = {
  phone: 0,
  mobile: 1,
  tablet: 2,
  compact: 3,
  wide: 4,
};

const HOST_COLUMN_MIN_LAYOUT: Record<ProxmoxHostTableColumnId, WorkloadTableLayoutMode> = {
  node: 'phone',
  cpu: 'phone',
  memory: 'phone',
  disk: 'phone',
  temp: 'phone',
  uptime: 'phone',
  cluster: 'tablet',
  vms: 'compact',
  cts: 'compact',
  version: 'wide',
};

// CPU, memory, and disk render the same kind of usage bar, so they share
// weights (matches the workloads table below, where the three metric
// columns are all 140px). Node gets a little extra headroom for longer
// cluster-style names like "Disaster Recovery A"; the short-content
// columns (version pill, uptime, temp gauge, vms/cts badges, cluster
// pill) take only what they need.
const HOST_COLUMN_DESKTOP_WIDTHS: Record<ProxmoxHostTableColumnId, number> = {
  node: 18,
  version: 7,
  uptime: 7,
  cpu: 13,
  memory: 13,
  disk: 13,
  temp: 6,
  vms: 5,
  cts: 5,
  cluster: 13,
};

const HOST_COLUMN_RESPONSIVE_WEIGHTS: Record<
  Exclude<WorkloadTableLayoutMode, 'wide'>,
  Partial<Record<ProxmoxHostTableColumnId, number>>
> = {
  phone: {
    node: 32,
    cpu: 13,
    memory: 13,
    disk: 13,
    temp: 15,
    uptime: 14,
  },
  mobile: {
    node: 32,
    cpu: 13,
    memory: 13,
    disk: 13,
    temp: 15,
    uptime: 14,
  },
  tablet: {
    node: 22,
    cpu: 15,
    memory: 15,
    disk: 15,
    temp: 8,
    uptime: 11,
    cluster: 14,
  },
  compact: {
    node: 20,
    cpu: 15,
    memory: 15,
    disk: 15,
    temp: 7,
    uptime: 8,
    vms: 5,
    cts: 5,
    cluster: 10,
  },
};

export const getProxmoxHostTableLayoutModeForContainer = (
  width: number,
): WorkloadTableLayoutMode => {
  if (Number.isFinite(width) && width < 440) return 'phone';
  if (!Number.isFinite(width) || width < 800) return 'mobile';
  if (width < 1040) return 'tablet';
  if (width < 1320) return 'compact';
  return 'wide';
};

// Column order follows the canonical recommended ordering documented in
// columnAlignment.ts: identity → context → bars (CPU/Memory/Disk
// contiguous) → diagnostic (Temp) → time (Uptime) → inventory counts
// → source context at end. Web-interface launches live beside the inert node
// name so every runtime table uses the canonical shared adjacent control
// instead of adding a separate action column.
export const PROXMOX_HOST_TABLE_COLUMNS: ProxmoxHostTableColumn[] = [
  { id: 'node', label: 'Node', kind: 'name' },
  { id: 'version', label: 'Version', kind: 'text' },
  { id: 'cpu', label: 'CPU', kind: 'metric-bar' },
  { id: 'memory', label: 'Memory', kind: 'metric-bar' },
  { id: 'disk', label: 'Disk', kind: 'metric-bar' },
  { id: 'temp', label: 'Temp', kind: 'numeric-value' },
  { id: 'uptime', label: 'Uptime', kind: 'numeric-value' },
  { id: 'vms', label: 'VMs', kind: 'numeric-value' },
  { id: 'cts', label: 'CTs', kind: 'numeric-value' },
  { id: 'cluster', label: 'Cluster', kind: 'text' },
];

export const getProxmoxHostVisibleColumnsForLayout = (
  layoutMode: WorkloadTableLayoutMode,
): ProxmoxHostTableColumn[] => {
  const layoutRank = HOST_TABLE_LAYOUT_ORDER[layoutMode];
  return PROXMOX_HOST_TABLE_COLUMNS.filter(
    (column) => HOST_TABLE_LAYOUT_ORDER[HOST_COLUMN_MIN_LAYOUT[column.id]] <= layoutRank,
  );
};

export const getProxmoxHostColumnWidthStyle = (
  columnId: ProxmoxHostTableColumnId,
  layoutMode: WorkloadTableLayoutMode,
  visibleColumnIds: readonly ProxmoxHostTableColumnId[],
): JSX.CSSProperties => {
  const weights =
    layoutMode === 'wide' ? HOST_COLUMN_DESKTOP_WIDTHS : HOST_COLUMN_RESPONSIVE_WEIGHTS[layoutMode];
  return getPlatformTableWeightedColumnWidthStyle(
    columnId,
    weights,
    visibleColumnIds,
    layoutMode === 'phone' || layoutMode === 'mobile'
      ? { columnId: 'node', widthPercent: PLATFORM_TABLE_PHONE_IDENTITY_WIDTH_PERCENT }
      : undefined,
  );
};

// The compact identity-plus-metrics layout is deliberately percentage based,
// so it can use the actual phone-width container without a hidden horizontal
// rail. Wider modes retain progressively larger floors so their extra context
// does not collapse into unreadable labels.
export const getProxmoxHostTableMinWidthClass = (
  layoutMode: WorkloadTableLayoutMode,
): 'min-w-[0px]' | 'min-w-[50rem]' | 'min-w-[64rem]' | 'min-w-[1240px]' => {
  if (layoutMode === 'phone' || layoutMode === 'mobile') return 'min-w-[0px]';
  if (layoutMode === 'tablet') return 'min-w-[50rem]';
  if (layoutMode === 'compact') return 'min-w-[64rem]';
  return 'min-w-[1240px]';
};
