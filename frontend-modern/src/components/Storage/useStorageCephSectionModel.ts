import { createMemo } from 'solid-js';
import { shouldShowCephSummaryCard } from '@/features/storageBackups/storagePagePresentation';
import type { CephSummaryStats } from '@/features/storageBackups/cephSummaryPresentation';
import type { StorageRecord } from '@/features/storageBackups/models';

type UseStorageCephSectionModelOptions = {
  view: () => 'pools' | 'disks';
  summary: () => CephSummaryStats | null;
  filteredRecords: () => StorageRecord[];
  isCephRecord: (record: StorageRecord) => boolean;
};

export const useStorageCephSectionModel = (options: UseStorageCephSectionModelOptions) => {
  const showSummary = createMemo(() => {
    const summary = options.summary();
    if (!summary) {
      return false;
    }

    return shouldShowCephSummaryCard(
      options.view(),
      summary,
      options.filteredRecords(),
      options.isCephRecord,
    );
  });

  return {
    showSummary,
  };
};
