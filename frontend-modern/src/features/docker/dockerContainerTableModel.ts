import type { JSX } from 'solid-js';

import type { WorkloadTableLayoutMode } from '@/components/Workloads/guestRowModel';
import type { PlatformTableColumnKind } from '@/features/platformPage/columnAlignment';
import {
  getPlatformTableWeightedColumnWidthStyle,
  PLATFORM_TABLE_NARROW_IDENTITY_WIDTH_PERCENT,
  PLATFORM_TABLE_PHONE_IDENTITY_WIDTH_PERCENT,
} from '@/features/platformPage/sharedPlatformPage';

export type DockerContainerTableColumnId =
  | 'container'
  | 'host'
  | 'runtime'
  | 'image'
  | 'state'
  | 'cpu'
  | 'memory'
  | 'restarts'
  | 'uptime'
  | 'ports'
  | 'networks'
  | 'mounts'
  | 'updates'
  | 'actions';

export type DockerContainerTableColumn = {
  id: DockerContainerTableColumnId;
  label: string;
  compactLabel?: string;
  kind: PlatformTableColumnKind;
};

// Columns a user can sort by. The multi-value summary columns (ports,
// networks, mounts) and the actions column carry no scalar to order on.
export const DOCKER_CONTAINER_SORTABLE_COLUMN_IDS = [
  'container',
  'host',
  'runtime',
  'image',
  'state',
  'cpu',
  'memory',
  'restarts',
  'uptime',
  'updates',
] as const satisfies readonly DockerContainerTableColumnId[];

export type DockerContainerSortKey = (typeof DOCKER_CONTAINER_SORTABLE_COLUMN_IDS)[number];

export const getDockerContainerSortKey = (
  columnId: DockerContainerTableColumnId,
): DockerContainerSortKey | undefined =>
  (DOCKER_CONTAINER_SORTABLE_COLUMN_IDS as readonly string[]).includes(columnId)
    ? (columnId as DockerContainerSortKey)
    : undefined;

const DOCKER_CONTAINER_TABLE_LAYOUT_ORDER: Record<WorkloadTableLayoutMode, number> = {
  narrow: 0,
  phone: 1,
  mobile: 2,
  tablet: 3,
  compact: 4,
  wide: 5,
};

const DOCKER_CONTAINER_COLUMN_MIN_LAYOUT: Record<
  DockerContainerTableColumnId,
  WorkloadTableLayoutMode
> = {
  container: 'narrow',
  state: 'narrow',
  cpu: 'narrow',
  memory: 'narrow',
  // Below a 720px table the Restarts header cannot fit beside State, the
  // metric bars and the Update control without clipping one of them; the
  // count stays in the row expansion there and returns at tablet width.
  restarts: 'tablet',
  // Uptime is the current run, so a short one shows a container that just
  // restarted. Its header needs about 57px, which a tablet row cannot spare
  // beside State, Restarts and the Update control; the drawer keeps it there.
  uptime: 'compact',
  updates: 'narrow',
  actions: 'mobile',
  host: 'tablet',
  image: 'compact',
  runtime: 'compact',
  ports: 'compact',
  networks: 'wide',
  mounts: 'wide',
};

const DOCKER_CONTAINER_COLUMNS: DockerContainerTableColumn[] = [
  { id: 'container', label: 'Container', kind: 'name' },
  { id: 'host', label: 'Host', kind: 'text' },
  { id: 'runtime', label: 'Engine', kind: 'text' },
  { id: 'image', label: 'Image', kind: 'text' },
  { id: 'state', label: 'State', kind: 'text' },
  { id: 'cpu', label: 'CPU', kind: 'metric-bar' },
  { id: 'memory', label: 'Memory', kind: 'metric-bar' },
  { id: 'restarts', label: 'Restarts', kind: 'numeric-value' },
  { id: 'uptime', label: 'Uptime', kind: 'numeric-value' },
  { id: 'ports', label: 'Ports', kind: 'text' },
  { id: 'networks', label: 'Networks', kind: 'text' },
  { id: 'mounts', label: 'Mounts', kind: 'text' },
  { id: 'updates', label: 'Updates', compactLabel: 'Update', kind: 'badge' },
  { id: 'actions', label: 'Actions', kind: 'badge' },
];

