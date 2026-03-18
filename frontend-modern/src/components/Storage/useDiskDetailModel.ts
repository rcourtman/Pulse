import { Accessor, createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import type { HistoryTimeRange, AggregatedMetricPoint } from '@/api/charts';
import {
  extractPhysicalDiskPresentationData,
  type PhysicalDiskPresentationData,
} from '@/features/storageBackups/diskPresentation';
import {
  getDiskDetailAttributeCards,
  getDiskDetailHistoryCharts,
} from '@/features/storageBackups/diskDetailPresentation';
import { getDiskMetricHistory, getDiskMetricsVersion } from '@/stores/diskMetricsHistory';
import type { Resource } from '@/types/resource';
import { resolvePhysicalDiskMetricResourceId } from './diskResourceUtils';

type UseDiskDetailModelOptions = {
  disk: Accessor<Resource>;
  nodes: Accessor<Resource[]>;
};

const toAggregatedSeries = (
  historyData: Array<{ timestamp: number; readBps: number; writeBps: number; ioTime: number }>,
  key: 'readBps' | 'writeBps' | 'ioTime',
): AggregatedMetricPoint[] =>
  historyData.map((point) => ({
    timestamp: point.timestamp,
    value: point[key],
    min: point[key],
    max: point[key],
  }));

export const useDiskDetailModel = (options: UseDiskDetailModelOptions) => {
  const [chartRange, setChartRange] = createSignal<HistoryTimeRange>('24h');
  const [diskVer, setDiskVer] = createSignal(getDiskMetricsVersion());

  const diskData = createMemo<PhysicalDiskPresentationData>(() =>
    extractPhysicalDiskPresentationData(options.disk()),
  );
  const resId = createMemo(() => diskData().serial || diskData().wwn || null);
  const attributeCards = createMemo(() => getDiskDetailAttributeCards(diskData()));
  const historyCharts = createMemo(() => getDiskDetailHistoryCharts(diskData()));
  const metricResourceId = createMemo(() =>
    resolvePhysicalDiskMetricResourceId(options.disk(), options.nodes(), diskData().devPath),
  );

  createEffect(() => {
    const timer = setInterval(() => setDiskVer(getDiskMetricsVersion()), 2000);
    onCleanup(() => clearInterval(timer));
  });

  const historyData = createMemo(() => {
    diskVer();
    const id = metricResourceId();
    if (!id) return [];
    return getDiskMetricHistory(id, 30 * 60 * 1000);
  });

  const readData = createMemo<AggregatedMetricPoint[]>(() =>
    toAggregatedSeries(historyData(), 'readBps'),
  );
  const writeData = createMemo<AggregatedMetricPoint[]>(() =>
    toAggregatedSeries(historyData(), 'writeBps'),
  );
  const ioData = createMemo<AggregatedMetricPoint[]>(() =>
    toAggregatedSeries(historyData(), 'ioTime'),
  );

  return {
    chartRange,
    setChartRange,
    diskData,
    resId,
    attributeCards,
    historyCharts,
    metricResourceId,
    readData,
    writeData,
    ioData,
  };
};
