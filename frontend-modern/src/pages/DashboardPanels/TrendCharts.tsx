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
    <div class="space-y-3">
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <Card>
          <p class="text-sm font-semibold text-gray-900 dark:text-gray-100">CPU Trends</p>
          <div class="mt-2 h-[200px]">
            <InteractiveSparkline
              series={cpuSeries()}
              yMode="percent"
              highlightNearestSeriesOnHover
              timeRange={selectedRange()}
              rangeLabel={rangeLabel()}
            />
          </div>
        </Card>

        <Card>
          <p class="text-sm font-semibold text-gray-900 dark:text-gray-100">Memory Trends</p>
          <div class="mt-2 h-[200px]">
            <InteractiveSparkline
              series={memorySeries()}
              yMode="percent"
              highlightNearestSeriesOnHover
              timeRange={selectedRange()}
              rangeLabel={rangeLabel()}
            />
          </div>
        </Card>
      </div>

      <div class="flex items-center justify-center gap-2 flex-wrap">
        <For each={SUMMARY_TIME_RANGES}>
          {(range) => {
            const active = () => selectedRange() === range;
            const className = () =>
              active()
                ? 'px-3 py-1.5 rounded-md bg-blue-600 text-white text-sm font-medium'
                : 'px-3 py-1.5 rounded-md border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 text-sm font-medium hover:bg-gray-50 dark:hover:bg-gray-700/40 transition-colors';

            return (
              <button type="button" class={className()} onClick={() => props.setTrendRange(range)}>
                {SUMMARY_TIME_RANGE_LABEL[range]}
              </button>
            );
          }}
        </For>
      </div>
    </div>
  );
}

export default TrendCharts;
