import { For, Show, createMemo } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { ALERTS_OVERVIEW_PATH, AI_PATROL_PATH } from '@/routing/resourceLinks';
import { formatRelativeTime } from '@/utils/format';
import type { Alert } from '@/types/api';

interface RecentAlertsPanelProps {
  alerts: Alert[];
  criticalCount: number;
  warningCount: number;
  totalCount: number;
}

const severityBadgeClass = (level: Alert['level']): string => {
  const base = 'inline-flex shrink-0 items-center rounded px-2 py-0.5 text-[10px] font-semibold uppercase';
  return level === 'critical'
    ? `${base} bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300`
    : `${base} bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300`;
};

function sortByStartTimeDesc(alerts: Alert[]): Alert[] {
  const sorted = [...alerts];
  sorted.sort((a, b) => {
    const aMs = Date.parse(a.startTime) || 0;
    const bMs = Date.parse(b.startTime) || 0;
    return bMs - aMs;
  });
  return sorted;
}

export function RecentAlertsPanel(props: RecentAlertsPanelProps) {
  const recent = createMemo(() => sortByStartTimeDesc(props.alerts).slice(0, 8));

  return (
    <Card>
      <div class="flex items-center justify-between gap-3">
        <h2 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-gray-100">Recent Alerts</h2>
        <div class="flex items-center gap-4">
          <a
            href={ALERTS_OVERVIEW_PATH}
            aria-label="View all alerts"
            class="text-sm font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
          >
            View all →
          </a>
          <a
            href={AI_PATROL_PATH}
            aria-label="View findings"
            class="text-sm font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
          >
            View findings →
          </a>
        </div>
      </div>
      <div class="mt-2 mb-3 border-b border-gray-100 dark:border-gray-700/50" />

      <Show
        when={props.alerts.length > 0}
        fallback={
          <div class="space-y-3">
            <p class="text-sm font-medium text-emerald-700 dark:text-emerald-300">✓ No active alerts</p>
            <div class="flex flex-wrap items-center gap-6">
              <div class="space-y-1">
                <p class="text-xs sm:text-sm font-medium text-gray-600 dark:text-gray-400">Critical</p>
                <p class="text-lg sm:text-xl font-semibold font-mono text-gray-900 dark:text-gray-100">
                  {props.criticalCount}
                </p>
              </div>
              <div class="space-y-1">
                <p class="text-xs sm:text-sm font-medium text-gray-600 dark:text-gray-400">Warning</p>
                <p class="text-lg sm:text-xl font-semibold font-mono text-gray-900 dark:text-gray-100">
                  {props.warningCount}
                </p>
              </div>
              <div class="space-y-1">
                <p class="text-xs sm:text-sm font-medium text-gray-600 dark:text-gray-400">Total active</p>
                <p class="text-lg sm:text-xl font-semibold font-mono text-gray-900 dark:text-gray-100">
                  {props.totalCount}
                </p>
              </div>
            </div>
          </div>
        }
      >
        <div class="space-y-3">
          <p class="text-sm text-gray-700 dark:text-gray-200">
            <span class="font-mono font-semibold text-red-600 dark:text-red-400">{props.criticalCount}</span>{' '}
            critical ·{' '}
            <span class="font-mono font-semibold text-amber-600 dark:text-amber-400">{props.warningCount}</span>{' '}
            warning
          </p>

          <ul class="space-y-1" role="list">
            <For each={recent()}>
              {(alert) => (
                <li class="flex items-start gap-3 rounded px-2 py-2 -mx-2 hover:bg-gray-50 dark:hover:bg-gray-700/40 transition-colors">
                  <span class={severityBadgeClass(alert.level)}>
                    {alert.level === 'critical' ? 'CRITICAL' : 'WARNING'}
                  </span>

                  <div class="min-w-0 flex-1">
                    <div class="flex items-baseline justify-between gap-3">
                      <p class="min-w-0 font-semibold text-sm text-gray-900 dark:text-gray-100 truncate">
                        {alert.resourceName}
                      </p>
                      <span class="shrink-0 text-xs font-mono text-gray-500 dark:text-gray-400">
                        {formatRelativeTime(alert.startTime, { compact: true })}
                      </span>
                    </div>
                    <p class="mt-0.5 text-sm text-gray-600 dark:text-gray-400 truncate">{alert.message}</p>
                  </div>
                </li>
              )}
            </For>
          </ul>
        </div>
      </Show>
    </Card>
  );
}

export default RecentAlertsPanel;
