import { Accessor, createMemo } from 'solid-js';
import type { StorageHealthFilter, StorageRecord } from '@/features/storageBackups/models';
import type { StorageCapacityDeltaPresentation } from '@/features/storageBackups/storageCapacityDeltaPresentation';
import { consolidateCephClusterPoolRecords } from '@/features/storageBackups/cephRecordPresentation';
import {
  buildStorageSourceOptions,
  filterStorageRecords,
  findSelectedStorageNode,
  groupStorageRecords,
  sortStorageRecords,
  summarizeStorageRecords,
  type StorageGroupKey,
  type StorageGroupedRecords,
  type StorageNodeOption,
  type StorageSortKey,
  type StorageSummary,
} from '@/features/storageBackups/storageModelCore';

export type {
  StorageGroupKey,
  StorageGroupedRecords,
  StorageNodeOption,
  StorageSortKey,
  StorageSummary,
} from '@/features/storageBackups/storageModelCore';

type UseStorageModelOptions = {
  records: Accessor<StorageRecord[]>;
  search: Accessor<string>;
  sourceFilter: Accessor<string>;
  healthFilter: Accessor<StorageHealthFilter>;
  selectedNodeId: Accessor<string>;
  nodeOptions: Accessor<StorageNodeOption[]>;
  sortKey: Accessor<StorageSortKey>;
  sortDirection: Accessor<'asc' | 'desc'>;
  storageGrowthBySeriesId?: Accessor<ReadonlyMap<string, StorageCapacityDeltaPresentation>>;
  groupBy: Accessor<StorageGroupKey>;
};

export const useStorageModel = (options: UseStorageModelOptions) => {
  const selectedNode = createMemo(() =>
    findSelectedStorageNode(options.selectedNodeId(), options.nodeOptions()),
  );

  const consolidatedRecords = createMemo(() =>
    consolidateCephClusterPoolRecords(options.records()),
  );

  const sourceOptions = createMemo(() => buildStorageSourceOptions(consolidatedRecords()));

  const filteredRecords = createMemo(() => {
    return filterStorageRecords(consolidatedRecords(), {
      search: options.search(),
      sourceFilter: options.sourceFilter(),
      healthFilter: options.healthFilter(),
      selectedNode: selectedNode(),
    });
  });

  const sortedRecords = createMemo(() =>
    sortStorageRecords(filteredRecords(), options.sortKey(), options.sortDirection(), {
      growthBySeriesId:
        options.sortKey() === 'growth' ? options.storageGrowthBySeriesId?.() : undefined,
    }),
  );

  const groupedRecords = createMemo<StorageGroupedRecords[]>(() =>
    groupStorageRecords(sortedRecords(), options.groupBy()),
  );

  const summary = createMemo<StorageSummary>(() => summarizeStorageRecords(filteredRecords()));

  return {
    sourceOptions,
    selectedNode,
    filteredRecords,
    sortedRecords,
    groupedRecords,
    summary,
  };
};
