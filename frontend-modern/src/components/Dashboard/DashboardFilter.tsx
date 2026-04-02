import { Component, For, Show } from 'solid-js';
import { Card } from '@/components/shared/Card';
import {
  FilterSegmentedControl,
  LabeledFilterSelect,
} from '@/components/shared/FilterToolbar';
import { PageControls } from '@/components/shared/PageControls';
import { SearchInput } from '@/components/shared/SearchInput';
import { STORAGE_KEYS } from '@/utils/localStorage';
import type { DashboardFilterProps } from './dashboardFilterModel';
import { useDashboardFilterState } from './useDashboardFilterState';

export const DashboardFilter: Component<DashboardFilterProps> = (props) => {
  const {
    activeFilterCount,
    isMobile,
    pageControlsColumnVisibility,
    resetFilters,
    showReset,
    showToolbarFilters,
    toggleFilters,
  } = useDashboardFilterState(props);

  return (
    <Card class="dashboard-filter mb-4" padding="sm">
      <PageControls
        search={
          <SearchInput
            value={props.search}
            onChange={props.setSearch}
            placeholder="Search or filter..."
            class="w-full"
            onBeforeAutoFocus={props.onBeforeAutoFocus}
            typeToSearch
            history={{ storageKey: STORAGE_KEYS.DASHBOARD_SEARCH_HISTORY }}
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
          class: 'ml-auto text-base-content',
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
        toolbarClass="lg:flex-nowrap"
      >
        <Show when={props.hostFilter}>
          {(hostFilter) => (
            <LabeledFilterSelect
              id={hostFilter().id ?? 'dashboard-host-filter'}
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
              id={platformFilter().id ?? 'dashboard-platform-filter'}
              label={platformFilter().label ?? 'Platform'}
              value={platformFilter().value}
              onChange={(e) => platformFilter().onChange(e.currentTarget.value)}
              selectClass="min-w-[8rem]"
            >
              <For each={platformFilter().options}>
                {(option) => (
                  <option
                    value={option.value}
                    selected={option.value === platformFilter().value}
                  >
                    {option.label}
                  </option>
                )}
              </For>
            </LabeledFilterSelect>
          )}
        </Show>

        <Show when={props.namespaceFilter}>
          {(namespaceFilter) => (
            <LabeledFilterSelect
              id={namespaceFilter().id ?? 'dashboard-namespace-filter'}
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

        <Show
          when={props.viewMode() === 'app-container' ? props.containerRuntimeFilter : undefined}
        >
          {(runtimeFilter) => (
            <LabeledFilterSelect
              id={runtimeFilter().id ?? 'dashboard-runtime-filter'}
              label={runtimeFilter().label ?? 'Runtime'}
              value={runtimeFilter().value}
              onChange={(e) => runtimeFilter().onChange(e.currentTarget.value)}
              selectClass="min-w-[7rem]"
            >
              <For each={runtimeFilter().options}>
                {(option) => <option value={option.value}>{option.label}</option>}
              </For>
            </LabeledFilterSelect>
          )}
        </Show>

        <LabeledFilterSelect
          id="dashboard-type-filter"
          label="Type"
          value={props.viewMode()}
          onChange={(event) =>
            props.setViewMode(
              event.currentTarget.value as
                | 'all'
                | 'vm'
                | 'system-container'
                | 'app-container'
                | 'pod',
            )
          }
          selectClass="min-w-[7rem]"
        >
          <option value="all">All</option>
          <option value="vm">VMs</option>
          <option value="system-container">System Containers</option>
          <option value="app-container">App Containers</option>
          <option value="pod">Pods</option>
        </LabeledFilterSelect>

        <LabeledFilterSelect
          id="dashboard-status-filter"
          label="Status"
          value={props.statusMode()}
          onChange={(event) =>
            props.setStatusMode(
              event.currentTarget.value as 'all' | 'running' | 'degraded' | 'stopped',
            )
          }
          selectClass="min-w-[8rem]"
        >
          <option value="all">All</option>
          <option value="running">Running</option>
          <option value="degraded">Degraded</option>
          <option value="stopped">Stopped</option>
        </LabeledFilterSelect>

        <FilterSegmentedControl
          value={props.groupingMode()}
          onChange={(value) => props.setGroupingMode(value as 'grouped' | 'flat')}
          aria-label="Group By"
          options={[
            {
              value: 'grouped',
              title: 'Group by node',
              label: (
                <>
                  <svg
                    class="w-3 h-3"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2v11z" />
                  </svg>
                  Grouped
                </>
              ),
            },
            {
              value: 'flat',
              title: 'Flat list view',
              label: (
                <>
                  <svg
                    class="w-3 h-3"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <line x1="8" y1="6" x2="21" y2="6" />
                    <line x1="8" y1="12" x2="21" y2="12" />
                    <line x1="8" y1="18" x2="21" y2="18" />
                    <line x1="3" y1="6" x2="3.01" y2="6" />
                    <line x1="3" y1="12" x2="3.01" y2="12" />
                    <line x1="3" y1="18" x2="3.01" y2="18" />
                  </svg>
                  List
                </>
              ),
            },
          ]}
        />

        <Show when={props.onChartsToggle}>
          <FilterSegmentedControl
            class="hidden lg:inline-flex"
            value={props.chartsCollapsed?.() ? 'hidden' : 'shown'}
            onChange={() => props.onChartsToggle?.()}
            aria-label="Charts"
            options={[
              {
                value: 'shown',
                title: props.chartsCollapsed?.() ? 'Show charts' : 'Hide charts',
                label: (
                  <>
                    <svg
                      class="w-3 h-3"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="2"
                    >
                      <polyline points="22 12 18 12 15 21 9 3 6 12 2 12" />
                    </svg>
                    Charts
                  </>
                ),
              },
            ]}
          />
        </Show>

      </PageControls>
    </Card>
  );
};
