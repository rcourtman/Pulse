import { Match, Switch } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { InteractiveSparkline } from '@/components/shared/InteractiveSparkline';
import { buildStoragePath } from '@/routing/resourceLinks';
import { formatBytes } from '@/utils/format';
import { getMetricColorClass } from '@/utils/metricThresholds';
import { deltaColorClass, formatDelta, formatPercent } from './dashboardHelpers';
import type { DashboardOverview } from '@/hooks/useDashboardOverview';
import type { TrendData } from '@/hooks/useDashboardTrends';

interface StoragePanelProps {
  storage: DashboardOverview['storage'];
  storageTrend: TrendData | null;
  loading: boolean;
}

function computeCapacityPercent(used: number, total: number): number {
  if (!Number.isFinite(used) || !Number.isFinite(total) || total <= 0) return 0;
  return Math.min(100, Math.max(0, (used / total) * 100));
}

export function StoragePanel(props: StoragePanelProps) {
  const capacityPercent = () => computeCapacityPercent(props.storage.totalUsed, props.storage.totalCapacity);
  const hasTrend = () => !!props.storageTrend && props.storageTrend.points.length >= 2;

  return (
    <Card aria-busy={props.loading}>
      <div class="flex items-center justify-between gap-3">
        <h2 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-gray-100">Storage Capacity</h2>
        <a
          href={buildStoragePath()}
          aria-label="View all storage"
          class="text-sm font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
        >
          View all →
        </a>
      </div>
      <div class="mt-2 mb-3 border-b border-gray-100 dark:border-gray-700/50" />

      <div class="space-y-4">
        <div class="space-y-1">
          <p class="text-xs sm:text-sm font-medium text-gray-600 dark:text-gray-400">Total pools</p>
          <p class="text-lg sm:text-xl font-semibold font-mono text-gray-900 dark:text-gray-100">
            {props.storage.total}
          </p>
        </div>

        <Switch>
          <Match when={props.storage.total === 0}>
            <p class="text-sm text-gray-500 dark:text-gray-400">No storage resources</p>
          </Match>
          <Match when={props.storage.total > 0}>
            <div class="space-y-2">
              <div class="flex items-center justify-between gap-3">
                <p class="text-xs sm:text-sm font-medium text-gray-600 dark:text-gray-400">Overall capacity</p>
                <span class="text-xs sm:text-sm font-mono font-semibold text-gray-700 dark:text-gray-200">
                  {formatPercent(capacityPercent())}
                </span>
              </div>

              <p class="text-xs sm:text-sm font-medium text-gray-600 dark:text-gray-400">
                {formatBytes(props.storage.totalUsed)} / {formatBytes(props.storage.totalCapacity)}
              </p>

              <div class="h-2 overflow-hidden rounded bg-gray-100 dark:bg-gray-700/70">
                <div
                  class={`h-full rounded ${getMetricColorClass(capacityPercent(), 'disk')}`}
                  style={{ width: `${capacityPercent()}%` }}
                />
              </div>

              <div class="flex flex-wrap items-center gap-3 text-xs sm:text-sm">
                {props.storage.warningCount > 0 && (
                  <span class="font-medium text-amber-600 dark:text-amber-400">
                    Warnings {props.storage.warningCount}
                  </span>
                )}
                {props.storage.criticalCount > 0 && (
                  <span class="font-medium text-red-600 dark:text-red-400">
                    Critical {props.storage.criticalCount}
                  </span>
                )}
              </div>

              <Switch>
                <Match when={!hasTrend()}>
                  <p
                    class="text-[10px] text-gray-400 dark:text-gray-500 mt-2"
                    aria-label="No storage trend data available"
                  >
                    No trend data
                  </p>
                </Match>
                <Match when={hasTrend()}>
                  <div class="mt-3 space-y-1">
                    <div class="flex items-center justify-between">
                      <p class="text-[10px] font-medium text-gray-500 dark:text-gray-400">24h capacity trend</p>
                      <span
                        class={`text-[10px] font-mono font-medium ${deltaColorClass(props.storageTrend?.delta ?? null)}`}
                        aria-label={`Storage capacity trend ${formatDelta(props.storageTrend?.delta ?? null) ?? 'unavailable'} over 24 hours`}
                      >
                        {formatDelta(props.storageTrend?.delta ?? null) ?? '—'}
                      </span>
                    </div>
                    <div class="h-[120px]">
                      <InteractiveSparkline
                        series={[
                          {
                            data: props.storageTrend?.points ?? [],
                            color: '#6366f1',
                            name: 'Capacity',
                          },
                        ]}
                        yMode="percent"
                        timeRange="24h"
                      />
                    </div>
                  </div>
                </Match>
              </Switch>
            </div>
          </Match>
        </Switch>
      </div>
    </Card>
  );
}

export default StoragePanel;

