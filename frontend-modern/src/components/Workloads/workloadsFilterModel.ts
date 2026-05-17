import type { ColumnDef } from '@/hooks/useColumnVisibility';
import type { ViewMode, WorkloadGuest } from '@/types/workloads';
import type { JSX } from 'solid-js';

import type { WorkloadTableMetricHistoryRange } from './workloadMetricHistoryModel';

export type WorkloadsStatusMode = 'all' | 'running' | 'degraded' | 'stopped';
export type WorkloadsGroupingMode = 'grouped' | 'flat';
export type WorkloadsMetricDisplayMode = 'bars' | 'sparklines';
export type WorkloadsSortKey = keyof WorkloadGuest | 'diskIo' | 'netIo';

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
  setSortKey: (value: WorkloadsSortKey) => void;
  setSortDirection: (value: 'asc' | 'desc') => void;
  onBeforeAutoFocus?: () => boolean;
  columnVisibility?: {
    availableColumns: ColumnDef[];
    isColumnHidden: (id: string) => boolean;
    onColumnToggle: (id: string) => void;
    onColumnReset?: () => void;
  };
  hostFilter?: WorkloadsToolbarFilterConfig;
  platformFilter?: WorkloadsToolbarFilterConfig;
  namespaceFilter?: WorkloadsToolbarFilterConfig;
  containerRuntimeFilter?: WorkloadsToolbarFilterConfig;
  chartsCollapsed?: () => boolean;
  onChartsToggle?: () => void;
  metricDisplayMode?: () => WorkloadsMetricDisplayMode;
  setMetricDisplayMode?: (value: WorkloadsMetricDisplayMode) => void;
  metricHistoryRange?: () => WorkloadTableMetricHistoryRange;
  setMetricHistoryRange?: (value: WorkloadTableMetricHistoryRange) => void;
  pinnedSelectionActive?: () => boolean;
  onClearPinnedSelection?: () => void;
  searchTrailing?: JSX.Element;
  utilityActions?: JSX.Element;
  mobileTrailing?: JSX.Element;
  forcedPlatform?: string;
}

export interface CountActiveWorkloadsFiltersOptions {
  search: string;
  viewMode: ViewMode;
  statusMode: WorkloadsStatusMode;
  hostFilterValue?: string;
  platformFilterValue?: string;
  namespaceFilterValue?: string;
}

export interface HasActiveWorkloadsFiltersOptions extends CountActiveWorkloadsFiltersOptions {
  groupingMode: WorkloadsGroupingMode;
}

export const DEFAULT_WORKLOADS_SORT_KEY: WorkloadsSortKey = 'type';
export const DEFAULT_WORKLOADS_SORT_DIRECTION = 'asc';
export const DEFAULT_WORKLOADS_VIEW_MODE: ViewMode = 'all';
export const DEFAULT_WORKLOADS_STATUS_MODE: WorkloadsStatusMode = 'all';
export const DEFAULT_WORKLOADS_GROUPING_MODE: WorkloadsGroupingMode = 'grouped';
export const DEFAULT_WORKLOADS_METRIC_DISPLAY_MODE: WorkloadsMetricDisplayMode = 'bars';

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

  return count;
};

export const hasActiveWorkloadsFilters = (options: HasActiveWorkloadsFiltersOptions): boolean =>
  countActiveWorkloadsFilters(options) > 0 ||
  options.groupingMode !== DEFAULT_WORKLOADS_GROUPING_MODE;
