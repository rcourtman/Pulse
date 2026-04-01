import type { ColumnDef } from '@/hooks/useColumnVisibility';
import type { WorkloadGuest, ViewMode } from '@/types/workloads';
import { createVisibleCanonicalTypeColumn } from '@/utils/typeColumnDefinition';

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
    severity: 'critical' | 'warning' | null;
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
  onCustomUrlUpdate?: (guestId: string, url: string) => void;
  isGroupedView?: boolean;
  visibleColumnIds?: string[];
  onClick?: () => void;
  isExpanded?: boolean;
  isSummaryHighlighted?: boolean;
  ioEmphasis?: WorkloadIOEmphasis;
  onHoverChange?: (guestId: string | null) => void;
}

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

export const GROUPED_FIRST_CELL_INDENT = 'pl-3 sm:pl-5 lg:pl-8';
export const DEFAULT_FIRST_CELL_INDENT = 'pl-2 sm:pl-3';

export const getOutlierEmphasis = (
  value: number,
  stats: IODistributionStats,
): IOEmphasis => {
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

export const GUEST_COLUMNS: ColumnDef[] = [
  { id: 'name', label: 'Name', width: '200px', sortKey: 'name' },
  createVisibleCanonicalTypeColumn(),
  { id: 'info', label: 'Info', width: '100px' },
  { id: 'vmid', label: 'ID', width: '45px', sortKey: 'vmid' },
  { id: 'cpu', label: 'CPU', width: '140px', sortKey: 'cpu' },
  { id: 'memory', label: 'Mem', width: '140px', sortKey: 'memory' },
  { id: 'disk', label: 'Disk', width: '140px', sortKey: 'disk' },
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
    width: '50px',
    toggleable: true,
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
  },
  { id: 'update', label: 'Update', width: '60px', toggleable: true },
  { id: 'os', label: 'OS', width: '45px', toggleable: true },
  {
    id: 'netIo',
    label: 'Net I/O',
    width: '130px',
    minWidth: '120px',
    toggleable: true,
    sortKey: 'netIo',
  },
  {
    id: 'diskIo',
    label: 'Disk I/O',
    width: '130px',
    minWidth: '120px',
    toggleable: true,
    sortKey: 'diskIo',
  },
  { id: 'link', label: '', width: '28px' },
];

export const VIEW_MODE_COLUMNS: Record<ViewMode, Set<string> | null> = {
  all: new Set([
    'name',
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
    'diskIo',
    'netIo',
    'link',
  ]),
  vm: new Set([
    'name',
    'vmid',
    'cpu',
    'memory',
    'disk',
    'ip',
    'uptime',
    'node',
    'backup',
    'tags',
    'os',
    'diskIo',
    'netIo',
    'link',
  ]),
  'system-container': new Set([
    'name',
    'vmid',
    'cpu',
    'memory',
    'disk',
    'ip',
    'uptime',
    'node',
    'backup',
    'tags',
    'os',
    'diskIo',
    'netIo',
    'link',
  ]),
  'app-container': new Set([
    'name',
    'cpu',
    'memory',
    'uptime',
    'image',
    'context',
    'tags',
    'update',
    'link',
  ]),
  pod: new Set(['name', 'cpu', 'memory', 'image', 'namespace', 'context', 'link']),
};
