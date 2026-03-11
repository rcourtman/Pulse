import type { Accessor } from 'solid-js';
import type { NormalizedHealth, StorageRecord } from '@/features/storageBackups/models';
import { isCephStorageRecord } from '@/features/storageBackups/cephRecordPresentation';
import type { Resource } from '@/types/resource';
import { getProxmoxData } from '@/utils/resourcePlatformData';
import { getResourceIdentityAliases } from '@/utils/resourceIdentity';
import { matchesPhysicalDiskNode } from './diskResourceUtils';
import type { StorageGroupKey, StorageSortKey } from './useStorageModel';
import type { StorageRouteStateFields } from './useStorageRouteState';

export type StorageView = 'pools' | 'disks';
export type StorageStatusFilterValue =
  | 'all'
  | 'available'
  | 'warning'
  | 'critical'
  | 'offline'
  | 'unknown';

export interface StorageOption {
  value: string;
  label: string;
}

export interface StorageFilterActivityState {
  search: string;
  sortKey: string;
  sortDirection: 'asc' | 'desc';
  groupBy?: StorageGroupKey;
  statusFilter?: StorageStatusFilterValue;
  sourceFilter?: string;
}

export const DEFAULT_STORAGE_VIEW: StorageView = 'pools';
export const DEFAULT_STORAGE_SELECTED_NODE_ID = 'all';
export const DEFAULT_STORAGE_SOURCE_FILTER = 'all';
export const DEFAULT_STORAGE_SORT_KEY: StorageSortKey = 'priority';
export const DEFAULT_STORAGE_SORT_DIRECTION: 'asc' | 'desc' = 'desc';
export const DEFAULT_STORAGE_GROUP_KEY: StorageGroupKey = 'none';
export const DEFAULT_STORAGE_STATUS_FILTER: StorageStatusFilterValue = 'all';

export type StoragePageNodeOption = {
  id: string;
  label: string;
  instance?: string;
  aliases?: string[];
};

export const DEFAULT_STORAGE_SORT_OPTIONS: Array<{ value: StorageSortKey; label: string }> = [
  { value: 'priority', label: 'Priority' },
  { value: 'name', label: 'Name' },
  { value: 'usage', label: 'Usage %' },
  { value: 'type', label: 'Type' },
];

export const STORAGE_STATUS_FILTER_OPTIONS: StorageOption[] = [
  { value: 'all', label: 'All' },
  { value: 'available', label: 'Healthy' },
  { value: 'warning', label: 'Warning' },
  { value: 'critical', label: 'Critical' },
  { value: 'offline', label: 'Offline' },
  { value: 'unknown', label: 'Unknown' },
];

export const STORAGE_GROUP_BY_OPTIONS: StorageOption[] = [
  { value: 'none', label: 'Flat' },
  { value: 'node', label: 'By Node' },
  { value: 'type', label: 'By Type' },
  { value: 'status', label: 'By Status' },
];

export const normalizeStorageHealthFilter = (value: string): 'all' | NormalizedHealth => {
  const normalized = (value || '').trim().toLowerCase();
  if (!normalized || normalized === 'all') return 'all';
  if (normalized === 'available' || normalized === 'online' || normalized === 'healthy') {
    return 'healthy';
  }
  if (normalized === 'degraded' || normalized === 'warning') return 'warning';
  if (normalized === 'critical') return 'critical';
  if (normalized === 'offline') return 'offline';
  if (normalized === 'unknown') return 'unknown';
  return 'all';
};

export const normalizeStorageSortKey = (value: string): StorageSortKey => {
  if (value === 'priority' || value === 'name' || value === 'usage' || value === 'type') {
    return value;
  }
  return 'priority';
};

export const normalizeStorageGroupKey = (value: string): StorageGroupKey => {
  if (value === 'none' || value === 'node' || value === 'type' || value === 'status') {
    return value;
  }
  return 'none';
};

export const normalizeStorageView = (value: string): StorageView =>
  value === 'disks' ? 'disks' : 'pools';

export const normalizeStorageSortDirection = (value: string): 'asc' | 'desc' =>
  value === 'asc' ? 'asc' : 'desc';

export const getStorageFilterGroupBy = (
  value: StorageGroupKey,
): 'node' | 'type' | 'status' | 'none' =>
  value === 'type' || value === 'status' || value === 'none' ? value : 'node';

export const getStorageStatusFilterValue = (
  value: 'all' | NormalizedHealth,
): StorageStatusFilterValue => {
  if (value === 'all') return 'all';
  if (value === 'healthy') return 'available';
  return value;
};

