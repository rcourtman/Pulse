import type { ColumnDef } from '@/hooks/useColumnVisibility';
import type { ViewMode, WorkloadGuest } from '@/types/workloads';
import type { JSX } from 'solid-js';

import type { WorkloadTableMetricHistoryRange } from './workloadMetricHistoryModel';

export type WorkloadsStatusMode = 'all' | 'running' | 'degraded' | 'stopped';
export type WorkloadsGroupingMode = 'grouped' | 'flat';
export type WorkloadsMetricDisplayMode = 'bars' | 'sparklines';
export type WorkloadsMetricHoverMode = 'details' | 'history';
export type WorkloadsMemoryDisplayBasis = 'guest' | 'host';
export type WorkloadsSortKey = keyof WorkloadGuest | 'diskIo' | 'info' | 'netIo';

export interface WorkloadsStatusOption {
  value: WorkloadsStatusMode;
  label: string;
}

export interface WorkloadsInventoryStats {
  total: number;
  running: number;
  degraded: number;
  stopped: number;
  vms: number;
  containers: number;
  appContainers: number;
  pods: number;
}

export interface WorkloadsFilterSelectOption {
  value: string;
  label: string;
}

export interface WorkloadsToolbarFilterConfig {
  id?: string;
  label?: string;
  value: string;
  options: WorkloadsFilterSelectOption[];
  onChange: (value: string) => void;
}

export interface WorkloadsFilterProps {
  search: () => string;
  setSearch: (value: string) => void;
  viewMode: () => ViewMode;
  setViewMode: (value: ViewMode) => void;
  statusMode: () => WorkloadsStatusMode;
  setStatusMode: (value: WorkloadsStatusMode) => void;
  groupingMode: () => WorkloadsGroupingMode;
  setGroupingMode: (value: WorkloadsGroupingMode) => void;
  defaultSortKey?: WorkloadsSortKey;
  setSortKey: (value: WorkloadsSortKey) => void;
  setSortDirection: (value: 'asc' | 'desc') => void;
  onBeforeAutoFocus?: () => boolean;
  ariaLabel?: string;
  searchPlaceholder?: string;
  searchEmptyMessage?: string;
  searchSuggestionWorkloads?: () => readonly WorkloadGuest[];
  statusOptions?: readonly WorkloadsStatusOption[];
  inventoryStats?: () => WorkloadsInventoryStats;
  inventoryCountsVisible?: () => boolean;
  setInventoryCountsVisible?: (visible: boolean) => void;
  columnVisibility?: {
    availableColumns: ColumnDef[];
    isColumnHidden: (id: string) => boolean;
    onColumnToggle: (id: string) => void;
    onColumnReset?: () => void;
    showReset?: boolean;
    onColumnWidthsReset?: () => void;
    hasColumnWidthOverrides?: boolean;
  };
  hostFilter?: WorkloadsToolbarFilterConfig;
  platformFilter?: WorkloadsToolbarFilterConfig;
  namespaceFilter?: WorkloadsToolbarFilterConfig;
  clusterFilter?: WorkloadsToolbarFilterConfig;
  containerRuntimeFilter?: WorkloadsToolbarFilterConfig;
  chartsCollapsed?: () => boolean;
  onChartsToggle?: () => void;
  metricDisplayMode?: () => WorkloadsMetricDisplayMode;
  setMetricDisplayMode?: (value: WorkloadsMetricDisplayMode) => void;
  metricHoverMode?: () => WorkloadsMetricHoverMode;
  setMetricHoverMode?: (value: WorkloadsMetricHoverMode) => void;
  metricHistoryRange?: () => WorkloadTableMetricHistoryRange;
  setMetricHistoryRange?: (value: WorkloadTableMetricHistoryRange) => void;
  metricHistoryHintVisible?: () => boolean;
  memoryDisplayBasis?: () => WorkloadsMemoryDisplayBasis;
  setMemoryDisplayBasis?: (value: WorkloadsMemoryDisplayBasis) => void;
  pinnedSelectionActive?: () => boolean;
  onClearPinnedSelection?: () => void;
  searchTrailing?: JSX.Element;
  utilityActions?: JSX.Element;
  mobileTrailing?: JSX.Element;
  forcedPlatform?: string;
  suppressTypeFilter?: boolean;
}

