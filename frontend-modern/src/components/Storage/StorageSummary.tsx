import { Component, Show, createEffect, createMemo, createSignal } from 'solid-js';
import { InteractiveSparkline } from '@/components/shared/InteractiveSparkline';
import type { InteractiveSparklineSeries } from '@/components/shared/InteractiveSparkline';
import {
  useSummaryContextualFocusState,
  type SummaryChartHoverSync,
} from '@/components/shared/contextualFocus';
import type { SummarySeriesGroupScope } from '@/components/shared/summaryCardInteraction';
import { SummaryJumpToRowButton } from '@/components/shared/SummaryJumpToRowButton';
import { SummaryPanel } from '@/components/shared/SummaryPanel';
import { SummaryMetricCard } from '@/components/shared/SummaryMetricCard';
import { SummarySynchronizedReadout } from '@/components/shared/SummarySynchronizedReadout';
import { buildInteractiveSparklineSynchronizedReadout } from '@/components/shared/interactiveSparklineModel';
import {
  type MetricPoint,
  type TimeRange,
  type StorageSummaryChartsResponse,
} from '@/api/charts';
import {
  SUMMARY_TIME_RANGE_LABEL,
  type SummaryTimeRange,
} from '@/components/shared/summaryTimeRange';
import { formatBytes } from '@/utils/format';
import { getChartSeriesColor } from '@/utils/chartSeriesPresentation';

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

