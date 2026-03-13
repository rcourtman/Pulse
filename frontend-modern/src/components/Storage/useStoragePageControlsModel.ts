import { createMemo } from 'solid-js';
import { normalizeStorageSortKey, type StorageView } from './storagePageState';
import type { StorageGroupKey, StorageSortKey } from './useStorageModel';
import type { StorageGroupByFilter } from './StorageFilter';

type UseStoragePageControlsModelOptions = {
  kioskMode: () => boolean;
  view: () => StorageView;
  setGroupBy: (value: StorageGroupKey) => void;
  setSortKey: (value: StorageSortKey) => void;
  storageFilterGroupBy: () => StorageGroupByFilter;
};

export const useStoragePageControlsModel = (options: UseStoragePageControlsModelOptions) => {
  const showControls = createMemo(() => !options.kioskMode());
  const sortDisabled = createMemo(() => options.view() !== 'pools');
  const groupBy = () => (options.view() === 'pools' ? options.storageFilterGroupBy : undefined);
  const setGroupBy = () =>
    options.view() === 'pools'
      ? ((value: StorageGroupByFilter) => options.setGroupBy(value as StorageGroupKey))
      : undefined;

  const setNormalizedSortKey = (value: StorageSortKey) => {
    options.setSortKey(normalizeStorageSortKey(value));
  };

  return {
    showControls,
    sortDisabled,
    groupBy,
    setGroupBy,
    setNormalizedSortKey,
  };
};
