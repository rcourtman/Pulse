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
  const base = 'inline-flex shrink-0 items-center rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase';
  return level === 'critical'
    ? `${base} bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300`
    : `${base} bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300`;
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
  const recent = createMemo(() => sortByStartTimeDesc(props.alerts).slice(0, 4));

  return (
    <Card padding="none" tone="glass" class="px-4 py-3.5 border-slate-100 dark:border-slate-700">
      <div class="flex items-center justify-between gap-2">
        <h2 class="text-sm font-semibold text-slate-900 dark:text-slate-100">Alerts</h2>
        <div class="flex items-center gap-2">
          <a
            href={ALERTS_OVERVIEW_PATH}
            aria-label="View all alerts"
            class="text-xs font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
          >
            All →
          </a>
          <a
            href={AI_PATROL_PATH}
            aria-label="View findings"
            class="text-xs font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
          >
            Findings →
          </a>
        </div>
      </div>

      <Show
        when={props.alerts.length > 0}
        fallback={
          <p class="text-xs font-medium text-emerald-700 dark:text-emerald-300 mt-1">No active alerts</p>
        }
      >
        <div class="mt-1">
          <p class="text-xs text-slate-500 dark:text-slate-400 mb-1">
            <span class="font-mono font-semibold text-red-600 dark:text-red-400">{props.criticalCount}</span> critical · <span class="font-mono font-semibold text-amber-600 dark:text-amber-400">{props.warningCount}</span> warning
          </p>

          <ul class="space-y-0.5" role="list">
            <For each={recent()}>
              {(alert) => (
                <li class="flex items-center gap-2 -mx-1 px-1 py-0.5 rounded hover:bg-slate-50 dark:hover:bg-slate-700 transition-colors">
                  <span class={severityBadgeClass(alert.level)}>
                    {alert.level === 'critical' ? 'CRIT' : 'WARN'}
                  </span>
                  <p class="min-w-0 text-xs text-slate-700 dark:text-slate-200 truncate">
                    {alert.resourceName}
                  </p>
                  <span class="shrink-0 ml-auto text-[10px] font-mono text-slate-400">
                    {formatRelativeTime(alert.startTime, { compact: true })}
                  </span>
                </li>
              )}
            </For>
          </ul>

          <Show when={props.alerts.length > 4}>
            <p class="mt-1 text-[11px] text-slate-500 dark:text-slate-400">
              <a href={ALERTS_OVERVIEW_PATH} class="text-blue-600 hover:underline dark:text-blue-400">
                +{props.alerts.length - 4} more
              </a>
            </p>
          </Show>
        </div>
      </Show>
    </Card>
  );
}

export default RecentAlertsPanel;
