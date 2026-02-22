import { Component, Show, createMemo, createSignal, type JSX } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { SearchInput } from '@/components/shared/SearchInput';
import { ColumnPicker } from '@/components/shared/ColumnPicker';
import type { ColumnDef } from '@/hooks/useColumnVisibility';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { STORAGE_KEYS } from '@/utils/localStorage';
import ListFilterIcon from 'lucide-solid/icons/list-filter';
import { segmentedButtonClass } from '@/utils/segmentedButton';
import type { StorageSourceOption } from './storageSourceOptions';

export type StorageStatusFilter = 'all' | 'available' | 'warning' | 'critical' | 'offline' | 'unknown';
export type StorageGroupByFilter = 'node' | 'type' | 'status';

interface StorageFilterProps {
  search: () => string;
  setSearch: (value: string) => void;
  groupBy?: () => StorageGroupByFilter;
  setGroupBy?: (value: StorageGroupByFilter) => void;
  sortKey: () => string;
  setSortKey: (value: string) => void;
  sortDirection: () => 'asc' | 'desc';
  setSortDirection: (value: 'asc' | 'desc') => void;
  sortOptions?: { value: string; label: string }[];
  sortDisabled?: boolean;
  searchInputRef?: (el: HTMLInputElement) => void;
  statusFilter?: () => StorageStatusFilter;
  setStatusFilter?: (value: StorageStatusFilter) => void;
  sourceFilter?: () => string;
  setSourceFilter?: (value: string) => void;
  sourceOptions?: StorageSourceOption[];
  // Slot for page-specific filters (e.g., view toggle, node selector).
  leadingFilters?: JSX.Element;
  // Column visibility (optional)
  columnVisibility?: {
    availableToggles: () => ColumnDef[];
    isHiddenByUser: (id: string) => boolean;
    toggle: (id: string) => void;
    resetToDefaults: () => void;
  };
}