interface StorageSummaryProps {
  poolCount: number;
  diskCount: number;
  poolsDegraded?: number;
  disksFailing?: number;
  data: StorageSummaryChartsResponse | null;
  loaded: boolean;
  fetchFailed: boolean;
  timeRange: SummaryTimeRange;
  onTimeRangeChange?: (range: SummaryTimeRange) => void;
  hoveredResourceId?: string | null;
  hoveredGroupScope?: SummarySeriesGroupScope | null;
  focusedResourceId?: string | null;
  focusedGroupScope?: SummarySeriesGroupScope | null;
  chartHoverSync?: SummaryChartHoverSync | null;
  onChartHoverSyncChange?: (value: SummaryChartHoverSync | null) => void;
  showJumpToActiveRow?: boolean;
  onJumpToActiveRow?: () => void;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

const StorageSummary: Component<StorageSummaryProps> = (props) => {
  const [localChartHoverSync, setLocalChartHoverSync] = createSignal<SummaryChartHoverSync | null>(
    null,
  );
  const chartHoverSync = () => props.chartHoverSync ?? localChartHoverSync();
  const setChartHoverSync = (value: SummaryChartHoverSync | null) => {
    if (props.chartHoverSync === undefined) {
      setLocalChartHoverSync(value);
    }
    props.onChartHoverSyncChange?.(value);
  };

  // ---------------------------------------------------------------------------
  // Series builders
  // ---------------------------------------------------------------------------

  const allPoolUsageSeries = createMemo((): InteractiveSparklineSeries[] => {
    const d = props.data;
    if (!d?.pools) return [];
    const entries = Object.entries(d.pools);
    return entries
      .filter(([, pool]) => pool.usage && pool.usage.length >= 2)
      .map(([id, pool], i) => ({
        id,
        name: pool.name || id,
        color: getChartSeriesColor(i),
        data: pool.usage as MetricPoint[],
      }));
  });

  const allPoolUsedSeries = createMemo((): InteractiveSparklineSeries[] => {
    const d = props.data;
    if (!d?.pools) return [];
    const entries = Object.entries(d.pools);
    return entries
      .filter(([, pool]) => pool.used && pool.used.length >= 2)
      .map(([id, pool], i) => ({
        id,
        name: pool.name || id,
        color: getChartSeriesColor(i),
        data: pool.used as MetricPoint[],
      }));
  });

  const allPoolAvailSeries = createMemo((): InteractiveSparklineSeries[] => {
    const d = props.data;
    if (!d?.pools) return [];
    const entries = Object.entries(d.pools);
    return entries
      .filter(([, pool]) => pool.avail && pool.avail.length >= 2)
      .map(([id, pool], i) => ({
        id,
        name: pool.name || id,
        color: getChartSeriesColor(i),
        data: pool.avail as MetricPoint[],
      }));
  });

  const allDiskTempSeries = createMemo((): InteractiveSparklineSeries[] => {
    const d = props.data;
    if (!d?.disks) return [];
    const entries = Object.entries(d.disks);
    return entries
      .filter(([, disk]) => disk.temperature && disk.temperature.length >= 2)
      .map(([id, disk], i) => ({
        id,
        name: disk.name || id,
        color: getChartSeriesColor(i),
        data: disk.temperature as MetricPoint[],
      }));
  });
  const interactiveSummarySeries = createMemo<InteractiveSparklineSeries[]>(() => [
      ...allPoolUsageSeries(),
      ...allPoolUsedSeries(),
      ...allPoolAvailSeries(),
      ...allDiskTempSeries(),
  ]);
  const summaryFocus = useSummaryContextualFocusState({
    chartHoveredSeriesId: () => chartHoverSync()?.seriesId ?? null,
    interactiveSeries: interactiveSummarySeries,
    focusedGroupScope: () => props.focusedGroupScope,
    hoveredGroupScope: () => props.hoveredGroupScope,
    hoveredSeriesId: () => props.hoveredResourceId,
    focusedSeriesId: () => props.focusedResourceId,
  });

  createEffect(() => {
    const hovered = chartHoverSync();
    if (!hovered) return;
    if (!summaryFocus.isSeriesIdVisibleInActiveScope(hovered.seriesId)) {
      setChartHoverSync(null);
    }
  });

  const poolUsageSeries = createMemo(() =>
    summaryFocus.filterSeriesForActiveScope(allPoolUsageSeries()),
  );
  const poolUsedSeries = createMemo(() =>
    summaryFocus.filterSeriesForActiveScope(allPoolUsedSeries()),
  );
  const poolAvailSeries = createMemo(() =>
    summaryFocus.filterSeriesForActiveScope(allPoolAvailSeries()),
  );
  const diskTempSeries = createMemo(() =>
    summaryFocus.filterSeriesForActiveScope(allDiskTempSeries()),
  );

  const hasPoolUsage = () => poolUsageSeries().length > 0;
  const hasDiskTemp = () => diskTempSeries().length > 0;
  const hasPoolUsed = () => poolUsedSeries().length > 0;
  const hasPoolAvail = () => poolAvailSeries().length > 0;

  const emptyLabel = () => {
    if (props.fetchFailed) return 'Trend data unavailable';
    if (summaryFocus.activeGroupScope()) return 'No group history yet';
    return 'No history yet';
  };

  const rangeLabel = () => SUMMARY_TIME_RANGE_LABEL[props.timeRange] ?? props.timeRange;

  const formatTemp = (value: number) => `${value.toFixed(0)}°C`;

  const showComponent = () => props.poolCount > 0 || props.diskCount > 0;
  const getFocusedSeriesName = (series: InteractiveSparklineSeries[]): string | null =>
    summaryFocus.getActiveSeriesName(series);
  const focusedLabel = (series: InteractiveSparklineSeries[]) => {
    const name = getFocusedSeriesName(series);
    if (!name) return undefined;
    return <span class="text-xs text-muted ml-1.5 truncate">&mdash; {name}</span>;
  };
  const renderSyncedReadout = (
    readout: { empty?: boolean; timestamp: number; value: string } | null,
  ) =>
    readout ? (
      <SummarySynchronizedReadout
        empty={readout.empty}
        timestamp={readout.timestamp}
        value={readout.value}
      />
    ) : undefined;
  const poolUsageSyncedReadout = () =>
    buildInteractiveSparklineSynchronizedReadout({
      hoverSourceKey: 'pool-usage',
      hoverSync: chartHoverSync(),
      series: poolUsageSeries(),
      timeRange: props.timeRange as TimeRange,
    });
  const diskTempSyncedReadout = () =>
    buildInteractiveSparklineSynchronizedReadout({
      formatValue: formatTemp,
      hoverSourceKey: 'disk-temperature',
      hoverSync: chartHoverSync(),
      series: diskTempSeries(),
      timeRange: props.timeRange as TimeRange,
    });
  const poolUsedSyncedReadout = () =>
    buildInteractiveSparklineSynchronizedReadout({
      formatValue: (value) => formatBytes(value),
      hoverSourceKey: 'used-capacity',
      hoverSync: chartHoverSync(),
      series: poolUsedSeries(),
      timeRange: props.timeRange as TimeRange,
    });
  const poolAvailSyncedReadout = () =>
    buildInteractiveSparklineSynchronizedReadout({
      formatValue: (value) => formatBytes(value),
      hoverSourceKey: 'available-space',
      hoverSync: chartHoverSync(),
      series: poolAvailSeries(),
      timeRange: props.timeRange as TimeRange,
    });

  return (
    <Show when={showComponent()}>
      <div class="space-y-2">
        <SummaryPanel
          testId="storage-summary"
          headerLeft={
            <>
              <span class="font-medium text-base-content">
                {props.poolCount} {props.poolCount === 1 ? 'pool' : 'pools'}
              </span>
              <Show
                when={(props.poolsDegraded ?? 0) > 0 || (props.disksFailing ?? 0) > 0}
                fallback={
                  <>
                    <Show when={props.poolCount > 0 || props.diskCount > 0}>
                      <span class="text-emerald-600 dark:text-emerald-400">all healthy</span>
                    </Show>
                    <Show when={props.diskCount > 0}>
                      <span class="text-muted">
                        {props.diskCount} {props.diskCount === 1 ? 'disk' : 'disks'}
                      </span>
                    </Show>
                  </>
                }
              >
                <Show when={(props.poolsDegraded ?? 0) > 0}>
                  <span class="text-amber-600 dark:text-amber-400">
                    {props.poolsDegraded} degraded
                  </span>
                </Show>
                <Show when={(props.disksFailing ?? 0) > 0}>
                  <span class="text-amber-600 dark:text-amber-400">
                    {props.disksFailing} {props.disksFailing === 1 ? 'disk failing' : 'disks failing'}
                  </span>
                </Show>
              </Show>
              <Show when={props.showJumpToActiveRow && props.onJumpToActiveRow}>
                <SummaryJumpToRowButton onClick={() => props.onJumpToActiveRow?.()} />
              </Show>
            </>
          }
          timeRange={props.timeRange}
          onTimeRangeChange={props.onTimeRangeChange}
        >
          <SummaryMetricCard
            label="Pool Usage"
            secondaryLabel={focusedLabel(poolUsageSeries())}
            headerValue={renderSyncedReadout(poolUsageSyncedReadout())}
            loaded={props.loaded}
            hasData={hasPoolUsage()}
            emptyMessage={emptyLabel()}
            interactionState={summaryFocus.interactionStateFor(poolUsageSeries())}
          >
            <InteractiveSparkline
              series={poolUsageSeries()}
              rangeLabel={rangeLabel()}
              timeRange={props.timeRange as TimeRange}
              activeSeriesDisplay="isolate"
              yMode="percent"
              highlightNearestSeriesOnHover
              hoverSourceKey="pool-usage"
              hoverSync={chartHoverSync()}
              highlightSeriesId={summaryFocus.activeSeriesId()}
              interactionState={summaryFocus.interactionStateFor(poolUsageSeries())}
              onHoverSyncChange={setChartHoverSync}
            />
          </SummaryMetricCard>

          <SummaryMetricCard
            label="Disk Temperature"
            secondaryLabel={focusedLabel(diskTempSeries())}
            headerValue={renderSyncedReadout(diskTempSyncedReadout())}
            loaded={props.loaded}
            hasData={hasDiskTemp()}
            emptyMessage={emptyLabel()}
            interactionState={summaryFocus.interactionStateFor(diskTempSeries())}
          >
            <InteractiveSparkline
              series={diskTempSeries()}
              rangeLabel={rangeLabel()}
              timeRange={props.timeRange as TimeRange}
              activeSeriesDisplay="isolate"
              yMode="auto"
              formatValue={formatTemp}
              formatTopLabel={(max) => `${max.toFixed(0)}°C`}
              highlightNearestSeriesOnHover
              hoverSourceKey="disk-temperature"
              hoverSync={chartHoverSync()}
              highlightSeriesId={summaryFocus.activeSeriesId()}
              interactionState={summaryFocus.interactionStateFor(diskTempSeries())}
              onHoverSyncChange={setChartHoverSync}
            />
          </SummaryMetricCard>

          <SummaryMetricCard
            label="Used Capacity"
            secondaryLabel={focusedLabel(poolUsedSeries())}
            headerValue={renderSyncedReadout(poolUsedSyncedReadout())}
            loaded={props.loaded}
            hasData={hasPoolUsed()}
            emptyMessage={emptyLabel()}
            interactionState={summaryFocus.interactionStateFor(poolUsedSeries())}
          >
            <InteractiveSparkline
              series={poolUsedSeries()}
              rangeLabel={rangeLabel()}
              timeRange={props.timeRange as TimeRange}
              activeSeriesDisplay="isolate"
              yMode="auto"
              formatValue={(v) => formatBytes(v)}
              formatTopLabel={(max) => formatBytes(max)}
              highlightNearestSeriesOnHover
              hoverSourceKey="used-capacity"
              hoverSync={chartHoverSync()}
              highlightSeriesId={summaryFocus.activeSeriesId()}
              interactionState={summaryFocus.interactionStateFor(poolUsedSeries())}
              onHoverSyncChange={setChartHoverSync}
            />
          </SummaryMetricCard>

          <SummaryMetricCard
            label="Available Space"
            secondaryLabel={focusedLabel(poolAvailSeries())}
            headerValue={renderSyncedReadout(poolAvailSyncedReadout())}
            loaded={props.loaded}
            hasData={hasPoolAvail()}
            emptyMessage={emptyLabel()}
            interactionState={summaryFocus.interactionStateFor(poolAvailSeries())}
          >
            <InteractiveSparkline
              series={poolAvailSeries()}
              rangeLabel={rangeLabel()}
              timeRange={props.timeRange as TimeRange}
              activeSeriesDisplay="isolate"
              yMode="auto"
              formatValue={(v) => formatBytes(v)}
              formatTopLabel={(max) => formatBytes(max)}
              highlightNearestSeriesOnHover
              hoverSourceKey="available-space"
              hoverSync={chartHoverSync()}
              highlightSeriesId={summaryFocus.activeSeriesId()}
              interactionState={summaryFocus.interactionStateFor(poolAvailSeries())}
              onHoverSyncChange={setChartHoverSync}
            />
          </SummaryMetricCard>
        </SummaryPanel>
      </div>
    </Show>
  );
};

export default StorageSummary;
