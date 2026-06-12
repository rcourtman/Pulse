import type { JSX } from 'solid-js';

import type { WorkloadTableLayoutMode } from '@/components/Workloads/guestRowModel';
import type { PlatformTableColumnKind } from '@/features/platformPage/columnAlignment';

export type DockerContainerTableColumnId =
  | 'container'
  | 'host'
  | 'runtime'
  | 'image'
  | 'state'
  | 'cpu'
  | 'memory'
  | 'restarts'
  | 'ports'
  | 'networks'
  | 'mounts'
  | 'updates';

export type DockerContainerTableColumn = {
  id: DockerContainerTableColumnId;
  label: string;
  kind: PlatformTableColumnKind;
};

const DOCKER_CONTAINER_TABLE_LAYOUT_ORDER: Record<WorkloadTableLayoutMode, number> = {
  mobile: 0,
  tablet: 1,
  compact: 2,
  wide: 3,
};

const DOCKER_CONTAINER_COLUMN_MIN_LAYOUT: Record<
  DockerContainerTableColumnId,
  WorkloadTableLayoutMode
> = {
  container: 'mobile',
  state: 'mobile',
  cpu: 'mobile',
  memory: 'mobile',
  updates: 'mobile',
  host: 'tablet',
  restarts: 'tablet',
  image: 'compact',
  runtime: 'compact',
  ports: 'compact',
  networks: 'wide',
  mounts: 'wide',
};

const DOCKER_CONTAINER_COLUMNS: DockerContainerTableColumn[] = [
  { id: 'container', label: 'Container', kind: 'name' },
  { id: 'host', label: 'Host', kind: 'text' },
  { id: 'runtime', label: 'Runtime', kind: 'text' },
  { id: 'image', label: 'Image', kind: 'text' },
  { id: 'state', label: 'State', kind: 'text' },
  { id: 'cpu', label: 'CPU', kind: 'metric-bar' },
  { id: 'memory', label: 'Memory', kind: 'metric-bar' },
  { id: 'restarts', label: 'Restarts', kind: 'numeric-value' },
  { id: 'ports', label: 'Ports', kind: 'text' },
  { id: 'networks', label: 'Networks', kind: 'text' },
  { id: 'mounts', label: 'Mounts', kind: 'text' },
  { id: 'updates', label: 'Updates', kind: 'badge' },
];

const DOCKER_CONTAINER_DESKTOP_WIDTHS: Record<DockerContainerTableColumnId, number> = {
  container: 16,
  host: 8,
  runtime: 7,
  image: 16,
  state: 6,
  cpu: 9,
  memory: 10,
  restarts: 6,
  ports: 10,
  networks: 8,
  mounts: 9,
  updates: 7,
};

const DOCKER_CONTAINER_RESPONSIVE_WIDTHS: Record<
  Exclude<WorkloadTableLayoutMode, 'wide'>,
  Partial<Record<DockerContainerTableColumnId, number>>
> = {
  mobile: {
    container: 32,
    state: 14,
    cpu: 18,
    memory: 22,
    updates: 14,
  },
  tablet: {
    container: 28,
    host: 16,
    state: 10,
    cpu: 15,
    memory: 18,
    restarts: 6,
    updates: 7,
  },
  compact: {
    container: 18,
    host: 10,
    runtime: 8,
    image: 20,
    state: 7,
    cpu: 11,
    memory: 12,
    restarts: 7,
    ports: 12,
    updates: 8,
  },
};

const formatPercentage = (value: number): string => `${Number(value.toFixed(4))}%`;

export const getDockerContainerVisibleColumnsForLayout = (
  layoutMode: WorkloadTableLayoutMode,
  includeRuntime: boolean,
  includeRestarts: boolean,
  includeState: boolean,
): DockerContainerTableColumn[] => {
  const layoutRank = DOCKER_CONTAINER_TABLE_LAYOUT_ORDER[layoutMode];
  return DOCKER_CONTAINER_COLUMNS.filter((column) => {
    if (column.id === 'runtime' && !includeRuntime) return false;
    if (column.id === 'restarts' && !includeRestarts) return false;
    if (column.id === 'state' && !includeState) return false;
    return (
      DOCKER_CONTAINER_TABLE_LAYOUT_ORDER[DOCKER_CONTAINER_COLUMN_MIN_LAYOUT[column.id]] <=
      layoutRank
    );
  });
};

export const getDockerContainerColumnWidthStyle = (
  columnId: DockerContainerTableColumnId,
  layoutMode: WorkloadTableLayoutMode,
  visibleColumnIds: readonly DockerContainerTableColumnId[],
): JSX.CSSProperties => {
  const weights =
    layoutMode === 'wide'
      ? DOCKER_CONTAINER_DESKTOP_WIDTHS
      : DOCKER_CONTAINER_RESPONSIVE_WIDTHS[layoutMode];
  const columnWeight = weights[columnId] ?? 0;
  const totalWeight = visibleColumnIds.reduce((total, id) => total + (weights[id] ?? 0), 0);
  const width = totalWeight > 0 ? (columnWeight / totalWeight) * 100 : 0;

  return { width: formatPercentage(width) };
};

// The Docker container table already has a row detail drawer for forensic
// fields. The overview should fit its page at every layout and reveal columns
// by priority instead of forcing a desktop scrollbar.
export const getDockerContainerTableMinWidthClass = (): 'min-w-full' => 'min-w-full';
