import { Accessor, createMemo } from 'solid-js';
import {
  getStoragePageBannerKind,
  isStoragePoolLoading,
} from '@/features/storageBackups/storagePageStatus';
import type { StorageView } from './storagePageState';

type UseStoragePageStatusOptions = {
  loading: Accessor<boolean>;
  error: Accessor<unknown>;
  filteredRecordCount: Accessor<number>;
  connected: Accessor<boolean>;
  initialDataReceived: Accessor<boolean>;
  reconnecting: Accessor<boolean>;
  view: Accessor<StorageView>;
};

export const useStoragePageStatus = (options: UseStoragePageStatusOptions) => {
  const hasFetchError = createMemo(() => Boolean(options.error()));

  const activeBannerKind = createMemo(() =>
    getStoragePageBannerKind({
      loading: options.loading(),
      filteredRecordCount: options.filteredRecordCount(),
      connected: options.connected(),
      initialDataReceived: options.initialDataReceived(),
      reconnecting: options.reconnecting(),
      hasFetchError: hasFetchError(),
    }),
  );

  const isLoadingPools = createMemo(() =>
    isStoragePoolLoading(options.loading(), options.view(), options.filteredRecordCount()),
  );

  return {
    hasFetchError,
    activeBannerKind,
    isLoadingPools,
  };
};

