import { Component, Show } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { SearchInput } from '@/components/shared/SearchInput';
import { MetricsViewToggle } from '@/components/shared/MetricsViewToggle';
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
}

export const DashboardFilter: Component<DashboardFilterProps> = (props) => {
  return (
    <Card class="dashboard-filter mb-3" padding="sm">
      <div class="flex flex-col gap-3">
        {/* Row 1: Search Bar */}
        <SearchInput
          value={props.search}
          onChange={props.setSearch}
          placeholder="Search or filter..."
          inputRef={props.searchInputRef}
          autoFocus
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

        {/* Row 2: All filters on one line */}
        <div class="flex flex-wrap items-center gap-x-1.5 sm:gap-x-2 gap-y-2">
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
              class={`inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${props.viewMode() === 'vm'
                ? 'bg-white dark:bg-gray-800 text-blue-600 dark:text-blue-400 shadow-sm ring-1 ring-blue-200 dark:ring-blue-800'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50'
                }`}
            >
              <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <rect x="2" y="3" width="20" height="14" rx="2" />
                <path d="M8 21h8M12 17v4" />
              </svg>
              VMs
            </button>
            <button
              type="button"
              onClick={() => props.setViewMode(props.viewMode() === 'lxc' ? 'all' : 'lxc')}
              class={`inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${props.viewMode() === 'lxc'
                ? 'bg-white dark:bg-gray-800 text-green-600 dark:text-green-400 shadow-sm ring-1 ring-green-200 dark:ring-green-800'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50'
                }`}
            >
              <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z" />
              </svg>
              LXCs
            </button>
            <button
              type="button"
              onClick={() => props.setViewMode(props.viewMode() === 'docker' ? 'all' : 'docker')}
              class={`inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${props.viewMode() === 'docker'
                ? 'bg-white dark:bg-gray-800 text-sky-600 dark:text-sky-400 shadow-sm ring-1 ring-sky-200 dark:ring-sky-800'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50'
                }`}
            >
              <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <rect x="3" y="6" width="18" height="12" rx="2" />
                <path d="M3 10h18M7 6v12M13 6v12" />
              </svg>
              Docker
            </button>
            <button
              type="button"
              onClick={() => props.setViewMode(props.viewMode() === 'k8s' ? 'all' : 'k8s')}
              class={`inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${props.viewMode() === 'k8s'
                ? 'bg-white dark:bg-gray-800 text-amber-600 dark:text-amber-400 shadow-sm ring-1 ring-amber-200 dark:ring-amber-800'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50'
                }`}
            >
              <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M12 2l7 4v8l-7 4-7-4V6l7-4z" />
                <path d="M12 6v12" />
              </svg>
              K8s
            </button>
          </div>

          <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>

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

          <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>

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
              <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2v11z" />
              </svg>
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

          <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>

          {/* Metrics View Toggle */}
          <MetricsViewToggle />

          <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>

          {/* Column Picker */}
          <Show when={props.availableColumns && props.isColumnHidden && props.onColumnToggle}>
            <ColumnPicker
              columns={props.availableColumns!}
              isHidden={props.isColumnHidden!}
              onToggle={props.onColumnToggle!}
              onReset={props.onColumnReset}
            />
          </Show>

          <Show when={props.availableColumns}>
            <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>
          </Show>

          {/* Reset Button - Only show when filters are active */}
          <Show when={
            props.search().trim() !== '' ||
            props.viewMode() !== 'all' ||
            props.statusMode() !== 'all' ||
            props.groupingMode() !== 'grouped'
          }>
            <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>
            <button
              onClick={() => {
                props.setSearch('');
                props.setSortKey('name');
                props.setSortDirection('asc');
                props.setViewMode('all');
                props.setStatusMode('all');
                props.setGroupingMode('grouped');
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
      </div>
    </Card>
  );
};
