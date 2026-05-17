import { Component, JSX, Show, createMemo } from 'solid-js';
import { FilterBar, type FilterDef } from '@/components/shared/FilterBar';
import { Subtabs } from '@/components/shared/Subtabs';
import { ChartVisibilityToggleButton } from '@/components/shared/FilterToolbar';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { STORAGE_KEYS } from '@/utils/localStorage';
import {
  DEFAULT_PHYSICAL_DISK_FACET_FILTER,
  type PhysicalDiskFilterOption,
} from '@/features/storageBackups/diskPresentation';
import { STORAGE_VIEW_OPTIONS } from '@/features/storageBackups/storagePagePresentation';
import {
  getNextStorageSortDirection,
  getStorageSortDirectionIconClass,
  getStorageSortDirectionTitle,
  STORAGE_FILTER_SORT_DIRECTION_BUTTON_CLASS,
  STORAGE_FILTER_SORT_ICON_CLASS,
  STORAGE_FILTER_SORT_SELECT_CLASS,
  STORAGE_FILTER_SORT_WRAP_CLASS,
} from '@/features/storageBackups/storageFilterPresentation';
import {
  DEFAULT_STORAGE_GROUP_KEY,
  DEFAULT_STORAGE_SELECTED_NODE_ID,
  DEFAULT_STORAGE_SORT_DIRECTION,
  DEFAULT_STORAGE_SORT_KEY,
  DEFAULT_STORAGE_SORT_OPTIONS,
  DEFAULT_STORAGE_SOURCE_FILTER,
  DEFAULT_STORAGE_STATUS_FILTER,
  hasActiveStorageFilters,
  normalizeStorageSortKey,
  STORAGE_GROUP_BY_OPTIONS,
  STORAGE_STATUS_FILTER_OPTIONS,
  type StorageStatusFilterValue,
  type StorageView,
} from './storagePageState';
import type { StorageGroupKey, StorageSortKey } from './useStorageModel';
import type { StorageSourceOption } from '@/utils/storageSources';

type StoragePageControlsProps = {
  kioskMode: () => boolean;
  view: () => StorageView;
  setView: (value: StorageView) => void;
  search: () => string;
  setSearch: (value: string) => void;
  searchTrailing?: JSX.Element;
  groupBy: () => StorageGroupKey;
  setGroupBy: (value: StorageGroupKey) => void;
  sortKey: () => StorageSortKey;
  setSortKey: (value: StorageSortKey) => void;
  sortDirection: () => 'asc' | 'desc';
  setSortDirection: (value: 'asc' | 'desc') => void;
  statusFilter: () => StorageStatusFilterValue;
  setStatusFilter: (value: StorageStatusFilterValue) => void;
  sourceFilter: () => string;
  setSourceFilter: (value: string) => void;
  sourceOptions: () => StorageSourceOption[];
  diskRoleFilter?: () => string;
  setDiskRoleFilter?: (value: string) => void;
  diskRoleOptions?: () => PhysicalDiskFilterOption[];
  diskGroupFilter?: () => string;
  setDiskGroupFilter?: (value: string) => void;
  diskGroupOptions?: () => PhysicalDiskFilterOption[];
  nodeFilterOptions: Array<{ value: string; label: string }>;
  selectedNodeId: () => string;
  setSelectedNodeId: (value: string) => void;
  storageFilterGroupBy: () => StorageGroupKey;
  chartsCollapsed?: () => boolean;
  onChartsToggle?: () => void;
  // Mirrors the WorkloadsFilter `suppressPlatformFilter` contract: when a
  // platform page mounts StorageSurface with `forcedSourceFilter`, the
  // Source filter chip is redundant (it is already locked by the page)
  // and would render as a user-visible pinned filter. Setting this drops
  // the Source chip from the rendered filter row.
  suppressSourceFilter?: boolean;
};

const VIEW_TABS = STORAGE_VIEW_OPTIONS as { value: string; label: string }[];
const storageStatusDot = (className: string) => (
  <span class={`h-2 w-2 rounded-full ${className}`} />
);

type StorageViewSwitcherProps = {
  view: () => StorageView;
  setView: (value: StorageView) => void;
  class?: string;
};