export const toStorageHealthFilterValue = (
  value: StorageStatusFilterValue,
): 'all' | NormalizedHealth => {
  if (value === 'all') return 'all';
  if (value === 'available') return 'healthy';
  return value;
};

export const countActiveStorageFilters = (state: StorageFilterActivityState): number => {
  let count = 0;
  if (state.search.trim() !== '') count++;
  if ((state.groupBy || 'none') !== 'none') count++;
  if ((state.statusFilter || 'all') !== 'all') count++;
  if ((state.sourceFilter || 'all') !== 'all') count++;
  return count;
};

export const hasActiveStorageFilters = (state: StorageFilterActivityState): boolean =>
  state.search.trim() !== '' ||
  state.sortKey !== DEFAULT_STORAGE_SORT_KEY ||
  state.sortDirection !== DEFAULT_STORAGE_SORT_DIRECTION ||
  (state.groupBy || DEFAULT_STORAGE_GROUP_KEY) !== DEFAULT_STORAGE_GROUP_KEY ||
  (state.statusFilter || DEFAULT_STORAGE_STATUS_FILTER) !== DEFAULT_STORAGE_STATUS_FILTER ||
  (state.sourceFilter || DEFAULT_STORAGE_SOURCE_FILTER) !== DEFAULT_STORAGE_SOURCE_FILTER;

export const getStorageNodeFilterLabel = (view: StorageView): string =>
  view === 'disks' ? 'All Disk Hosts' : 'All Nodes';

export const readStorageRouteValue = (
  value: string | undefined,
  defaultValue: string,
): string => value || defaultValue;

export const writeStorageRouteValue = (
  value: string,
  defaultValue: string,
): string | null => (value !== defaultValue ? value : null);

type StorageRouteFieldBuilderOptions = {
  view: Accessor<StorageView>;
  setView: (value: StorageView) => void;
  sourceFilter: Accessor<string>;
  setSourceFilter: (value: string) => void;
  healthFilter: Accessor<'all' | NormalizedHealth>;
  setHealthFilter: (value: 'all' | NormalizedHealth) => void;
  selectedNodeId: Accessor<string>;
  setSelectedNodeId: (value: string) => void;
  groupBy: Accessor<StorageGroupKey>;
  setGroupBy: (value: StorageGroupKey) => void;
  sortKey: Accessor<StorageSortKey>;
  setSortKey: (value: StorageSortKey) => void;
  sortDirection: Accessor<'asc' | 'desc'>;
  setSortDirection: (value: 'asc' | 'desc') => void;
  search: Accessor<string>;
  setSearch: (value: string) => void;
};

export const buildStorageRouteFields = (
  options: StorageRouteFieldBuilderOptions,
): StorageRouteStateFields => ({
  tab: {
    get: options.view,
    set: options.setView,
    read: (parsed) => normalizeStorageView(parsed.tab),
    write: (value) => (value !== DEFAULT_STORAGE_VIEW ? value : null),
  },
  source: {
    get: options.sourceFilter,
    set: options.setSourceFilter,
    read: (parsed) => readStorageRouteValue(parsed.source, DEFAULT_STORAGE_SOURCE_FILTER),
    write: (value) => writeStorageRouteValue(value, DEFAULT_STORAGE_SOURCE_FILTER),
  },
  status: {
    get: options.healthFilter,
    set: options.setHealthFilter,
    read: (parsed) => normalizeStorageHealthFilter(parsed.status),
    write: (value) => writeStorageRouteValue(value, DEFAULT_STORAGE_STATUS_FILTER),
  },
  node: {
    get: options.selectedNodeId,
    set: options.setSelectedNodeId,
    read: (parsed) => readStorageRouteValue(parsed.node, DEFAULT_STORAGE_SELECTED_NODE_ID),
    write: (value) => writeStorageRouteValue(value, DEFAULT_STORAGE_SELECTED_NODE_ID),
  },
  group: {
    get: options.groupBy,
    set: options.setGroupBy,
    read: (parsed) => normalizeStorageGroupKey(parsed.group),
    write: (value) => writeStorageRouteValue(value, DEFAULT_STORAGE_GROUP_KEY),
  },
  sort: {
    get: options.sortKey,
    set: options.setSortKey,
    read: (parsed) => normalizeStorageSortKey(parsed.sort),
    write: (value) => writeStorageRouteValue(value, DEFAULT_STORAGE_SORT_KEY),
  },
  order: {
    get: options.sortDirection,
    set: options.setSortDirection,
    read: (parsed) => normalizeStorageSortDirection(parsed.order),
    write: (value) => writeStorageRouteValue(value, DEFAULT_STORAGE_SORT_DIRECTION),
  },
  query: {
    get: options.search,
    set: options.setSearch,
    read: (parsed) => parsed.query,
    write: (value) => value.trim() || null,
  },
});

