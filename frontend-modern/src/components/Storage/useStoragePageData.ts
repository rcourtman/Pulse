import { Accessor, createMemo } from 'solid-js';
import { buildStorageRecords } from '@/features/storageBackups/storageAdapters';
import type { StorageHealthFilter } from '@/features/storageBackups/models';
import type { StorageCapacityDeltaPresentation } from '@/features/storageBackups/storageCapacityDeltaPresentation';
import {
  buildPhysicalDiskGroupFilterOptions,
  buildPhysicalDiskRoleFilterOptions,
  getPhysicalDiskSourceKey,
} from '@/features/storageBackups/diskPresentation';
import type { State } from '@/types/api';
import type { Resource } from '@/types/resource';
import { useStorageAlertState } from './useStorageAlertState';
import { useStorageCephModel } from './useStorageCephModel';
import {
  buildStorageNodeOnlineByLabel,
  buildStorageNodeOptions,
  filterStorageDiskNodeOptions,
  storageResourceMatchesSourceFilter,
} from './storagePageState';
import { type StorageGroupKey, type StorageSortKey, useStorageModel } from './useStorageModel';

type UseStoragePageDataOptions = {
  state: Accessor<State>;
  resources: Accessor<Resource[]>;
  activeAlerts: Accessor<unknown> | unknown;
  alertsEnabled: Accessor<boolean>;
  nodes: Accessor<Resource[]>;
  physicalDisks: Accessor<Resource[]>;
  cephResources: Accessor<Resource[]>;
  search: Accessor<string>;
  sourceFilter: Accessor<string>;
  healthFilter: Accessor<StorageHealthFilter>;
  selectedNodeId: Accessor<string>;
  sortKey: Accessor<StorageSortKey>;
  sortDirection: Accessor<'asc' | 'desc'>;
  storageGrowthBySeriesId?: Accessor<ReadonlyMap<string, StorageCapacityDeltaPresentation>>;
  groupBy: Accessor<StorageGroupKey>;
};

export const useStoragePageData = (options: UseStoragePageDataOptions) => {
  const records = createMemo(() =>
    buildStorageRecords({ state: options.state(), resources: options.resources() }),
  );

  const { getRecordAlertState } = useStorageAlertState({
    records,
    activeAlerts: options.activeAlerts,
    alertsEnabled: options.alertsEnabled,
  });

  const sourceScopedNodes = createMemo(() =>
    options
      .nodes()
      .filter((node) => storageResourceMatchesSourceFilter(node, options.sourceFilter())),
  );

  const nodeOptions = createMemo(() => buildStorageNodeOptions(sourceScopedNodes()));

  const diskNodeOptions = createMemo(() =>
    filterStorageDiskNodeOptions(nodeOptions(), options.physicalDisks()),
  );

  const nodeOnlineByLabel = createMemo(() => buildStorageNodeOnlineByLabel(sourceScopedNodes()));

  const { sourceOptions, filteredRecords, groupedRecords } = useStorageModel({
    records,
    search: options.search,
    sourceFilter: options.sourceFilter,
    healthFilter: options.healthFilter,
    selectedNodeId: options.selectedNodeId,
    nodeOptions,
    sortKey: options.sortKey,
    sortDirection: options.sortDirection,
    storageGrowthBySeriesId: options.storageGrowthBySeriesId,
    groupBy: options.groupBy,
  });

  const diskSourceOptions = createMemo(() => [
    'all',
    ...Array.from(new Set(options.physicalDisks().map((disk) => getPhysicalDiskSourceKey(disk))))
      .filter((key) => key !== 'all')
      .sort(),
  ]);
  const diskRoleOptions = createMemo(() =>
    buildPhysicalDiskRoleFilterOptions(options.physicalDisks()),
  );
  const diskGroupOptions = createMemo(() =>
    buildPhysicalDiskGroupFilterOptions(options.physicalDisks()),
  );

  const { cephSummaryStats } = useStorageCephModel({
    records,
    cephResources: options.cephResources,
  });

  return {
    records,
    getRecordAlertState,
    nodeOptions,
    diskNodeOptions,
    nodeOnlineByLabel,
    sourceOptions,
    diskSourceOptions,
    diskRoleOptions,
    diskGroupOptions,
    filteredRecords,
    groupedRecords,
    cephSummaryStats,
  };
};
