import { createMemo, createSignal } from 'solid-js';

import { useBreakpoint } from '@/hooks/useBreakpoint';

import {
  countActiveWorkloadsFilters,
  DEFAULT_WORKLOADS_GROUPING_MODE,
  DEFAULT_WORKLOADS_SORT_DIRECTION,
  DEFAULT_WORKLOADS_SORT_KEY,
  DEFAULT_WORKLOADS_STATUS_MODE,
  DEFAULT_WORKLOADS_VIEW_MODE,
  hasActiveWorkloadsFilters,
  type WorkloadsFilterProps,
} from './workloadsFilterModel';

export const useWorkloadsFilterState = (props: WorkloadsFilterProps) => {
  const { isMobile } = useBreakpoint();
  const [filtersOpen, setFiltersOpen] = createSignal(false);

  const activeFilterCount = createMemo(() =>
    countActiveWorkloadsFilters({
      search: props.search(),
      viewMode: props.viewMode(),
      statusMode: props.statusMode(),
      hostFilterValue: props.hostFilter?.value,
      platformFilterValue: props.platformFilter?.value,
      namespaceFilterValue: props.namespaceFilter?.value,
    }),
  );

  const showReset = createMemo(() =>
    hasActiveWorkloadsFilters({
      search: props.search(),
      viewMode: props.viewMode(),
      statusMode: props.statusMode(),
      groupingMode: props.groupingMode(),
      hostFilterValue: props.hostFilter?.value,
      platformFilterValue: props.platformFilter?.value,
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
    props.setSortKey(DEFAULT_WORKLOADS_SORT_KEY);
    props.setSortDirection(DEFAULT_WORKLOADS_SORT_DIRECTION);
    props.setViewMode(DEFAULT_WORKLOADS_VIEW_MODE);
    props.setStatusMode(DEFAULT_WORKLOADS_STATUS_MODE);
    props.setGroupingMode(DEFAULT_WORKLOADS_GROUPING_MODE);

    props.hostFilter?.onChange('');
    props.platformFilter?.onChange('');
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
