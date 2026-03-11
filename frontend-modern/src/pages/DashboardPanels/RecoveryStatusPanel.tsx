import { For, Show, createMemo } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { buildRecoveryPath } from '@/routing/resourceLinks';
import { formatRelativeTime } from '@/utils/format';
import {
  RECOVERY_OUTCOMES,
  getRecoveryOutcomeBadgeClass,
} from '@/utils/recoveryOutcomePresentation';
import {
  DASHBOARD_RECOVERY_EMPTY_STATE,
  DASHBOARD_RECOVERY_STALE_MESSAGE,
} from '@/utils/dashboardRecoveryPresentation';
import type { DashboardRecoverySummary } from '@/hooks/useDashboardRecovery';

interface RecoveryStatusPanelProps {
  recovery: DashboardRecoverySummary;
}

export function RecoveryStatusPanel(props: RecoveryStatusPanelProps) {
  const latestAgeMs = createMemo(() => {
    const ts = props.recovery.latestEventTimestamp;
    if (typeof ts !== 'number' || !Number.isFinite(ts)) return null;
    return Date.now() - ts;
  });

  const isStale = createMemo(() => (latestAgeMs() ?? 0) > 24 * 60 * 60_000);

  return (
    <Card padding="none" class="px-4 py-3.5">
      <div class="flex items-center justify-between gap-3">
        <h2 class="text-sm font-semibold text-base-content">Recovery Status</h2>
        <a
          href={buildRecoveryPath()}
          aria-label="View all recovery"
          class="text-xs font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
        >
          View all →
        </a>
      </div>
      <Show
        when={props.recovery.hasData}
        fallback={<p class="text-xs text-muted mt-1">{DASHBOARD_RECOVERY_EMPTY_STATE}</p>}
      >
        <div class="space-y-1.5">
          <div class="flex items-baseline justify-between gap-4">
            <p class="text-xs text-muted">
              <span class="font-mono font-semibold text-base text-base-content">
                {props.recovery.totalProtected}
              </span>{' '}
              total
            </p>
            <p class="text-xs text-muted">
              Last:{' '}
              <span class="font-mono font-medium text-base-content">
                {formatRelativeTime(props.recovery.latestEventTimestamp ?? undefined, {
                  compact: true,
                }) || '—'}
              </span>
            </p>
          </div>

          <Show when={isStale()}>
            <p class="text-sm font-medium text-amber-700 dark:text-amber-300">
              {DASHBOARD_RECOVERY_STALE_MESSAGE}
            </p>
          </Show>

          <div class="flex flex-wrap gap-2">
            <For each={RECOVERY_OUTCOMES}>
              {(outcome) => {
                const count = () => props.recovery.byOutcome[outcome] ?? 0;
                return (
                  <Show when={count() > 0}>
                    <span class={getRecoveryOutcomeBadgeClass(outcome)}>
                      <span class="font-mono">{count()}</span>
                      <span class="ml-1">{outcome}</span>
                    </span>
                  </Show>
                );
              }}
            </For>
          </div>
        </div>
      </Show>
    </Card>
  );
}

export default RecoveryStatusPanel;
