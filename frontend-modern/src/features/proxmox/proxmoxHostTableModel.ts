import type { JSX } from 'solid-js';

import type { WorkloadTableLayoutMode } from '@/components/Workloads/guestRowModel';
import type { PlatformTableColumnKind } from '@/features/platformPage/columnAlignment';

export type ProxmoxHostTableColumnId =
  | 'node'
  | 'version'
  | 'uptime'
  | 'cpu'
  | 'memory'
  | 'disk'
  | 'temp'
  | 'vms'
  | 'cts'
  | 'cluster';

export type ProxmoxHostTableColumn = {
  id: ProxmoxHostTableColumnId;
  label: string;
  kind: PlatformTableColumnKind;
};

const HOST_TABLE_LAYOUT_ORDER: Record<WorkloadTableLayoutMode, number> = {
  mobile: 0,
  tablet: 1,
  compact: 2,
  wide: 3,
};

const HOST_COLUMN_MIN_LAYOUT: Record<ProxmoxHostTableColumnId, WorkloadTableLayoutMode> = {
  node: 'mobile',
  cpu: 'mobile',
  memory: 'mobile',
  disk: 'mobile',
  temp: 'tablet',
  vms: 'tablet',
  cts: 'tablet',
  version: 'compact',
  uptime: 'compact',
  cluster: 'compact',
};

// CPU, memory, and disk render the same kind of usage bar, so they share
// weights (matches the workloads table below, where the three metric
// columns are all 140px). Node gets a little extra headroom for longer
// cluster-style names like "Disaster Recovery A"; the short-content
// columns (version pill, uptime, temp gauge, vms/cts badges, cluster
// pill) take only what they need.
const HOST_COLUMN_DESKTOP_WIDTHS: Record<ProxmoxHostTableColumnId, number> = {
  node: 17,
  version: 6,
  uptime: 6,
  cpu: 13,
  memory: 13,
  disk: 13,
  temp: 5,
  vms: 4,
  cts: 4,
  cluster: 10,
};

const HOST_COLUMN_RESPONSIVE_WEIGHTS: Record<
  Exclude<WorkloadTableLayoutMode, 'wide'>,
  Partial<Record<ProxmoxHostTableColumnId, number>>
> = {
  mobile: {
    node: 40,
    cpu: 20,
    memory: 20,
    disk: 20,
  },
  tablet: {
    node: 28,
    cpu: 18,
    memory: 18,
    disk: 18,
    temp: 6,
    vms: 6,
    cts: 6,
  },
  compact: HOST_COLUMN_DESKTOP_WIDTHS,
};

const formatPercentage = (value: number): string => `${Number(value.toFixed(4))}%`;

// Column order follows the canonical recommended ordering documented in
// columnAlignment.ts: identity → context → bars (CPU/Memory/Disk
// contiguous) → diagnostic (Temp) → time (Uptime) → inventory counts
// → external owner reference at end.
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
  const columnWeight = weights[columnId] ?? 0;
  const totalWeight = visibleColumnIds.reduce((total, id) => total + (weights[id] ?? 0), 0);
  const width = totalWeight > 0 ? (columnWeight / totalWeight) * 100 : 0;

  return { width: formatPercentage(width) };
};

export const getProxmoxHostTableMinWidthClass = (
  layoutMode: WorkloadTableLayoutMode,
): 'min-w-full' | 'min-w-[1240px]' =>
  layoutMode === 'mobile' || layoutMode === 'tablet' ? 'min-w-full' : 'min-w-[1240px]';
