import { Component, For, Show, createMemo, createSignal } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { SearchInput } from '@/components/shared/SearchInput';
import { ColumnPicker } from '@/components/shared/ColumnPicker';
import type { ColumnDef } from '@/hooks/useColumnVisibility';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { STORAGE_KEYS } from '@/utils/localStorage';
import ListFilterIcon from 'lucide-solid/icons/list-filter';
import { segmentedButtonClass } from '@/utils/segmentedButton';

interface DashboardFilterProps {
  search: () => string;
  setSearch: (value: string) => void;
  isSearchLocked: () => boolean;
  viewMode: () => 'all' | 'vm' | 'lxc' | 'docker' | 'k8s';
  setViewMode: (value: 'all' | 'vm' | 'lxc' | 'docker' | 'k8s') => void;
  statusMode: () => 'all' | 'running' | 'degraded' | 'stopped';
  setStatusMode: (value: 'all' | 'running' | 'degraded' | 'stopped') => void;
  groupingMode: () => 'grouped' | 'flat';
  setGroupingMode: (value: 'grouped' | 'flat') => void;
  setSortKey: (value: string) => void;
  setSortDirection: (value: string) => void;
  searchInputRef?: (el: HTMLInputElement) => void;
  onBeforeAutoFocus?: () => boolean;
  // Column visibility
  availableColumns?: ColumnDef[];
  isColumnHidden?: (id: string) => boolean;
  onColumnToggle?: (id: string) => void;
  onColumnReset?: () => void;
  hostFilter?: {
    id?: string;
    label?: string;
    value: string;
    options: { value: string; label: string }[];
    onChange: (value: string) => void;
  };
  namespaceFilter?: {
    id?: string;
    label?: string;
    value: string;
    options: { value: string; label: string }[];
    onChange: (value: string) => void;
  };
  containerRuntimeFilter?: {
    id?: string;
    label?: string;
    value: string;
    options: { value: string; label: string }[];
    onChange: (value: string) => void;
  };
  chartsCollapsed?: () => boolean;
  onChartsToggle?: () => void;
}

