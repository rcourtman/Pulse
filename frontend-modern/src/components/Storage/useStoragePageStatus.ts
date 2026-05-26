import { Accessor, createMemo } from 'solid-js';
import { isStoragePoolLoading } from '@/features/storageBackups/storagePageStatus';
import type { StorageView } from './storagePageState';

type UseStoragePageStatusOptions = {
  loading: Accessor<boolean>;
  filteredRecordCount: Accessor<number>;
  view: Accessor<StorageView>;
};

export const useStoragePageStatus = (options: UseStoragePageStatusOptions) => {
  const isLoadingPools = createMemo(() =>
    isStoragePoolLoading(options.loading(), options.view(), options.filteredRecordCount()),
  );

  return {
    isLoadingPools,
  };
};
