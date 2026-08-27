import type { JSX } from 'solid-js';

import type { ColumnDef } from '@/hooks/useColumnVisibility';
import type { SummaryGroupMemberInteractionState } from '@/components/shared/summaryCardInteraction';
import {
  PLATFORM_TABLE_NARROW_IDENTITY_WIDTH_PERCENT,
  getPlatformTableWeightedColumnWidthStyle,
  PLATFORM_TABLE_PHONE_IDENTITY_WIDTH_PERCENT,
} from '@/features/platformPage/sharedPlatformPage';
import type { WorkloadGuest, WorkloadType, ViewMode } from '@/types/workloads';
import { getShortImageName } from '@/utils/format';
import { resolveWorkloadType } from '@/utils/workloads';
import { createVisibleCanonicalTypeColumn } from '@/utils/typeColumnDefinition';
import type {
  WorkloadMetricHistoryReader,
  WorkloadTableMetric,
} from './workloadMetricHistoryModel';
import type { NestedWorkloadContext } from './nestedWorkloadContext';
import type { WorkloadColumnWidths } from './workloadColumnWidths';
import type { WorkloadsMemoryDisplayBasis } from './workloadsFilterModel';

export type WorkloadTableLayoutMode = 'narrow' | 'phone' | 'mobile' | 'tablet' | 'compact' | 'wide';

export const WORKLOAD_TABLE_NARROW_LAYOUT_WIDTH = 360;
export const WORKLOAD_TABLE_PHONE_LAYOUT_WIDTH = 480;
export const WORKLOAD_TABLE_MOBILE_LAYOUT_WIDTH = 768;
export const WORKLOAD_TABLE_TABLET_LAYOUT_WIDTH = 900;
// Layout stages key off the viewport, but the table renders inside a shell
// that is ~120px narrower (nav rail + page padding). The wide column set sums
// to ~1370px, so engaging it at a 1440px viewport left the default layout
// horizontally scrolling by ~50px. The responsive column set must fit without
// relying on horizontal scroll, so wide waits until the shell can actually
// hold the full set.
export const WORKLOAD_TABLE_WIDE_LAYOUT_WIDTH = 1536;
export const WORKLOAD_TABLE_CONTAINER_NARROW_WIDTH = 360;
export const WORKLOAD_TABLE_CONTAINER_PHONE_WIDTH = 440;
export const WORKLOAD_TABLE_CONTAINER_MOBILE_WIDTH = 720;
export const WORKLOAD_TABLE_CONTAINER_TABLET_WIDTH = 900;
export const WORKLOAD_TABLE_CONTAINER_WIDE_WIDTH = 1440;

const WORKLOAD_TABLE_LAYOUT_ORDER: Record<WorkloadTableLayoutMode, number> = {
  narrow: 0,
  phone: 1,
  mobile: 2,
  tablet: 3,
  compact: 4,
  wide: 5,
};

const WORKLOAD_COLUMN_MIN_LAYOUT: Record<string, WorkloadTableLayoutMode> = {
  name: 'narrow',
  availability: 'phone',
  runtime: 'tablet',
  cpu: 'narrow',
  memory: 'narrow',
  // Phone rows retain the compact operational scan: identity, all three
  // capacity metrics, and age. Kind and numeric ID move to the expanded row;
  // seven simultaneous tracks made every value unreadable at phone widths.
  disk: 'narrow',
  type: 'mobile',
  info: 'mobile',
  vmid: 'tablet',
  uptime: 'narrow',
  backup: 'compact',
  image: 'compact',
  namespace: 'compact',
  context: 'compact',
  aiContext: 'compact',
  update: 'compact',
  ip: 'wide',
  node: 'wide',
  tags: 'wide',
  os: 'wide',
  netIo: 'wide',
  diskIo: 'wide',
};

export interface IODistributionStats {
  median: number;
  mad: number;
  max: number;
  p97: number;
  p99: number;
  count: number;
}

export interface WorkloadIOEmphasis {
  network: IODistributionStats;
  diskIO: IODistributionStats;
}

