import { Component, Show, createMemo } from 'solid-js';
import BoxIcon from 'lucide-solid/icons/box';
import BoxesIcon from 'lucide-solid/icons/boxes';
import MonitorIcon from 'lucide-solid/icons/monitor';
import XIcon from 'lucide-solid/icons/x';
import { ColumnPicker } from '@/components/shared/ColumnPicker';
import { FilterBar, type FilterDef, type FilterSelectOption } from '@/components/shared/FilterBar';
import { ChartVisibilityToggleButton, FilterActionButton } from '@/components/shared/FilterToolbar';
import { GroupedTableModeSegmentedControl } from '@/components/shared/GroupedTableModeSegmentedControl';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { isContainerWorkloadViewMode } from '@/utils/workloads';
import type { ViewMode } from '@/types/workloads';
import { MetricDisplayModeSegmentedControl } from './MetricDisplayModeSegmentedControl';
import type { WorkloadsFilterProps, WorkloadsStatusMode } from './workloadsFilterModel';
import {
  DEFAULT_WORKLOADS_SORT_DIRECTION,
  DEFAULT_WORKLOADS_SORT_KEY,
  DEFAULT_WORKLOADS_STATUS_MODE,
  DEFAULT_WORKLOADS_VIEW_MODE,
  hasActiveWorkloadsFilters,
} from './workloadsFilterModel';
import { WORKLOAD_STATUS_FILTER_OPTIONS, WORKLOAD_TYPE_OPTIONS } from './workloadFilterConfigModel';

const PROXMOX_PLATFORM_FILTER = 'proxmox-pve';

const statusDot = (className: string) => <span class={`h-2 w-2 rounded-full ${className}`} />;

export const WorkloadsFilter: Component<WorkloadsFilterProps> = (props) => {
  const { isMobile } = useBreakpoint();

  const typeValue = () =>
    isContainerWorkloadViewMode(props.viewMode()) ? 'container' : props.viewMode();

  const isProxmoxScope = () =>
    (props.forcedPlatform ?? '').trim().toLowerCase() === PROXMOX_PLATFORM_FILTER;

  const workloadTypeOptions = (): FilterSelectOption[] =>
    (isProxmoxScope()
      ? WORKLOAD_TYPE_OPTIONS.filter(
          (option) =>
            option.value === 'all' || option.value === 'vm' || option.value === 'container',
        ).map((option) => (option.value === 'container' ? { ...option, label: 'LXCs' } : option))
      : WORKLOAD_TYPE_OPTIONS
    ).map((option) => ({
      value: option.value,
      label: option.label,
      icon:
        option.value === 'vm'
          ? MonitorIcon
          : option.value === 'container'
            ? BoxIcon
            : option.value === 'pod'
              ? BoxesIcon
              : undefined,
      tone: option.value === 'vm' ? 'info' : option.value === 'container' ? 'success' : undefined,
    }));

  const workloadStatusOptions = (): FilterSelectOption[] =>
    (props.statusOptions ?? WORKLOAD_STATUS_FILTER_OPTIONS).map((option) => ({
      value: option.value,
      label: option.label,
      leading:
        option.value === 'running'
          ? statusDot('bg-emerald-500')
          : option.value === 'degraded'
            ? statusDot('bg-amber-500')
            : option.value === 'stopped'
              ? statusDot('bg-red-500')
              : undefined,
      tone:
        option.value === 'running'
          ? 'success'
          : option.value === 'degraded'
            ? 'warning'
            : option.value === 'stopped'
              ? 'danger'
              : undefined,
    }));

  const showRuntimeFilter = () =>
    isContainerWorkloadViewMode(props.viewMode()) && Boolean(props.containerRuntimeFilter);

  const showClearAll = createMemo(() =>
    hasActiveWorkloadsFilters({
      search: props.search(),
      viewMode: props.suppressTypeFilter ? DEFAULT_WORKLOADS_VIEW_MODE : props.viewMode(),
      statusMode: props.statusMode(),
      hostFilterValue: props.hostFilter?.value,
      platformFilterValue: props.platformFilter?.value,
      namespaceFilterValue: props.namespaceFilter?.value,
      containerRuntimeFilterValue: props.containerRuntimeFilter?.value,
    }),
  );

  const handleClearAll = () => {
    props.setSearch('');
    props.setSortKey(props.defaultSortKey ?? DEFAULT_WORKLOADS_SORT_KEY);
    props.setSortDirection(DEFAULT_WORKLOADS_SORT_DIRECTION);
    if (!props.suppressTypeFilter) {
      props.setViewMode(DEFAULT_WORKLOADS_VIEW_MODE);
    }
    props.setStatusMode(DEFAULT_WORKLOADS_STATUS_MODE);
    props.hostFilter?.onChange('');
    props.platformFilter?.onChange('');
    props.namespaceFilter?.onChange('');
    props.containerRuntimeFilter?.onChange('');
  };

  const buildFilters = (): FilterDef[] => {
    const filters: FilterDef[] = [];

    if (!props.suppressTypeFilter) {
      filters.push({
        id: 'workloads-type',
        label: 'Type',
        group: 'properties',
        inline: true,
        value: typeValue,
        setValue: (value: string) => props.setViewMode(value as ViewMode),
        defaultValue: DEFAULT_WORKLOADS_VIEW_MODE,
        options: workloadTypeOptions,
      });
    }

    filters.push({
      id: 'workloads-status',
      label: 'Status',
      group: 'status',
      inline: true,
      value: props.statusMode,
      setValue: (value: string) => props.setStatusMode(value as WorkloadsStatusMode),
      defaultValue: DEFAULT_WORKLOADS_STATUS_MODE,
      options: workloadStatusOptions,
    });

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
        inline: true,
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
      ariaLabel={props.ariaLabel ?? 'Workloads filters'}
      isMobile={isMobile}
      search={{
        value: props.search,
        setValue: props.setSearch,
        placeholder: props.searchPlaceholder ?? 'Search workloads by name, ID, node, or image',
        historyKey: STORAGE_KEYS.WORKLOADS_SEARCH_HISTORY,
        emptyMessage: props.searchEmptyMessage ?? 'Recent workload searches appear here.',
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
          <GroupedTableModeSegmentedControl
            value={props.groupingMode()}
            onChange={props.setGroupingMode}
          />
          <Show when={props.metricDisplayMode && props.setMetricDisplayMode}>
            <MetricDisplayModeSegmentedControl
              value={props.metricDisplayMode!()}
              onChange={props.setMetricDisplayMode!}
              range={props.metricHistoryRange?.()}
              onRangeChange={props.setMetricHistoryRange}
            />
          </Show>
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
