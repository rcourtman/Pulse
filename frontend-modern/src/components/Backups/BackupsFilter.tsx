import { Component, Show } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { SearchTipsPopover } from '@/components/shared/SearchTipsPopover';

interface BackupsFilterProps {
  search: () => string;
  setSearch: (value: string) => void;
  viewMode: () => 'all' | 'snapshot' | 'pve' | 'pbs';
  setViewMode: (value: 'all' | 'snapshot' | 'pve' | 'pbs') => void;
  groupBy: () => 'date' | 'guest';
  setGroupBy: (value: 'date' | 'guest') => void;
  searchInputRef?: (el: HTMLInputElement) => void;
  typeFilter?: () => 'all' | 'VM' | 'LXC' | 'Host';
  setTypeFilter?: (value: 'all' | 'VM' | 'LXC' | 'Host') => void;
  hasHostBackups?: () => boolean;
  sortKey: () => string;
  setSortKey: (value: string) => void;
  sortDirection: () => 'asc' | 'desc';
  setSortDirection: (value: 'asc' | 'desc') => void;
  sortOptions?: { value: string; label: string }[];
  onReset?: () => void;
}

export const BackupsFilter: Component<BackupsFilterProps> = (props) => {
  const sortOptions = props.sortOptions ?? [
    { value: 'backupTime', label: 'Time' },
    { value: 'name', label: 'Guest Name' },
    { value: 'node', label: 'Node' },
    { value: 'vmid', label: 'VMID' },
    { value: 'backupType', label: 'Backup Type' },
    { value: 'size', label: 'Size' },
    { value: 'storage', label: 'Storage' },
    { value: 'verified', label: 'Verified' },
    { value: 'type', label: 'Guest Type' },
    { value: 'owner', label: 'Owner' },
  ];
  return (
    <Card class="backups-filter mb-3" padding="sm">
      <div class="flex flex-col lg:flex-row gap-3">
        {/* Search Bar */}
        <div class="flex gap-2 flex-1 items-center">
          <div class="relative flex-1">
            <input
              ref={props.searchInputRef}
              type="text"
              placeholder="Search backups or node:nodename"
              value={props.search()}
              onInput={(e) => props.setSearch(e.currentTarget.value)}
              class="w-full pl-9 pr-9 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg 
                     bg-white dark:bg-gray-900 text-gray-800 dark:text-gray-200 placeholder-gray-400 dark:placeholder-gray-500
                     focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 dark:focus:border-blue-400 outline-none transition-all"
              title="Search backups by name or filter by node"
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
          </div>
          <SearchTipsPopover
            class="flex-shrink-0"
            popoverId="backups-search-help"
            intro="Quick examples"
            tips={[
              { code: 'media', description: 'Backups with "media" in the name' },
              { code: 'node:pve1', description: 'Show backups from a specific node' },
              { code: 'vm-104', description: 'Locate backups for VM 104' },
            ]}
            footerHighlight="node:pve1 vm-104"
            footerText="Mix terms to focus on the exact backups you need."
          />
        </div>

        {/* Filters */}
        <div class="flex flex-wrap items-center gap-2">
          {/* Source Filter */}
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
              onClick={() => props.setViewMode(props.viewMode() === 'snapshot' ? 'all' : 'snapshot')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                props.viewMode() === 'snapshot'
                  ? 'bg-white dark:bg-gray-800 text-yellow-600 dark:text-yellow-400 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
              }`}
            >
              Snapshots
            </button>
            <button
              type="button"
              onClick={() => props.setViewMode(props.viewMode() === 'pve' ? 'all' : 'pve')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                props.viewMode() === 'pve'
                  ? 'bg-white dark:bg-gray-800 text-orange-600 dark:text-orange-400 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
              }`}
            >
              PVE
            </button>
            <button
              type="button"
              onClick={() => props.setViewMode(props.viewMode() === 'pbs' ? 'all' : 'pbs')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                props.viewMode() === 'pbs'
                  ? 'bg-white dark:bg-gray-800 text-purple-600 dark:text-purple-400 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
              }`}
            >
              PBS
            </button>
          </div>

          <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>

          {/* Type Filter - Only show when there are Host backups */}
          <Show
            when={
              props.hasHostBackups &&
              props.hasHostBackups() &&
              props.typeFilter &&
              props.setTypeFilter
            }
          >
            <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
              <button
                type="button"
                onClick={() => props.setTypeFilter!('all')}
                class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                  props.typeFilter!() === 'all'
                    ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                    : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                }`}
              >
                All Types
              </button>
              <button
                type="button"
                onClick={() => props.setTypeFilter!(props.typeFilter!() === 'VM' ? 'all' : 'VM')}
                class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                  props.typeFilter!() === 'VM'
                    ? 'bg-white dark:bg-gray-800 text-blue-600 dark:text-blue-400 shadow-sm'
                    : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                }`}
              >
                VM
              </button>
              <button
                type="button"
                onClick={() => props.setTypeFilter!(props.typeFilter!() === 'LXC' ? 'all' : 'LXC')}
                class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                  props.typeFilter!() === 'LXC'
                    ? 'bg-white dark:bg-gray-800 text-green-600 dark:text-green-400 shadow-sm'
                    : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                }`}
              >
                LXC
              </button>
              <button
                type="button"
                onClick={() => props.setTypeFilter!(props.typeFilter!() === 'Host' ? 'all' : 'Host')}
                class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                  props.typeFilter!() === 'Host'
                    ? 'bg-white dark:bg-gray-800 text-orange-600 dark:text-orange-400 shadow-sm'
                    : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                }`}
              >
                PMG
              </button>
            </div>
            <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>
          </Show>

          {/* Group By Filter */}
          <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
            <button
              type="button"
              onClick={() => props.setGroupBy('date')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                props.groupBy() === 'date'
                  ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
              }`}
            >
              By Date
            </button>
            <button
              type="button"
              onClick={() => props.setGroupBy(props.groupBy() === 'guest' ? 'date' : 'guest')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                props.groupBy() === 'guest'
                  ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
              }`}
            >
              By Guest
            </button>
          </div>

          <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>

          {/* Sort controls */}
          <div class="flex items-center gap-2">
            <span class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
              Sort
            </span>
            <select
              value={props.sortKey()}
              onChange={(e) => props.setSortKey(e.currentTarget.value)}
              class="px-2 py-1 text-xs border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-200 focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 dark:focus:border-blue-400"
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

          <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>

          {/* Reset Button */}
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
            class={`flex items-center justify-center px-2.5 py-1 text-xs font-medium rounded-lg transition-colors ${
              props.search().trim() !== '' ||
              props.viewMode() !== 'all' ||
              props.groupBy() !== 'date' ||
              props.sortKey() !== 'backupTime' ||
              props.sortDirection() !== 'desc' ||
              (props.typeFilter && props.typeFilter() !== 'all')
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
