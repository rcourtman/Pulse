import { Component, For, Show } from 'solid-js';
import { Card } from '@/components/shared/Card';
import {
  ChartVisibilityToggleButton,
  LabeledFilterSelect,
  LabeledFilterToggleGroup,
} from '@/components/shared/FilterToolbar';
import { GroupedTableModeSegmentedControl } from '@/components/shared/GroupedTableModeSegmentedControl';
import { PageControls } from '@/components/shared/PageControls';
import { SearchInput } from '@/components/shared/SearchInput';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { isContainerWorkloadViewMode } from '@/utils/workloads';
import type { ViewMode } from '@/types/workloads';
import type { WorkloadsFilterProps } from './workloadsFilterModel';
import { WORKLOAD_STATUS_FILTER_OPTIONS, WORKLOAD_TYPE_OPTIONS } from './workloadFilterConfigModel';
import { useWorkloadsFilterState } from './useWorkloadsFilterState';

export const WorkloadsFilter: Component<WorkloadsFilterProps> = (props) => {
  const {
    activeFilterCount,
    isMobile,
    pageControlsColumnVisibility,
    resetFilters,
    showReset,
    showToolbarFilters,
    toggleFilters,
  } = useWorkloadsFilterState(props);
  const runtimeFilter = () =>
    isContainerWorkloadViewMode(props.viewMode()) ? props.containerRuntimeFilter : undefined;
  const typeFilterValue = () =>
    isContainerWorkloadViewMode(props.viewMode()) ? 'container' : props.viewMode();
  const hasSecondaryFilters = () =>
    Boolean(props.hostFilter || props.platformFilter || props.namespaceFilter || runtimeFilter());

  return (
    <Card class="workloads-filter mb-4" padding="sm">
      <PageControls
        search={
          <SearchInput
            value={props.search}
            onChange={props.setSearch}
            placeholder="Search or filter..."
            class="w-full"
            onBeforeAutoFocus={props.onBeforeAutoFocus}
            typeToSearch
            history={{ storageKey: STORAGE_KEYS.WORKLOADS_SEARCH_HISTORY }}
          />
        }
        searchTrailing={props.searchTrailing}
        mobileFilters={{
          enabled: isMobile(),
          onToggle: toggleFilters,
          count: activeFilterCount(),
        }}
        mobileTrailing={props.mobileTrailing}
        columnVisibility={pageControlsColumnVisibility()}
        resetAction={{
          show: showReset(),
          onClick: resetFilters,
          title: 'Reset all filters',
          class: 'text-base-content',
          icon: (
            <svg
              width="12"
              height="12"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8" />
              <path d="M21 3v5h-5" />
              <path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16" />
              <path d="M8 16H3v5" />
            </svg>
          ),
        }}
        showFilters={showToolbarFilters()}
        actionsLayout="stacked"
        toolbarActionsClass="page-controls-toolbar-actions inline-flex max-w-full flex-wrap items-center gap-2 rounded-md bg-surface-hover p-0.5"
        toolbarTrailing={
          <>
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
          </>
        }
      >
        <div class="workloads-filter-control-stack flex w-full min-w-0 flex-col gap-1.5">
          <div class="workloads-filter-primary-controls flex min-w-0 flex-wrap items-center gap-1.5 xl:flex-col xl:items-start">
            <LabeledFilterToggleGroup
              id="workloads-type-filter"
              label="Type"
              value={typeFilterValue()}
              onChange={(value) => props.setViewMode(value as ViewMode)}
              selectClass="min-w-[7rem]"
              options={WORKLOAD_TYPE_OPTIONS}
            />

            <LabeledFilterToggleGroup
              id="workloads-status-filter"
              label="Status"
              value={props.statusMode()}
              onChange={(value) =>
                props.setStatusMode(value as 'all' | 'running' | 'degraded' | 'stopped')
              }
              options={WORKLOAD_STATUS_FILTER_OPTIONS}
            />
          </div>

          <Show when={hasSecondaryFilters()}>
            <div class="workloads-filter-secondary-controls flex min-w-0 flex-wrap items-center gap-2 rounded-md bg-surface-alt/60 p-1">
              <Show when={props.hostFilter}>
                {(hostFilter) => (
                  <LabeledFilterSelect
                    id={hostFilter().id ?? 'workloads-host-filter'}
                    label={hostFilter().label ?? 'Agent'}
                    value={hostFilter().value}
                    onChange={(e) => hostFilter().onChange(e.currentTarget.value)}
                    selectClass="min-w-[8rem]"
                  >
                    <For each={hostFilter().options}>
                      {(option) => <option value={option.value}>{option.label}</option>}
                    </For>
                  </LabeledFilterSelect>
                )}
              </Show>

              <Show when={props.platformFilter}>
                {(platformFilter) => (
                  <LabeledFilterSelect
                    id={platformFilter().id ?? 'workloads-platform-filter'}
                    label={platformFilter().label ?? 'Platform'}
                    value={platformFilter().value}
                    onChange={(e) => platformFilter().onChange(e.currentTarget.value)}
                    selectClass="min-w-[8rem]"
                  >
                    <For each={platformFilter().options}>
                      {(option) => <option value={option.value}>{option.label}</option>}
                    </For>
                  </LabeledFilterSelect>
                )}
              </Show>

              <Show when={props.namespaceFilter}>
                {(namespaceFilter) => (
                  <LabeledFilterSelect
                    id={namespaceFilter().id ?? 'workloads-namespace-filter'}
                    label={namespaceFilter().label ?? 'Namespace'}
                    value={namespaceFilter().value}
                    onChange={(e) => namespaceFilter().onChange(e.currentTarget.value)}
                    selectClass="min-w-[8rem]"
                  >
                    <For each={namespaceFilter().options}>
                      {(option) => <option value={option.value}>{option.label}</option>}
                    </For>
                  </LabeledFilterSelect>
                )}
              </Show>

              <Show when={runtimeFilter()}>
                {(activeRuntimeFilter) => (
                  <LabeledFilterSelect
                    id={activeRuntimeFilter().id ?? 'workloads-runtime-filter'}
                    label={activeRuntimeFilter().label ?? 'Runtime'}
                    value={activeRuntimeFilter().value}
                    onChange={(e) => activeRuntimeFilter().onChange(e.currentTarget.value)}
                    selectClass="min-w-[7rem]"
                  >
                    <For each={activeRuntimeFilter().options}>
                      {(option) => <option value={option.value}>{option.label}</option>}
                    </For>
                  </LabeledFilterSelect>
                )}
              </Show>
            </div>
          </Show>
        </div>
      </PageControls>
    </Card>
  );
};
