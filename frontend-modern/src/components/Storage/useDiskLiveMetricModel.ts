import { createEffect, createMemo, createSignal, onCleanup, type Accessor } from 'solid-js';
import { getLatestDiskMetric, getDiskMetricsVersion } from '@/stores/diskMetricsHistory';
import {
  getDiskLiveMetricFormattedValue,
  getDiskLiveMetricTextClass,
  type DiskLiveMetricType,
} from '@/features/storageBackups/diskLiveMetricPresentation';

type UseDiskLiveMetricModelOptions = {
  resourceId: Accessor<string>;
  type: Accessor<DiskLiveMetricType>;
};

export const useDiskLiveMetricModel = (options: UseDiskLiveMetricModelOptions) => {
  const [version, setVersion] = createSignal(getDiskMetricsVersion());

  createEffect(() => {
    const timer = setInterval(() => setVersion(getDiskMetricsVersion()), 2000);
    onCleanup(() => clearInterval(timer));
  });

  const latestMetric = createMemo(() => {
    version();
    return getLatestDiskMetric(options.resourceId());
  });

  const value = createMemo(() => {
    const metric = latestMetric();
    if (!metric) return 0;

    if (options.type() === 'read') return metric.readBps;
    if (options.type() === 'write') return metric.writeBps;
    return metric.ioTime;
  });

  const formatted = createMemo(() =>
    getDiskLiveMetricFormattedValue(value(), options.type()),
  );
  const colorClass = createMemo(() => getDiskLiveMetricTextClass(value(), options.type()));

  return {
    value,
    formatted,
    colorClass,
  };
};
