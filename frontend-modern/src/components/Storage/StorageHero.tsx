import { Component, createMemo, Show } from 'solid-js';

import { Card } from '@/components/shared/Card';
import { MiniDonut, MiniGauge } from '@/pages/DashboardPanels/Visualizations';
import { deltaColorClass, formatDelta } from '@/pages/DashboardPanels/dashboardHelpers';
import { formatBytes, formatPercent } from '@/utils/format';
import { getMetricSeverity } from '@/utils/metricThresholds';
import type { MetricSeverity } from '@/utils/metricThresholds';

export interface StorageHeroProps {
  summary: { count: number; totalBytes: number; usedBytes: number; usagePercent: number };
  healthBreakdown: { healthy: number; warning: number; critical: number; offline: number; unknown: number };
  diskCount?: number;
  trend?: { delta: number | null } | null;
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
    { value: props.healthBreakdown.offline + props.healthBreakdown.unknown, color: 'text-slate-300 dark:text-slate-600' },
  ]);

  return (
    <Card padding="sm">
      <div class="grid grid-cols-2 sm:grid-cols-4 gap-3">
        {/* Pools + Health donut */}
        <div class="flex items-center gap-3 rounded-md border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800 px-3 py-2.5">
          <MiniDonut size={32} strokeWidth={4} data={donutData()} centerText={String(props.summary.count)} />
          <div class="min-w-0">
            <div class="text-[10px] font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">Pools</div>
            <div class="text-sm font-bold text-slate-900 dark:text-white">{props.summary.count}</div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">
              {props.healthBreakdown.healthy} healthy
              {props.healthBreakdown.warning > 0 && <span class="text-amber-600 dark:text-amber-400"> · {props.healthBreakdown.warning} warn</span>}
              {props.healthBreakdown.critical > 0 && <span class="text-red-600 dark:text-red-400"> · {props.healthBreakdown.critical} crit</span>}
            </div>
          </div>
        </div>

        {/* Capacity gauge */}
        <div class="flex items-center gap-3 rounded-md border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800 px-3 py-2.5">
          <MiniGauge percent={props.summary.usagePercent} size={32} strokeWidth={4} color={gaugeColor()} />
          <div class="min-w-0">
            <div class="text-[10px] font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">Capacity</div>
            <div class="text-sm font-bold text-slate-900 dark:text-white">{formatBytes(props.summary.totalBytes)}</div>
            <div class={`text-[10px] font-medium ${gaugeColor()}`}>
              {formatPercent(props.summary.usagePercent)} used
              <Show when={props.trend?.delta != null}>
                <span class={`text-[10px] font-medium ${deltaColorClass(props.trend?.delta ?? null)}`}>
                  {' '}{formatDelta(props.trend?.delta ?? null)} / 7d
                </span>
              </Show>
            </div>
          </div>
        </div>

        {/* Disks (replaces redundant Used card) */}
        <div class="flex items-center gap-3 rounded-md border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800 px-3 py-2.5">
          <div class="flex items-center justify-center w-8 h-8 rounded-full bg-slate-100 dark:bg-slate-800 flex-shrink-0">
            <svg class="w-4 h-4 text-slate-500 dark:text-slate-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <rect x="2" y="2" width="20" height="8" rx="2" ry="2" />
              <rect x="2" y="14" width="20" height="8" rx="2" ry="2" />
              <circle cx="6" cy="6" r="1" fill="currentColor" />
              <circle cx="6" cy="18" r="1" fill="currentColor" />
            </svg>
          </div>
          <div class="min-w-0">
            <div class="text-[10px] font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">Disks</div>
            <Show
              when={props.diskCount != null && props.diskCount > 0}
              fallback={<div class="text-sm font-bold text-slate-400 dark:text-slate-500">-</div>}
            >
              <div class="text-sm font-bold text-slate-900 dark:text-white">{props.diskCount}</div>
              <div class="text-[10px] text-slate-500 dark:text-slate-400">physical</div>
            </Show>
          </div>
        </div>

        {/* Used / Free summary (replaces separate Used + Free cards) */}
        <div class="flex items-center gap-3 rounded-md border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800 px-3 py-2.5">
          <div class="min-w-0">
            <div class="text-[10px] font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">Allocation</div>
            <div class="text-sm font-bold text-slate-900 dark:text-white">{formatBytes(props.summary.usedBytes)}</div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">
              {formatBytes(freeBytes())} free
            </div>
          </div>
        </div>
      </div>
    </Card>
  );
};