export const coerceSelectedStorageNodeId = (
  selectedNodeId: string,
  nodeOptions: StoragePageNodeOption[],
): string =>
  selectedNodeId === DEFAULT_STORAGE_SELECTED_NODE_ID ||
  nodeOptions.some((node) => node.id === selectedNodeId)
    ? selectedNodeId
    : DEFAULT_STORAGE_SELECTED_NODE_ID;

export const buildStorageNodeFilterOptions = (
  view: StorageView,
  nodeOptions: StoragePageNodeOption[],
): StorageOption[] => [
  { value: DEFAULT_STORAGE_SELECTED_NODE_ID, label: getStorageNodeFilterLabel(view) },
  ...nodeOptions.map((node) => ({ value: node.id, label: node.label })),
];

export const getStorageMetaBoolean = (
  record: StorageRecord,
  key: 'isCeph' | 'isZfs',
): boolean | null => {
  const details = (record.details || {}) as Record<string, unknown>;
  const value = details[key];
  return typeof value === 'boolean' ? value : null;
};

export const isStorageRecordCeph = (record: StorageRecord): boolean => {
  const isCephMeta = getStorageMetaBoolean(record, 'isCeph');
  if (isCephMeta !== null) return isCephMeta;
  return isCephStorageRecord(record);
};

export const buildStorageNodeOptions = (nodes: Resource[]): StoragePageNodeOption[] =>
  nodes.map((node) => ({
    id: node.id,
    label: node.name,
    instance: getProxmoxData(node)?.instance,
    aliases: getResourceIdentityAliases(node),
  }));

export const filterStorageDiskNodeOptions = (
  nodeOptions: StoragePageNodeOption[],
  physicalDisks: Resource[],
): StoragePageNodeOption[] =>
  nodeOptions.filter((node) =>
    physicalDisks.some((disk) =>
      matchesPhysicalDiskNode(disk, {
        id: node.id,
        name: node.label,
        instance: node.instance,
      }),
    ),
  );

export const getActiveStorageNodeOptions = (
  view: StorageView,
  nodeOptions: StoragePageNodeOption[],
  diskNodeOptions: StoragePageNodeOption[],
): StoragePageNodeOption[] => (view === 'disks' ? diskNodeOptions : nodeOptions);

export const buildStorageNodeOnlineByLabel = (nodes: Resource[]): Map<string, boolean> => {
  const map = new Map<string, boolean>();
  nodes.forEach((node) => {
    const key = (node.name || '').trim().toLowerCase();
    if (!key) return;
    const hasNodeStatus = typeof node.status === 'string' && node.status.trim().length > 0;
    const hasNodeUptime = typeof node.uptime === 'number';
    if (!hasNodeStatus && !hasNodeUptime) {
      return;
    }
    map.set(key, node.status === 'online' && (node.uptime || 0) > 0);
  });
  return map;
};

export const syncExpandedStorageGroups = (
  previous: Set<string>,
  allKeys: string[],
): Set<string> => {
  if (previous.size === 0) return new Set(allKeys);

  const next = new Set(previous);
  let changed = false;
  for (const key of allKeys) {
    if (!next.has(key)) {
      next.add(key);
      changed = true;
    }
  }
  return changed ? next : previous;
};

export const toggleExpandedStorageGroup = (
  previous: Set<string>,
  key: string,
): Set<string> => {
  const next = new Set(previous);
  if (next.has(key)) next.delete(key);
  else next.add(key);
  return next;
};

export const countVisiblePhysicalDisksForNode = (
  selectedNodeId: string,
  nodeOptions: StoragePageNodeOption[],
  physicalDisks: Resource[],
): number => {
  if (selectedNodeId === 'all') return physicalDisks.length;
  const node = nodeOptions.find((candidate) => candidate.id === selectedNodeId);
  if (!node) return physicalDisks.length;
  return physicalDisks.filter((disk) =>
    matchesPhysicalDiskNode(disk, { id: node.id, name: node.label, instance: node.instance }),
  ).length;
};
