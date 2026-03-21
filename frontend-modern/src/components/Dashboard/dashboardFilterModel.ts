import type { ColumnDef } from '@/hooks/useColumnVisibility';
import type { ViewMode } from '@/types/workloads';

export type DashboardStatusMode = 'all' | 'running' | 'degraded' | 'stopped';
export type DashboardGroupingMode = 'grouped' | 'flat';

export interface DashboardFilterSelectOption {
  value: string;
  label: string;
}

export interface DashboardToolbarFilterConfig {
  id?: string;
  label?: string;
  value: string;
  options: DashboardFilterSelectOption[];
  onChange: (value: string) => void;
}

export interface DashboardFilterProps {
  search: () => string;
  setSearch: (value: string) => void;
  viewMode: () => ViewMode;
  setViewMode: (value: ViewMode) => void;
  statusMode: () => DashboardStatusMode;
  setStatusMode: (value: DashboardStatusMode) => void;
  groupingMode: () => DashboardGroupingMode;
  setGroupingMode: (value: DashboardGroupingMode) => void;
  setSortKey: (value: string) => void;
  setSortDirection: (value: string) => void;
  onBeforeAutoFocus?: () => boolean;
  columnVisibility?: {
    availableColumns: ColumnDef[];
    isColumnHidden: (id: string) => boolean;
    onColumnToggle: (id: string) => void;
    onColumnReset?: () => void;
  };
  hostFilter?: DashboardToolbarFilterConfig;
  namespaceFilter?: DashboardToolbarFilterConfig;
  containerRuntimeFilter?: DashboardToolbarFilterConfig;
  chartsCollapsed?: () => boolean;
  onChartsToggle?: () => void;
}

export interface CountActiveDashboardFiltersOptions {
  search: string;
  viewMode: ViewMode;
  statusMode: DashboardStatusMode;
  hostFilterValue?: string;
  namespaceFilterValue?: string;
}

export interface HasActiveDashboardFiltersOptions extends CountActiveDashboardFiltersOptions {
  groupingMode: DashboardGroupingMode;
}

export const DEFAULT_DASHBOARD_SORT_KEY = 'name';
export const DEFAULT_DASHBOARD_SORT_DIRECTION = 'asc';
export const DEFAULT_DASHBOARD_VIEW_MODE: ViewMode = 'all';
export const DEFAULT_DASHBOARD_STATUS_MODE: DashboardStatusMode = 'all';
export const DEFAULT_DASHBOARD_GROUPING_MODE: DashboardGroupingMode = 'grouped';

export const countActiveDashboardFilters = (
  options: CountActiveDashboardFiltersOptions,
): number => {
  let count = 0;

  if (options.search.trim() !== '') count++;
  if (options.viewMode !== DEFAULT_DASHBOARD_VIEW_MODE) count++;
  if (options.statusMode !== DEFAULT_DASHBOARD_STATUS_MODE) count++;
  if ((options.hostFilterValue ?? '') !== '') count++;
  if ((options.namespaceFilterValue ?? '') !== '') count++;

  return count;
};

export const hasActiveDashboardFilters = (
  options: HasActiveDashboardFiltersOptions,
): boolean =>
  countActiveDashboardFilters(options) > 0 ||
  options.groupingMode !== DEFAULT_DASHBOARD_GROUPING_MODE;
