import { Match, Switch } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { buildStoragePath } from '@/routing/resourceLinks';
import { formatBytes } from '@/utils/format';
import { deltaColorClass, formatDelta, formatPercent } from './dashboardHelpers';
import type { DashboardOverview } from '@/hooks/useDashboardOverview';
import type { TrendData } from '@/hooks/useDashboardTrends';
import {
  computeDashboardStorageCapacityPercent,
  DASHBOARD_STORAGE_EMPTY_STATE,
  getDashboardStorageCapacityBarClass,
  getDashboardStorageIssueBadges,
} from '@/utils/dashboardStoragePresentation';

interface StoragePanelProps {
  storage: DashboardOverview['storage'];
  storageTrend: TrendData | null;
  loading: boolean;
}

export function StoragePanel(props: StoragePanelProps) {
  const capacityPercent = () =>
    computeDashboardStorageCapacityPercent(props.storage.totalUsed, props.storage.totalCapacity);
  const hasTrend = () => !!props.storageTrend && props.storageTrend.points.length >= 2;

  return (
    <Card padding="none" class="px-4 py-3.5" aria-busy={props.loading}>
      <div class="flex items-center justify-between gap-3">
        <h2 class="text-sm font-semibold text-base-content">
          Storage <span class="font-normal text-xs text-muted">({props.storage.total} pools)</span>
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
          <p class="text-xs text-muted mt-1">{DASHBOARD_STORAGE_EMPTY_STATE}</p>
        </Match>
        <Match when={props.storage.total > 0}>
          <div class="mt-1.5 space-y-1.5">
            <div class="flex items-center justify-between gap-3">
              <p class="text-xs text-muted">
                {formatBytes(props.storage.totalUsed)} / {formatBytes(props.storage.totalCapacity)}
              </p>
              <span class="text-xs font-mono font-semibold text-base-content">
                {formatPercent(capacityPercent())}
              </span>
            </div>

            <div class="h-2 overflow-hidden rounded bg-surface-alt">
              <div
                class={`h-full rounded ${getDashboardStorageCapacityBarClass(capacityPercent())}`}
                style={{ width: `${capacityPercent()}%` }}
              />
            </div>

            <div class="flex flex-wrap items-center gap-3 text-xs">
              {getDashboardStorageIssueBadges(props.storage).map((badge) => (
                <span class={badge.className}>{badge.label}</span>
              ))}
              {hasTrend() && (
                <span
                  class={`font-mono font-medium ${deltaColorClass(props.storageTrend?.delta ?? null)}`}
                >
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
