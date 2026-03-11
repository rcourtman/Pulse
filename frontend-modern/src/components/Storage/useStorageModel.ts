import { Accessor, createMemo } from 'solid-js';
import type { NormalizedHealth, StorageRecord } from '@/features/storageBackups/models';
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
  healthFilter: Accessor<'all' | NormalizedHealth>;
  selectedNodeId: Accessor<string>;
  nodeOptions: Accessor<StorageNodeOption[]>;
  sortKey: Accessor<StorageSortKey>;
  sortDirection: Accessor<'asc' | 'desc'>;
  groupBy: Accessor<StorageGroupKey>;
};

export const useStorageModel = (options: UseStorageModelOptions) => {
  const selectedNode = createMemo(() =>
    findSelectedStorageNode(options.selectedNodeId(), options.nodeOptions()),
  );

  const sourceOptions = createMemo(() => buildStorageSourceOptions(options.records()));

  const filteredRecords = createMemo(() => {
    return filterStorageRecords(options.records(), {
      search: options.search(),
      sourceFilter: options.sourceFilter(),
      healthFilter: options.healthFilter(),
      selectedNode: selectedNode(),
    });
  });

  const sortedRecords = createMemo(() =>
    sortStorageRecords(filteredRecords(), options.sortKey(), options.sortDirection()),
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