export interface GuestRowProps {
  guest: WorkloadGuest;
  alertStyles?: {
    rowClass: string;
    indicatorClass: string;
    badgeClass: string;
    hasAlert: boolean;
    alertCount: number;
    severity: 'critical' | 'warning' | 'info' | null;
    hasPoweredOffAlert?: boolean;
    hasNonPoweredOffAlert?: boolean;
    hasUnacknowledgedAlert?: boolean;
    unacknowledgedCount?: number;
    acknowledgedCount?: number;
    hasAcknowledgedOnlyAlert?: boolean;
  };
  customUrl?: string;
  onTagClick?: (tag: string) => void;
  activeSearch?: string;
  parentNodeOnline?: boolean;
  parentMemoryTotal?: number;
  parentNodeName?: string;
  onCustomUrlUpdate?: (guestId: string, url: string) => void;
  isGroupedView?: boolean;
  visibleColumnIds?: string[];
  columnWidths?: WorkloadColumnWidths;
  onClick?: () => void;
  isExpanded?: boolean;
  isSummaryHighlighted?: boolean;
  summaryGroupMemberState?: SummaryGroupMemberInteractionState;
  ioEmphasis?: WorkloadIOEmphasis;
  metricDisplayMode?: 'bars' | 'sparklines';
  memoryDisplayBasis?: WorkloadsMemoryDisplayBasis;
  metricHistory?: WorkloadMetricHistoryReader;
  nestedWorkloadContext?: Pick<NestedWorkloadContext, 'count' | 'label'>;
  workloadTableLayoutMode?: WorkloadTableLayoutMode;
  onHoverChange?: (guestId: string | null) => void;
}

export type GuestRowMetric = WorkloadTableMetric;

export interface IOEmphasis {
  className: string;
  showOutlierHint: boolean;
}

export const EMPTY_IO_DISTRIBUTION: IODistributionStats = {
  median: 0,
  mad: 0,
  max: 0,
  p97: 0,
  p99: 0,
  count: 0,
};

export const EMPTY_IO_EMPHASIS: WorkloadIOEmphasis = {
  network: EMPTY_IO_DISTRIBUTION,
  diskIO: EMPTY_IO_DISTRIBUTION,
};

export const GROUPED_FIRST_CELL_INDENT = 'pl-1 sm:pl-5 lg:pl-8';
export const DEFAULT_FIRST_CELL_INDENT = 'pl-1 sm:pl-3';

export const getOutlierEmphasis = (value: number, stats: IODistributionStats): IOEmphasis => {
  if (!Number.isFinite(value) || value <= 0 || stats.max <= 0) {
    return { className: 'text-muted', showOutlierHint: false };
  }

  if (stats.count < 4) {
    const ratio = value / stats.max;
    if (ratio >= 0.995) {
      return { className: 'text-base-content font-medium', showOutlierHint: true };
    }
    return { className: 'text-muted', showOutlierHint: false };
  }

  if (stats.mad > 0) {
    const modifiedZ = (0.6745 * (value - stats.median)) / stats.mad;
    if (modifiedZ >= 6.5 && value >= stats.p99) {
      return { className: 'text-base-content font-semibold', showOutlierHint: true };
    }
    if (modifiedZ >= 5.5 && value >= stats.p97) {
      return { className: 'text-base-content font-medium', showOutlierHint: true };
    }
    return { className: 'text-muted', showOutlierHint: false };
  }

  if (value >= stats.p99) {
    return { className: 'text-base-content font-semibold', showOutlierHint: true };
  }
  if (value >= stats.p97) {
    return { className: 'text-base-content font-medium', showOutlierHint: true };
  }
  if (value > 0) {
    return { className: 'text-muted', showOutlierHint: false };
  }
  return { className: 'text-muted', showOutlierHint: false };
};

export const getWorkloadDisplayId = (guest: WorkloadGuest): string => {
  const provided = guest.displayId?.trim();
  if (provided) return provided;
  if (typeof guest.vmid === 'number' && guest.vmid > 0) return String(guest.vmid);
  return '';
};

