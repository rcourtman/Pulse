import { For, Show, createMemo } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { buildBackupsPath } from '@/routing/resourceLinks';
import { formatRelativeTime } from '@/utils/format';
import type { DashboardBackupSummary } from '@/hooks/useDashboardBackups';

interface BackupStatusPanelProps {
  backups: DashboardBackupSummary;
}

type ProtectionOutcome = 'success' | 'warning' | 'failed' | 'running' | 'unknown';
const OUTCOMES: ProtectionOutcome[] = ['success', 'warning', 'failed', 'running', 'unknown'];

const outcomeBadgeClass = (outcome: ProtectionOutcome): string => {
  const base = 'inline-flex items-center rounded-full px-2 py-1 text-xs font-medium';
  switch (outcome) {
    case 'success':
      return `${base} bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300`;
    case 'warning':
      return `${base} bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300`;
    case 'failed':
      return `${base} bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300`;
    case 'running':
      return `${base} bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300`;
    default:
      return `${base} bg-gray-100 text-gray-600 dark:bg-gray-700/60 dark:text-gray-300`;
  }
};

export function BackupStatusPanel(props: BackupStatusPanelProps) {
  const latestAgeMs = createMemo(() => {
    const ts = props.backups.latestBackupTimestamp;
    if (typeof ts !== 'number' || !Number.isFinite(ts)) return null;
    return Date.now() - ts;
  });

  const isStale = createMemo(() => (latestAgeMs() ?? 0) > 24 * 60 * 60_000);

  return (
    <Card padding="none" class="px-4 py-3.5">
      <div class="flex items-center justify-between gap-3">
        <h2 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Backup Status</h2>
        <a
          href={buildBackupsPath()}
          aria-label="View all backups"
          class="text-xs font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
        >
          View all →
        </a>
      </div>
      <Show
        when={props.backups.hasData}
        fallback={<p class="text-xs text-gray-500 dark:text-gray-400 mt-1">No backup data available</p>}
      >
        <div class="space-y-1.5">
          <div class="flex items-baseline justify-between gap-4">
            <p class="text-xs text-gray-500 dark:text-gray-400">
              <span class="font-mono font-semibold text-base text-gray-900 dark:text-gray-100">{props.backups.totalBackups}</span> total
            </p>
            <p class="text-xs text-gray-500 dark:text-gray-400">
              Last: <span class="font-mono font-medium text-gray-700 dark:text-gray-200">{formatRelativeTime(props.backups.latestBackupTimestamp ?? undefined, { compact: true }) || '—'}</span>
            </p>
          </div>

          <Show when={isStale()}>
            <p class="text-sm font-medium text-amber-700 dark:text-amber-300">Last backup over 24 hours ago</p>
          </Show>

          <div class="flex flex-wrap gap-2">
            <For each={OUTCOMES}>
              {(outcome) => {
                const count = () => props.backups.byOutcome[outcome] ?? 0;
                return (
                  <Show when={count() > 0}>
                    <span class={outcomeBadgeClass(outcome)}>
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

export default BackupStatusPanel;
