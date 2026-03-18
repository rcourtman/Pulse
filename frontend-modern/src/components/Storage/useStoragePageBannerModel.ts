import { createMemo } from 'solid-js';
import {
  getStoragePageBannerActionLabel,
  getStoragePageBannerMessage,
  type StoragePageBannerKind,
} from '@/features/storageBackups/storagePagePresentation';

type UseStoragePageBannerModelOptions = {
  kind: () => StoragePageBannerKind;
};

export const useStoragePageBannerModel = (options: UseStoragePageBannerModelOptions) => {
  const message = createMemo(() => getStoragePageBannerMessage(options.kind()));
  const actionLabel = createMemo(() => getStoragePageBannerActionLabel(options.kind()));

  return {
    message,
    actionLabel,
  };
};

