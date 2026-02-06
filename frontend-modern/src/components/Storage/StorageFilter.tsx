import { Component, Show } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { SearchInput } from '@/components/shared/SearchInput';
import { ColumnPicker } from '@/components/shared/ColumnPicker';
import type { ColumnDef } from '@/hooks/useColumnVisibility';
import { STORAGE_KEYS } from '@/utils/localStorage';

interface StorageFilterProps {
  search: () => string;
  setSearch: (value: string) => void;
  groupBy?: () => 'node' | 'storage';
  setGroupBy?: (value: 'node' | 'storage') => void;
  sortKey: () => string;
  setSortKey: (value: string) => void;
  sortDirection: () => 'asc' | 'desc';
  setSortDirection: (value: 'asc' | 'desc') => void;
  sortOptions?: { value: string; label: string }[];
  searchInputRef?: (el: HTMLInputElement) => void;
  statusFilter?: () => 'all' | 'available' | 'offline';
  setStatusFilter?: (value: 'all' | 'available' | 'offline') => void;
  sourceFilter?: () => 'all' | 'proxmox' | 'pbs' | 'ceph';
  setSourceFilter?: (value: 'all' | 'proxmox' | 'pbs' | 'ceph') => void;
  // Column visibility (optional)
  columnVisibility?: {
    availableToggles: () => ColumnDef[];
    isHiddenByUser: (id: string) => boolean;
    toggle: (id: string) => void;
    resetToDefaults: () => void;
  };
}