// The Updates column holds the update control or its status badge, which
// must not clip: Update and Current are about 74px, and Check failed,
// Updating... and Completed up to 100px. Every layout gives the column room
// for the 74px states at its narrowest table width, whichever optional
// columns are showing. The 100px states fit in most layouts too, but can
// still clip at a layout's narrow end when several optional columns show.
// The room comes only from
// the CPU bar, which never shows a sublabel, and from the collapsed actions
// menu, so every other column keeps its width. Below the 34rem phone
// container the badge wraps instead (index.css). Wide rows expand the
// lifecycle controls to about 92px, so actions gains a little there.
// State names a problem in words ("Exited (139)", "Restarting", "Unhealthy"),
// about 77px on a phone and 86px on desktop with padding; it takes that room
// from the CPU and memory bars, ports and networks, which truncate anyway.
const DOCKER_CONTAINER_DESKTOP_WIDTHS: Record<DockerContainerTableColumnId, number> = {
  container: 16,
  host: 8,
  runtime: 7,
  image: 16,
  state: 7.5,
  cpu: 6,
  memory: 7,
  restarts: 6,
  uptime: 5,
  ports: 10,
  networks: 8,
  mounts: 5.5,
  updates: 9,
  actions: 9,
};

const DOCKER_CONTAINER_RESPONSIVE_WIDTHS: Record<
  Exclude<WorkloadTableLayoutMode, 'wide'>,
  Partial<Record<DockerContainerTableColumnId, number>>
> = {
  narrow: {
    container: 40,
    state: 15,
    cpu: 15,
    memory: 17,
    updates: 13,
  },
  phone: {
    container: 32,
    state: 18,
    cpu: 13,
    memory: 13,
    updates: 16,
  },
  mobile: {
    container: 30,
    state: 14.5,
    cpu: 10,
    memory: 13.5,
    updates: 16.25,
    actions: 7.75,
  },
  tablet: {
    container: 27,
    host: 15,
    state: 14,
    cpu: 9.5,
    memory: 12,
    restarts: 10,
    updates: 14.5,
    actions: 6,
  },
  // Compact is what a 1280-1536px laptop window gets. Host, Engine and the
  // Update control are short values that were clipped ("docker 2…") while the
  // percentage-only CPU and memory bars had room to spare.
  compact: {
    container: 18,
    host: 12,
    runtime: 9.5,
    image: 16.5,
    state: 10,
    cpu: 6.5,
    memory: 8,
    restarts: 8.5,
    uptime: 7,
    ports: 8,
    updates: 12.5,
    actions: 6,
  },
};

// Whether a layout has room for a column at all (before row-set gates such
// as "only when some container restarted").
export const isDockerContainerColumnInLayout = (
  columnId: DockerContainerTableColumnId,
  layoutMode: WorkloadTableLayoutMode,
): boolean =>
  DOCKER_CONTAINER_TABLE_LAYOUT_ORDER[DOCKER_CONTAINER_COLUMN_MIN_LAYOUT[columnId]] <=
  DOCKER_CONTAINER_TABLE_LAYOUT_ORDER[layoutMode];

export const getDockerContainerVisibleColumnsForLayout = (
  layoutMode: WorkloadTableLayoutMode,
  includeRuntime: boolean,
  includeRestarts: boolean,
  includeState: boolean,
  options: { groupedByHost?: boolean; singleHost?: boolean; includeUptime?: boolean } = {},
): DockerContainerTableColumn[] => {
  const layoutRank = DOCKER_CONTAINER_TABLE_LAYOUT_ORDER[layoutMode];
  return DOCKER_CONTAINER_COLUMNS.filter((column) => {
    // Grouped by host, every row in a group shares its host and engine; the
    // group header carries both instead of repeating them on each row.
    if (options.groupedByHost && (column.id === 'host' || column.id === 'runtime')) return false;
    // With one host in view the Host column would repeat its name on every row.
    if (options.singleHost && column.id === 'host') return false;
    if (column.id === 'runtime' && !includeRuntime) return false;
    if (column.id === 'restarts' && !includeRestarts) return false;
    if (column.id === 'uptime' && options.includeUptime === false) return false;
    // Ultra-narrow rows still need five stable scan fields. Keep the explicit
    // state label there even when every current row is running; wider layouts
    // may continue to remove the otherwise repetitive column.
    if (column.id === 'state' && !includeState && layoutMode !== 'narrow') return false;
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
  return getPlatformTableWeightedColumnWidthStyle(
    columnId,
    weights,
    visibleColumnIds,
    layoutMode === 'narrow'
      ? { columnId: 'container', widthPercent: PLATFORM_TABLE_NARROW_IDENTITY_WIDTH_PERCENT }
      : layoutMode === 'phone' || layoutMode === 'mobile'
        ? { columnId: 'container', widthPercent: PLATFORM_TABLE_PHONE_IDENTITY_WIDTH_PERCENT }
        : undefined,
  );
};

// The Docker container table already has a row detail drawer for forensic
// fields. The overview should fit its page at every layout and reveal columns
// by priority instead of forcing a desktop scrollbar.
export const getDockerContainerTableMinWidthClass = (): 'min-w-full' => 'min-w-full';
