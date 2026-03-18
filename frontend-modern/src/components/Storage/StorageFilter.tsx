import { Component, Show, type JSX } from 'solid-js';
import { Card } from '@/components/shared/Card';
import {
  FilterDivider,
  FilterSegmentedControl,
  LabeledFilterSelect,
} from '@/components/shared/FilterToolbar';
import { PageControls } from '@/components/shared/PageControls';
import { SearchInput } from '@/components/shared/SearchInput';
import type { ColumnDef } from '@/hooks/useColumnVisibility';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { STORAGE_KEYS } from '@/utils/localStorage';
import type { StorageSourceOption } from '@/utils/storageSources';
import {
  STORAGE_FILTER_COMPACT_SELECT_CLASS,
  STORAGE_FILTER_RESET_ACTION_CLASS,
  STORAGE_FILTER_SEGMENTED_WRAP_CLASS,
  STORAGE_FILTER_SORT_DIRECTION_BUTTON_CLASS,
  STORAGE_FILTER_SORT_ICON_CLASS,
  STORAGE_FILTER_SORT_WRAP_CLASS,
  STORAGE_FILTER_SORT_SELECT_CLASS,
} from '@/features/storageBackups/storageFilterPresentation';
import {
  DEFAULT_STORAGE_SORT_OPTIONS,
  STORAGE_GROUP_BY_OPTIONS,
  STORAGE_STATUS_FILTER_OPTIONS,
} from './storagePageState';
import { useStorageFilterToolbarModel } from './useStorageFilterToolbarModel';

export type StorageStatusFilter =
  | 'all'
  | 'available'
  | 'warning'
  | 'critical'
  | 'offline'
  | 'unknown';
export type StorageGroupByFilter = 'node' | 'type' | 'status' | 'none';

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
  const {
    filtersOpen,
    setFiltersOpen,
    activeFilterCount,
    showReset,
    sortOptions,
    sourceOptions,
    sortDirectionTitle,
    sortDirectionIconClass,
    toggleSortDirection,
    resetFilters,
  } = useStorageFilterToolbarModel({
    search: props.search,
    setSearch: props.setSearch,
    groupBy: props.groupBy,
    setGroupBy: props.setGroupBy,
    sortKey: props.sortKey,
    setSortKey: props.setSortKey,
    sortDirection: props.sortDirection,
    setSortDirection: props.setSortDirection,
    statusFilter: props.statusFilter,
    setStatusFilter: props.setStatusFilter,
    sourceFilter: props.sourceFilter,
    setSourceFilter: props.setSourceFilter,
    sortOptions: props.sortOptions ?? DEFAULT_STORAGE_SORT_OPTIONS,
    sourceOptions: props.sourceOptions,
  });

  return (
    <Card class="storage-filter mb-3" padding="sm">
      <PageControls
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
        mobileFilters={{
          enabled: isMobile(),
          onToggle: () => setFiltersOpen((o) => !o),
          count: activeFilterCount(),
        }}
        columnVisibility={props.columnVisibility}
        resetAction={{
          show: showReset(),
          onClick: resetFilters,
          title: 'Reset all filters',
          class: STORAGE_FILTER_RESET_ACTION_CLASS,
          icon: (
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
          ),
        }}
        showFilters={!isMobile() || filtersOpen()}
        toolbarClass="gap-x-1.5 gap-y-2 sm:gap-x-2"
      >
        {props.leadingFilters}

        <Show when={props.groupBy && props.setGroupBy}>
          <div class={STORAGE_FILTER_SEGMENTED_WRAP_CLASS}>
            <FilterSegmentedControl
              value={props.groupBy!()}
              onChange={(value) => props.setGroupBy!(value as StorageGroupByFilter)}
              aria-label="Group By"
              options={STORAGE_GROUP_BY_OPTIONS}
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
            selectClass={STORAGE_FILTER_COMPACT_SELECT_CLASS}
          >
            {sourceOptions().map((option: StorageSourceOption) => (
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
          selectClass={STORAGE_FILTER_COMPACT_SELECT_CLASS}
        >
          {STORAGE_STATUS_FILTER_OPTIONS.map((option) => (
            <option value={option.value}>{option.label}</option>
          ))}
        </LabeledFilterSelect>

        <FilterDivider />

        <div class={STORAGE_FILTER_SORT_WRAP_CLASS}>
          <select
            value={props.sortKey()}
            onChange={(e) => props.setSortKey(e.currentTarget.value)}
            disabled={props.sortDisabled}
            aria-label="Sort By"
            class={STORAGE_FILTER_SORT_SELECT_CLASS}
          >
            {sortOptions().map((option) => (
              <option value={option.value}>{option.label}</option>
            ))}
          </select>
          <button
            type="button"
            title={sortDirectionTitle()}
            onClick={toggleSortDirection}
            disabled={props.sortDisabled}
            aria-label="Sort Direction"
            class={STORAGE_FILTER_SORT_DIRECTION_BUTTON_CLASS}
          >
            <svg
              class={`${STORAGE_FILTER_SORT_ICON_CLASS} ${sortDirectionIconClass()}`}
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <path stroke-linecap="round" stroke-linejoin="round" d="M8 9l4-4 4 4m0 6l-4 4-4-4" />
            </svg>
          </button>
        </div>
      </PageControls>
    </Card>
  );
};
