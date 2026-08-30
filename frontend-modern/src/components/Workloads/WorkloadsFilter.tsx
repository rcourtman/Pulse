import { Component, JSX, Show, createMemo } from 'solid-js';
import BoxIcon from 'lucide-solid/icons/box';
import BoxesIcon from 'lucide-solid/icons/boxes';
import MonitorIcon from 'lucide-solid/icons/monitor';
import XIcon from 'lucide-solid/icons/x';
import { ColumnPicker } from '@/components/shared/ColumnPicker';
import {
  FilterBar,
  filterChipStatusDot,
  type FilterDef,
  type FilterSelectOption,
} from '@/components/shared/FilterBar';
import {
  ChartVisibilityToggleButton,
  FilterActionButton,
  FilterSegmentedControl,
} from '@/components/shared/FilterToolbar';
import { GroupedTableModeSegmentedControl } from '@/components/shared/GroupedTableModeSegmentedControl';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import {
  PLATFORM_ESTATE_COUNTS_STORAGE_KEY,
  deserializePlatformEstateCountsVisibility,
} from '@/features/platformPage/platformEstateOverviewModel';
import { STORAGE_KEYS } from '@/utils/localStorage';
import {
  normalizeSourcePlatformQueryValue,
  sourcePlatformScopeMatchesFilter,
} from '@/utils/sourcePlatforms';
import { isContainerWorkloadViewMode } from '@/utils/workloads';
import type { ViewMode } from '@/types/workloads';
import {
  MetricDisplayModeSegmentedControl,
  MetricHistoryRangeSegmentedControl,
  MetricHoverModeSegmentedControl,
} from './MetricDisplayModeSegmentedControl';
import type {
  WorkloadsFilterProps,
  WorkloadsMemoryDisplayBasis,
  WorkloadsStatusMode,
} from './workloadsFilterModel';
import { buildWorkloadSearchSuggestions } from './workloadSearchSuggestions';
import {
  DEFAULT_WORKLOADS_SORT_DIRECTION,
  DEFAULT_WORKLOADS_SORT_KEY,
  DEFAULT_WORKLOADS_STATUS_MODE,
  DEFAULT_WORKLOADS_VIEW_MODE,
  hasActiveWorkloadsFilters,
} from './workloadsFilterModel';
import { WORKLOAD_STATUS_FILTER_OPTIONS, WORKLOAD_TYPE_OPTIONS } from './workloadFilterConfigModel';