// The mixed Info column projects one scalar from each workload kind. Sorting
// must use this same projection so the header never orders rows by a hidden or
// provider-specific field that differs from the rendered value.
export const getWorkloadInfoValue = (guest: WorkloadGuest): string => {
  const type = resolveWorkloadType(guest);
  if (type === 'vm' || type === 'system-container') return getWorkloadDisplayId(guest);
  if (type === 'app-container') {
    const image = guest.image?.trim() ?? '';
    return image ? getShortImageName(image) : '';
  }
  if (type === 'pod') return guest.namespace?.trim() ?? '';
  return '';
};

export const GUEST_COLUMNS: ColumnDef[] = [
  {
    id: 'name',
    label: 'Name',
    width: '200px',
    minWidth: '180px',
    maxWidth: '220px',
    sortKey: 'name',
    kind: 'name',
  },
  {
    id: 'availability',
    label: 'Avail',
    width: '56px',
    minWidth: '48px',
    maxWidth: '64px',
    toggleable: true,
    kind: 'badge',
  },
  {
    id: 'runtime',
    label: 'Runtime',
    width: '104px',
    minWidth: '96px',
    maxWidth: '112px',
    toggleable: true,
    kind: 'text',
  },
  createVisibleCanonicalTypeColumn(),
  { id: 'info', label: 'Info', width: '100px', sortKey: 'info', kind: 'numeric-value' },
  { id: 'vmid', label: 'ID', width: '45px', sortKey: 'vmid', kind: 'numeric-value' },
  { id: 'cpu', label: 'CPU', width: '140px', sortKey: 'cpu', kind: 'metric-bar' },
  { id: 'memory', label: 'Mem', width: '140px', sortKey: 'memory', kind: 'metric-bar' },
  {
    id: 'disk',
    label: 'Disk',
    width: '140px',
    toggleable: true,
    sortKey: 'disk',
    kind: 'metric-bar',
  },
  {
    id: 'ip',
    label: 'IP',
    icon: (
      <svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9"
        />
      </svg>
    ),
    width: '45px',
    toggleable: true,
    kind: 'text',
  },
  {
    id: 'uptime',
    label: 'Uptime',
    icon: (
      <svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
        />
      </svg>
    ),
    width: '60px',
    toggleable: true,
    sortKey: 'uptime',
    kind: 'numeric-value',
  },
  {
    id: 'node',
    label: 'Node',
    icon: (
      <svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01"
        />
      </svg>
    ),
    width: '70px',
    toggleable: true,
    sortKey: 'node',
    kind: 'text',
  },
  {
    id: 'image',
    label: 'Image',
    icon: (
      <svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <rect x="3" y="6" width="18" height="12" rx="2" />
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M3 10h18M7 6v12M13 6v12"
        />
      </svg>
    ),
    width: '140px',
    minWidth: '120px',
    toggleable: true,
    sortKey: 'image',
    kind: 'text',
  },
  {
    id: 'namespace',
    label: 'Namespace',
    icon: (
      <svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M12 2l7 4v8l-7 4-7-4V6l7-4z"
        />
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v12" />
      </svg>
    ),
    width: '110px',
    minWidth: '90px',
    toggleable: true,
    sortKey: 'namespace',
    kind: 'text',
  },
  {
    id: 'context',
    label: 'Context',
    icon: (
      <svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M12 6v6m0 6h.01M4 6a8 8 0 018-4 8 8 0 018 4M4 18a8 8 0 008 4 8 8 0 008-4"
        />
      </svg>
    ),
    width: '120px',
    minWidth: '100px',
    toggleable: true,
    sortKey: 'contextLabel',
    kind: 'text',
  },
  {
    id: 'aiContext',
    label: 'AI Context',
    icon: (
      <svg
        class="w-3.5 h-3.5 block"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
        aria-hidden="true"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M12 5a3 3 0 1 0-5.997.125 4 4 0 0 0-2.526 5.77 4 4 0 0 0 .556 6.588A4 4 0 1 0 12 18Z"
        />
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M9 13a4.5 4.5 0 0 0 3-4M6.003 5.125A3 3 0 0 0 6.401 6.5M3.477 10.896a4 4 0 0 1 .585-.396M6 18a4 4 0 0 1-1.967-.516M12 13h4M12 18h6a2 2 0 0 1 2 2v1M12 8h8M16 8V5a2 2 0 0 1 2-2"
        />
        <circle cx="16" cy="13" r=".5" fill="currentColor" stroke="none" />
        <circle cx="18" cy="3" r=".5" fill="currentColor" stroke="none" />
        <circle cx="20" cy="21" r=".5" fill="currentColor" stroke="none" />
        <circle cx="20" cy="8" r=".5" fill="currentColor" stroke="none" />
      </svg>
    ),
    width: '92px',
    minWidth: '84px',
    toggleable: true,
    defaultHidden: true,
    kind: 'badge',
  },
  {
    id: 'backup',
    label: 'Backup',
    icon: (
      <svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"
        />
      </svg>
    ),
    width: '72px',
    minWidth: '68px',
    toggleable: true,
    kind: 'badge',
  },
  {
    id: 'tags',
    label: 'Tags',
    icon: (
      <svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z"
        />
      </svg>
    ),
    width: '60px',
    toggleable: true,
    kind: 'badge',
  },
  { id: 'os', label: 'OS', width: '45px', toggleable: true, kind: 'badge' },
  {
    id: 'netIo',
    label: 'Net I/O',
    width: '170px',
    minWidth: '170px',
    toggleable: true,
    sortKey: 'netIo',
    kind: 'numeric-value',
  },
  {
    id: 'diskIo',
    label: 'Disk I/O',
    width: '170px',
    minWidth: '170px',
    toggleable: true,
    sortKey: 'diskIo',
    kind: 'numeric-value',
  },
  { id: 'update', label: 'Update', width: '86px', toggleable: true, kind: 'badge' },
];

