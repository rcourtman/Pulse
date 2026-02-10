import { Match, Switch } from 'solid-js';
import { Card } from '@/components/shared/Card';
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
    <Card padding="none" class="px-4 py-3.5" aria-busy={props.loading}>
      <div class="flex items-center justify-between gap-3">
        <h2 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
          Storage <span class="font-normal text-xs text-gray-500 dark:text-gray-400">({props.storage.total} pools)</span>
        </h2>
        <a
          href={buildStoragePath()}
          aria-label="View all storage"
          class="text-xs font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
        >
          View all →
        </a>
      </div>

      <Switch>
        <Match when={props.storage.total === 0}>
          <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">No storage resources</p>
        </Match>
        <Match when={props.storage.total > 0}>
          <div class="mt-1.5 space-y-1.5">
            <div class="flex items-center justify-between gap-3">
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {formatBytes(props.storage.totalUsed)} / {formatBytes(props.storage.totalCapacity)}
              </p>
              <span class="text-xs font-mono font-semibold text-gray-700 dark:text-gray-200">
                {formatPercent(capacityPercent())}
              </span>
            </div>

            <div class="h-2 overflow-hidden rounded bg-gray-100 dark:bg-gray-700/70">
              <div
                class={`h-full rounded ${getMetricColorClass(capacityPercent(), 'disk')}`}
                style={{ width: `${capacityPercent()}%` }}
              />
            </div>

            <div class="flex flex-wrap items-center gap-3 text-xs">
              {props.storage.warningCount > 0 && (
                <span class="font-medium text-amber-600 dark:text-amber-400">
                  {props.storage.warningCount} warnings
                </span>
              )}
              {props.storage.criticalCount > 0 && (
                <span class="font-medium text-red-600 dark:text-red-400">
                  {props.storage.criticalCount} critical
                </span>
              )}
              {hasTrend() && (
                <span class={`font-mono font-medium ${deltaColorClass(props.storageTrend?.delta ?? null)}`}>
                  24h: {formatDelta(props.storageTrend?.delta ?? null) ?? '—'}
                </span>
              )}
            </div>
          </div>
        </Match>
      </Switch>
    </Card>
  );
}

export default StoragePanel;
