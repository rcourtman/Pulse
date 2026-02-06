import { Component, Show } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { SearchInput } from '@/components/shared/SearchInput';
import { ColumnPicker } from '@/components/shared/ColumnPicker';
import type { ColumnDef } from '@/hooks/useColumnVisibility';
import { STORAGE_KEYS } from '@/utils/localStorage';

interface BackupsFilterProps {
  search: () => string;
  setSearch: (value: string) => void;
  viewMode: () => 'all' | 'snapshot' | 'pve' | 'pbs';
  setViewMode: (value: 'all' | 'snapshot' | 'pve' | 'pbs') => void;
  groupBy: () => 'date' | 'guest';
  setGroupBy: (value: 'date' | 'guest') => void;
  searchInputRef?: (el: HTMLInputElement) => void;
  sortKey: () => string;
  setSortKey: (value: string) => void;
  sortDirection: () => 'asc' | 'desc';
  setSortDirection: (value: 'asc' | 'desc') => void;
  sortOptions?: { value: string; label: string }[];
  onReset?: () => void;
  // Column visibility (optional)
  columnVisibility?: {
    availableToggles: () => ColumnDef[];
    isHiddenByUser: (id: string) => boolean;
    toggle: (id: string) => void;
    resetToDefaults: () => void;
  };
}

export const BackupsFilter: Component<BackupsFilterProps> = (props) => {
  const sortOptions = props.sortOptions ?? [
    { value: 'backupTime', label: 'Time' },
    { value: 'name', label: 'Name' },
    { value: 'vmid', label: 'VMID' },
    { value: 'size', label: 'Size' },
  ];

  const hasActiveFilters = () =>
    props.search().trim() !== '' ||
    props.viewMode() !== 'all' ||
    props.groupBy() !== 'date' ||
    props.sortKey() !== 'backupTime' ||
    props.sortDirection() !== 'desc';

  return (
    <Card class="backups-filter mb-3" padding="sm">
      <div class="flex flex-col gap-3">
        {/* Row 1: Search Bar - Full width */}
        <SearchInput
          value={props.search}
          onChange={props.setSearch}
          placeholder="Search backups... (e.g., media, vm-104, node:pve1)"
          title="Search backups by name, VMID, or filter by node"
          inputRef={props.searchInputRef}
          autoFocus
          history={{
            storageKey: STORAGE_KEYS.BACKUPS_SEARCH_HISTORY,
            emptyMessage: 'Your recent backup searches will show here.',
          }}
          tips={{
            popoverId: 'backups-search-help',
            intro: 'Quick examples',
            tips: [
              { code: 'media', description: 'Backups with "media" in the name' },
              { code: 'node:pve1', description: 'Show backups from a specific node' },
              { code: 'vm-104', description: 'Locate backups for VM 104' },
            ],
            footerHighlight: 'node:pve1 vm-104',
            footerText: 'Mix terms to focus on the exact backups you need.',
          }}
        />

        {/* Row 2: Filters - Compact horizontal layout */}
        <div class="flex flex-wrap items-center gap-1.5 sm:gap-2">
          {/* Source Filter */}
          <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
            <button
              type="button"
              onClick={() => props.setViewMode('all')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${props.viewMode() === 'all'
                ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                }`}
            >
              All
            </button>
            <button
              type="button"
              onClick={() => props.setViewMode(props.viewMode() === 'snapshot' ? 'all' : 'snapshot')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${props.viewMode() === 'snapshot'
                ? 'bg-white dark:bg-gray-800 text-yellow-600 dark:text-yellow-400 shadow-sm'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                }`}
            >
              Snapshots
            </button>
            <button
              type="button"
              onClick={() => props.setViewMode(props.viewMode() === 'pve' ? 'all' : 'pve')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${props.viewMode() === 'pve'
                ? 'bg-white dark:bg-gray-800 text-orange-600 dark:text-orange-400 shadow-sm'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                }`}
            >
              PVE
            </button>
            <button
              type="button"
              onClick={() => props.setViewMode(props.viewMode() === 'pbs' ? 'all' : 'pbs')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${props.viewMode() === 'pbs'
                ? 'bg-white dark:bg-gray-800 text-purple-600 dark:text-purple-400 shadow-sm'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                }`}
            >
              PBS
            </button>
          </div>

          <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>

          {/* Group By Filter */}
          <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
            <button
              type="button"
              onClick={() => props.setGroupBy('date')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${props.groupBy() === 'date'
                ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                }`}
            >
              By Date
            </button>
            <button
              type="button"
              onClick={() => props.setGroupBy(props.groupBy() === 'guest' ? 'date' : 'guest')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${props.groupBy() === 'guest'
                ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                }`}
            >
              By Guest
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

          {/* Column Picker - Only if provided */}
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
                if (props.onReset) {
                  props.onReset();
                } else {
                  props.setSearch('');
                  props.setViewMode('all');
                  props.setGroupBy('date');
                }
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
