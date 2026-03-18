import { createMemo } from 'solid-js';
import { getStorageTableHeading } from '@/features/storageBackups/storagePagePresentation';
import type { StorageView } from './storagePageState';

type UseStorageContentCardModelOptions = {
  view: () => StorageView;
  selectedNodeId: () => string;
};

export const useStorageContentCardModel = (options: UseStorageContentCardModelOptions) => {
  const heading = createMemo(() => getStorageTableHeading(options.view()));
  const showDisks = createMemo(() => options.view() === 'disks');
  const showPools = createMemo(() => options.view() === 'pools');
  const selectedDiskNodeId = createMemo(() =>
    options.selectedNodeId() === 'all' ? null : options.selectedNodeId(),
  );

  return {
    heading,
    showDisks,
    showPools,
    selectedDiskNodeId,
  };
};
