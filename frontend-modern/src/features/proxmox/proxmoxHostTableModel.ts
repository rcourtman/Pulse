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

const HOST_COLUMN_DESKTOP_WIDTHS: Record<ProxmoxHostTableColumnId, number> = {
  node: 15,
  version: 8,
  uptime: 7,
  cpu: 13,
  memory: 13,
  disk: 7,
  temp: 7,
  vms: 5,
  cts: 5,
  cluster: 12,
};

const HOST_COLUMN_RESPONSIVE_WEIGHTS: Record<
  Exclude<WorkloadTableLayoutMode, 'wide'>,
  Partial<Record<ProxmoxHostTableColumnId, number>>
> = {
  mobile: {
    node: 44,
    cpu: 23,
    memory: 23,
    disk: 10,
  },
  tablet: {
    node: 30,
    cpu: 19,
    memory: 19,
    disk: 11,
    temp: 8,
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