const GUEST_COLUMN_BY_ID = new Map(GUEST_COLUMNS.map((column) => [column.id, column] as const));

type GuestColumnWidthOverride = {
  width?: string | null;
  minWidth?: string | null;
  maxWidth?: string | null;
};

const percentageColumn = (width: string): GuestColumnWidthOverride => ({
  width,
  minWidth: null,
  maxWidth: width,
});

// Responsive weights are normalized against the currently visible column set.
// That keeps each workload view mode full-width without assuming one fixed set.
const GUEST_COLUMN_RESPONSIVE_WEIGHTS: Record<
  Exclude<WorkloadTableLayoutMode, 'wide'>,
  Record<string, number>
> = {
  narrow: {
    name: 40,
    cpu: 16,
    memory: 16,
    disk: 16,
    uptime: 12,
  },
  phone: {
    name: 33,
    availability: 7,
    type: 9,
    info: 8,
    cpu: 11,
    memory: 11,
    disk: 11,
    uptime: 11,
  },
  mobile: {
    name: 33,
    availability: 7,
    type: 9,
    info: 8,
    cpu: 11,
    memory: 11,
    disk: 11,
    uptime: 11,
  },
  tablet: {
    name: 30,
    runtime: 9,
    type: 8,
    info: 8,
    vmid: 8,
    cpu: 17,
    memory: 17,
    disk: 17,
  },
  compact: {
    name: 26,
    runtime: 10,
    type: 7,
    info: 7,
    vmid: 7,
    cpu: 13,
    memory: 14,
    disk: 14,
    uptime: 8,
    backup: 5,
    image: 18,
    namespace: 11,
    context: 13,
    aiContext: 9,
    update: 10,
  },
};