export const WorkloadsFilter: Component<WorkloadsFilterProps> = (props) => {
  const { isMobile } = useBreakpoint();
  const persistedInventoryCounts = props.inventoryCountsVisible
    ? undefined
    : usePersistentSignal(PLATFORM_ESTATE_COUNTS_STORAGE_KEY, true, {
        deserialize: deserializePlatformEstateCountsVisibility,
      });
  const inventoryCountsVisible = () =>
    props.inventoryCountsVisible?.() ?? persistedInventoryCounts?.[0]() ?? true;
  const setInventoryCountsVisible = (visible: boolean) => {
    if (props.setInventoryCountsVisible) {
      props.setInventoryCountsVisible(visible);
      return;
    }
    persistedInventoryCounts?.[1](visible);
  };

  const typeValue = () =>
    isContainerWorkloadViewMode(props.viewMode()) ? 'container' : props.viewMode();

  const isProxmoxScope = () => {
    const forcedPlatform = normalizeSourcePlatformQueryValue(props.forcedPlatform);
    return (
      forcedPlatform !== '' &&
      forcedPlatform !== 'all' &&
      sourcePlatformScopeMatchesFilter('proxmox-pve', forcedPlatform)
    );
  };

  const workloadTypeCount = (value: string): number | undefined => {
    if (!inventoryCountsVisible() || !props.inventoryStats) return undefined;
    const stats = props.inventoryStats();
    if (value === 'all') return stats.total;
    if (value === 'vm') return stats.vms;
    if (value === 'container') return stats.containers + stats.appContainers;
    if (value === 'pod') return stats.pods;
    return undefined;
  };

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
      count: workloadTypeCount(option.value),
    }));

  const workloadStatusCount = (value: string): number | undefined => {
    if (!inventoryCountsVisible() || !props.inventoryStats) return undefined;
    const stats = props.inventoryStats();
    if (value === 'all') return stats.total;
    if (value === 'running') return stats.running;
    if (value === 'degraded') return stats.degraded;
    if (value === 'stopped') return stats.stopped;
    return undefined;
  };

  const workloadStatusOptions = (): FilterSelectOption[] =>
    (props.statusOptions ?? WORKLOAD_STATUS_FILTER_OPTIONS).map((option) => ({
      value: option.value,
      label: option.label,
      leading:
        option.value === 'running'
          ? filterChipStatusDot('bg-emerald-500')
          : option.value === 'degraded'
            ? filterChipStatusDot('bg-amber-500')
            : option.value === 'stopped'
              ? filterChipStatusDot('bg-red-500')
              : undefined,
      tone:
        option.value === 'running'
          ? 'success'
          : option.value === 'degraded'
            ? 'warning'
            : option.value === 'stopped'
              ? 'danger'
              : undefined,
      count: workloadStatusCount(option.value),
    }));

  const runtimeChipLabel = (value: string): string => {
    if (value === '') return 'All';
    if (value === 'docker') return 'Docker';
    if (value === 'podman') return 'Podman';
    return value;
  };

  const runtimeChipDot = (value: string): JSX.Element | undefined => {
    if (value === 'docker') return filterChipStatusDot('bg-sky-500');
    if (value === 'podman') return filterChipStatusDot('bg-violet-500');
    return undefined;
  };

  const workloadRuntimeOptions = (sourceOptions: FilterSelectOption[]): FilterSelectOption[] =>
    sourceOptions.map((option) => ({
      ...option,
      label: runtimeChipLabel(option.value),
      leading: runtimeChipDot(option.value),
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
      clusterFilterValue: props.clusterFilter?.value,
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
    props.clusterFilter?.onChange('');
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

    const clusterFilter = props.clusterFilter;
    if (clusterFilter) {
      filters.push({
        id: clusterFilter.id ?? 'workloads-cluster',
        label: clusterFilter.label ?? 'Cluster',
        group: 'scope',
        value: () => clusterFilter.value,
        setValue: clusterFilter.onChange,
        defaultValue: '',
        options: () => clusterFilter.options,
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
        options: () => workloadRuntimeOptions(runtimeFilter.options),
      });
    }

    return filters;
  };

  const searchSuggestions = createMemo(() =>
    buildWorkloadSearchSuggestions(props.searchSuggestionWorkloads?.() ?? []),
  );

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
        suggestions: searchSuggestions,
        onBeforeAutoFocus: props.onBeforeAutoFocus,
      }}
      searchTrailing={props.searchTrailing}
      filters={buildFilters()}
      showAddFilterLabel={false}
      leadingControls={
        props.pinnedSelectionActive?.() && props.onClearPinnedSelection ? (
          <FilterActionButton
            aria-label="Clear pinned selection"
            title="Clear pinned selection"
            onClick={() => props.onClearPinnedSelection?.()}
          >
            <XIcon class="h-3 w-3" />
            Clear selection
          </FilterActionButton>
        ) : undefined
      }
      viewOptions={
        <>
          <div>
            <div class="mb-1.5 text-[10px] font-semibold uppercase tracking-wide text-muted">
              Layout
            </div>
            <GroupedTableModeSegmentedControl
              value={props.groupingMode()}
              onChange={props.setGroupingMode}
            />
          </div>

          <Show when={props.metricDisplayMode && props.setMetricDisplayMode}>
            <div>
              <div class="mb-1.5 text-[10px] font-semibold uppercase tracking-wide text-muted">
                Metrics
              </div>
              <MetricDisplayModeSegmentedControl
                value={props.metricDisplayMode!()}
                onChange={props.setMetricDisplayMode!}
              />
            </div>
          </Show>

          <Show
            when={
              !isMobile() &&
              props.metricDisplayMode?.() === 'bars' &&
              props.metricHoverMode &&
              props.setMetricHoverMode
            }
          >
            <div>
              <div class="mb-1.5 text-[10px] font-semibold uppercase tracking-wide text-muted">
                Row hover
              </div>
              <MetricHoverModeSegmentedControl
                value={props.metricHoverMode!()}
                onChange={props.setMetricHoverMode!}
              />
            </div>
          </Show>

          <Show when={props.memoryDisplayBasis && props.setMemoryDisplayBasis}>
            <div>
              <div class="mb-1.5 text-[10px] font-semibold uppercase tracking-wide text-muted">
                Memory relative to
              </div>
              <FilterSegmentedControl
                aria-label="Memory percentage basis"
                value={props.memoryDisplayBasis!()}
                onChange={(value) =>
                  props.setMemoryDisplayBasis!(value as WorkloadsMemoryDisplayBasis)
                }
                options={[
                  {
                    value: 'guest',
                    label: 'Guest',
                    ariaLabel: 'Guest',
                    title: 'Show memory as a percentage of each guest allocation',
                  },
                  {
                    value: 'host',
                    label: 'Host',
                    ariaLabel: 'Host',
                    title: 'Show memory as a percentage of the Proxmox host total',
                  },
                ]}
              />
            </div>
          </Show>

          <Show when={props.inventoryStats}>
            <div>
              <div class="mb-1.5 text-[10px] font-semibold uppercase tracking-wide text-muted">
                Inventory totals
              </div>
              <FilterSegmentedControl
                aria-label="Inventory totals visibility"
                value={inventoryCountsVisible() ? 'shown' : 'hidden'}
                onChange={(value) => setInventoryCountsVisible(value === 'shown')}
                options={[
                  { value: 'shown', label: 'Show' },
                  { value: 'hidden', label: 'Hide' },
                ]}
              />
            </div>
          </Show>

          <Show when={props.onChartsToggle}>
            <div>
              <div class="mb-1.5 text-[10px] font-semibold uppercase tracking-wide text-muted">
                Summary
              </div>
              <ChartVisibilityToggleButton
                class="!inline-flex"
                collapsed={props.chartsCollapsed?.() ?? false}
                onToggle={() => props.onChartsToggle?.()}
              />
            </div>
          </Show>

          <Show when={props.columnVisibility}>
            {(visibility) => (
              <div>
                <div class="mb-1.5 text-[10px] font-semibold uppercase tracking-wide text-muted">
                  Table
                </div>
                <ColumnPicker
                  inline
                  columns={visibility().availableColumns}
                  isHidden={visibility().isColumnHidden}
                  onToggle={visibility().onColumnToggle}
                  onReset={visibility().onColumnReset}
                  showReset={visibility().showReset}
                  onResetWidths={visibility().onColumnWidthsReset}
                  hasManualWidths={visibility().hasColumnWidthOverrides}
                />
              </div>
            )}
          </Show>
        </>
      }
      trailingControls={
        <Show
          when={
            props.metricHistoryRange &&
            props.setMetricHistoryRange &&
            (props.metricDisplayMode?.() !== 'bars' ||
              (!isMobile() && (props.metricHoverMode?.() ?? 'history') === 'history'))
          }
        >
          <div class="flex items-center gap-2">
            <Show when={props.metricHistoryHintVisible?.()}>
              <span
                class="hidden whitespace-nowrap text-[11px] text-muted/80 lg:inline"
                data-testid="workload-history-hover-hint"
              >
                Hover a guest to preview history
              </span>
            </Show>
            <MetricHistoryRangeSegmentedControl
              label="History"
              range={props.metricHistoryRange!()}
              onRangeChange={props.setMetricHistoryRange!}
            />
          </div>
        </Show>
      }
      onClearAll={handleClearAll}
      showClearAll={showClearAll}
    />
  );
};
