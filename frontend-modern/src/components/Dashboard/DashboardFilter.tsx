import { Component, For, Show } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { CollapsibleSearchInput } from '@/components/shared/CollapsibleSearchInput';
import { ColumnPicker } from '@/components/shared/ColumnPicker';
import type { ColumnDef } from '@/hooks/useColumnVisibility';
import { STORAGE_KEYS } from '@/utils/localStorage';

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
}

export const DashboardFilter: Component<DashboardFilterProps> = (props) => {
  return (
    <Card class="dashboard-filter mb-3" padding="sm">
      <div class="flex flex-wrap items-center gap-2">
        <Show when={props.hostFilter}>
          {(hostFilter) => (
            <div class="inline-flex items-center rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
              <label
                for={hostFilter().id ?? 'dashboard-host-filter'}
                class="px-2 py-1 text-xs font-medium text-gray-600 dark:text-gray-300"
              >
                {hostFilter().label ?? 'Host'}
              </label>
              <select
                id={hostFilter().id ?? 'dashboard-host-filter'}
                value={hostFilter().value}
                onChange={(e) => hostFilter().onChange(e.currentTarget.value)}
                class="min-w-[9rem] rounded-md border border-gray-200 bg-white px-2.5 py-1 text-xs font-medium text-gray-900 shadow-sm transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500/20 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
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

        {/* Type Filter */}
        <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
          <button
            type="button"
            onClick={() => props.setViewMode('all')}
            class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${props.viewMode() === 'all'
              ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm ring-1 ring-gray-200 dark:ring-gray-600'
              : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50'
              }`}
          >
            All
          </button>
          <button
            type="button"
            onClick={() => props.setViewMode(props.viewMode() === 'vm' ? 'all' : 'vm')}
            class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${props.viewMode() === 'vm'
              ? 'bg-white dark:bg-gray-800 text-blue-600 dark:text-blue-400 shadow-sm ring-1 ring-blue-200 dark:ring-blue-800'
              : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50'
              }`}
          >
            VMs
          </button>
          <button
            type="button"
            onClick={() => props.setViewMode(props.viewMode() === 'lxc' ? 'all' : 'lxc')}
            class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${props.viewMode() === 'lxc'
              ? 'bg-white dark:bg-gray-800 text-green-600 dark:text-green-400 shadow-sm ring-1 ring-green-200 dark:ring-green-800'
              : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50'
              }`}
          >
            LXCs
          </button>
          <button
            type="button"
            onClick={() => props.setViewMode(props.viewMode() === 'docker' ? 'all' : 'docker')}
            class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${props.viewMode() === 'docker'
              ? 'bg-white dark:bg-gray-800 text-sky-600 dark:text-sky-400 shadow-sm ring-1 ring-sky-200 dark:ring-sky-800'
              : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50'
              }`}
          >
            Docker
          </button>
          <button
            type="button"
            onClick={() => props.setViewMode(props.viewMode() === 'k8s' ? 'all' : 'k8s')}
            class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${props.viewMode() === 'k8s'
              ? 'bg-white dark:bg-gray-800 text-amber-600 dark:text-amber-400 shadow-sm ring-1 ring-amber-200 dark:ring-amber-800'
              : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50'
              }`}
          >
            K8s
          </button>
        </div>

          {/* Status Filter */}
          <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
            <button
              type="button"
              onClick={() => props.setStatusMode('all')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${props.statusMode() === 'all'
                ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm ring-1 ring-gray-200 dark:ring-gray-600'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50'
                }`}
            >
              All
            </button>
            <button
              type="button"
              onClick={() => props.setStatusMode(props.statusMode() === 'running' ? 'all' : 'running')}
              class={`inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${props.statusMode() === 'running'
                ? 'bg-white dark:bg-gray-800 text-green-600 dark:text-green-400 shadow-sm ring-1 ring-green-200 dark:ring-green-800'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50'
                }`}
            >
              <span class={`w-2 h-2 rounded-full ${props.statusMode() === 'running' ? 'bg-green-500 shadow-sm shadow-green-500/50' : 'bg-green-400/60'}`} />
              Running
            </button>
            <button
              type="button"
              onClick={() => props.setStatusMode(props.statusMode() === 'degraded' ? 'all' : 'degraded')}
              class={`inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${props.statusMode() === 'degraded'
                ? 'bg-white dark:bg-gray-800 text-amber-600 dark:text-amber-400 shadow-sm ring-1 ring-amber-200 dark:ring-amber-800'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50'
                }`}
            >
              <span class={`w-2 h-2 rounded-full ${props.statusMode() === 'degraded' ? 'bg-amber-500 shadow-sm shadow-amber-500/50' : 'bg-amber-400/60'}`} />
              Degraded
            </button>
            <button
              type="button"
              onClick={() => props.setStatusMode(props.statusMode() === 'stopped' ? 'all' : 'stopped')}
              class={`inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${props.statusMode() === 'stopped'
                ? 'bg-white dark:bg-gray-800 text-red-600 dark:text-red-400 shadow-sm ring-1 ring-red-200 dark:ring-red-800'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50'
                }`}
            >
              <span class={`w-2 h-2 rounded-full ${props.statusMode() === 'stopped' ? 'bg-red-500 shadow-sm shadow-red-500/50' : 'bg-red-400/60'}`} />
              Stopped
            </button>
          </div>

          {/* Grouping Mode Toggle */}
          <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
            <button
              type="button"
              onClick={() => props.setGroupingMode('grouped')}
              class={`inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${props.groupingMode() === 'grouped'
                ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm ring-1 ring-gray-200 dark:ring-gray-600'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50'
                }`}
              title="Group by node"
            >
              Grouped
            </button>
            <button
              type="button"
              onClick={() => props.setGroupingMode(props.groupingMode() === 'flat' ? 'grouped' : 'flat')}
              class={`inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${props.groupingMode() === 'flat'
                ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm ring-1 ring-gray-200 dark:ring-gray-600'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50'
                }`}
              title="Flat list view"
            >
              List
            </button>
          </div>

          {/* Column Picker */}
          <Show when={props.availableColumns && props.isColumnHidden && props.onColumnToggle}>
            <ColumnPicker
              columns={props.availableColumns!}
              isHidden={props.isColumnHidden!}
              onToggle={props.onColumnToggle!}
              onReset={props.onColumnReset}
            />
          </Show>

          <CollapsibleSearchInput
            value={props.search}
            onChange={props.setSearch}
            placeholder="Search or filter..."
            triggerLabel="Search"
            class="w-56 shrink-0 sm:w-64"
            fullWidthWhenExpanded
            inputRef={props.searchInputRef}
            onBeforeAutoFocus={props.onBeforeAutoFocus}
            history={{ storageKey: STORAGE_KEYS.DASHBOARD_SEARCH_HISTORY }}
            tips={{
              popoverId: 'dashboard-search-help',
              intro: 'Combine filters to narrow results',
              tips: [
                { code: 'media', description: 'Guests with "media" in the name' },
                { code: 'cpu>80', description: 'Highlight guests using more than 80% CPU' },
                { code: 'memory<20', description: 'Find guests under 20% memory usage' },
                { code: 'tags:prod', description: 'Filter by tag' },
                { code: 'node:pve1', description: 'Show guests on a specific node' },
              ],
              footerHighlight: 'node:pve1 cpu>60',
              footerText: 'Stack filters to get laser-focused results.',
            }}
          />

          {/* Reset Button - Only show when filters are active */}
          <Show when={
            props.search().trim() !== '' ||
            props.viewMode() !== 'all' ||
            props.statusMode() !== 'all' ||
            props.groupingMode() !== 'grouped' ||
            (props.hostFilter ? props.hostFilter.value !== '' : false)
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
              }}
              title="Reset all filters"
              class="flex items-center gap-1 px-2.5 py-1.5 text-xs font-medium rounded-lg transition-all duration-150 active:scale-95
                     text-blue-700 dark:text-blue-300 bg-blue-100 dark:bg-blue-900/50 hover:bg-blue-200 dark:hover:bg-blue-900/70"
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
    </Card>
  );
};
