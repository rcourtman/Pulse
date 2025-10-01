import { Component, Show } from 'solid-js';
import { Card } from '@/components/shared/Card';

interface DashboardFilterProps {
  search: () => string;
  setSearch: (value: string) => void;
  isSearchLocked: () => boolean;
  viewMode: () => 'all' | 'vm' | 'lxc';
  setViewMode: (value: 'all' | 'vm' | 'lxc') => void;
  statusMode: () => 'all' | 'running' | 'stopped';
  setStatusMode: (value: 'all' | 'running' | 'stopped') => void;
  groupingMode: () => 'grouped' | 'flat';
  setGroupingMode: (value: 'grouped' | 'flat') => void;
  setSortKey: (value: string) => void;
  setSortDirection: (value: string) => void;
  searchInputRef?: (el: HTMLInputElement) => void;
}

export const DashboardFilter: Component<DashboardFilterProps> = (props) => {
  return (
    <Card class="dashboard-filter mb-3" padding="sm">
      <div class="flex flex-col lg:flex-row gap-3">
        {/* Search Bar */}
        <div class="flex gap-2 flex-1">
          <div class="relative flex-1">
            <input
              ref={props.searchInputRef}
              type="text"
              placeholder="Search by name, cpu>80, memory<20, tags:prod, node:pve1"
              value={props.search()}
              onInput={(e) => props.setSearch(e.currentTarget.value)}
              class={`w-full pl-9 pr-9 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg 
                     bg-white dark:bg-gray-900 text-gray-800 dark:text-gray-200 placeholder-gray-400 dark:placeholder-gray-500
                     focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 dark:focus:border-blue-400 outline-none transition-all`}
              title="Search guests or use filters like cpu>80"
            />
            <svg
              class="absolute left-3 top-2 h-4 w-4 text-gray-400 dark:text-gray-500"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
              />
            </svg>
            <Show when={props.search()}>
              <button
                type="button"
                class="absolute right-3 top-2 text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
                onClick={() => props.setSearch('')}
                aria-label="Clear search"
                title="Clear search"
              >
                <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M6 18L18 6M6 6l12 12"
                  />
                </svg>
              </button>
            </Show>
          </div>
        </div>

        {/* Filters */}
        <div class="flex flex-wrap items-center gap-2">
          {/* Type Filter */}
          <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
            <button
              type="button"
              onClick={() => props.setViewMode('all')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                props.viewMode() === 'all'
                  ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
              }`}
            >
              All
            </button>
            <button
              type="button"
              onClick={() => props.setViewMode(props.viewMode() === 'vm' ? 'all' : 'vm')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                props.viewMode() === 'vm'
                  ? 'bg-white dark:bg-gray-800 text-blue-600 dark:text-blue-400 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
              }`}
            >
              VMs
            </button>
            <button
              type="button"
              onClick={() => props.setViewMode(props.viewMode() === 'lxc' ? 'all' : 'lxc')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                props.viewMode() === 'lxc'
                  ? 'bg-white dark:bg-gray-800 text-green-600 dark:text-green-400 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
              }`}
            >
              LXCs
            </button>
          </div>

          <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>

          {/* Status Filter */}
          <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
            <button
              type="button"
              onClick={() => props.setStatusMode('all')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                props.statusMode() === 'all'
                  ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
              }`}
            >
              All
            </button>
            <button
              type="button"
              onClick={() => props.setStatusMode(props.statusMode() === 'running' ? 'all' : 'running')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                props.statusMode() === 'running'
                  ? 'bg-white dark:bg-gray-800 text-green-600 dark:text-green-400 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
              }`}
            >
              Running
            </button>
            <button
              type="button"
              onClick={() => props.setStatusMode(props.statusMode() === 'stopped' ? 'all' : 'stopped')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                props.statusMode() === 'stopped'
                  ? 'bg-white dark:bg-gray-800 text-red-600 dark:text-red-400 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
              }`}
            >
              Stopped
            </button>
          </div>

          <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>

          {/* Grouping Mode Toggle */}
          <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
            <button
              type="button"
              onClick={() => props.setGroupingMode('grouped')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                props.groupingMode() === 'grouped'
                  ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
              }`}
              title="Group by node"
            >
              Grouped
            </button>
            <button
              type="button"
              onClick={() => props.setGroupingMode(props.groupingMode() === 'flat' ? 'grouped' : 'flat')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                props.groupingMode() === 'flat'
                  ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
              }`}
              title="Flat list view"
            >
              List
            </button>
          </div>

          <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>

          {/* Reset Button */}
          <button
            onClick={() => {
              props.setSearch('');
              props.setSortKey('vmid');
              props.setSortDirection('asc');
              props.setViewMode('all');
              props.setStatusMode('all');
              props.setGroupingMode('grouped');
            }}
            title="Reset all filters"
            class={`flex items-center justify-center px-2.5 py-1 text-xs font-medium rounded-lg transition-colors ${
              props.search() ||
              props.viewMode() !== 'all' ||
              props.statusMode() !== 'all' ||
              props.groupingMode() !== 'grouped'
                ? 'text-blue-700 dark:text-blue-300 bg-blue-100 dark:bg-blue-900/50 hover:bg-blue-200 dark:hover:bg-blue-900/70'
                : 'text-gray-600 dark:text-gray-400 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600'
            }`}
          >
            <svg
              width="14"
              height="14"
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
            <span class="ml-1 hidden sm:inline">Reset</span>
          </button>
        </div>
      </div>
    </Card>
  );
};
