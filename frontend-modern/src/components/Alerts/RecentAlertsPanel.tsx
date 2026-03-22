import { For, Show, createMemo } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { ALERTS_OVERVIEW_PATH, AI_PATROL_PATH } from '@/routing/resourceLinks';
import { formatRelativeTime } from '@/utils/format';
import {
  ALERTS_EMPTY_STATE,
  getDashboardAlertSummaryText,
} from '@/utils/alertOverviewPresentation';
import {
  getAlertSeverityBadgeClass,
  getAlertSeverityCompactLabel,
} from '@/utils/alertSeverityPresentation';
import type { Alert } from '@/types/api';
import { getCanonicalAlertId } from '@/features/alerts/identity';
import { useAlertAcknowledgementState } from '@/features/alerts/useAlertAcknowledgementState';
import BellIcon from 'lucide-solid/icons/bell';

const MAX_SHOWN = 8;

interface RecentAlertsPanelProps {
  alerts: Alert[];
}

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
  const {
    effectiveAlerts,
    unacknowledgedAlerts,
    processingAlerts,
    bulkAckProcessing,
    handleAlertAcknowledgement,
    handleBulkAcknowledge,
  } = useAlertAcknowledgementState({
    alerts: () => props.alerts,
  });
  const recent = createMemo(() => sortByStartTimeDesc(effectiveAlerts()).slice(0, MAX_SHOWN));
  const activeCriticalCount = createMemo(
    () =>
      effectiveAlerts().filter((alert) => !alert.acknowledged && alert.level === 'critical').length,
  );
  const activeWarningCount = createMemo(
    () =>
      effectiveAlerts().filter((alert) => !alert.acknowledged && alert.level === 'warning').length,
  );
  const alertSummaryText = createMemo(() =>
    getDashboardAlertSummaryText({
      activeCritical: activeCriticalCount(),
      activeWarning: activeWarningCount(),
    }),
  );

  return (
    <Card padding="none" tone="default" class="border-border-subtle overflow-hidden">
      <div class="px-4 py-3 flex items-center justify-between gap-2">
        <div class="flex items-center gap-2">
          <BellIcon class="w-4 h-4 text-muted" aria-hidden="true" />
          <h2 class="text-sm font-semibold text-base-content">Alerts</h2>
        </div>
        <div class="flex items-center gap-2">
          <Show when={unacknowledgedAlerts().length > 1}>
            <button
              type="button"
              onClick={() => {
                void handleBulkAcknowledge();
              }}
              disabled={bulkAckProcessing()}
              class="text-[10px] font-medium text-base-content bg-surface-alt hover:bg-surface-hover disabled:opacity-50 px-2 py-0.5 rounded transition-colors"
            >
              {bulkAckProcessing() ? 'Acking...' : 'Ack All'}
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
              {ALERTS_EMPTY_STATE}
            </p>
          }
        >
          <p class="text-xs text-muted mb-1.5">{alertSummaryText()}</p>

          <ul class="space-y-0.5" role="list">
            <For each={recent()}>
              {(alert) => (
                <li class="flex items-center gap-2 -mx-1 px-1 py-1 rounded hover:bg-surface-hover transition-colors">
                  <span class={getAlertSeverityBadgeClass(alert.level)}>
                    {getAlertSeverityCompactLabel(alert.level)}
                  </span>
                  <p class="min-w-0 text-xs text-base-content truncate">{alert.resourceName}</p>
                  <p class="min-w-0 text-xs text-muted truncate flex-1">{alert.message}</p>
                  <span class="shrink-0 ml-auto text-[10px] font-mono text-slate-400">
                    {formatRelativeTime(alert.startTime, { compact: true })}
                  </span>
                  <Show when={!alert.acknowledged}>
                    <button
                      type="button"
                      onClick={() => {
                        void handleAlertAcknowledgement(alert);
                      }}
                      disabled={processingAlerts().has(getCanonicalAlertId(alert))}
                      class="shrink-0 px-1.5 py-0.5 text-[10px] font-medium text-base-content bg-surface-alt hover:bg-surface-hover disabled:opacity-50 rounded transition-colors"
                    >
                      {processingAlerts().has(getCanonicalAlertId(alert)) ? '...' : 'Ack'}
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
