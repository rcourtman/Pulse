import { Component, createMemo } from 'solid-js';

import { Card } from '@/components/shared/Card';
import { MiniDonut, MiniGauge } from '@/pages/DashboardPanels/Visualizations';
import { formatBytes, formatPercent } from '@/utils/format';
import { getMetricSeverity } from '@/utils/metricThresholds';
import type { MetricSeverity } from '@/utils/metricThresholds';

export interface StorageHeroProps {
  summary: { count: number; totalBytes: number; usedBytes: number; usagePercent: number };
  healthBreakdown: { healthy: number; warning: number; critical: number; offline: number; unknown: number };
  diskCount?: number;
}

const GAUGE_COLOR_BY_SEVERITY: Record<MetricSeverity, string> = {
  normal: 'text-emerald-500 dark:text-emerald-400',
  warning: 'text-amber-500 dark:text-amber-400',
  critical: 'text-red-500 dark:text-red-400',
};

export const StorageHero: Component<StorageHeroProps> = (props) => {
  const freeBytes = createMemo(() => Math.max(0, props.summary.totalBytes - props.summary.usedBytes));
  const gaugeColor = createMemo(() => GAUGE_COLOR_BY_SEVERITY[getMetricSeverity(props.summary.usagePercent, 'disk')]);

  const donutData = createMemo(() => [
    { value: props.healthBreakdown.healthy, color: 'text-emerald-500 dark:text-emerald-400' },
    { value: props.healthBreakdown.warning, color: 'text-amber-500 dark:text-amber-400' },
    { value: props.healthBreakdown.critical, color: 'text-red-500 dark:text-red-400' },
    { value: props.healthBreakdown.offline + props.healthBreakdown.unknown, color: 'text-gray-300 dark:text-gray-600' },
  ]);

  return (
    <Card padding="sm">
      <div class="grid grid-cols-2 sm:grid-cols-4 gap-3">
        {/* Pools */}
        <div class="flex items-center gap-3 rounded-lg border border-gray-200/70 dark:border-gray-700/60 bg-gray-50/50 dark:bg-gray-800/30 px-3 py-2.5">
          <MiniDonut size={32} strokeWidth={4} data={donutData()} centerText={String(props.summary.count)} />
          <div class="min-w-0">
            <div class="text-[10px] font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">Pools</div>
            <div class="text-sm font-bold text-gray-900 dark:text-white">{props.summary.count}</div>
            <div class="text-[10px] text-gray-500 dark:text-gray-400">
              {props.healthBreakdown.healthy} healthy
              {props.healthBreakdown.warning > 0 && <span class="text-amber-600 dark:text-amber-400"> · {props.healthBreakdown.warning} warn</span>}
              {props.healthBreakdown.critical > 0 && <span class="text-red-600 dark:text-red-400"> · {props.healthBreakdown.critical} crit</span>}
            </div>
          </div>
        </div>

        {/* Capacity */}
        <div class="flex items-center gap-3 rounded-lg border border-gray-200/70 dark:border-gray-700/60 bg-gray-50/50 dark:bg-gray-800/30 px-3 py-2.5">
          <MiniGauge percent={props.summary.usagePercent} size={32} strokeWidth={4} color={gaugeColor()} />
          <div class="min-w-0">
            <div class="text-[10px] font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">Capacity</div>
            <div class="text-sm font-bold text-gray-900 dark:text-white">{formatBytes(props.summary.totalBytes)}</div>
            <div class={`text-[10px] font-medium ${gaugeColor()}`}>
              {formatPercent(props.summary.usagePercent)} used
            </div>
          </div>
        </div>

        {/* Used */}
        <div class="flex items-center gap-3 rounded-lg border border-gray-200/70 dark:border-gray-700/60 bg-gray-50/50 dark:bg-gray-800/30 px-3 py-2.5">
          <div class="min-w-0">
            <div class="text-[10px] font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">Used</div>
            <div class="text-sm font-bold text-gray-900 dark:text-white">{formatBytes(props.summary.usedBytes)}</div>
            <div class="text-[10px] text-gray-500 dark:text-gray-400">
              {formatPercent(props.summary.usagePercent)} of total
            </div>
          </div>
        </div>

        {/* Free */}
        <div class="flex items-center gap-3 rounded-lg border border-gray-200/70 dark:border-gray-700/60 bg-gray-50/50 dark:bg-gray-800/30 px-3 py-2.5">
          <div class="min-w-0">
            <div class="text-[10px] font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">Free</div>
            <div class="text-sm font-bold text-gray-900 dark:text-white">{formatBytes(freeBytes())}</div>
            <div class="text-[10px] text-gray-500 dark:text-gray-400">remaining</div>
          </div>
        </div>
      </div>
    </Card>
  );
};