const getResponsiveColumnOverride = (
  columnId: string,
  layoutMode: WorkloadTableLayoutMode,
  visibleColumnIds?: readonly string[],
): GuestColumnWidthOverride | undefined => {
  if (layoutMode === 'wide') return undefined;

  const weights = GUEST_COLUMN_RESPONSIVE_WEIGHTS[layoutMode];
  if (!weights[columnId]) return undefined;

  const activeIds = visibleColumnIds?.length ? visibleColumnIds : Object.keys(weights);
  const style = getPlatformTableWeightedColumnWidthStyle(
    columnId,
    weights,
    activeIds,
    layoutMode === 'narrow'
      ? { columnId: 'name', widthPercent: PLATFORM_TABLE_NARROW_IDENTITY_WIDTH_PERCENT }
      : layoutMode === 'phone' || layoutMode === 'mobile'
        ? { columnId: 'name', widthPercent: PLATFORM_TABLE_PHONE_IDENTITY_WIDTH_PERCENT }
        : undefined,
  );
  return typeof style.width === 'string' ? percentageColumn(style.width) : undefined;
};

const getGuestColumnSizing = (
  columnId: string,
  isMobile = false,
  layoutMode: WorkloadTableLayoutMode = isMobile ? 'phone' : 'wide',
  visibleColumnIds?: readonly string[],
  manualWidths?: WorkloadColumnWidths,
): Pick<ColumnDef, 'width' | 'minWidth' | 'maxWidth'> | undefined => {
  const column = GUEST_COLUMN_BY_ID.get(columnId);
  if (!column) return undefined;

  const sizing: Pick<ColumnDef, 'width' | 'minWidth' | 'maxWidth'> = {
    width: column.width,
    minWidth: column.minWidth,
    maxWidth: column.maxWidth,
  };

  const pinned = manualWidths?.[columnId];
  const manualOverride =
    typeof pinned === 'number' && Number.isFinite(pinned) && pinned > 0
      ? (() => {
          const width = `${Math.round(pinned)}px`;
          return { width, minWidth: width, maxWidth: width };
        })()
      : undefined;
  const override =
    manualOverride ?? getResponsiveColumnOverride(columnId, layoutMode, visibleColumnIds);
  if (override) {
    if ('width' in override) sizing.width = override.width ?? undefined;
    if ('minWidth' in override) sizing.minWidth = override.minWidth ?? undefined;
    if ('maxWidth' in override) sizing.maxWidth = override.maxWidth ?? undefined;
  }

  return sizing;
};

export const getGuestColumnStyle = (
  columnId: string,
  isMobile = false,
  layoutMode?: WorkloadTableLayoutMode,
  visibleColumnIds?: readonly string[],
  manualWidths?: WorkloadColumnWidths,
): JSX.CSSProperties | undefined => {
  const sizing = getGuestColumnSizing(
    columnId,
    isMobile,
    layoutMode,
    visibleColumnIds,
    manualWidths,
  );
  if (!sizing) return undefined;

  const style: JSX.CSSProperties = {};

  if (sizing.width) style.width = sizing.width;
  if (sizing.minWidth) style['min-width'] = sizing.minWidth;
  if (sizing.maxWidth) style['max-width'] = sizing.maxWidth;

  return Object.keys(style).length > 0 ? style : undefined;
};

export const getGuestColumnWidthStyle = (
  columnId: string,
  isMobile = false,
  layoutMode?: WorkloadTableLayoutMode,
  visibleColumnIds?: readonly string[],
  manualWidths?: WorkloadColumnWidths,
): JSX.CSSProperties | undefined => {
  const sizing = getGuestColumnSizing(
    columnId,
    isMobile,
    layoutMode,
    visibleColumnIds,
    manualWidths,
  );
  if (!sizing?.width) return undefined;
  return { width: sizing.width };
};

export const getWorkloadTableLayoutMode = (width: number): WorkloadTableLayoutMode => {
  if (!Number.isFinite(width) || width < WORKLOAD_TABLE_NARROW_LAYOUT_WIDTH) return 'narrow';
  if (!Number.isFinite(width) || width < WORKLOAD_TABLE_PHONE_LAYOUT_WIDTH) return 'phone';
  if (width < WORKLOAD_TABLE_MOBILE_LAYOUT_WIDTH) return 'mobile';
  if (width < WORKLOAD_TABLE_TABLET_LAYOUT_WIDTH) return 'tablet';
  if (width < WORKLOAD_TABLE_WIDE_LAYOUT_WIDTH) return 'compact';
  return 'wide';
};