export type WorkloadsMetricFilterProps = Pick<
  WorkloadsFilterProps,
  | 'metricDisplayMode'
  | 'setMetricDisplayMode'
  | 'metricHoverMode'
  | 'setMetricHoverMode'
  | 'metricHistoryRange'
  | 'setMetricHistoryRange'
  | 'metricHistoryHintVisible'
>;

export interface WorkloadsMetricFilterState {
  workloadMetricDisplayMode: NonNullable<WorkloadsFilterProps['metricDisplayMode']>;
  setWorkloadMetricDisplayMode: NonNullable<WorkloadsFilterProps['setMetricDisplayMode']>;
  workloadMetricHoverMode: NonNullable<WorkloadsFilterProps['metricHoverMode']>;
  setWorkloadMetricHoverMode: NonNullable<WorkloadsFilterProps['setMetricHoverMode']>;
  workloadMetricHistoryRange: NonNullable<WorkloadsFilterProps['metricHistoryRange']>;
  setWorkloadMetricHistoryRange: NonNullable<WorkloadsFilterProps['setMetricHistoryRange']>;
  workloadMetricHistoryHintVisible: NonNullable<WorkloadsFilterProps['metricHistoryHintVisible']>;
}

/**
 * Canonical metric controls for every WorkloadsFilter composition.
 *
 * Provider pages may own the surrounding toolbar, but they must not select a
 * subset of display, hover, range, and discovery state independently. Keeping
 * this binding atomic prevents a bespoke platform toolbar from silently
 * dropping History hover while the shared workload table still supports it.
 */
export const getWorkloadsMetricFilterProps = (
  state: WorkloadsMetricFilterState,
): WorkloadsMetricFilterProps => ({
  metricDisplayMode: state.workloadMetricDisplayMode,
  setMetricDisplayMode: state.setWorkloadMetricDisplayMode,
  metricHoverMode: state.workloadMetricHoverMode,
  setMetricHoverMode: state.setWorkloadMetricHoverMode,
  metricHistoryRange: state.workloadMetricHistoryRange,
  setMetricHistoryRange: state.setWorkloadMetricHistoryRange,
  metricHistoryHintVisible: state.workloadMetricHistoryHintVisible,
});

export interface CountActiveWorkloadsFiltersOptions {
  search: string;
  viewMode: ViewMode;
  statusMode: WorkloadsStatusMode;
  hostFilterValue?: string;
  platformFilterValue?: string;
  namespaceFilterValue?: string;
  clusterFilterValue?: string;
  containerRuntimeFilterValue?: string;
}

export type HasActiveWorkloadsFiltersOptions = CountActiveWorkloadsFiltersOptions;

export const DEFAULT_WORKLOADS_SORT_KEY: WorkloadsSortKey = 'type';
export const DEFAULT_WORKLOADS_SORT_DIRECTION = 'asc';
export const DEFAULT_WORKLOADS_VIEW_MODE: ViewMode = 'all';
export const DEFAULT_WORKLOADS_STATUS_MODE: WorkloadsStatusMode = 'all';
export const DEFAULT_WORKLOADS_METRIC_DISPLAY_MODE: WorkloadsMetricDisplayMode = 'bars';
export const DEFAULT_WORKLOADS_METRIC_HOVER_MODE: WorkloadsMetricHoverMode = 'history';

export const countActiveWorkloadsFilters = (
  options: CountActiveWorkloadsFiltersOptions,
): number => {
  let count = 0;

  if (options.search.trim() !== '') count++;
  if (options.viewMode !== DEFAULT_WORKLOADS_VIEW_MODE) count++;
  if (options.statusMode !== DEFAULT_WORKLOADS_STATUS_MODE) count++;
  if ((options.hostFilterValue ?? '') !== '') count++;
  if ((options.platformFilterValue ?? '') !== '') count++;
  if ((options.namespaceFilterValue ?? '') !== '') count++;
  if ((options.clusterFilterValue ?? '') !== '') count++;
  if ((options.containerRuntimeFilterValue ?? '') !== '') count++;

  return count;
};

export const hasActiveWorkloadsFilters = (options: HasActiveWorkloadsFiltersOptions): boolean =>
  countActiveWorkloadsFilters(options) > 0;
