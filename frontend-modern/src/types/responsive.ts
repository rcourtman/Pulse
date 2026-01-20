import type { Component, JSX } from 'solid-js';
import type { ColumnPriority, Breakpoint } from '@/hooks/useBreakpoint';

/**
 * Configuration for a responsive table column
 */
export interface ColumnConfig {
  /** Unique identifier for the column, typically matches a property key */
  id: string;

  /** Display label for the column header */
  label: string;

  /** Priority determines at which breakpoint the column becomes visible */
  priority: ColumnPriority;

  /** Icon component to show in header on mobile (optional) */
  icon?: Component<{ class?: string }>;

  /** Whether the column is sortable */
  sortable?: boolean;

  /** Sort key - defaults to id if sortable is true */
  sortKey?: string;

  /** Minimum width for the column (CSS value, e.g., '100px', '10%') */
  minWidth?: string;

  /** Maximum width for the column (CSS value) */
  maxWidth?: string;

  /** Fixed width for the column (CSS value) - used with table-layout: fixed */
  width?: string;

  /** Flex grow factor for grid layout (default: 1) */
  flex?: number;

  /** Text alignment */
  align?: 'left' | 'center' | 'right';

  /** Whether this column should be sticky (typically for name columns) */
  sticky?: boolean;

  /** Custom CSS class for the column */
  className?: string;

  /** Custom CSS class for the header */
  headerClassName?: string;

  /** Custom CSS class for cells */
  cellClassName?: string;
}

/**
 * Extended column config with render functions
 */
export interface ColumnConfigWithRender<T> extends ColumnConfig {
  /**
   * Render function for mobile view (compact)
   * If not provided, falls back to renderDesktop
   */
  renderMobile?: (item: T, index: number) => JSX.Element;

  /**
   * Render function for desktop view (full)
   * Required if renderMobile is provided
   */
  renderDesktop?: (item: T, index: number) => JSX.Element;

  /**
   * Simple render function for both mobile and desktop
   * Use this for columns that don't need different mobile/desktop rendering
   */
  render?: (item: T, index: number) => JSX.Element;

  /**
   * Get sort value from item (for custom sorting)
   */
  getSortValue?: (item: T) => string | number | null | undefined;
}

/**
 * Common metric column configuration for CPU, Memory, Disk metrics
 */
export interface MetricColumnConfig {
  type: 'cpu' | 'memory' | 'disk';
  getValue: (item: unknown) => number;
  getLabel?: (item: unknown) => string;
  getSublabel?: (item: unknown) => string | undefined;
  getResourceId?: (item: unknown) => string;
}

/**
 * Sort state for a table
 */
export interface SortState {
  key: string | null;
  direction: 'asc' | 'desc';
}

/**
 * Grid template configuration for responsive tables
 */
export interface GridTemplateConfig {
  /** Column configurations */
  columns: ColumnConfig[];

  /** Current breakpoint for visibility calculations */
  breakpoint: Breakpoint;

  /** Whether to include sticky column handling */
  hasStickyColumn?: boolean;
}

/**
 * Props for responsive table header
 */
export interface ResponsiveHeaderProps {
  /** Column configuration */
  column: ColumnConfig;

  /** Current sort state */
  sortState?: SortState;

  /** Sort handler */
  onSort?: (key: string) => void;

  /** Whether currently on mobile */
  isMobile?: boolean;
}

/**
 * Props for responsive metric cell
 */
export interface ResponsiveMetricCellProps {
  /** Metric value (0-100 percentage) */
  value: number;

  /** Metric type for theming */
  type: 'cpu' | 'memory' | 'disk';

  /** Primary label (usually formatted percentage) */
  label?: string;

  /** Secondary label (e.g., "4.2GB / 8GB") */
  sublabel?: string;

  /** Resource ID for sparkline tracking */
  resourceId?: string;

  /** Whether the resource is running/online */
  isRunning?: boolean;

  /** Whether currently on mobile */
  isMobile?: boolean;

  /** Fallback content when not running */
  fallback?: JSX.Element;
}

/**
 * Standard column definitions that can be reused across tables
 */
