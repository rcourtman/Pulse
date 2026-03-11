import { Accessor, createMemo, createSignal } from 'solid-js';
import type { SummaryTimeRange } from '@/components/shared/summaryTimeRange';
import type { Resource } from '@/types/resource';
import type { StoragePageNodeOption } from './storagePageState';
import { countVisiblePhysicalDisksForNode } from './storagePageState';

type UseStoragePageSummaryOptions = {
  filteredRecordCount: Accessor<number>;
  selectedNodeId: Accessor<string>;
  nodeOptions: Accessor<StoragePageNodeOption[]>;
  physicalDisks: Accessor<Resource[]>;
};

export const useStoragePageSummary = (options: UseStoragePageSummaryOptions) => {
  const [summaryTimeRange, setSummaryTimeRange] = createSignal<SummaryTimeRange>('1h');

  const poolCount = createMemo(() => options.filteredRecordCount());
  const diskCount = createMemo(() =>
    countVisiblePhysicalDisksForNode(
      options.selectedNodeId(),
      options.nodeOptions(),
      options.physicalDisks(),
    ),
  );

  return {
    summaryTimeRange,
    setSummaryTimeRange,
    poolCount,
    diskCount,
  };
};
