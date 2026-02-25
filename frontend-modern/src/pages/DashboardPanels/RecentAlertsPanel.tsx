import { For, Show, createMemo, createSignal } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { ALERTS_OVERVIEW_PATH, AI_PATROL_PATH } from '@/routing/resourceLinks';
import { AlertsAPI } from '@/api/alerts';
import { formatRelativeTime } from '@/utils/format';
import { notificationStore } from '@/stores/notifications';
import type { Alert } from '@/types/api';
import BellIcon from 'lucide-solid/icons/bell';

const MAX_SHOWN = 8;

interface RecentAlertsPanelProps {
  alerts: Alert[];
  criticalCount: number;
  warningCount: number;
  totalCount: number;
}

const severityBadgeClass = (level: Alert['level']): string => {
  const base =
    'inline-flex shrink-0 items-center rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase';
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
  const recent = createMemo(() => sortByStartTimeDesc(props.alerts).slice(0, MAX_SHOWN));

  const [ackLoading, setAckLoading] = createSignal<string | null>(null);
  const [ackAllLoading, setAckAllLoading] = createSignal(false);

  const unackedAlerts = createMemo(() => props.alerts.filter((a) => !a.acknowledged));

  const handleAck = async (alert: Alert) => {
    setAckLoading(alert.id);
    try {
      await AlertsAPI.acknowledge(alert.id);
      notificationStore.success('Alert acknowledged');
    } catch (err) {
      notificationStore.error((err as Error).message || 'Failed to acknowledge');
    } finally {
      setAckLoading(null);
    }
  };

  const handleAckAll = async () => {
    const ids = unackedAlerts().map((a) => a.id);
    if (ids.length === 0) return;
    setAckAllLoading(true);
    try {
      const result = await AlertsAPI.bulkAcknowledge(ids);
      const failCount = result.results.filter((r) => !r.success).length;
      if (failCount === 0) {
        notificationStore.success(`${ids.length} alert${ids.length !== 1 ? 's' : ''} acknowledged`);
      } else {
        notificationStore.error(`${failCount} of ${ids.length} alerts failed to acknowledge`);
      }
    } catch (err) {
      notificationStore.error((err as Error).message || 'Failed to acknowledge alerts');
    } finally {
      setAckAllLoading(false);
    }
  };

  return (
    <Card padding="none" tone="default" class="border-border-subtle overflow-hidden">
      <div class="px-4 py-3 flex items-center justify-between gap-2">
        <div class="flex items-center gap-2">
          <BellIcon class="w-4 h-4 text-muted" aria-hidden="true" />
          <h2 class="text-sm font-semibold text-base-content">Alerts</h2>
        </div>
        <div class="flex items-center gap-2">
          <Show when={unackedAlerts().length > 1}>
            <button
              type="button"
              onClick={handleAckAll}
              disabled={ackAllLoading()}
              class="text-[10px] font-medium text-base-content bg-surface-alt hover:bg-surface-hover disabled:opacity-50 px-2 py-0.5 rounded transition-colors"
            >
              {ackAllLoading() ? 'Acking...' : 'Ack All'}
            </button>
          </Show>
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

      <div class="px-4 pb-3">
        <Show
          when={props.alerts.length > 0}
          fallback={
            <p class="text-xs font-medium text-emerald-700 dark:text-emerald-300 pt-0.5">
              No active alerts
            </p>
          }
        >
          <p class="text-xs text-muted mb-1.5">
            <span class="font-mono font-semibold text-red-600 dark:text-red-400">
              {props.criticalCount}
            </span>{' '}
            critical ·{' '}
            <span class="font-mono font-semibold text-amber-600 dark:text-amber-400">
              {props.warningCount}
            </span>{' '}
            warning
          </p>

          <ul class="space-y-0.5" role="list">
            <For each={recent()}>
              {(alert) => (
                <li class="flex items-center gap-2 -mx-1 px-1 py-1 rounded hover:bg-surface-hover transition-colors">
                  <span class={severityBadgeClass(alert.level)}>
                    {alert.level === 'critical' ? 'CRIT' : 'WARN'}
                  </span>
                  <p class="min-w-0 text-xs text-base-content truncate">{alert.resourceName}</p>
                  <span class="shrink-0 ml-auto text-[10px] font-mono text-slate-400">
                    {formatRelativeTime(alert.startTime, { compact: true })}
                  </span>
                  <Show when={!alert.acknowledged}>
                    <button
                      type="button"
                      onClick={() => handleAck(alert)}
                      disabled={ackLoading() === alert.id}
                      class="shrink-0 px-1.5 py-0.5 text-[10px] font-medium text-base-content bg-surface-alt hover:bg-surface-hover disabled:opacity-50 rounded transition-colors"
                    >
                      {ackLoading() === alert.id ? '...' : 'Ack'}
                    </button>
                  </Show>
                </li>
              )}
            </For>
          </ul>

          <Show when={props.alerts.length > MAX_SHOWN}>
            <p class="mt-1.5 text-[11px] text-muted">
              <a
                href={ALERTS_OVERVIEW_PATH}
                class="text-blue-600 hover:underline dark:text-blue-400"
              >
                +{props.alerts.length - MAX_SHOWN} more
              </a>
            </p>
          </Show>
        </Show>
      </div>
    </Card>
  );
}

export default RecentAlertsPanel;
