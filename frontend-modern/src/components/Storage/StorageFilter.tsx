import { Component, Show, createMemo, createSignal, type JSX } from 'solid-js';
import { Card } from '@/components/shared/Card';
import {
  FilterActionButton,
  FilterDivider,
  FilterHeader,
  FilterMobileToggleButton,
  FilterSegmentedControl,
  LabeledFilterSelect,
} from '@/components/shared/FilterToolbar';
import { SearchInput } from '@/components/shared/SearchInput';
import { ColumnPicker } from '@/components/shared/ColumnPicker';
import type { ColumnDef } from '@/hooks/useColumnVisibility';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { STORAGE_KEYS } from '@/utils/localStorage';
import type { StorageSourceOption } from './storageSourceOptions';

export type StorageStatusFilter =
  | 'all'
  | 'available'
  | 'warning'
  | 'critical'
  | 'offline'
  | 'unknown';
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
      <FilterHeader
        contentClass="gap-3"
        search={
          <SearchInput
            value={props.search}
            onChange={props.setSearch}
            placeholder="Search storage... (e.g., local, nfs, node:pve1)"
            title="Search storage by name or filter by node"
            typeToSearch
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
        }
        searchAccessory={
          <Show when={isMobile()}>
            <FilterMobileToggleButton
              onClick={() => setFiltersOpen((o) => !o)}
              count={activeFilterCount()}
            />
          </Show>
        }
        showFilters={!isMobile() || filtersOpen()}
        toolbarClass="gap-x-1.5 gap-y-2 sm:gap-x-2"
      >
        {props.leadingFilters}

        <Show when={props.groupBy && props.setGroupBy}>
          <div class="max-w-full overflow-x-auto scrollbar-hide">
            <FilterSegmentedControl
              value={props.groupBy!()}
              onChange={(value) => props.setGroupBy!(value as StorageGroupByFilter)}
              aria-label="Group By"
              options={[
                { value: 'node', label: 'By Node' },
                { value: 'type', label: 'By Type' },
                { value: 'status', label: 'By Status' },
              ]}
            />
          </div>
          <FilterDivider />
        </Show>

        <Show when={props.sourceFilter && props.setSourceFilter}>
          <LabeledFilterSelect
            id="storage-source-filter"
            label="Source"
            value={props.sourceFilter!()}
            onChange={(e) => props.setSourceFilter!(e.currentTarget.value)}
            selectClass="min-w-[8rem]"
          >
            {sourceOptions().map((option) => (
              <option value={option.key}>{option.label}</option>
            ))}
          </LabeledFilterSelect>
          <FilterDivider />
        </Show>

        <LabeledFilterSelect
          id="storage-status-filter"
          label="Status"
          value={props.statusFilter?.() ?? 'all'}
          onChange={(e) => props.setStatusFilter?.(e.currentTarget.value as StorageStatusFilter)}
          selectClass="min-w-[8rem]"
        >
          <option value="all">All</option>
          <option value="available">Healthy</option>
          <option value="warning">Warning</option>
          <option value="critical">Critical</option>
          <option value="offline">Offline</option>
          <option value="unknown">Unknown</option>
        </LabeledFilterSelect>

        <FilterDivider />

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
              <path stroke-linecap="round" stroke-linejoin="round" d="M8 9l4-4 4 4m0 6l-4 4-4-4" />
            </svg>
          </button>
        </div>

        <Show when={props.columnVisibility}>
          <FilterDivider />
          <ColumnPicker
            columns={props.columnVisibility!.availableToggles()}
            isHidden={props.columnVisibility!.isHiddenByUser}
            onToggle={props.columnVisibility!.toggle}
            onReset={props.columnVisibility!.resetToDefaults}
          />
        </Show>

        <Show when={hasActiveFilters()}>
          <FilterDivider />
          <FilterActionButton
            onClick={() => {
              props.setSearch('');
              props.setSortKey('name');
              props.setSortDirection('asc');
              if (props.setGroupBy) props.setGroupBy('node');
              if (props.setStatusFilter) props.setStatusFilter('all');
              if (props.setSourceFilter) props.setSourceFilter('all');
            }}
            title="Reset all filters"
            class="text-base-content"
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
          </FilterActionButton>
        </Show>
      </FilterHeader>
    </Card>
  );
};
