import { createSignal } from 'solid-js';
import { buildStorageRouteSearch } from '@/routing/resourceLinks';
import type { StorageHealthFilter } from '@/features/storageBackups/models';
import { useStorageRouteState } from './useStorageRouteState';
import {
  buildStorageRouteFields,
  DEFAULT_STORAGE_GROUP_KEY,
  DEFAULT_STORAGE_DISK_GROUP_FILTER,
  DEFAULT_STORAGE_DISK_ROLE_FILTER,
  DEFAULT_STORAGE_SORT_DIRECTION,
  DEFAULT_STORAGE_SORT_KEY,
  DEFAULT_STORAGE_SOURCE_FILTER,
  DEFAULT_STORAGE_SELECTED_NODE_ID,
  DEFAULT_STORAGE_VIEW,
  type StorageView,
} from './storagePageState';
import type { StorageGroupKey, StorageSortKey } from './useStorageModel';

type UseStoragePageFiltersOptions = {
  location: {
    pathname: string;
    search: string;
  };
  navigate: (path: string, options: { replace: true }) => void;
  lockedSourceFilter?: () => string | undefined;
};

export const useStoragePageFilters = (options: UseStoragePageFiltersOptions) => {
  const [search, setSearch] = createSignal('');
  const [sourceFilter, setSourceFilter] = createSignal(DEFAULT_STORAGE_SOURCE_FILTER);
  const [healthFilter, setHealthFilter] = createSignal<StorageHealthFilter>('all');
  const [diskRoleFilter, setDiskRoleFilter] = createSignal(DEFAULT_STORAGE_DISK_ROLE_FILTER);
  const [diskGroupFilter, setDiskGroupFilter] = createSignal(DEFAULT_STORAGE_DISK_GROUP_FILTER);
  const [view, setView] = createSignal<StorageView>(DEFAULT_STORAGE_VIEW);
  const [selectedNodeId, setSelectedNodeId] = createSignal(DEFAULT_STORAGE_SELECTED_NODE_ID);
  const [sortKey, setSortKey] = createSignal<StorageSortKey>(DEFAULT_STORAGE_SORT_KEY);
  const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>(
    DEFAULT_STORAGE_SORT_DIRECTION,
  );
  const [groupBy, setGroupBy] = createSignal<StorageGroupKey>(DEFAULT_STORAGE_GROUP_KEY);

  // Storage can remain mounted as a warmed Proxmox tab. Route effects must
  // stay dormant while another tab is visible or the hidden surface will
  // parse and rewrite unrelated query state on every platform navigation.
  const isActiveStorageRoute = () => options.location.pathname.endsWith('/storage');

  const routeFields = buildStorageRouteFields({
    view,
    setView,
    sourceFilter,
    setSourceFilter,
    healthFilter,
    setHealthFilter,
    diskRoleFilter,
    setDiskRoleFilter,
    diskGroupFilter,
    setDiskGroupFilter,
    selectedNodeId,
    setSelectedNodeId,
    groupBy,
    setGroupBy,
    sortKey,
    setSortKey,
    sortDirection,
    setSortDirection,
    search,
    setSearch,
  });
  if (options.lockedSourceFilter?.()?.trim() && routeFields.source) {
    routeFields.source = {
      ...routeFields.source,
      read: () => options.lockedSourceFilter?.()?.trim() || DEFAULT_STORAGE_SOURCE_FILTER,
      write: () => null,
    };
  }

  useStorageRouteState({
    location: options.location,
    navigate: options.navigate,
    buildPath: (routeOptions) => {
      const search = buildStorageRouteSearch(routeOptions);
      return `${options.location.pathname}${search}`;
    },
    isReadEnabled: isActiveStorageRoute,
    isWriteEnabled: isActiveStorageRoute,
    useCurrentPathForNavigation: true,
    fields: routeFields,
  });

  return {
    search,
    setSearch,
    sourceFilter,
    setSourceFilter,
    healthFilter,
    setHealthFilter,
    diskRoleFilter,
    setDiskRoleFilter,
    diskGroupFilter,
    setDiskGroupFilter,
    view,
    setView,
    selectedNodeId,
    setSelectedNodeId,
    sortKey,
    setSortKey,
    sortDirection,
    setSortDirection,
    groupBy,
    setGroupBy,
  };
};