export const StorageViewSwitcher: Component<StorageViewSwitcherProps> = (props) => (
  <Subtabs
    class={props.class}
    value={props.view()}
    onChange={(value) => props.setView(value as StorageView)}
    ariaLabel="Storage view"
    tabs={VIEW_TABS}
  />
);

export const StoragePageControls: Component<StoragePageControlsProps> = (props) => {
  const { isMobile } = useBreakpoint();
  const isPoolsView = () => props.view() === 'pools';
  const sortDisabled = () => !isPoolsView();

  const showClearAll = createMemo(
    () =>
      hasActiveStorageFilters({
        search: props.search(),
        sortKey: props.sortKey(),
        sortDirection: props.sortDirection(),
        groupBy: props.storageFilterGroupBy(),
        statusFilter: props.statusFilter(),
        sourceFilter: props.sourceFilter(),
        diskRoleFilter: props.diskRoleFilter?.(),
        diskGroupFilter: props.diskGroupFilter?.(),
      }) ||
      (props.selectedNodeId() || DEFAULT_STORAGE_SELECTED_NODE_ID) !==
        DEFAULT_STORAGE_SELECTED_NODE_ID,
  );

  const handleClearAll = () => {
    props.setSearch('');
    props.setSortKey(DEFAULT_STORAGE_SORT_KEY);
    props.setSortDirection(DEFAULT_STORAGE_SORT_DIRECTION);
    props.setGroupBy(DEFAULT_STORAGE_GROUP_KEY);
    props.setStatusFilter(DEFAULT_STORAGE_STATUS_FILTER as StorageStatusFilterValue);
    props.setSourceFilter(DEFAULT_STORAGE_SOURCE_FILTER);
    props.setDiskRoleFilter?.(DEFAULT_PHYSICAL_DISK_FACET_FILTER);
    props.setDiskGroupFilter?.(DEFAULT_PHYSICAL_DISK_FACET_FILTER);
    props.setSelectedNodeId(DEFAULT_STORAGE_SELECTED_NODE_ID);
  };

  const buildFilters = (): FilterDef[] => {
    const filters: FilterDef[] = [
      {
        id: 'storage-node',
        label: 'Node',
        group: 'scope',
        value: props.selectedNodeId,
        setValue: props.setSelectedNodeId,
        defaultValue: DEFAULT_STORAGE_SELECTED_NODE_ID,
        options: () =>
          props.nodeFilterOptions.map((option) => ({
            value: option.value,
            label: option.label,
          })),
      },
    ];

    if (isPoolsView()) {
      filters.push({
        id: 'storage-group-by',
        label: 'Group by',
        group: 'properties',
        inline: true,
        value: () => props.storageFilterGroupBy(),
        setValue: (value: string) => props.setGroupBy(value as StorageGroupKey),
        defaultValue: DEFAULT_STORAGE_GROUP_KEY,
        options: () =>
          STORAGE_GROUP_BY_OPTIONS.map((option) => ({
            value: option.value,
            label: option.label,
          })),
      });

      if (!props.suppressSourceFilter) {
        filters.push({
          id: 'storage-source',
          label: 'Source',
          group: 'scope',
          value: props.sourceFilter,
          setValue: props.setSourceFilter,
          defaultValue: DEFAULT_STORAGE_SOURCE_FILTER,
          options: () =>
            props.sourceOptions().map((option) => ({
              value: option.key,
              label: option.label,
            })),
        });
      }

      filters.push({
        id: 'storage-status',
        label: 'Status',
        group: 'status',
        inline: true,
        value: () => props.statusFilter(),
        setValue: (value: string) => props.setStatusFilter(value as StorageStatusFilterValue),
        defaultValue: DEFAULT_STORAGE_STATUS_FILTER as StorageStatusFilterValue,
        options: () =>
          STORAGE_STATUS_FILTER_OPTIONS.map((option) => ({
            value: option.value as string,
            label: option.label,
            leading:
              option.value === 'available'
                ? storageStatusDot('bg-emerald-500')
                : option.value === 'warning' || option.value === 'attention'
                  ? storageStatusDot('bg-amber-500')
                  : option.value === 'critical'
                    ? storageStatusDot('bg-red-500')
                    : option.value === 'offline' || option.value === 'unknown'
                      ? storageStatusDot('bg-slate-400')
                      : undefined,
            tone:
              option.value === 'available'
                ? 'success'
                : option.value === 'warning' || option.value === 'attention'
                  ? 'warning'
                  : option.value === 'critical'
                    ? 'danger'
                    : option.value === 'offline' || option.value === 'unknown'
                      ? 'muted'
                      : undefined,
          })),
      });
    } else {
      const diskRoleFilter = props.diskRoleFilter;
      const setDiskRoleFilter = props.setDiskRoleFilter;
      const diskRoleOptions = props.diskRoleOptions;
      if (diskRoleFilter && setDiskRoleFilter && diskRoleOptions && diskRoleOptions().length > 1) {
        filters.push({
          id: 'storage-disk-role',
          label: 'Role',
          group: 'properties',
          value: () => diskRoleFilter(),
          setValue: setDiskRoleFilter,
          defaultValue: DEFAULT_PHYSICAL_DISK_FACET_FILTER,
          options: () =>
            diskRoleOptions().map((option) => ({
              value: option.value,
              label: option.label,
            })),
        });
      }

      const diskGroupFilter = props.diskGroupFilter;
      const setDiskGroupFilter = props.setDiskGroupFilter;
      const diskGroupOptions = props.diskGroupOptions;
      if (
        diskGroupFilter &&
        setDiskGroupFilter &&
        diskGroupOptions &&
        diskGroupOptions().length > 1
      ) {
        filters.push({
          id: 'storage-disk-group',
          label: 'Group',
          group: 'properties',
          value: () => diskGroupFilter(),
          setValue: setDiskGroupFilter,
          defaultValue: DEFAULT_PHYSICAL_DISK_FACET_FILTER,
          options: () =>
            diskGroupOptions().map((option) => ({
              value: option.value,
              label: option.label,
            })),
        });
      }
    }

    return filters;
  };

  return (
    <Show when={!props.kioskMode()}>
      <div class="flex flex-col gap-2">
        <StorageViewSwitcher view={props.view} setView={props.setView} />

        <FilterBar
          role="group"
          ariaLabel="Storage filters"
          isMobile={isMobile}
          search={{
            value: props.search,
            setValue: props.setSearch,
            placeholder: 'Search storage... (e.g., local, nfs, node:pve1)',
            historyKey: STORAGE_KEYS.STORAGE_SEARCH_HISTORY,
            emptyMessage: 'Your recent storage searches will show here.',
          }}
          searchTrailing={props.searchTrailing}
          filters={buildFilters()}
          viewOptionsTrailing={
            <>
              <div class={STORAGE_FILTER_SORT_WRAP_CLASS}>
                <select
                  value={props.sortKey()}
                  onChange={(event) =>
                    props.setSortKey(
                      normalizeStorageSortKey(event.currentTarget.value) as StorageSortKey,
                    )
                  }
                  disabled={sortDisabled()}
                  aria-label="Sort by"
                  class={STORAGE_FILTER_SORT_SELECT_CLASS}
                >
                  {DEFAULT_STORAGE_SORT_OPTIONS.map((option) => (
                    <option value={option.value}>{option.label}</option>
                  ))}
                </select>
                <button
                  type="button"
                  title={getStorageSortDirectionTitle(props.sortDirection())}
                  onClick={() =>
                    props.setSortDirection(getNextStorageSortDirection(props.sortDirection()))
                  }
                  disabled={sortDisabled()}
                  aria-label="Sort direction"
                  class={STORAGE_FILTER_SORT_DIRECTION_BUTTON_CLASS}
                >
                  <svg
                    class={`${STORAGE_FILTER_SORT_ICON_CLASS} ${getStorageSortDirectionIconClass(
                      props.sortDirection(),
                    )}`}
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
              <Show when={props.onChartsToggle}>
                <ChartVisibilityToggleButton
                  collapsed={props.chartsCollapsed?.() ?? false}
                  onToggle={() => props.onChartsToggle?.()}
                />
              </Show>
            </>
          }
          onClearAll={handleClearAll}
          showClearAll={showClearAll}
        />
      </div>
    </Show>
  );
};

export default StoragePageControls;
