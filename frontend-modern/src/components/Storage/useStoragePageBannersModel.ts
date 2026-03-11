import { createMemo } from 'solid-js';
import type { StoragePageBannerKind } from '@/features/storageBackups/storagePagePresentation';

type UseStoragePageBannersModelOptions = {
  kind: () => StoragePageBannerKind | null;
};

export const useStoragePageBannersModel = (options: UseStoragePageBannersModelOptions) => {
  const reconnectActionKind = createMemo<StoragePageBannerKind | null>(() => {
    const kind = options.kind();
    return kind === 'reconnecting' || kind === 'disconnected' ? kind : null;
  });

  return {
    reconnectActionKind,
  };
};
