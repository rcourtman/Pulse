import { Accessor, createMemo } from 'solid-js';
import { buildStorageRecords } from '@/features/storageBackups/storageAdapters';
import type { NormalizedHealth } from '@/features/storageBackups/models';
import type { State } from '@/types/api';
import type { Resource } from '@/types/resource';
import { useStorageAlertState } from './useStorageAlertState';
import { useStorageCephModel } from './useStorageCephModel';
import {
  buildStorageNodeOnlineByLabel,
  buildStorageNodeOptions,
  filterStorageDiskNodeOptions,
} from './storagePageState';
import {
  type StorageGroupKey,
  type StorageSortKey,
  useStorageModel,
} from './useStorageModel';

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
  healthFilter: Accessor<'all' | NormalizedHealth>;
  selectedNodeId: Accessor<string>;
  sortKey: Accessor<StorageSortKey>;
  sortDirection: Accessor<'asc' | 'desc'>;
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

  const nodeOptions = createMemo(() => buildStorageNodeOptions(options.nodes()));

  const diskNodeOptions = createMemo(() =>
    filterStorageDiskNodeOptions(nodeOptions(), options.physicalDisks()),
  );

  const nodeOnlineByLabel = createMemo(() => buildStorageNodeOnlineByLabel(options.nodes()));

  const { sourceOptions, filteredRecords, groupedRecords } = useStorageModel({
    records,
    search: options.search,
    sourceFilter: options.sourceFilter,
    healthFilter: options.healthFilter,
    selectedNodeId: options.selectedNodeId,
    nodeOptions,
    sortKey: options.sortKey,
    sortDirection: options.sortDirection,
    groupBy: options.groupBy,
  });

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
    filteredRecords,
    groupedRecords,
    cephSummaryStats,
  };
};
