import { createMemo } from 'solid-js';
import type { CephSummaryStats } from '@/features/storageBackups/cephSummaryPresentation';
import {
  getCephSummaryClusterCards,
  getCephSummaryHeaderPresentation,
} from '@/features/storageBackups/cephSummaryCardPresentation';

type UseStorageCephSummaryCardModelOptions = {
  summary: () => CephSummaryStats;
};

export const useStorageCephSummaryCardModel = (
  options: UseStorageCephSummaryCardModelOptions,
) => {
  const header = createMemo(() => getCephSummaryHeaderPresentation(options.summary()));
  const clusterCards = createMemo(() => getCephSummaryClusterCards(options.summary()));

  return {
    header,
    clusterCards,
  };
};