export const STANDARD_COLUMNS = {
  /** Name/identifier column - always visible, typically sticky on mobile */
  name: {
    id: 'name',
    label: 'Name',
    priority: 'essential' as ColumnPriority,
    sortable: true,
    sticky: true,
    minWidth: '150px',
    flex: 1.5,
  },

  /** Type badge column (VM/LXC, Container/Service, etc.) */
  type: {
    id: 'type',
    label: 'Type',
    priority: 'essential' as ColumnPriority,
    sortable: true,
    minWidth: '60px',
    maxWidth: '80px',
    align: 'center' as const,
  },

  /** VMID column */
  vmid: {
    id: 'vmid',
    label: 'VMID',
    priority: 'essential' as ColumnPriority,
    sortable: true,
    minWidth: '60px',
    maxWidth: '80px',
    align: 'center' as const,
  },

  /** Status column */
  status: {
    id: 'status',
    label: 'Status',
    priority: 'primary' as ColumnPriority,
    sortable: true,
    minWidth: '70px',
    maxWidth: '100px',
    align: 'center' as const,
  },

  /** Uptime column */
  uptime: {
    id: 'uptime',
    label: 'Uptime',
    priority: 'secondary' as ColumnPriority,
    sortable: true,
    minWidth: '80px',
    maxWidth: '100px',
  },

  /** CPU metric column */
  cpu: {
    id: 'cpu',
    label: 'CPU',
    priority: 'essential' as ColumnPriority,
    sortable: true,
    minWidth: '100px',
    flex: 1.5,
    align: 'center' as const,
  },

  /** Memory metric column */
  memory: {
    id: 'memory',
    label: 'Memory',
    priority: 'essential' as ColumnPriority,
    sortable: true,
    minWidth: '100px',
    flex: 1.5,
    align: 'center' as const,
  },

  /** Disk metric column */
  disk: {
    id: 'disk',
    label: 'Disk',
    priority: 'essential' as ColumnPriority,
    sortable: true,
    minWidth: '100px',
    flex: 1.5,
    align: 'center' as const,
  },

  /** Disk read I/O column */
  diskRead: {
    id: 'diskRead',
    label: 'D Read',
    priority: 'detailed' as ColumnPriority,
    sortable: true,
    minWidth: '100px',
    flex: 1,
  },

  /** Disk write I/O column */
  diskWrite: {
    id: 'diskWrite',
    label: 'D Write',
    priority: 'detailed' as ColumnPriority,
    sortable: true,
    minWidth: '100px',
    flex: 1,
  },

  /** Network in column */
  networkIn: {
    id: 'networkIn',
    label: 'Net In',
    priority: 'detailed' as ColumnPriority,
    sortable: true,
    minWidth: '100px',
    flex: 1,
  },

  /** Network out column */
  networkOut: {
    id: 'networkOut',
    label: 'Net Out',
    priority: 'detailed' as ColumnPriority,
    sortable: true,
    minWidth: '100px',
    flex: 1,
  },

  /** Temperature column */
  temperature: {
    id: 'temperature',
    label: 'Temp',
    priority: 'supplementary' as ColumnPriority,
    sortable: true,
    minWidth: '45px',
    maxWidth: '65px',
    align: 'center' as const,
  },

  /** Node column (for grouped views) */
  node: {
    id: 'node',
    label: 'Node',
    priority: 'supplementary' as ColumnPriority,
    sortable: true,
    minWidth: '100px',
  },

  /** Storage column */
  storage: {
    id: 'storage',
    label: 'Storage',
    priority: 'supplementary' as ColumnPriority,
    sortable: true,
    minWidth: '100px',
  },

  /** Size column */
  size: {
    id: 'size',
    label: 'Size',
    priority: 'secondary' as ColumnPriority,
    sortable: true,
    minWidth: '80px',
    align: 'right' as const,
  },

  /** Time/date column */
  time: {
    id: 'time',
    label: 'Time',
    priority: 'primary' as ColumnPriority,
    sortable: true,
    minWidth: '120px',
  },

  /** Actions column (expand buttons, etc.) */
  actions: {
    id: 'actions',
    label: '',
    priority: 'essential' as ColumnPriority,
    sortable: false,
    minWidth: '32px',
    maxWidth: '48px',
  },
} satisfies Record<string, ColumnConfig>;

/**
 * Helper to create a column config from a standard column with overrides
 */
export function createColumn<T>(
  base: ColumnConfig,
  overrides?: Partial<ColumnConfigWithRender<T>>
): ColumnConfigWithRender<T> {
  return { ...base, ...overrides };
}
