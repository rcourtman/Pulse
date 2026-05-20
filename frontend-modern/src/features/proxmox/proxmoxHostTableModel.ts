import type { JSX } from 'solid-js';

import type { WorkloadTableLayoutMode } from '@/components/Workloads/guestRowModel';

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
  align?: 'left' | 'right' | 'center';
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

export const PROXMOX_HOST_TABLE_COLUMNS: ProxmoxHostTableColumn[] = [
  { id: 'node', label: 'Node' },
  { id: 'version', label: 'Version' },
  { id: 'uptime', label: 'Uptime', align: 'right' },
  { id: 'cpu', label: 'CPU' },
  { id: 'memory', label: 'Memory' },
  { id: 'disk', label: 'Disk' },
  { id: 'temp', label: 'Temp', align: 'right' },
  { id: 'vms', label: 'VMs', align: 'center' },
  { id: 'cts', label: 'CTs', align: 'center' },
  { id: 'cluster', label: 'Cluster' },
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
): 'min-w-full' | 'min-w-[1080px]' =>
  layoutMode === 'mobile' || layoutMode === 'tablet' ? 'min-w-full' : 'min-w-[1080px]';
