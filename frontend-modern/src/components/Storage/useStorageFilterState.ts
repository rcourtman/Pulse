import { createEffect, createMemo, type Accessor } from 'solid-js';
import { buildStorageSourceOptionsFromKeys } from '@/utils/storageSources';
import type { StorageHealthFilter } from '@/features/storageBackups/models';
import { normalizeStorageSourceKey } from '@/utils/storageSources';
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
  diskSourceOptions?: Accessor<string[]>;
  sourceFilter: Accessor<string>;
  setSourceFilter: (value: string) => void;
  healthFilter: Accessor<StorageHealthFilter>;
  setHealthFilter: (value: StorageHealthFilter) => void;
  groupBy: Accessor<StorageGroupKey>;
};

export const useStorageFilterState = (options: UseStorageFilterStateOptions) => {
  const activeNodeOptions = createMemo(() =>
    getActiveStorageNodeOptions(options.view(), options.nodeOptions(), options.diskNodeOptions()),
  );
  const nodeFilterOptions = createMemo(() =>
    buildStorageNodeFilterOptions(options.view(), activeNodeOptions()),
  );
  const activeSourceKeys = createMemo(() =>
    options.view() === 'disks' && options.diskSourceOptions
      ? options.diskSourceOptions()
      : options.sourceOptions(),
  );
  const sourceFilterOptions = createMemo(() =>
    buildStorageSourceOptionsFromKeys(activeSourceKeys()),
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
    const next = coerceSelectedStorageNodeId(options.selectedNodeId(), activeNodeOptions());
    if (next !== options.selectedNodeId()) {
      options.setSelectedNodeId(next);
    }
  });

  createEffect(() => {
    const selectedSource = normalizeStorageSourceKey(options.sourceFilter());
    if (selectedSource === 'all') {
      return;
    }
    const availableSources = new Set(
      activeSourceKeys().map((key) => normalizeStorageSourceKey(key)),
    );
    if (!availableSources.has(selectedSource)) {
      options.setSourceFilter('all');
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
