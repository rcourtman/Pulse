import { createEffect, createMemo, type Accessor } from 'solid-js';
import { buildStorageSourceOptionsFromKeys } from '@/utils/storageSources';
import type { NormalizedHealth } from '@/features/storageBackups/models';
import {
  buildStorageNodeFilterOptions,
  coerceSelectedStorageNodeId,
  getActiveStorageNodeOptions,
  getStorageFilterGroupBy,
  getStorageStatusFilterValue,
  toStorageHealthFilterValue,
  type StorageView,
} from './storagePageState';
import type { StorageNodeOption, StorageGroupKey } from './useStorageModel';
import type { StorageGroupByFilter, StorageStatusFilter } from './StorageFilter';

type UseStorageFilterStateOptions = {
  view: Accessor<StorageView>;
  nodeOptions: Accessor<StorageNodeOption[]>;
  diskNodeOptions: Accessor<StorageNodeOption[]>;
  selectedNodeId: Accessor<string>;
  setSelectedNodeId: (value: string) => void;
  sourceOptions: Accessor<string[]>;
  healthFilter: Accessor<'all' | NormalizedHealth>;
  setHealthFilter: (value: 'all' | NormalizedHealth) => void;
  groupBy: Accessor<StorageGroupKey>;
};

export const useStorageFilterState = (options: UseStorageFilterStateOptions) => {
  const activeNodeOptions = createMemo(() =>
    getActiveStorageNodeOptions(options.view(), options.nodeOptions(), options.diskNodeOptions()),
  );
  const nodeFilterOptions = createMemo(() =>
    buildStorageNodeFilterOptions(options.view(), activeNodeOptions()),
  );
  const sourceFilterOptions = createMemo(() =>
    buildStorageSourceOptionsFromKeys(options.sourceOptions()),
  );

  const storageFilterGroupBy = (): StorageGroupByFilter => {
    return getStorageFilterGroupBy(options.groupBy());
  };

  const storageFilterStatus = (): StorageStatusFilter => {
    return getStorageStatusFilterValue(options.healthFilter());
  };

  const setStorageFilterStatus = (value: StorageStatusFilter) => {
    options.setHealthFilter(toStorageHealthFilterValue(value));
  };

  createEffect(() => {
    const next = coerceSelectedStorageNodeId(
      options.selectedNodeId(),
      activeNodeOptions(),
    );
    if (next !== options.selectedNodeId()) {
      options.setSelectedNodeId(next);
    }
  });

  return {
    activeNodeOptions,
    nodeFilterOptions,
    sourceFilterOptions,
    storageFilterGroupBy,
    storageFilterStatus,
    setStorageFilterStatus,
  };
};
