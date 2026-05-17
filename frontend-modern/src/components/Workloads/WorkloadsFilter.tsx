import { Component, Show, createMemo } from 'solid-js';
import XIcon from 'lucide-solid/icons/x';
import { ColumnPicker } from '@/components/shared/ColumnPicker';
import { FilterBar, type FilterDef } from '@/components/shared/FilterBar';
import { ChartVisibilityToggleButton, FilterActionButton } from '@/components/shared/FilterToolbar';
import { GroupedTableModeSegmentedControl } from '@/components/shared/GroupedTableModeSegmentedControl';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { isContainerWorkloadViewMode } from '@/utils/workloads';
import type { ViewMode } from '@/types/workloads';
import { MetricDisplayModeSegmentedControl } from './MetricDisplayModeSegmentedControl';
import type { WorkloadsFilterProps, WorkloadsStatusMode } from './workloadsFilterModel';
import {
  DEFAULT_WORKLOADS_GROUPING_MODE,
  DEFAULT_WORKLOADS_SORT_DIRECTION,
  DEFAULT_WORKLOADS_SORT_KEY,
  DEFAULT_WORKLOADS_STATUS_MODE,
  DEFAULT_WORKLOADS_VIEW_MODE,
  hasActiveWorkloadsFilters,
} from './workloadsFilterModel';
import { WORKLOAD_STATUS_FILTER_OPTIONS, WORKLOAD_TYPE_OPTIONS } from './workloadFilterConfigModel';

export const WorkloadsFilter: Component<WorkloadsFilterProps> = (props) => {
  const { isMobile } = useBreakpoint();

  const typeValue = () =>
    isContainerWorkloadViewMode(props.viewMode()) ? 'container' : props.viewMode();

  const showRuntimeFilter = () =>
    isContainerWorkloadViewMode(props.viewMode()) && Boolean(props.containerRuntimeFilter);

  const showClearAll = createMemo(() =>
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

  const handleClearAll = () => {
    props.setSearch('');
    props.setSortKey(DEFAULT_WORKLOADS_SORT_KEY);
    props.setSortDirection(DEFAULT_WORKLOADS_SORT_DIRECTION);
    props.setViewMode(DEFAULT_WORKLOADS_VIEW_MODE);
    props.setStatusMode(DEFAULT_WORKLOADS_STATUS_MODE);
    props.setGroupingMode(DEFAULT_WORKLOADS_GROUPING_MODE);
    props.hostFilter?.onChange('');
    props.platformFilter?.onChange('');
    props.namespaceFilter?.onChange('');
    props.containerRuntimeFilter?.onChange('');
  };

  const buildFilters = (): FilterDef[] => {
    const filters: FilterDef[] = [
      {
        id: 'workloads-type',
        label: 'Type',
        group: 'properties',
        value: typeValue,
        setValue: (value: string) => props.setViewMode(value as ViewMode),
        defaultValue: DEFAULT_WORKLOADS_VIEW_MODE,
        options: () =>
          WORKLOAD_TYPE_OPTIONS.map((option) => ({
            value: option.value,
            label: option.label,
          })),
      },
      {
        id: 'workloads-status',
        label: 'Status',
        group: 'status',
        value: props.statusMode,
        setValue: (value: string) => props.setStatusMode(value as WorkloadsStatusMode),
        defaultValue: DEFAULT_WORKLOADS_STATUS_MODE,
        options: () =>
          WORKLOAD_STATUS_FILTER_OPTIONS.map((option) => ({
            value: option.value,
            label: option.label,
          })),
      },
    ];

    const hostFilter = props.hostFilter;
    if (hostFilter) {
      filters.push({
        id: hostFilter.id ?? 'workloads-host',
        label: hostFilter.label ?? 'Agent',
        group: 'scope',
        value: () => hostFilter.value,
        setValue: hostFilter.onChange,
        defaultValue: '',
        options: () => hostFilter.options,
      });
    }

    const platformFilter = props.platformFilter;
    if (platformFilter) {
      filters.push({
        id: platformFilter.id ?? 'workloads-platform',
        label: platformFilter.label ?? 'Platform',
        group: 'scope',
        value: () => platformFilter.value,
        setValue: platformFilter.onChange,
        defaultValue: '',
        options: () => platformFilter.options,
      });
    }

    const namespaceFilter = props.namespaceFilter;
    if (namespaceFilter) {
      filters.push({
        id: namespaceFilter.id ?? 'workloads-namespace',
        label: namespaceFilter.label ?? 'Namespace',
        group: 'scope',
        value: () => namespaceFilter.value,
        setValue: namespaceFilter.onChange,
        defaultValue: '',
        options: () => namespaceFilter.options,
      });
    }

    const runtimeFilter = props.containerRuntimeFilter;
    if (runtimeFilter && showRuntimeFilter()) {
      filters.push({
        id: runtimeFilter.id ?? 'workloads-runtime',
        label: runtimeFilter.label ?? 'Runtime',
        group: 'properties',
        value: () => runtimeFilter.value,
        setValue: runtimeFilter.onChange,
        defaultValue: '',
        options: () => runtimeFilter.options,
      });
    }

    return filters;
  };

  return (
    <FilterBar
      role="group"
      ariaLabel="Workloads filters"
      isMobile={isMobile}
      savedViewsKey="workloads"
      search={{
        value: props.search,
        setValue: props.setSearch,
        placeholder: 'Search or filter...',
        historyKey: STORAGE_KEYS.WORKLOADS_SEARCH_HISTORY,
        onBeforeAutoFocus: props.onBeforeAutoFocus,
      }}
      searchTrailing={props.searchTrailing}
      filters={buildFilters()}
      viewOptionsTrailing={
        <>
          <Show when={props.pinnedSelectionActive?.() && props.onClearPinnedSelection}>
            <FilterActionButton
              aria-label="Clear pinned selection"
              title="Clear pinned selection"
              onClick={() => props.onClearPinnedSelection?.()}
            >
              <XIcon class="h-3 w-3" />
              Clear selection
            </FilterActionButton>
          </Show>
          <Show when={props.metricDisplayMode && props.setMetricDisplayMode}>
            <MetricDisplayModeSegmentedControl
              value={props.metricDisplayMode!()}
              onChange={props.setMetricDisplayMode!}
              range={props.metricHistoryRange?.()}
              onRangeChange={props.setMetricHistoryRange}
            />
          </Show>
          <GroupedTableModeSegmentedControl
            value={props.groupingMode()}
            onChange={props.setGroupingMode}
          />
          <Show when={props.onChartsToggle}>
            <ChartVisibilityToggleButton
              collapsed={props.chartsCollapsed?.() ?? false}
              onToggle={() => props.onChartsToggle?.()}
            />
          </Show>
          <Show when={props.columnVisibility}>
            {(visibility) => (
              <ColumnPicker
                columns={visibility().availableColumns}
                isHidden={visibility().isColumnHidden}
                onToggle={visibility().onColumnToggle}
                onReset={visibility().onColumnReset}
              />
            )}
          </Show>
        </>
      }
      onClearAll={handleClearAll}
      showClearAll={showClearAll}
    />
  );
};
