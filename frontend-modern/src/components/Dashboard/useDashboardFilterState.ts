import { createMemo, createSignal } from 'solid-js';

import { useBreakpoint } from '@/hooks/useBreakpoint';

import {
  countActiveDashboardFilters,
  DEFAULT_DASHBOARD_GROUPING_MODE,
  DEFAULT_DASHBOARD_SORT_DIRECTION,
  DEFAULT_DASHBOARD_SORT_KEY,
  DEFAULT_DASHBOARD_STATUS_MODE,
  DEFAULT_DASHBOARD_VIEW_MODE,
  hasActiveDashboardFilters,
  type DashboardFilterProps,
} from './dashboardFilterModel';

export const useDashboardFilterState = (props: DashboardFilterProps) => {
  const { isMobile } = useBreakpoint();
  const [filtersOpen, setFiltersOpen] = createSignal(false);

  const activeFilterCount = createMemo(() =>
    countActiveDashboardFilters({
      search: props.search(),
      viewMode: props.viewMode(),
      statusMode: props.statusMode(),
      hostFilterValue: props.hostFilter?.value,
      namespaceFilterValue: props.namespaceFilter?.value,
    }),
  );

  const showReset = createMemo(() =>
    hasActiveDashboardFilters({
      search: props.search(),
      viewMode: props.viewMode(),
      statusMode: props.statusMode(),
      groupingMode: props.groupingMode(),
      hostFilterValue: props.hostFilter?.value,
      namespaceFilterValue: props.namespaceFilter?.value,
    }),
  );

  const pageControlsColumnVisibility = createMemo(() =>
    props.columnVisibility
      ? {
          availableToggles: () => props.columnVisibility!.availableColumns,
          isHiddenByUser: props.columnVisibility!.isColumnHidden,
          toggle: props.columnVisibility!.onColumnToggle,
          resetToDefaults: props.columnVisibility!.onColumnReset ?? (() => undefined),
        }
      : undefined,
  );

  const toggleFilters = () => {
    setFiltersOpen((open) => !open);
  };

  const resetFilters = () => {
    props.setSearch('');
    props.setSortKey(DEFAULT_DASHBOARD_SORT_KEY);
    props.setSortDirection(DEFAULT_DASHBOARD_SORT_DIRECTION);
    props.setViewMode(DEFAULT_DASHBOARD_VIEW_MODE);
    props.setStatusMode(DEFAULT_DASHBOARD_STATUS_MODE);
    props.setGroupingMode(DEFAULT_DASHBOARD_GROUPING_MODE);

    props.hostFilter?.onChange('');
    props.namespaceFilter?.onChange('');
  };

  return {
    activeFilterCount,
    filtersOpen,
    isMobile,
    pageControlsColumnVisibility,
    resetFilters,
    setFiltersOpen,
    showReset,
    showToolbarFilters: createMemo(() => !isMobile() || filtersOpen()),
    toggleFilters,
  };
};