export const StorageFilter: Component<StorageFilterProps> = (props) => {
  const sortOptions = props.sortOptions ?? [
    { value: 'name', label: 'Name' },
    { value: 'usage', label: 'Usage %' },
    { value: 'free', label: 'Free' },
    { value: 'total', label: 'Total' },
  ];

  const hasActiveFilters = () =>
    props.search().trim() !== '' ||
    props.sortKey() !== 'name' ||
    props.sortDirection() !== 'asc' ||
    (props.groupBy && props.groupBy() !== 'node') ||
    (props.statusFilter && props.statusFilter() !== 'all') ||
    (props.sourceFilter && props.sourceFilter() !== 'all');

  return (
    <Card class="storage-filter mb-3" padding="sm">
      <div class="flex flex-col gap-3">
        {/* Row 1: Search Bar - Full width */}
        <SearchInput
          value={props.search}
          onChange={props.setSearch}
          placeholder="Search storage... (e.g., local, nfs, node:pve1)"
          title="Search storage by name or filter by node"
          inputRef={props.searchInputRef}
          autoFocus
          history={{
            storageKey: STORAGE_KEYS.STORAGE_SEARCH_HISTORY,
            emptyMessage: 'Your recent storage searches will show here.',
          }}
          tips={{
            popoverId: 'storage-search-help',
            intro: 'Quick examples',
            tips: [
              { code: 'local', description: 'Storage with "local" in the name' },
              { code: 'node:pve1', description: 'Show storage on a specific node' },
              { code: 'nfs', description: 'Find NFS storage' },
            ],
            footerHighlight: 'node:pve1 nfs',
            footerText: 'Combine filters to zero in on exactly what you need.',
          }}
        />

        {/* Row 2: Filters - Compact horizontal layout */}
        <div class="flex flex-wrap items-center gap-x-1.5 sm:gap-x-2 gap-y-2">
          {/* Group By Filter */}
          <Show when={props.groupBy && props.setGroupBy}>
            <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
              <button
                type="button"
                onClick={() => props.setGroupBy!('node')}
                class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${props.groupBy!() === 'node'
                  ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
              >
                By Node
              </button>
              <button
                type="button"
                onClick={() => props.setGroupBy!(props.groupBy!() === 'storage' ? 'node' : 'storage')}
                class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${props.groupBy!() === 'storage'
                  ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
              >
                By Storage
              </button>
            </div>
            <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>
          </Show>

          {/* Source Filter */}
          <Show when={props.sourceFilter && props.setSourceFilter}>
            <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
              <button
                type="button"
                onClick={() => props.setSourceFilter!('all')}
                class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${props.sourceFilter!() === 'all'
                  ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
              >
                All Sources
              </button>
              <button
                type="button"
                onClick={() => props.setSourceFilter!('proxmox')}
                class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${props.sourceFilter!() === 'proxmox'
                  ? 'bg-white dark:bg-gray-800 text-blue-600 dark:text-blue-300 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
              >
                Proxmox
              </button>
              <button
                type="button"
                onClick={() => props.setSourceFilter!('pbs')}
                class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${props.sourceFilter!() === 'pbs'
                  ? 'bg-white dark:bg-gray-800 text-emerald-600 dark:text-emerald-300 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
              >
                PBS
              </button>
              <button
                type="button"
                onClick={() => props.setSourceFilter!('ceph')}
                class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${props.sourceFilter!() === 'ceph'
                  ? 'bg-white dark:bg-gray-800 text-purple-600 dark:text-purple-300 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
              >
                Ceph
              </button>
            </div>
            <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>
          </Show>

          {/* Status Filter */}
          <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
            <button
              type="button"
              onClick={() => props.setStatusFilter && props.setStatusFilter('all')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${props.statusFilter && props.statusFilter() === 'all'
                ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                }`}
            >
              All
            </button>
            <button
              type="button"
              onClick={() => props.setStatusFilter && props.setStatusFilter('available')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${props.statusFilter && props.statusFilter() === 'available'
                ? 'bg-white dark:bg-gray-800 text-green-600 dark:text-green-400 shadow-sm'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                }`}
            >
              Available
            </button>
            <button
              type="button"
              onClick={() => props.setStatusFilter && props.setStatusFilter('offline')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${props.statusFilter && props.statusFilter() === 'offline'
                ? 'bg-white dark:bg-gray-800 text-red-600 dark:text-red-400 shadow-sm'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                }`}
            >
              Offline
            </button>
          </div>

          <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>

          {/* Sort controls */}
          <div class="flex items-center gap-1.5">
            <select
              value={props.sortKey()}
              onChange={(e) => props.setSortKey(e.currentTarget.value)}
              class="px-2 py-1 text-xs border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-200 focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500"
            >
              {sortOptions.map((option) => (
                <option value={option.value}>{option.label}</option>
              ))}
            </select>
            <button
              type="button"
              title={`Sort ${props.sortDirection() === 'asc' ? 'descending' : 'ascending'}`}
              onClick={() =>
                props.setSortDirection(props.sortDirection() === 'asc' ? 'desc' : 'asc')
              }
              class="inline-flex items-center justify-center h-7 w-7 rounded-lg border border-gray-300 dark:border-gray-600 text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
            >
              <svg
                class={`h-4 w-4 transition-transform ${props.sortDirection() === 'asc' ? 'rotate-180' : ''}`}
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M8 9l4-4 4 4m0 6l-4 4-4-4"
                />
              </svg>
            </button>
          </div>

          {/* Column Picker */}
          <Show when={props.columnVisibility}>
            <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>
            <ColumnPicker
              columns={props.columnVisibility!.availableToggles()}
              isHidden={props.columnVisibility!.isHiddenByUser}
              onToggle={props.columnVisibility!.toggle}
              onReset={props.columnVisibility!.resetToDefaults}
            />
          </Show>

          {/* Reset Button - Only show when filters are active */}
          <Show when={hasActiveFilters()}>
            <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>
            <button
              onClick={() => {
                props.setSearch('');
                props.setSortKey('name');
                props.setSortDirection('asc');
              if (props.setGroupBy) props.setGroupBy('node');
              if (props.setStatusFilter) props.setStatusFilter('all');
              if (props.setSourceFilter) props.setSourceFilter('all');
            }}
              title="Reset all filters"
              class="flex items-center gap-1 px-2.5 py-1.5 text-xs font-medium rounded-lg transition-colors
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
