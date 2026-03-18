import { Accessor, createMemo } from 'solid-js';
import type { ZFSPool } from '@/types/api';
import { getMetricColorClass } from '@/utils/metricThresholds';
import {
  getStorageBarLabel,
  getStorageBarTooltipRows,
  getStorageBarUsagePercent,
  getStorageBarZfsSummary,
} from '@/features/storageBackups/storageBarPresentation';

type UseEnhancedStorageBarModelOptions = {
  used: Accessor<number>;
  total: Accessor<number>;
  free: Accessor<number>;
  zfsPool: Accessor<ZFSPool | undefined>;
};

export const useEnhancedStorageBarModel = (options: UseEnhancedStorageBarModelOptions) => {
  const usagePercent = createMemo(() =>
    getStorageBarUsagePercent(options.used(), options.total()),
  );
  const barColor = createMemo(() => getMetricColorClass(usagePercent(), 'disk'));
  const label = createMemo(() => getStorageBarLabel(options.used(), options.total()));
  const tooltipRows = createMemo(() =>
    getStorageBarTooltipRows(options.used(), options.free(), options.total()),
  );
  const zfsSummary = createMemo(() => getStorageBarZfsSummary(options.zfsPool()));

  return {
    usagePercent,
    barColor,
    label,
    tooltipRows,
    zfsSummary,
  };
};
