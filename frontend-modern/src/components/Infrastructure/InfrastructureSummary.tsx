import type { Component } from 'solid-js';
import { Show } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { InteractiveSparkline } from '@/components/shared/InteractiveSparkline';
import { DensityMap } from '@/components/shared/DensityMap';
import { SummaryPanel } from '@/components/shared/SummaryPanel';
import { SummaryMetricCard } from '@/components/shared/SummaryMetricCard';
import { formatThroughputRate } from '@/utils/throughputPresentation';
import type { InfrastructureSummaryProps } from './infrastructureSummaryModel';
import { useInfrastructureSummaryState } from './useInfrastructureSummaryState';

export const InfrastructureSummary: Component<InfrastructureSummaryProps> = (props) => {
  const state = useInfrastructureSummaryState(props);

  const rangeLabel = () => props.timeRange || '1h';

  const focusedLabel = () => {
    const name = state.focusedResourceName();
    if (!name) return undefined;
    return <span class="text-xs text-muted ml-1.5 truncate">&mdash; {name}</span>;
  };

  return (
    <Show when={props.resources.length > 0}>
      <div class="space-y-2">
        <SummaryPanel
          testId="infrastructure-summary"
          headerLeft={
            <>
              <span class="font-medium text-base-content">
                {state.resourceCounts().total}{' '}
                {state.resourceCounts().total === 1 ? 'resource' : 'resources'}
              </span>
              <Show when={state.resourceCounts().online > 0}>
                <span class="text-emerald-600 dark:text-emerald-400">
                  {state.resourceCounts().online} online
                </span>
              </Show>
              <Show when={state.resourceCounts().offline > 0}>
                <span class="text-muted">{state.resourceCounts().offline} offline</span>
              </Show>
            </>
          }
          timeRange={state.selectedRange()}
          onTimeRangeChange={props.onTimeRangeChange}
        >
          <SummaryMetricCard
            label="CPU"
            secondaryLabel={focusedLabel()}
            loaded={state.isCurrentRangeLoaded()}
            hasData={state.hasData('cpu')}
            emptyMessage={state.emptyMessage()}
            interactionState={state.interactionStateFor(state.seriesFor('cpu'))}
          >
            <InteractiveSparkline
              series={state.seriesFor('cpu')}
              rangeLabel={rangeLabel()}
              timeRange={props.timeRange}
              activeSeriesDisplay="isolate"
              yMode="percent"
              highlightNearestSeriesOnHover
              hoverSourceKey="cpu"
              hoverSync={state.chartHoverSync()}
              highlightSeriesId={state.activeSeriesId()}
              interactionState={state.interactionStateFor(state.seriesFor('cpu'))}
              onHoverSyncChange={state.setChartHoverSync}
            />
          </SummaryMetricCard>

          <SummaryMetricCard
            label="Memory"
            secondaryLabel={focusedLabel()}
            loaded={state.isCurrentRangeLoaded()}
            hasData={state.hasData('memory')}
            emptyMessage={state.emptyMessage()}
            interactionState={state.interactionStateFor(state.seriesFor('memory'))}
          >
            <InteractiveSparkline
              series={state.seriesFor('memory')}
              rangeLabel={rangeLabel()}
              timeRange={props.timeRange}
              activeSeriesDisplay="isolate"
              yMode="percent"
              highlightNearestSeriesOnHover
              hoverSourceKey="memory"
              hoverSync={state.chartHoverSync()}
              highlightSeriesId={state.activeSeriesId()}
              interactionState={state.interactionStateFor(state.seriesFor('memory'))}
              onHoverSyncChange={state.setChartHoverSync}
            />
          </SummaryMetricCard>

          <SummaryMetricCard
            label="Disk I/O"
            secondaryLabel={
              <>
                {focusedLabel()}
                <Show when={!state.focusedResourceName() && state.avgDiskCapacity() !== null}>
                  <span class="text-[10px] text-muted ml-auto shrink-0">
                    Capacity: {state.avgDiskCapacity()}%
                  </span>
                </Show>
              </>
            }
            loaded={state.isCurrentRangeLoaded()}
            hasData={state.hasDiskIOData()}
            emptyMessage={state.emptyMessage()}
            interactionState={state.interactionStateFor(state.diskioSeries())}
          >
            <DensityMap
              series={state.diskioSeries()}
              rangeLabel={rangeLabel()}
              timeRange={props.timeRange}
              formatValue={formatThroughputRate}
              hoverSourceKey="diskio"
              hoverSync={state.chartHoverSync()}
              highlightSeriesId={state.activeSeriesId()}
              interactionState={state.interactionStateFor(state.diskioSeries())}
              onHoverSyncChange={state.setChartHoverSync}
            />
          </SummaryMetricCard>

          <Show
            when={state.shouldShowNetworkCard()}
            fallback={
              <Card padding="sm" class="h-full">
                <div class="flex flex-col h-full">
                  <div class="flex items-center justify-between mb-1.5">
                    <span class="text-xs font-medium text-muted uppercase tracking-wide">
                      Workloads
                    </span>
                    <svg
                      class="w-4 h-4 text-green-500"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                      stroke-width="1.5"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        d="M6 6.878V6a2.25 2.25 0 012.25-2.25h7.5A2.25 2.25 0 0118 6v.878m-12 0c.235-.083.487-.128.75-.128h10.5c.263 0 .515.045.75.128m-12 0A2.25 2.25 0 004.5 9v.878m13.5-3A2.25 2.25 0 0119.5 9v.878m0 0a2.246 2.246 0 00-.75-.128H5.25c-.263 0-.515.045-.75.128m15 0A2.25 2.25 0 0121 12v6a2.25 2.25 0 01-2.25 2.25H5.25A2.25 2.25 0 013 18v-6c0-.98.626-1.813 1.5-2.122"
                      />
                    </svg>
                  </div>
                  <div class="text-xl sm:text-2xl font-bold text-base-content">
                    {state.workloadStats().running}
                    <span class="text-sm font-normal text-muted ml-1">running</span>
                  </div>
                  <Show
                    when={state.workloadStats().total > 0}
                    fallback={<div class="text-[10px] text-muted mt-1">No workloads detected</div>}
                  >
                    <div class="text-[10px] text-muted mt-1">
                      <Show when={state.workloadStats().vms > 0}>
                        <span>{state.workloadStats().vms} VMs</span>
                      </Show>
                      <Show
                        when={state.workloadStats().vms > 0 && state.workloadStats().containers > 0}
                      >
                        <span class="mx-0.5">&middot;</span>
                      </Show>
                      <Show when={state.workloadStats().containers > 0}>
                        <span>{state.workloadStats().containers} containers</span>
                      </Show>
                    </div>
                    <Show when={state.workloadStats().stopped > 0}>
                      <div class="text-[10px] text-muted">
                        {state.workloadStats().stopped} stopped
                      </div>
                    </Show>
                  </Show>
                </div>
              </Card>
            }
          >
            <SummaryMetricCard
              label="Network"
              secondaryLabel={focusedLabel()}
              loaded={state.isCurrentRangeLoaded()}
              hasData={state.hasNetData()}
              emptyMessage={state.emptyHistoryLabel()}
              interactionState={state.interactionStateFor(state.networkSeries())}
            >
              <DensityMap
                series={state.networkSeries()}
                rangeLabel={rangeLabel()}
                timeRange={props.timeRange}
                formatValue={formatThroughputRate}
                hoverSourceKey="network"
                hoverSync={state.chartHoverSync()}
                highlightSeriesId={state.activeSeriesId()}
                interactionState={state.interactionStateFor(state.networkSeries())}
                onHoverSyncChange={state.setChartHoverSync}
              />
            </SummaryMetricCard>
          </Show>
        </SummaryPanel>
      </div>
    </Show>
  );
};

export default InfrastructureSummary;
