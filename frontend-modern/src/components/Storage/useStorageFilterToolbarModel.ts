import { createMemo, createSignal, type Accessor } from 'solid-js';
import { DEFAULT_STORAGE_SOURCE_OPTIONS, type StorageSourceOption } from '@/utils/storageSources';
import {
  getNextStorageSortDirection,
  getStorageSortDirectionIconClass,
  getStorageSortDirectionTitle,
} from '@/features/storageBackups/storageFilterPresentation';
import {
  countActiveStorageFilters,
  DEFAULT_STORAGE_GROUP_KEY,
  DEFAULT_STORAGE_SORT_DIRECTION,
  DEFAULT_STORAGE_SORT_KEY,
  DEFAULT_STORAGE_SOURCE_FILTER,
  DEFAULT_STORAGE_STATUS_FILTER,
  hasActiveStorageFilters,
  type StorageOption,
} from './storagePageState';
import type { StorageGroupByFilter, StorageStatusFilter } from './StorageFilter';

type UseStorageFilterToolbarModelOptions = {
  search: Accessor<string>;
  setSearch: (value: string) => void;
  groupBy?: Accessor<StorageGroupByFilter | undefined>;
  setGroupBy?: (value: StorageGroupByFilter) => void;
  sortKey: Accessor<string>;
  setSortKey: (value: string) => void;
  sortDirection: Accessor<'asc' | 'desc'>;
  setSortDirection: (value: 'asc' | 'desc') => void;
  statusFilter?: Accessor<StorageStatusFilter | undefined>;
  setStatusFilter?: (value: StorageStatusFilter) => void;
  sourceFilter?: Accessor<string | undefined>;
  setSourceFilter?: (value: string) => void;
  sortOptions?: StorageOption[];
  sourceOptions?: StorageSourceOption[];
};

export const useStorageFilterToolbarModel = (options: UseStorageFilterToolbarModelOptions) => {
  const [filtersOpen, setFiltersOpen] = createSignal(false);

  const activeFilterCount = createMemo(() =>
    countActiveStorageFilters({
      search: options.search(),
      sortKey: options.sortKey(),
      sortDirection: options.sortDirection(),
      groupBy: options.groupBy?.(),
      statusFilter: options.statusFilter?.(),
      sourceFilter: options.sourceFilter?.(),
    }),
  );

  const showReset = createMemo(() =>
    hasActiveStorageFilters({
      search: options.search(),
      sortKey: options.sortKey(),
      sortDirection: options.sortDirection(),
      groupBy: options.groupBy?.(),
      statusFilter: options.statusFilter?.(),
      sourceFilter: options.sourceFilter?.(),
    }),
  );

  const sortOptions = createMemo(() => options.sortOptions ?? []);
  const sourceOptions = createMemo(() => options.sourceOptions ?? DEFAULT_STORAGE_SOURCE_OPTIONS);
  const sortDirectionTitle = createMemo(() =>
    getStorageSortDirectionTitle(options.sortDirection()),
  );
  const sortDirectionIconClass = createMemo(() =>
    getStorageSortDirectionIconClass(options.sortDirection()),
  );

  const toggleSortDirection = () => {
    options.setSortDirection(getNextStorageSortDirection(options.sortDirection()));
  };

  const resetFilters = () => {
    options.setSearch('');
    options.setSortKey(DEFAULT_STORAGE_SORT_KEY);
    options.setSortDirection(DEFAULT_STORAGE_SORT_DIRECTION);
    if (options.setGroupBy) options.setGroupBy(DEFAULT_STORAGE_GROUP_KEY);
    if (options.setStatusFilter) options.setStatusFilter(DEFAULT_STORAGE_STATUS_FILTER);
    if (options.setSourceFilter) options.setSourceFilter(DEFAULT_STORAGE_SOURCE_FILTER);
  };

  return {
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
  };
};
