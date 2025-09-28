import { Component, Show } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { showTooltip, hideTooltip } from '@/components/shared/Tooltip';

interface StorageFilterProps {
  search: () => string;
  setSearch: (value: string) => void;
  groupBy?: () => 'node' | 'storage';
  setGroupBy?: (value: 'node' | 'storage') => void;
  setSortKey: (value: string) => void;
  setSortDirection: (value: 'asc' | 'desc') => void;
  searchInputRef?: (el: HTMLInputElement) => void;
}

export const StorageFilter: Component<StorageFilterProps> = (props) => {
  return (
    <Card class="storage-filter mb-3" padding="sm">
      <div class="flex flex-col lg:flex-row gap-3">
        {/* Search Bar */}
        <div class="flex gap-2 flex-1">
          <div class="relative flex-1">
            <input
              ref={props.searchInputRef}
              type="text"
              placeholder="Search storage or node:nodename"
              value={props.search()}
              onInput={(e) => props.setSearch(e.currentTarget.value)}
              class="w-full pl-9 pr-9 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg 
                     bg-white dark:bg-gray-900 text-gray-800 dark:text-gray-200 placeholder-gray-400 dark:placeholder-gray-500
                     focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 dark:focus:border-blue-400 outline-none transition-all"
              title="Search storage by name or filter by node"
            />
            <svg class="absolute left-3 top-2 h-4 w-4 text-gray-400 dark:text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
            <button type="button"
              class="absolute right-3 top-2 text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
              onMouseEnter={(e) => {
                const rect = e.currentTarget.getBoundingClientRect();
                const tooltipContent = `
                  <div class="space-y-2 p-1">
                    <div class="font-semibold mb-2">Search Examples:</div>
                    <div class="space-y-1">
                      <div><span class="font-mono bg-gray-700 px-1 rounded">local</span> - Find storage with "local" in name</div>
                      <div><span class="font-mono bg-gray-700 px-1 rounded">node:pve1</span> - Show storage on specific node</div>
                      <div><span class="font-mono bg-gray-700 px-1 rounded">nfs</span> - Find NFS storage</div>
                    </div>
                  </div>
                `;
                showTooltip(tooltipContent, rect.left, rect.top);
              }}
              onMouseLeave={() => hideTooltip()}
              onClick={(e) => e.preventDefault()}
              aria-label="Search help"
            >
              <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </button>
          </div>
        </div>

        {/* Filters */}
        <div class="flex flex-wrap items-center gap-2">
          {/* Group By Filter */}
          <Show when={props.groupBy && props.setGroupBy}>
            <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
              <button type="button"
                onClick={() => props.setGroupBy!('node')}
                class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                  props.groupBy!() === 'node'
                    ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm' 
                    : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                }`}
              >
                By Node
              </button>
              <button type="button"
                onClick={() => props.setGroupBy!('storage')}
                class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                  props.groupBy!() === 'storage'
                    ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm' 
                    : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                }`}
              >
                By Storage
              </button>
            </div>
            <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>
          </Show>

          {/* Reset Button */}
          <button 
            onClick={() => {
              props.setSearch('');
              props.setSortKey('name');
              props.setSortDirection('asc');
              if (props.setGroupBy) props.setGroupBy('node');
            }}
            title="Reset all filters"
            class="flex items-center justify-center px-2.5 py-1 text-xs font-medium text-gray-600 dark:text-gray-400 
                   bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 
                   rounded-lg transition-colors"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8"/>
              <path d="M21 3v5h-5"/>
              <path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16"/>
              <path d="M8 16H3v5"/>
            </svg>
            <span class="ml-1 hidden sm:inline">Reset</span>
          </button>

          {/* Active Indicator */}
          <Show when={props.search()}>
            <span class="text-xs bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 px-2 py-0.5 rounded-full font-medium">
              Active
            </span>
          </Show>
        </div>
      </div>
    </Card>
  );
};