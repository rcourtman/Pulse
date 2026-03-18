import { Accessor, createMemo, createSignal } from 'solid-js';
import type { HistoryTimeRange } from '@/api/charts';
import type { StorageRecord } from '@/features/storageBackups/models';
import {
  buildStoragePoolDetailConfigRows,
  buildStoragePoolDetailZfsSummary,
  getStoragePoolLinkedDisks,
  resolveStoragePoolDetailChartTarget,
} from '@/features/storageBackups/storagePoolDetailPresentation';
import type { Resource } from '@/types/resource';

type UseStoragePoolDetailModelOptions = {
  record: Accessor<StorageRecord>;
  physicalDisks: Accessor<Resource[]>;
};

export const useStoragePoolDetailModel = (options: UseStoragePoolDetailModelOptions) => {
  const [chartRange, setChartRange] = createSignal<HistoryTimeRange>('7d');

  const chartTarget = createMemo(() => resolveStoragePoolDetailChartTarget(options.record()));
  const configRows = createMemo(() => buildStoragePoolDetailConfigRows(options.record()));
  const zfsSummary = createMemo(() => buildStoragePoolDetailZfsSummary(options.record()));
  const linkedDisks = createMemo(() =>
    getStoragePoolLinkedDisks(options.record(), options.physicalDisks()),
  );

  return {
    chartRange,
    setChartRange,
    chartTarget,
    configRows,
    zfsSummary,
    linkedDisks,
  };
};