export const StorageFilter: Component<StorageFilterProps> = (props) => {
  const { isMobile } = useBreakpoint();
  const [filtersOpen, setFiltersOpen] = createSignal(false);

  const activeFilterCount = createMemo(() => {
    let count = 0;
    if (props.search().trim() !== '') count++;
    if (props.groupBy && props.groupBy() !== 'node') count++;
    if (props.statusFilter && props.statusFilter() !== 'all') count++;
    if (props.sourceFilter && props.sourceFilter() !== 'all') count++;
    return count;
  });

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

  const sourceOptions = (): StorageSourceOption[] =>
    props.sourceOptions ?? [
      { key: 'all', label: 'All Sources', tone: 'slate' },
      { key: 'proxmox', label: 'PVE', tone: 'blue' },
      { key: 'pbs', label: 'PBS', tone: 'emerald' },
      { key: 'ceph', label: 'Ceph', tone: 'violet' },
    ];

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

        <Show when={isMobile()}>
          <button
            type="button"
            onClick={() => setFiltersOpen((o) => !o)}
            class="flex items-center gap-1.5 rounded-md bg-surface-hover px-2.5 py-1.5 text-xs font-medium text-muted"
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

        {/* Row 2: Filters - Compact horizontal layout */}
        <Show when={!isMobile() || filtersOpen()}>
          <div class="flex flex-wrap items-center gap-x-1.5 sm:gap-x-2 gap-y-2">
            {props.leadingFilters}

            {/* Group By Filter */}
            <Show when={props.groupBy && props.setGroupBy}>
              <div class="max-w-full overflow-x-auto scrollbar-hide">
                <div class="inline-flex rounded-md bg-surface-hover p-0.5" role="group" aria-label="Group By">
                  <button
                    type="button"
                    onClick={() => props.setGroupBy!('node')}
                    aria-pressed={props.groupBy!() === 'node'}
                    class={segmentedButtonClass(props.groupBy!() === 'node')}
                  >
                    By Node
                  </button>
                  <button
                    type="button"
                    onClick={() => props.setGroupBy!('type')}
                    aria-pressed={props.groupBy!() === 'type'}
                    class={segmentedButtonClass(props.groupBy!() === 'type')}
                  >
                    By Type
                  </button>
                  <button
                    type="button"
                    onClick={() => props.setGroupBy!('status')}
                    aria-pressed={props.groupBy!() === 'status'}
                    class={segmentedButtonClass(props.groupBy!() === 'status')}
                  >
                    By Status
                  </button>
                </div>
              </div>
              <div class="h-5 w-px bg-surface-hover hidden sm:block"></div>
            </Show>

            {/* Source Filter */}
            <Show when={props.sourceFilter && props.setSourceFilter}>
              <div class="inline-flex items-center gap-1 rounded-md bg-surface-hover p-0.5">
                <label for="storage-source-filter" class="px-1.5 text-[9px] font-semibold uppercase tracking-wide text-muted">Source</label>
                <select
                  id="storage-source-filter"
                  value={props.sourceFilter!()}
                  onChange={(e) => props.setSourceFilter!(e.currentTarget.value)}
                  class="min-w-[8rem] rounded-md border border-border bg-surface px-2 py-1 text-xs font-medium text-base-content shadow-sm transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500"
                >
                  {sourceOptions().map((option) => (
                    <option value={option.key}>{option.label}</option>
                  ))}
                </select>
              </div>
              <div class="h-5 w-px bg-surface-hover hidden sm:block"></div>
            </Show>

            {/* Status Filter */}
            <div class="inline-flex items-center gap-1 rounded-md bg-surface-hover p-0.5">
              <label for="storage-status-filter" class="px-1.5 text-[9px] font-semibold uppercase tracking-wide text-muted">Status</label>
              <select
                id="storage-status-filter"
                value={props.statusFilter?.() ?? 'all'}
                onChange={(e) => props.setStatusFilter?.(e.currentTarget.value as StorageStatusFilter)}
                class="min-w-[8rem] rounded-md border px-2 py-1 text-xs font-medium shadow-sm transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500"
              >
                <option value="all">All</option>
                <option value="available">Healthy</option>
                <option value="warning">Warning</option>
                <option value="critical">Critical</option>
                <option value="offline">Offline</option>
                <option value="unknown">Unknown</option>
              </select>
            </div>

            <div class="h-5 w-px bg-surface-hover hidden sm:block"></div>

            {/* Sort controls */}
            <div class="flex items-center gap-1.5">
              <select
                value={props.sortKey()}
                onChange={(e) => props.setSortKey(e.currentTarget.value)}
                disabled={props.sortDisabled}
                aria-label="Sort By"
                class="px-2 py-1 text-xs border border-border rounded-md bg-surface text-base-content focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
              >
                {sortOptions.map((option) => (
                  <option value={option.value}>{option.label}</option>
                ))}
              </select>
              <button
                type="button"
                title={`Sort ${props.sortDirection() === 'asc' ? 'descending' : 'ascending'}`}
                onClick={() => props.setSortDirection(props.sortDirection() === 'asc' ? 'desc' : 'asc')}
                disabled={props.sortDisabled}
                aria-label="Sort Direction"
                class="inline-flex items-center justify-center h-7 w-7 rounded-md border border-border text-muted hover:bg-surface-hover transition-colors"
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
              <div class="h-5 w-px bg-surface-hover hidden sm:block"></div>
              <ColumnPicker
                columns={props.columnVisibility!.availableToggles()}
                isHidden={props.columnVisibility!.isHiddenByUser}
                onToggle={props.columnVisibility!.toggle}
                onReset={props.columnVisibility!.resetToDefaults}
              />
            </Show>

            {/* Reset Button - Only show when filters are active */}
            <Show when={hasActiveFilters()}>
              <div class="h-5 w-px bg-surface-hover hidden sm:block"></div>
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
                class="flex items-center gap-1 px-2.5 py-1.5 text-xs font-medium rounded-md transition-colors text-blue-700 dark:text-blue-300 bg-blue-100 dark:bg-blue-900 hover:bg-blue-200 dark:hover:bg-blue-900"
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
