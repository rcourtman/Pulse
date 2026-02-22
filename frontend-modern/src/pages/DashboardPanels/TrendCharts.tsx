import { For, createMemo } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { InteractiveSparkline, type InteractiveSparklineSeries } from '@/components/shared/InteractiveSparkline';
import {
  SUMMARY_TIME_RANGES,
  SUMMARY_TIME_RANGE_LABEL,
  isSummaryTimeRange,
  type SummaryTimeRange,
} from '@/components/shared/summaryTimeRange';
import type { HistoryTimeRange, MetricPoint } from '@/api/charts';
import { HOST_COLORS } from './hostColors';

interface TrendChartsProps {
  trends: import('@/hooks/useDashboardTrends').DashboardTrends;
  overview: import('@/hooks/useDashboardOverview').DashboardOverview;
  trendRange: import('solid-js').Accessor<HistoryTimeRange>;
  setTrendRange: (range: HistoryTimeRange) => void;
}

function normalizeRange(range: HistoryTimeRange): SummaryTimeRange {
  if (isSummaryTimeRange(range)) return range;
  switch (range) {
    case '6h':
      return '12h';
    case '30d':
    case '90d':
      return '7d';
    default:
      return '24h';
  }
}

export function TrendCharts(props: TrendChartsProps) {
  const selectedRange = createMemo(() => normalizeRange(props.trendRange()));

  const cpuSeries = createMemo<InteractiveSparklineSeries[]>(() => {
    const hosts = props.overview.infrastructure.topCPU.slice(0, 5);
    const series: InteractiveSparklineSeries[] = [];

    for (let i = 0; i < hosts.length; i++) {
      const host = hosts[i];
      const trend = props.trends.infrastructure.cpu.get(host.id);
      if (!trend) continue;

      const data = trend.points.map(
        (point): MetricPoint => ({ timestamp: point.timestamp, value: point.value }),
      );

      series.push({
        id: host.id,
        data,
        color: HOST_COLORS[i % HOST_COLORS.length],
        name: host.name,
      });
    }

    return series;
  });

  const memorySeries = createMemo<InteractiveSparklineSeries[]>(() => {
    const hosts = props.overview.infrastructure.topMemory.slice(0, 5);
    const series: InteractiveSparklineSeries[] = [];

    for (let i = 0; i < hosts.length; i++) {
      const host = hosts[i];
      const trend = props.trends.infrastructure.memory.get(host.id);
      if (!trend) continue;

      const data = trend.points.map(
        (point): MetricPoint => ({ timestamp: point.timestamp, value: point.value }),
      );

      series.push({
        id: host.id,
        data,
        color: HOST_COLORS[i % HOST_COLORS.length],
        name: host.name,
      });
    }

    return series;
  });

  const rangeLabel = createMemo(() => SUMMARY_TIME_RANGE_LABEL[selectedRange()]);

  return (
    <div>
      <div class="flex items-center justify-between mb-1.5">
        <p class="text-xs font-semibold text-muted uppercase tracking-wide">Trends</p>
        <div class="flex items-center gap-1.5">
          <For each={SUMMARY_TIME_RANGES}>
            {(range) => {
              const active = () => selectedRange() === range;
              const className = () =>
                active()
                  ? 'px-2 py-0.5 rounded bg-blue-600 text-white text-[11px] font-medium'
                  : 'px-2 py-0.5 rounded border border-slate-300 dark:border-slate-600 text-slate-700 dark:text-slate-200 text-[11px] font-medium hover:bg-surface-hover transition-colors';

              return (
                <button type="button" class={className()} onClick={() => props.setTrendRange(range)}>
                  {SUMMARY_TIME_RANGE_LABEL[range]}
                </button>
              );
            }}
          </For>
        </div>
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-2 gap-3">
        <Card padding="none" tone="default" class="px-4 py-3 border-slate-100 dark:border-slate-700">
          <p class="text-[11px] font-medium text-muted uppercase tracking-wide">CPU</p>
          <div class="h-[240px] mt-1">
            <InteractiveSparkline
              series={cpuSeries()}
              yMode="percent"
              highlightNearestSeriesOnHover
              timeRange={selectedRange()}
              rangeLabel={rangeLabel()}
              size="lg"
            />
          </div>
        </Card>

        <Card padding="none" tone="default" class="px-4 py-3 border-slate-100 dark:border-slate-700">
          <p class="text-[11px] font-medium text-muted uppercase tracking-wide">Memory</p>
          <div class="h-[240px] mt-1">
            <InteractiveSparkline
              series={memorySeries()}
              yMode="percent"
              highlightNearestSeriesOnHover
              timeRange={selectedRange()}
              rangeLabel={rangeLabel()}
              size="lg"
            />
          </div>
        </Card>
      </div>
    </div>
  );
}

export default TrendCharts;