export const DashboardFilter: Component<DashboardFilterProps> = (props) => {
  const { isMobile } = useBreakpoint();
  const [filtersOpen, setFiltersOpen] = createSignal(false);

  const activeFilterCount = createMemo(() => {
    let count = 0;
    if (props.search().trim() !== '') count++;
    if (props.viewMode() !== 'all') count++;
    if (props.statusMode() !== 'all') count++;
    if (props.hostFilter && props.hostFilter.value !== '') count++;
    if (props.namespaceFilter && props.namespaceFilter.value !== '') count++;
    return count;
  });

  return (
    <Card class="dashboard-filter mb-4" padding="sm">
      <div class="flex flex-col gap-2">
        <SearchInput
          value={props.search}
          onChange={props.setSearch}
          placeholder="Search or filter..."
          class="w-full"
          inputRef={props.searchInputRef}
          onBeforeAutoFocus={props.onBeforeAutoFocus}
          autoFocus
          history={{ storageKey: STORAGE_KEYS.DASHBOARD_SEARCH_HISTORY }}
        />

        <Show when={isMobile()}>
          <button
            type="button"
            onClick={() => setFiltersOpen((o) => !o)}
            class="flex items-center gap-1.5 rounded-md bg-slate-100 dark:bg-slate-700 px-2.5 py-1.5 text-xs font-medium text-slate-600 dark:text-slate-400"
          >
            <ListFilterIcon class="w-3.5 h-3.5" />
            Filters
            <Show when={activeFilterCount() > 0}>
              <span class="ml-0.5 rounded-full bg-blue-500 px-1.5 py-0.5 text-[10px] font-semibold text-white leading-none">
                {activeFilterCount()}
              </span>
            </Show>
          </button>
        </Show>

        <Show when={!isMobile() || filtersOpen()}>
          <div class="flex flex-wrap items-center gap-2 text-xs text-slate-500 dark:text-slate-400 lg:flex-nowrap">
            <Show when={props.hostFilter}>
              {(hostFilter) => (
                <div class="inline-flex items-center rounded-md bg-slate-100 dark:bg-slate-700 p-0.5">
                  <label
                    for={hostFilter().id ?? 'dashboard-host-filter'}
                    class="px-1.5 text-[9px] font-semibold uppercase tracking-wide text-slate-400 dark:text-slate-500"
                  >
                    {hostFilter().label ?? 'Host'}
                  </label>
                  <select
                    id={hostFilter().id ?? 'dashboard-host-filter'}
                    value={hostFilter().value}
                    onChange={(e) => hostFilter().onChange(e.currentTarget.value)}
                    class="min-w-[8rem] rounded-md border border-slate-200 bg-white px-2 py-1 text-xs font-medium text-slate-900 shadow-sm transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100"
                  >
                    <For each={hostFilter().options}>
                      {(option) => (
                        <option value={option.value}>{option.label}</option>
                      )}
                    </For>
                  </select>
                </div>
              )}
            </Show>

            <Show when={props.namespaceFilter}>
              {(namespaceFilter) => (
                <div class="inline-flex items-center rounded-md bg-slate-100 dark:bg-slate-700 p-0.5">
                  <label
                    for={namespaceFilter().id ?? 'dashboard-namespace-filter'}
                    class="px-1.5 text-[9px] font-semibold uppercase tracking-wide text-slate-400 dark:text-slate-500"
                  >
                    {namespaceFilter().label ?? 'Namespace'}
                  </label>
                  <select
                    id={namespaceFilter().id ?? 'dashboard-namespace-filter'}
                    value={namespaceFilter().value}
                    onChange={(e) => namespaceFilter().onChange(e.currentTarget.value)}
                    class="min-w-[8rem] rounded-md border border-slate-200 bg-white px-2 py-1 text-xs font-medium text-slate-900 shadow-sm transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100"
                  >
                    <For each={namespaceFilter().options}>
                      {(option) => (
                        <option value={option.value}>{option.label}</option>
                      )}
                    </For>
                  </select>
                </div>
              )}
            </Show>

            <Show when={props.viewMode() === 'docker' ? props.containerRuntimeFilter : undefined}>
              {(runtimeFilter) => (
                <div class="inline-flex items-center rounded-md bg-slate-100 dark:bg-slate-700 p-0.5">
                  <label
                    for={runtimeFilter().id ?? 'dashboard-runtime-filter'}
                    class="px-1.5 text-[9px] font-semibold uppercase tracking-wide text-slate-400 dark:text-slate-500"
                  >
                    {runtimeFilter().label ?? 'Runtime'}
                  </label>
                  <select
                    id={runtimeFilter().id ?? 'dashboard-runtime-filter'}
                    value={runtimeFilter().value}
                    onChange={(e) => runtimeFilter().onChange(e.currentTarget.value)}
                    class="min-w-[7rem] rounded-md border border-slate-200 bg-white px-2 py-1 text-xs font-medium text-slate-900 shadow-sm transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100"
                  >
                    <For each={runtimeFilter().options}>
                      {(option) => (
                        <option value={option.value}>{option.label}</option>
                      )}
                    </For>
                  </select>
                </div>
              )}
            </Show>

            <div class="inline-flex items-center gap-1 rounded-md bg-slate-100 dark:bg-slate-700 p-0.5">
              <label
                for="dashboard-type-filter"
                class="px-1.5 text-[9px] font-semibold uppercase tracking-wide text-slate-400 dark:text-slate-500"
              >
                Type
              </label>
              <select
                id="dashboard-type-filter"
                value={props.viewMode()}
                onChange={(event) => props.setViewMode(event.currentTarget.value as 'all' | 'vm' | 'lxc' | 'docker' | 'k8s')}
                class="min-w-[7rem] rounded-md border border-slate-200 bg-white px-2 py-1 text-xs font-medium text-slate-900 shadow-sm transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100"
              >
                <option value="all">All</option>
                <option value="vm">VMs</option>
                <option value="lxc">LXCs</option>
                <option value="docker">Containers</option>
                <option value="k8s">K8s</option>
              </select>
            </div>

            <div class="inline-flex items-center gap-1 rounded-md bg-slate-100 dark:bg-slate-700 p-0.5">
              <label
                for="dashboard-status-filter"
                class="px-1.5 text-[9px] font-semibold uppercase tracking-wide text-slate-400 dark:text-slate-500"
              >
                Status
              </label>
              <select
                id="dashboard-status-filter"
                value={props.statusMode()}
                onChange={(event) => props.setStatusMode(event.currentTarget.value as 'all' | 'running' | 'degraded' | 'stopped')}
                class="min-w-[8rem] rounded-md border border-slate-200 bg-white px-2 py-1 text-xs font-medium text-slate-900 shadow-sm transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100"
              >
                <option value="all">All</option>
                <option value="running">Running</option>
                <option value="degraded">Degraded</option>
                <option value="stopped">Stopped</option>
              </select>
            </div>

            <div class="inline-flex rounded-md bg-slate-100 dark:bg-slate-700 p-0.5">
              <button
                type="button"
                onClick={() => props.setGroupingMode('grouped')}
                class={segmentedButtonClass(props.groupingMode() === 'grouped')}
                title="Group by node"
              >
                <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2v11z" />
                </svg>
                Grouped
              </button>
              <button
                type="button"
                onClick={() => props.setGroupingMode('flat')}
                class={segmentedButtonClass(props.groupingMode() === 'flat')}
                title="Flat list view"
              >
                <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <line x1="8" y1="6" x2="21" y2="6" />
                  <line x1="8" y1="12" x2="21" y2="12" />
                  <line x1="8" y1="18" x2="21" y2="18" />
                  <line x1="3" y1="6" x2="3.01" y2="6" />
                  <line x1="3" y1="12" x2="3.01" y2="12" />
                  <line x1="3" y1="18" x2="3.01" y2="18" />
                </svg>
                List
              </button>
            </div>

            <Show when={props.onChartsToggle}>
              <div class="hidden lg:inline-flex rounded-md bg-slate-100 dark:bg-slate-700 p-0.5">
                <button
                  type="button"
                  onClick={props.onChartsToggle}
                  class={segmentedButtonClass(!props.chartsCollapsed?.())}
                  title={props.chartsCollapsed?.() ? 'Show charts' : 'Hide charts'}
                >
                  <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <polyline points="22 12 18 12 15 21 9 3 6 12 2 12" />
                  </svg>
                  Charts
                </button>
              </div>
            </Show>

            <Show when={props.availableColumns && props.isColumnHidden && props.onColumnToggle}>
              <ColumnPicker
                columns={props.availableColumns!}
                isHidden={props.isColumnHidden!}
                onToggle={props.onColumnToggle!}
                onReset={props.onColumnReset}
              />
            </Show>

            <Show when={
              props.search().trim() !== '' ||
              props.viewMode() !== 'all' ||
              props.statusMode() !== 'all' ||
              props.groupingMode() !== 'grouped' ||
              (props.hostFilter ? props.hostFilter.value !== '' : false) ||
              (props.namespaceFilter ? props.namespaceFilter.value !== '' : false)
            }>
              <button
                onClick={() => {
                  props.setSearch('');
                  props.setSortKey('name');
                  props.setSortDirection('asc');
                  props.setViewMode('all');
                  props.setStatusMode('all');
                  props.setGroupingMode('grouped');
                  if (props.hostFilter) {
                    props.hostFilter.onChange('');
                  }
                  if (props.namespaceFilter) {
                    props.namespaceFilter.onChange('');
                  }
                }}
                title="Reset all filters"
                class="ml-auto flex items-center gap-1 px-2.5 py-1.5 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 text-blue-700 dark:text-blue-300 bg-blue-100 dark:bg-blue-900 hover:bg-blue-200 dark:hover:bg-blue-900"
              >
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
                Reset
              </button>
            </Show>
          </div>
        </Show>
      </div>
    </Card>
  );
};