export const getWorkloadTableLayoutModeForContainer = (width: number): WorkloadTableLayoutMode => {
  if (!Number.isFinite(width) || width < WORKLOAD_TABLE_CONTAINER_NARROW_WIDTH) return 'narrow';
  if (!Number.isFinite(width) || width < WORKLOAD_TABLE_CONTAINER_PHONE_WIDTH) return 'phone';
  if (width < WORKLOAD_TABLE_CONTAINER_MOBILE_WIDTH) return 'mobile';
  if (width < WORKLOAD_TABLE_CONTAINER_TABLET_WIDTH) return 'tablet';
  if (width < WORKLOAD_TABLE_CONTAINER_WIDE_WIDTH) return 'compact';
  return 'wide';
};

export const getWorkloadVisibleColumnsForLayout = (
  columns: ColumnDef[],
  layoutMode: WorkloadTableLayoutMode,
  options?: { manualSizing?: boolean },
): ColumnDef[] => {
  if (
    options?.manualSizing &&
    WORKLOAD_TABLE_LAYOUT_ORDER[layoutMode] >= WORKLOAD_TABLE_LAYOUT_ORDER.tablet
  ) {
    return [...columns];
  }

  const layoutRank = WORKLOAD_TABLE_LAYOUT_ORDER[layoutMode];
  return columns.filter((column) => {
    const minimumLayout = WORKLOAD_COLUMN_MIN_LAYOUT[column.id] ?? 'wide';
    return WORKLOAD_TABLE_LAYOUT_ORDER[minimumLayout] <= layoutRank;
  });
};

export const VIEW_MODE_COLUMNS: Record<ViewMode, Set<string> | null> = {
  all: new Set([
    'name',
    'availability',
    'type',
    'info',
    'cpu',
    'memory',
    'disk',
    'ip',
    'uptime',
    'node',
    'backup',
    'aiContext',
    'tags',
    'os',
    'diskIo',
    'netIo',
  ]),
  vm: new Set([
    'name',
    'availability',
    'vmid',
    'cpu',
    'memory',
    'disk',
    'ip',
    'uptime',
    'node',
    'backup',
    'aiContext',
    'tags',
    'os',
    'diskIo',
    'netIo',
  ]),
  'system-container': new Set([
    'name',
    'availability',
    'vmid',
    'cpu',
    'memory',
    'disk',
    'ip',
    'uptime',
    'node',
    'backup',
    'aiContext',
    'tags',
    'os',
    'diskIo',
    'netIo',
  ]),
  container: new Set([
    'name',
    'availability',
    'runtime',
    'type',
    'info',
    'cpu',
    'memory',
    'disk',
    'ip',
    'uptime',
    'node',
    'backup',
    'tags',
    'os',
    'image',
    'context',
    'aiContext',
    'update',
    'diskIo',
    'netIo',
  ]),
  'app-container': new Set([
    'name',
    'availability',
    'runtime',
    'cpu',
    'memory',
    'disk',
    'uptime',
    'image',
    'context',
    'aiContext',
    'tags',
    'update',
    'diskIo',
    'netIo',
  ]),
  pod: new Set(['name', 'cpu', 'memory', 'image', 'namespace', 'context', 'aiContext']),
};

/**
 * The public `container` view intentionally covers both system containers and
 * application containers. Platform-owned surfaces can exclude either subtype;
 * in that case the column profile must narrow with the same canonical boundary
 * instead of exposing fields that every remaining row renders empty.
 */
export const resolveWorkloadColumnViewMode = (
  viewMode: ViewMode,
  excludedWorkloadTypes: readonly WorkloadType[] = [],
): ViewMode => {
  if (viewMode !== 'container') return viewMode;

  const includesSystemContainers = !excludedWorkloadTypes.includes('system-container');
  const includesAppContainers = !excludedWorkloadTypes.includes('app-container');

  if (includesSystemContainers === includesAppContainers) return viewMode;
  return includesSystemContainers ? 'system-container' : 'app-container';
};
