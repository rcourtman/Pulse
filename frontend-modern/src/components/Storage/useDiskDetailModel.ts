import { Accessor, createMemo, createSignal } from 'solid-js';
import type { HistoryTimeRange } from '@/api/charts';
import {
  extractPhysicalDiskPresentationData,
  type PhysicalDiskPresentationData,
} from '@/features/storageBackups/diskPresentation';
import {
  getDiskDetailAttributeCards,
  getDiskDetailHistoryCharts,
} from '@/features/storageBackups/diskDetailPresentation';
import { useAlertsActivation } from '@/stores/alertsActivation';
import type { Resource } from '@/types/resource';
import { resolvePhysicalDiskHistoryResourceId } from './diskResourceUtils';

type UseDiskDetailModelOptions = {
  disk: Accessor<Resource>;
};

export const useDiskDetailModel = (options: UseDiskDetailModelOptions) => {
  const [chartRange, setChartRange] = createSignal<HistoryTimeRange>('24h');
  const { getDiskTemperatureThresholds } = useAlertsActivation();

  const diskData = createMemo<PhysicalDiskPresentationData>(() =>
    extractPhysicalDiskPresentationData(options.disk()),
  );
  const historyResourceId = createMemo(() => resolvePhysicalDiskHistoryResourceId(options.disk()));
  const attributeCards = createMemo(() =>
    getDiskDetailAttributeCards(diskData(), getDiskTemperatureThresholds(diskData().type)),
  );
  const historyCharts = createMemo(() => getDiskDetailHistoryCharts(diskData()));
  const metricResourceId = createMemo(() => historyResourceId());

  return {
    chartRange,
    setChartRange,
    diskData,
    historyResourceId,
    attributeCards,
    historyCharts,
    metricResourceId,
  };
};
