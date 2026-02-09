import { For, Show, createMemo } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { buildBackupsPath } from '@/routing/resourceLinks';
import { formatRelativeTime } from '@/utils/format';
import type { DashboardBackupSummary } from '@/hooks/useDashboardBackups';
import type { BackupOutcome } from '@/features/storageBackupsV2/models';

interface BackupStatusPanelProps {
  backups: DashboardBackupSummary;
}

const OUTCOMES: BackupOutcome[] = ['success', 'warning', 'failed', 'running', 'offline', 'unknown'];

const outcomeBadgeClass = (outcome: BackupOutcome): string => {
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
    case 'offline':
      return `${base} bg-gray-100 text-gray-700 dark:bg-gray-700/60 dark:text-gray-200`;
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
    <Card>
      <div class="flex items-center justify-between gap-3">
        <h2 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-gray-100">Backup Status</h2>
        <a
          href={buildBackupsPath()}
          aria-label="View all backups"
          class="text-sm font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
        >
          View all →
        </a>
      </div>
      <div class="mt-2 mb-3 border-b border-gray-100 dark:border-gray-700/50" />

      <Show
        when={props.backups.hasData}
        fallback={<p class="text-sm text-gray-500 dark:text-gray-400">No backup data available</p>}
      >
        <div class="space-y-3">
          <div class="flex items-end justify-between gap-4">
            <div>
              <p class="text-xs sm:text-sm font-medium text-gray-600 dark:text-gray-400">Total backups</p>
              <p class="mt-1 text-3xl font-mono font-semibold text-gray-900 dark:text-gray-100">
                {props.backups.totalBackups}
              </p>
            </div>
            <div class="text-right">
              <p class="text-xs sm:text-sm font-medium text-gray-600 dark:text-gray-400">Last backup</p>
              <p class="mt-1 text-sm font-mono font-semibold text-gray-700 dark:text-gray-200">
                {formatRelativeTime(props.backups.latestBackupTimestamp ?? undefined, { compact: true }) || '—'}
              </p>
            </div>
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

